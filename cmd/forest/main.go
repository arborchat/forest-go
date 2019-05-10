package main

import (
	"encoding"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	forest "git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/fields"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/packet"
)

const (
	usageError = 1

	commandIdentity     = "identity"
	commandCommunity    = "community"
	commandConversation = "conversation"
	commandReply        = "reply"
	commandShow         = "show"

	subcommandCreate = "create"
	subcommandShow   = "show"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, `forest

A CLI for manipulating nodes in the arbor forest.

Subcommands:

`+commandIdentity+"\n"+commandCommunity)
		flag.PrintDefaults()
		os.Exit(usageError)
	}
	if len(os.Args) < 2 {
		flag.Usage()
	}

	var cmdHandler handler
	switch os.Args[1] {
	case commandIdentity:
		cmdHandler = identity
	case commandCommunity:
		cmdHandler = community
	case commandConversation:
		cmdHandler = conversation
	case commandReply:
		cmdHandler = reply
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
	return showNode(args, commandShow, func(b []byte) (interface{}, error) {
		return forest.UnmarshalBinaryNode(b)
	})
}

func identity(args []string) error {
	flags := flag.NewFlagSet(commandIdentity, flag.ExitOnError)
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
	case subcommandCreate:
		cmdHandler = createIdentity
	case subcommandShow:
		cmdHandler = showIdentity
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
		name, metadata, keyfile string
	)
	flags := flag.NewFlagSet(commandIdentity+" "+subcommandCreate, flag.ExitOnError)
	flags.StringVar(&name, "name", "forest", "username for the identity node")
	flags.StringVar(&metadata, "metadata", "forest", "metadata for the identity node")
	flags.StringVar(&keyfile, "key", "arbor.privkey", "the openpgp private key for the identity node")
	usage := func() {
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		usage()
		return err
	}
	qName, err := fields.NewQualifiedContent(fields.ContentTypeUTF8String, []byte(name))
	if err != nil {
		return err
	}
	qMeta, err := fields.NewQualifiedContent(fields.ContentTypeUTF8String, []byte(metadata))
	if err != nil {
		return err
	}
	privkey, err := getPrivateKey(keyfile, &PGPKeyConfig{
		Name:    "Arbor identity key",
		Comment: "Automatically generated",
		Email:   "none@arbor.chat",
	})
	if err != nil {
		return err
	}
	identity, err := forest.NewIdentity(privkey, qName, qMeta)
	if err != nil {
		return err
	}

	fname, err := filename(identity.ID())
	if err != nil {
		return err
	}

	if err := saveAs(fname, identity); err != nil {
		return err
	}

	fmt.Println(fname)

	return nil
}

func showIdentity(args []string) error {
	return showNode(args, commandIdentity, func(b []byte) (interface{}, error) {
		node, err := forest.UnmarshalIdentity(b)
		return interface{}(node), err
	})
}

func community(args []string) error {
	flags := flag.NewFlagSet(commandCommunity, flag.ExitOnError)
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
	case subcommandCreate:
		cmdHandler = createCommunity
	case subcommandShow:
		cmdHandler = showCommunity
	default:
		usage()
	}
	if err := cmdHandler(flags.Args()[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return nil
}

func createCommunity(args []string) error {
	var (
		name, metadata, keyfile, identity string
	)
	flags := flag.NewFlagSet(commandCommunity+" "+subcommandCreate, flag.ExitOnError)
	flags.StringVar(&name, "name", "forest", "username for the community node")
	flags.StringVar(&metadata, "metadata", "forest", "metadata for the community node")
	flags.StringVar(&keyfile, "key", "arbor.privkey", "the openpgp private key for the signing identity node")
	flags.StringVar(&identity, "as", "", "[required] the id of the signing identity node")
	usage := func() {
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		usage()
		return err
	}
	qName, err := fields.NewQualifiedContent(fields.ContentTypeUTF8String, []byte(name))
	if err != nil {
		return err
	}
	qMeta, err := fields.NewQualifiedContent(fields.ContentTypeUTF8String, []byte(metadata))
	if err != nil {
		return err
	}
	privkey, err := getPrivateKey(keyfile, &PGPKeyConfig{
		Name:    "Arbor identity key",
		Comment: "Automatically generated",
		Email:   "none@arbor.chat",
	})
	if err != nil {
		return err
	}

	idNode, err := getIdentity(identity)
	if err != nil {
		return err
	}

	community, err := forest.As(idNode, privkey).NewCommunity(qName, qMeta)
	if err != nil {
		return err
	}

	fname, err := filename(community.ID())
	if err != nil {
		return err
	}

	if err := saveAs(fname, community); err != nil {
		return err
	}

	fmt.Println(fname)

	return nil
}

func showCommunity(args []string) error {
	return showNode(args, commandCommunity, func(b []byte) (interface{}, error) {
		node, err := forest.UnmarshalCommunity(b)
		return interface{}(node), err
	})
}

func conversation(args []string) error {
	flags := flag.NewFlagSet(commandConversation, flag.ExitOnError)
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
	case subcommandCreate:
		cmdHandler = createConversation
	case subcommandShow:
		cmdHandler = showConversation
	default:
		usage()
	}
	if err := cmdHandler(flags.Args()[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return nil
}

func createConversation(args []string) error {
	var (
		content, metadata, community, keyfile, identity string
	)
	flags := flag.NewFlagSet(commandConversation+" "+subcommandCreate, flag.ExitOnError)
	flags.StringVar(&metadata, "metadata", "forest", "metadata for the conversation node")
	flags.StringVar(&keyfile, "key", "arbor.privkey", "the openpgp private key for the signing identity node")
	flags.StringVar(&identity, "as", "", "[required] the id of the signing identity node")
	flags.StringVar(&community, "in", "", "[required] the id of the parent community node")
	flags.StringVar(&content, "content", "", "[required] content of the conversation node")

	usage := func() {
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		usage()
		return err
	}

	qContent, err := fields.NewQualifiedContent(fields.ContentTypeUTF8String, []byte(content))
	if err != nil {
		return err
	}

	qMeta, err := fields.NewQualifiedContent(fields.ContentTypeUTF8String, []byte(metadata))
	if err != nil {
		return err
	}

	privkey, err := getPrivateKey(keyfile, &PGPKeyConfig{
		Name:    "Arbor identity key",
		Comment: "Automatically generated",
		Email:   "none@arbor.chat",
	})
	if err != nil {
		return err
	}

	idNode, err := getIdentity(identity)
	if err != nil {
		return err
	}

	communityNode, err := getCommunity(community)
	if err != nil {
		return err
	}

	conversation, err := forest.As(idNode, privkey).NewConversation(communityNode, qContent, qMeta)
	if err != nil {
		return err
	}

	fname, err := filename(conversation.ID())
	if err != nil {
		return err
	}

	if err := saveAs(fname, conversation); err != nil {
		return err
	}

	fmt.Println(fname)

	return nil
}

func showConversation(args []string) error {
	return showNode(args, commandConversation, func(b []byte) (interface{}, error) {
		node, err := forest.UnmarshalConversation(b)
		return interface{}(node), err
	})
}

func showNode(args []string, commandName string, fromBytes func([]byte) (interface{}, error)) error {
	flags := flag.NewFlagSet(commandName+" "+subcommandShow, flag.ExitOnError)
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
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
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

func reply(args []string) error {
	flags := flag.NewFlagSet(commandReply, flag.ExitOnError)
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
	case subcommandCreate:
		cmdHandler = createReply
	case subcommandShow:
		cmdHandler = showReply
	default:
		usage()
	}
	if err := cmdHandler(flags.Args()[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return nil
}

func createReply(args []string) error {
	var (
		content, metadata, parent, keyfile, identity string
	)
	flags := flag.NewFlagSet(commandReply+" "+subcommandCreate, flag.ExitOnError)
	flags.StringVar(&metadata, "metadata", "forest", "metadata for the conversation node")
	flags.StringVar(&keyfile, "key", "arbor.privkey", "the openpgp private key for the signing identity node")
	flags.StringVar(&identity, "as", "", "[required] the id of the signing identity node")
	flags.StringVar(&parent, "to", "", "[required] the id of the parent community node")
	flags.StringVar(&content, "content", "", "[required] content of the conversation node")

	usage := func() {
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		usage()
		return err
	}

	qContent, err := fields.NewQualifiedContent(fields.ContentTypeUTF8String, []byte(content))
	if err != nil {
		return err
	}

	qMeta, err := fields.NewQualifiedContent(fields.ContentTypeUTF8String, []byte(metadata))
	if err != nil {
		return err
	}

	privkey, err := getPrivateKey(keyfile, &PGPKeyConfig{
		Name:    "Arbor identity key",
		Comment: "Automatically generated",
		Email:   "none@arbor.chat",
	})
	if err != nil {
		return err
	}

	idNode, err := getIdentity(identity)
	if err != nil {
		return err
	}

	parentNode, err := getReplyOrConversation(parent)
	if err != nil {
		return err
	}

	reply, err := forest.As(idNode, privkey).NewReply(parentNode, qContent, qMeta)
	if err != nil {
		return err
	}

	fname, err := filename(reply.ID())
	if err != nil {
		return err
	}

	if err := saveAs(fname, reply); err != nil {
		return err
	}

	fmt.Println(fname)

	return nil
}

func showReply(args []string) error {
	return showNode(args, commandReply, func(b []byte) (interface{}, error) {
		node, err := forest.UnmarshalReply(b)
		return interface{}(node), err
	})
}

func filename(desc *fields.QualifiedHash) (string, error) {
	b, err := desc.MarshalBinary()
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
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

func loadReplyOrConversation(r io.Reader) (interface{}, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil && err != io.EOF {
		return nil, err
	}
	nodeType, err := forest.NodeTypeOf(b)
	switch nodeType {
	case fields.NodeTypeConversation:
		return forest.UnmarshalConversation(b)
	case fields.NodeTypeReply:
		return forest.UnmarshalReply(b)
	default:
		return nil, fmt.Errorf("Expected node type of reply or conversation, got %v", nodeType)
	}
}

func getReplyOrConversation(filename string) (interface{}, error) {
	idFile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer idFile.Close()
	return loadReplyOrConversation(idFile)
}

func readKey(in io.Reader) (*openpgp.Entity, error) {
	return openpgp.ReadEntity(packet.NewReader(in))
}

type PGPKeyConfig struct {
	Name    string
	Comment string
	Email   string
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
