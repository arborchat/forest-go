package main

import (
	"encoding"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/gdamore/tcell"
	"github.com/gdamore/tcell/views"
	"github.com/pkg/profile"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/packet"

	forest "git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/fields"
)

func save(w io.Writer, node encoding.BinaryMarshaler) error {
	b, err := node.MarshalBinary()
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func saveAs(name string, node encoding.BinaryMarshaler) error {
	outfile, err := os.Create(name)
	if err != nil {
		return err
	}
	log.Printf("saving to %s", outfile.Name())
	defer outfile.Close()

	return save(outfile, node)
}

func index(element *fields.QualifiedHash, group []*fields.QualifiedHash) int {
	for i, current := range group {
		if element.Equals(current) {
			return i
		}
	}
	return -1
}

func in(element *fields.QualifiedHash, group []*fields.QualifiedHash) bool {
	return index(element, group) >= 0
}

func nth(input string, n int) rune {
	for i, r := range input {
		if i == n {
			return r
		}
	}
	return '?'
}

type NodeList []*forest.Reply

func (h NodeList) Sort() {
	sort.SliceStable(h, func(i, j int) bool {
		return h[i].Created < h[j].Created
	})
}

func (h NodeList) IndexForID(id *fields.QualifiedHash) int {
	for i, n := range h {
		if n.ID().Equals(id) {
			return i
		}
	}
	return -1
}

type Archive struct {
	NodeList
	forest.Store
}

func (a *Archive) Read(r io.ReadCloser) error {
	defer r.Close()
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	node, err := forest.UnmarshalBinaryNode(b)
	if err != nil {
		return err
	}
	if err := a.Store.Add(node); err != nil {
		return err
	}
	if r, ok := node.(*forest.Reply); ok {
		a.NodeList = append(a.NodeList, r)
	}
	return nil

}

func readAllInto(store forest.Store) (*Archive, error) {
	workdir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	dir, err := os.Open(workdir)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	names, err := dir.Readdirnames(0)
	if err != nil {
		return nil, err
	}
	var nodes []*forest.Reply
	archive := &Archive{NodeList: nodes, Store: store}
	for _, name := range names {
		nodeFile, err := os.Open(name)
		if err != nil {
			log.Println(err)
			continue
		}
		err = archive.Read(nodeFile)
		if err != nil {
			log.Printf("Failed parsing %s: %v", nodeFile.Name(), err)
			continue
		}
	}
	return archive, nil
}

func (v *Archive) AncestryOf(id *fields.QualifiedHash) ([]*fields.QualifiedHash, error) {
	node, present, err := v.Store.Get(id)
	if err != nil {
		return nil, err
	} else if !present {
		return []*fields.QualifiedHash{}, nil
	}
	ancestors := make([]*fields.QualifiedHash, 0, node.TreeDepth())
	next := node.ParentID()
	for !next.Equals(fields.NullHash()) {
		parent, present, err := v.Store.Get(next)
		if err != nil {
			return nil, err
		} else if !present {
			return ancestors, nil
		}
		ancestors = append(ancestors, next)
		next = parent.ParentID()
	}
	return ancestors, nil
}

func (v *Archive) DescendantsOf(id *fields.QualifiedHash) ([]*fields.QualifiedHash, error) {
	descendants := make([]*fields.QualifiedHash, 0)
	directChildren := []*fields.QualifiedHash{id}

	for len(directChildren) > 0 {
		target := directChildren[0]
		directChildren = directChildren[1:]
		for _, node := range v.NodeList {
			if node.ParentID().Equals(target) {
				descendants = append(descendants, node.ID())
				directChildren = append(directChildren, node.ID())
			}
		}
	}
	return descendants, nil
}

type RenderedLine struct {
	ID    *fields.QualifiedHash
	Style tcell.Style
	Text  string
}

type Point struct {
	X, Y int
}

type HistoryView struct {
	*Archive
	rendered []RenderedLine
	Cursor   Point
}

var _ views.CellModel = &HistoryView{}

func (v *HistoryView) CurrentID() *fields.QualifiedHash {
	if len(v.rendered) > v.Cursor.Y {
		return v.rendered[v.Cursor.Y].ID
	} else if len(v.Archive.NodeList) > 0 {
		return v.Archive.NodeList[0].ID()
	}
	return fields.NullHash()
}

func (v *HistoryView) CurrentReply() (*forest.Reply, error) {
	node, has, err := v.Get(v.CurrentID())
	if err != nil {
		return nil, err
	} else if !has {
		return nil, err
	} else if reply, ok := node.(*forest.Reply); !ok {
		return nil, fmt.Errorf("Current node is not a reply: %v", node)
	} else {
		return reply, nil
	}

}

func (v *HistoryView) Render() error {
	currentID := v.CurrentID()
	currentIDText, _ := currentID.MarshalString()
	log.Printf("Starting Render() with %s as current", currentIDText)
	v.rendered = []RenderedLine{}
	ancestry, err := v.AncestryOf(currentID)
	if err != nil {
		return err
	}
	log.Printf("len(ancestry) = %d", len(ancestry))
	descendants, err := v.DescendantsOf(currentID)
	if err != nil {
		return err
	}
	log.Printf("len(descendants) = %d", len(descendants))
	for _, n := range v.NodeList {
		config := renderConfig{}
		if n.ID().Equals(currentID) {
			config.state = current
		} else if in(n.ID(), ancestry) {
			config.state = ancestor
		} else if in(n.ID(), descendants) {
			config.state = descendant
		}
		lines, err := renderNode(n, v.Store, config)
		if err != nil {
			return err
		}
		v.rendered = append(v.rendered, lines...)
	}
	return nil
}

func (v *HistoryView) GetCell(x, y int) (rune, tcell.Style, []rune, int) {
	if y < len(v.rendered) && x < len(v.rendered[y].Text) {
		return nth(v.rendered[y].Text, x), v.rendered[y].Style, nil, 1
	}
	return ' ', tcell.StyleDefault, nil, 1
}

func (v *HistoryView) GetBounds() (int, int) {
	width := 0
	for _, line := range v.rendered {
		if len(line.Text) > width {
			width = len(line.Text)
		}
	}
	height := len(v.rendered)
	return height - 1, width - 1
}

func (v *HistoryView) SetCursor(x, y int) {
	v.Cursor.X = x
	v.Cursor.Y = y
	if err := v.Render(); err != nil {
		log.Println("Error rendering after SetCursor():", err)
	}
}

func (v *HistoryView) GetCursor() (int, int, bool, bool) {
	return v.Cursor.X, v.Cursor.Y, false, false
}

func (v *HistoryView) MoveCursor(offx, offy int) {
	if v.Cursor.X+offx >= 0 {
		v.Cursor.X += offx
	}
	if v.Cursor.Y+offy >= 0 {
		v.Cursor.Y += offy
	}
	log.Printf("Moved cursor to (%d,%d)", v.Cursor.X, v.Cursor.Y)
	if err := v.Render(); err != nil {
		log.Printf("Error during post-cursor move render: %v", err)
	}
}

type HistoryWidget struct {
	*HistoryView
	*views.CellView
	*views.Application
	*forest.Builder
}

var _ views.Widget = &HistoryWidget{}

func (v *HistoryWidget) HandleEvent(event tcell.Event) bool {
	if v.CellView.HandleEvent(event) {
		return true
	}
	switch keyEvent := event.(type) {
	case *tcell.EventKey:
		switch keyEvent.Key() {
		case tcell.KeyCtrlC:
			v.Application.Quit()
		case tcell.KeyEnter:
			reply, err := v.CurrentReply()
			if err != nil {
				log.Println(err)
				return false
			}
			msg := strings.Join(strings.Split(string(reply.Content.Blob), "\n"), "\n#")
			file, err := ioutil.TempFile("", "arbor-msg")
			if err != nil {
				log.Println(err)
				return false
			}
			_, err = file.Write([]byte(fmt.Sprintf("# replying to %s\n", msg)))
			if err != nil {
				file.Close()
				log.Println(err)
				return false
			}
			file.Close()
			editor := exec.Command("gnome-terminal", "--wait", "--", os.ExpandEnv("$EDITOR"), file.Name())
			log.Print("starting editor")
			if err := editor.Run(); err != nil {
				log.Println(err)
				return false
			}
			log.Print("editor done")
			replyContent, err := ioutil.ReadFile(file.Name())
			if err != nil {
				log.Println(err)
				return false
			}
			log.Print(string(replyContent))
			reply, err = v.NewReply(reply, stripCommentLines(string(replyContent)), "")
			if err != nil {
				log.Println(err)
				return false
			}
			outfile, err := reply.ID().MarshalString()
			if err != nil {
				log.Println(err)
				return false
			}
			err = saveAs(outfile, reply)
			if err != nil {
				log.Println(err)
				return false
			}

		case tcell.KeyRune:
			// break if it's a normal keypress
		default:
			return false
		}
		switch keyEvent.Rune() {
		case 'j':
			v.MoveCursor(0, 1)
			return true
		case 'k':
			v.MoveCursor(0, -1)
			return true
		}
	}
	return false
}

func main() {
	defer profile.Start().Stop()
	var (
		signer forest.Signer
		err    error
	)
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	logFile, err := os.OpenFile("viewer.log", os.O_APPEND|os.O_RDWR|os.O_CREATE, 0660)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(logFile)
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	user := flag.String("gpguser", "", "gpg user to sign new messages with")
	key := flag.String("key", "arbor.privkey", "PGP key to sign messages with")
	identityName := flag.String("identity", "", "arbor identity node to sign with")
	flag.Parse()
	if *user != "" {
		signer, err = forest.NewGPGSigner(*user)
	} else if *key != "" {
		keyfile, _ := os.Open(*key)
		defer keyfile.Close()
		entity, _ := openpgp.ReadEntity(packet.NewReader(keyfile))
		signer, err = forest.NewNativeSigner(entity)
	}
	if err != nil {
		log.Fatal(err)
	}
	idBytes, err := ioutil.ReadFile(*identityName)
	if err != nil {
		log.Fatal(err)
	}
	identity, err := forest.UnmarshalIdentity(idBytes)
	if err != nil {
		log.Fatal(err)
	}
	store := forest.NewMemoryStore()
	history, err := readAllInto(store)
	if err != nil {
		log.Fatal(err)
	}
	history.Sort()
	historyView := &HistoryView{
		Archive: history,
	}
	if err := historyView.Render(); err != nil {
		log.Fatal(err)
	}
	cv := views.NewCellView()
	cv.SetModel(historyView)
	cv.MakeCursorVisible()
	app := new(views.Application)
	hw := &HistoryWidget{
		historyView,
		cv,
		app,
		forest.As(identity, signer),
	}
	app.SetRootWidget(hw)

	if _, err := Watch(cwd, func(filename string) {
		log.Println("Found new file", filename)
		app.PostFunc(func() {
			file, err := os.Open(filename)
			if err != nil {
				log.Println(err)
			}
			defer file.Close()
			err = historyView.Read(file)
			if err != nil {
				log.Println(err)
			}
			historyView.Sort()
			err = historyView.Render()
			if err != nil {
				log.Println(err)
			}
			app.Update()
		})
	}); err != nil {
		log.Fatal(err)
	} else {
		//		defer watcher.Close()
	}

	if e := app.Run(); e != nil {
		log.Println(e.Error())
		os.Exit(1)
	}
}

func Watch(dir string, handler func(filename string)) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Create != 0 {
					log.Println("Got create event for", event.Name)
					handler(event.Name)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					log.Println("Got watch error", err)
					return
				}
			}
		}
	}()
	err = watcher.Add(dir)
	if err != nil {
		return nil, err
	}
	log.Println("Watching", dir)
	return watcher, nil
}

type nodeState uint

const (
	none nodeState = iota
	ancestor
	descendant
	current
)

type renderConfig struct {
	state nodeState
}

func renderNode(node forest.Node, store forest.Store, config renderConfig) ([]RenderedLine, error) {
	var (
		ancestorColor   = tcell.StyleDefault.Foreground(tcell.ColorYellow)
		descendantColor = tcell.StyleDefault.Foreground(tcell.ColorGreen)
		currentColor    = tcell.StyleDefault.Foreground(tcell.ColorRed)
	)
	idstring, _ := node.ID().MarshalString()
	log.Printf("%s => %d", idstring, config.state)
	var out []RenderedLine
	var style tcell.Style
	switch n := node.(type) {
	case *forest.Reply:
		author, present, err := store.Get(&n.Author)
		if err != nil {
			return nil, err
		} else if !present {
			return nil, fmt.Errorf("Node %v is not in the store", n.Author)
		}
		asIdent := author.(*forest.Identity)
		switch config.state {
		case ancestor:
			style = ancestorColor
		case descendant:
			style = descendantColor
		case current:
			style = currentColor
		default:
			style = tcell.StyleDefault
		}
		rendered := fmt.Sprintf("%s: %s", string(asIdent.Name.Blob), string(n.Content.Blob))
		// drop all trailing newline characters
		for rendered[len(rendered)-1] == "\n"[0] {
			rendered = rendered[:len(rendered)-1]
		}
		for _, line := range strings.Split(rendered, "\n") {
			out = append(out, RenderedLine{
				ID:    n.ID(),
				Style: style,
				Text:  line,
			})
		}
	}
	return out, nil
}

func stripCommentLines(input string) string {
	lines := strings.Split(input, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if !strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}
