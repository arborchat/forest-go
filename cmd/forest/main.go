package main

import (
	"encoding"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"

	forest "git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/fields"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/packet"
)

const (
	usageError = 1

	commandIdentity  = "identity"
	commandCommunity = "community"
	commandReply     = "reply"

	commandShow   = "show"
	commandCreate = "create"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, `forest

A CLI for manipulating nodes in the arbor forest.

Subcommands:

`+commandCreate+" ("+commandIdentity+"|"+commandCommunity+"|"+commandReply+`)
show <node-id>
`)
		flag.PrintDefaults()
		os.Exit(usageError)
	}
	if len(os.Args) < 2 {
		flag.Usage()
	}

	var cmdHandler handler
	switch os.Args[1] {
	case commandCreate:
		cmdHandler = create
	case commandShow:
		cmdHandler = show
	default:
		flag.Usage()
	}
	if err := cmdHandler(os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

type handler func(args []string) error

func show(args []string) error {
	flags := flag.NewFlagSet(commandShow, flag.ExitOnError)
	usage := func() {
		flags.PrintDefaults()
		os.Exit(usageError)
	}
	err := flags.Parse(args)
	if err != nil {
		return err
	}
	if len(flags.Args()) < 1 {
		usage()
	}
	return showNode(args, commandShow, forest.UnmarshalBinaryNode)
}

func create(args []string) error {
	flags := flag.NewFlagSet(commandCreate, flag.ExitOnError)
	usage := func() {
		flags.PrintDefaults()
		os.Exit(usageError)
	}
	err := flags.Parse(args)
	if err != nil {
		return err
	}
	if len(flags.Args()) < 1 {
		usage()
	}
	var cmdHandler handler
	switch flags.Arg(0) {
	case commandIdentity:
		cmdHandler = createIdentity
	case commandCommunity:
		cmdHandler = createCommunity
	case commandReply:
		cmdHandler = createReply
	default:
		usage()
	}
	if err := cmdHandler(flags.Args()[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return nil
}

func createIdentity(args []string) error {
	var (
		name, metadata, keyfile, gpguser string
	)
	flags := flag.NewFlagSet(commandCreate+" "+commandIdentity, flag.ExitOnError)
	flags.StringVar(&name, "name", "forest", "username for the identity node")
	flags.StringVar(&metadata, "metadata", "\"forest\"", "metadata for the identity node")
	flags.StringVar(&keyfile, "key", "arbor.privkey", "the openpgp private key for the identity node")
	flags.StringVar(&gpguser, "gpguser", "", "gpg2 user whose private key should be used to create this node. Supercedes -key.")
	usage := func() {
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		usage()
		return err
	}
	newline = regexp.MustCompile(`(\n)`)
	if newline.MatchString(name) {
		return errors.new("Newline in username")
	}
	signer, err := getSigner(gpguser, keyfile)
	if err != nil {
		return err
	}
	identity, err := forest.NewIdentity(signer, name, metadata)
	if err != nil {
		return err
	}

	fname, err := identity.ID().MarshalString()
	if err != nil {
		return err
	}

	if err := saveAs(fname, identity); err != nil {
		return err
	}

	fmt.Println(fname)

	return nil
}

func createCommunity(args []string) error {
	var (
		name, metadata, keyfile, identity, gpguser string
	)
	flags := flag.NewFlagSet(commandCreate+" "+commandCommunity, flag.ExitOnError)
	flags.StringVar(&name, "name", "forest", "username for the community node")
	flags.StringVar(&metadata, "metadata", "\"forest\"", "metadata for the community node")
	flags.StringVar(&keyfile, "key", "arbor.privkey", "the openpgp private key for the signing identity node")
	flags.StringVar(&identity, "as", "", "[required] the id of the signing identity node")
	flags.StringVar(&gpguser, "gpguser", "", "gpg2 user whose private key should be used to create this node. Supercedes -key.")
	usage := func() {
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		usage()
		return err
	}
	signer, err := getSigner(gpguser, keyfile)
	if err != nil {
		return err
	}
	idNode, err := getIdentity(identity)
	if err != nil {
		return err
	}

	community, err := forest.As(idNode, signer).NewCommunity(name, metadata)
	if err != nil {
		return err
	}

	fname, err := community.ID().MarshalString()
	if err != nil {
		return err
	}

	if err := saveAs(fname, community); err != nil {
		return err
	}

	fmt.Println(fname)

	return nil
}

func createReply(args []string) error {
	var (
		content, metadata, parent, keyfile, identity, gpguser string
	)
	flags := flag.NewFlagSet(commandCreate+" "+commandReply, flag.ExitOnError)
	flags.StringVar(&metadata, "metadata", "\"forest\"", "metadata for the reply node")
	flags.StringVar(&keyfile, "key", "arbor.privkey", "the openpgp private key for the signing identity node")
	flags.StringVar(&gpguser, "gpguser", "", "gpg2 user whose private key should be used to create this node. Supercedes -key.")
	flags.StringVar(&identity, "as", "", "[required] the id of the signing identity node")
	flags.StringVar(&parent, "to", "", "[required] the id of the parent reply or community node")
	flags.StringVar(&content, "content", "", "[required] content of the reply node")

	usage := func() {
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		usage()
		return err
	}

	signer, err := getSigner(gpguser, keyfile)
	if err != nil {
		return err
	}
	idNode, err := getIdentity(identity)
	if err != nil {
		return err
	}

	parentNode, err := getReplyOrCommunity(parent)
	if err != nil {
		return err
	}

	reply, err := forest.As(idNode, signer).NewReply(parentNode, content, metadata)
	if err != nil {
		return err
	}

	fname, err := reply.ID().MarshalString()
	if err != nil {
		return err
	}

	if err := saveAs(fname, reply); err != nil {
		return err
	}

	fmt.Println(fname)

	return nil
}

func showNode(args []string, commandName string, fromBytes func([]byte) (forest.Node, error)) error {
	flags := flag.NewFlagSet(commandName+" "+commandShow, flag.ExitOnError)
	usage := func() {
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		usage()
		return err
	}
	if len(flags.Args()) < 1 {
		return fmt.Errorf("missing required argument [node id]")
	}
	b, err := ioutil.ReadFile(args[0])
	if err != nil && err != io.EOF {
		return err
	}
	c, err := fromBytes(b)
	if err != nil {
		return err
	}
	if err := c.ValidateShallow(); err != nil {
		return err
	}
	text, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if _, err := os.Stdout.Write(text); err != nil {
		return err
	}
	return nil
}

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
	defer outfile.Close()

	return save(outfile, node)
}

func loadIdentity(r io.Reader) (*forest.Identity, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return forest.UnmarshalIdentity(b)
}

func getIdentity(filename string) (*forest.Identity, error) {
	idFile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer idFile.Close()

	return loadIdentity(idFile)
}

func loadCommunity(r io.Reader) (*forest.Community, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return forest.UnmarshalCommunity(b)
}

func getCommunity(filename string) (*forest.Community, error) {
	idFile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer idFile.Close()

	return loadCommunity(idFile)
}

func loadReplyOrCommunity(r io.Reader) (interface{}, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil && err != io.EOF {
		return nil, err
	}
	nodeType, err := forest.NodeTypeOf(b)
	switch nodeType {
	case fields.NodeTypeReply:
		return forest.UnmarshalReply(b)
	case fields.NodeTypeCommunity:
		return forest.UnmarshalCommunity(b)
	default:
		return nil, fmt.Errorf("Expected node type of reply, got %v", nodeType)
	}
}

func getReplyOrCommunity(filename string) (interface{}, error) {
	idFile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer idFile.Close()
	return loadReplyOrCommunity(idFile)
}

func readKey(in io.Reader) (*openpgp.Entity, error) {
	return openpgp.ReadEntity(packet.NewReader(in))
}

type PGPKeyConfig struct {
	Name    string
	Comment string
	Email   string
}

// getSigner returns a Signer. If the gpguser parameter is not the empty string, it
// uses a GPGSigner with that username. Otherwise, it uses a NativeSigner with the
// given privkeyFile as the source of the private key.
func getSigner(gpguser, privkeyFile string) (forest.Signer, error) {
	var (
		signer forest.Signer
		err    error
	)
	if gpguser != "" {
		signer, err = forest.NewGPGSigner(gpguser)
	} else {
		privkey, err := getPrivateKey(privkeyFile, &PGPKeyConfig{
			Name:    "Arbor identity key",
			Comment: "Automatically generated",
			Email:   "none@arbor.chat",
		})
		if err != nil {
			return nil, err
		}
		return forest.NewNativeSigner(privkey)
	}
	return signer, err
}

// getPrivateKey gets a private key for creating the identity based on the value
// of filename. If filename is:
// "-" => read a private key from stdin, do not write private key to a file
// existing file => read key from file, do not write private key to a file
// nonexistent file => create new private key, write to filename
//
// the value of config is only used when creating a new key
func getPrivateKey(filename string, config *PGPKeyConfig) (*openpgp.Entity, error) {
	var privkey *openpgp.Entity
	var err error
	if filename == "-" {
		// if stdin, try to read key
		return readKey(os.Stdin)

	}
	// check if privkeyfile exists
	keyOutFile, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0400)
	if err != nil {
		// keyfile may exist, use key from it
		keyOutFile, err := os.Open(filename)
		if err != nil {
			// keyfile doesn't exist or we can't open it
			return nil, err
		}
		return readKey(keyOutFile)
	}
	// keyfile did not exist, create new key and write it there
	privkey, err = openpgp.NewEntity(config.Name, config.Comment, config.Email, nil)
	if err != nil {
		return nil, err
	}

	if err := privkey.SerializePrivate(keyOutFile, nil); err != nil {
		return nil, err
	}
	return privkey, nil

}
