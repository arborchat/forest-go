package forest

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"time"

	"git.sr.ht/~whereswaldon/forest-go/fields"
	"golang.org/x/crypto/openpgp"
)

// Signer can sign any binary data
type Signer interface {
	Sign(data []byte) (signature []byte, err error)
	PublicKey() (key []byte, err error)
}

// NativeSigner uses golang's native openpgp operation for signing data. It
// only supports private keys without a passphrase.
type NativeSigner openpgp.Entity

// Sign signs the input data with the contained private key and returns the resulting signature.
func (s NativeSigner) Sign(data []byte) ([]byte, error) {
	signedData := bytes.NewBuffer(data)
	signature := new(bytes.Buffer)
	if err := openpgp.DetachSign(signature, (*openpgp.Entity)(&s), signedData, nil); err != nil {
		return nil, err
	}
	return signature.Bytes(), nil
}

// NewNativeSigner creates a native Golang PGP signer. This will fail if the provided key is
// encrypted. GPGSigner should be used for all encrypted keys.
func NewNativeSigner(privatekey *openpgp.Entity) (Signer, error) {
	if privatekey.PrivateKey.Encrypted {
		return nil, fmt.Errorf("Cannot build NativeSigner with an encrypted key")
	}
	return NativeSigner(*privatekey), nil
}

// PublicKey returns the raw bytes of the binary openpgp public key used by this signer.
func (s NativeSigner) PublicKey() ([]byte, error) {
	keybuf := new(bytes.Buffer)
	if err := (*openpgp.Entity)(&s).Serialize(keybuf); err != nil {
		return nil, err
	}
	return keybuf.Bytes(), nil
}

// FindGPG returns the path to the local gpg executable if one can be found. Otherwise it
// returns an error.
func FindGPG() (path string, err error) {
	candidates := []string{"gpg2", "gpg1", "gpg"}
	for _, executable := range candidates {
		if path, err := exec.LookPath(executable); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("None of %v are installed", candidates)
}

// GPGSigner uses a local gpg2 installation for key management. It will invoke gpg2 as a subprocess
// to sign data and to acquire the public key for its signing key. The public fields can be used
// to modify its behavior in order to change how it prompts for passphrases and other details.
type GPGSigner struct {
	gpgExecutable string
	GPGUserName   string
	// Rewriter is invoked on each invocation of exec.Command that spawns GPG. You can use it to modify
	// flags or any other property of the subcommand (environment variables). This is especially useful
	// to control how GPG prompts for key passphrases.
	Rewriter func(*exec.Cmd) error
}

// NewGPGSigner wraps the private key so that it can sign using the local system's implementation of GPG.
func NewGPGSigner(gpgUserName string) (*GPGSigner, error) {
	var err error
	g := &GPGSigner{GPGUserName: gpgUserName, Rewriter: func(_ *exec.Cmd) error { return nil }}
	g.gpgExecutable, err = FindGPG()
	if err != nil {
		return nil, fmt.Errorf("missing both gpg and gpg2, unable to create gpg signer")
	}

	return g, nil
}

// Sign invokes gpg2 to sign the data as this Signer's configured PGP user. It returns the signature or
// an error (if any).
func (s *GPGSigner) Sign(data []byte) ([]byte, error) {
	gpg2 := exec.Command(s.gpgExecutable, "--local-user", s.GPGUserName, "--detach-sign")
	if err := s.Rewriter(gpg2); err != nil {
		return nil, fmt.Errorf("Error invoking Rewrite: %v", err)
	}
	in, err := gpg2.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("Error getting stdin pipe: %v", err)
	}
	out, err := gpg2.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("Error getting stdout pipe: %v", err)
	}
	if _, err := in.Write(data); err != nil {
		return nil, fmt.Errorf("Error writing data to stdin: %v", err)
	}
	if err := gpg2.Start(); err != nil {
		return nil, fmt.Errorf("Error starting gpg command: %v", err)
	}
	if err := in.Close(); err != nil {
		return nil, fmt.Errorf("Error closing stdin: %v", err)
	}
	signature, err := ioutil.ReadAll(out)
	if err != nil {
		return nil, fmt.Errorf("Error reading signature data: %v", err)
	}
	if err := gpg2.Wait(); err != nil {
		return nil, fmt.Errorf("Error running gpg: %v", err)
	}
	return signature, nil
}

// PublicKey returns the bytes of the OpenPGP public key used by this signer.
func (s GPGSigner) PublicKey() ([]byte, error) {
	gpg2 := exec.Command(s.gpgExecutable, "--export", s.GPGUserName)
	if err := s.Rewriter(gpg2); err != nil {
		return nil, fmt.Errorf("Error invoking Rewrite: %v", err)
	}
	out, err := gpg2.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("Error getting stdout pipe: %v", err)
	}
	if err := gpg2.Start(); err != nil {
		return nil, fmt.Errorf("Error starting gpg command: %v", err)
	}
	pubkey, err := ioutil.ReadAll(out)
	if err != nil {
		return nil, fmt.Errorf("Error reading pubkey data: %v", err)
	}
	if err := gpg2.Wait(); err != nil {
		return nil, fmt.Errorf("Error running gpg: %v", err)
	}
	return pubkey, nil
}

// NewIdentity builds an Identity node for the user with the given name and metadata, using
// the OpenPGP Entity privkey to define the Identity. That Entity must contain a
// private key with no passphrase.
func NewIdentity(signer Signer, name string, metadata []byte) (*Identity, error) {
	qname, err := fields.NewQualifiedContent(fields.ContentTypeUTF8String, []byte(name))
	if err != nil {
		return nil, fmt.Errorf("Failed to create qualified content of type %d from %s", fields.ContentTypeUTF8String, name)
	}
	qmeta, err := fields.NewQualifiedContent(fields.ContentTypeTwig, metadata)
	if err != nil {
		return nil, fmt.Errorf("Failed to create qualified content of type %d from %s", fields.ContentTypeTwig, metadata)
	}
	return NewIdentityQualified(signer, qname, qmeta)
}

func NewIdentityQualified(signer Signer, name *fields.QualifiedContent, metadata *fields.QualifiedContent) (*Identity, error) {
	// make an empty identity and populate all fields that need to be known before
	// signing the data
	identity := newIdentity()
	identity.Version = fields.CurrentVersion
	identity.Type = fields.NodeTypeIdentity
	identity.Parent = *fields.NullHash()
	identity.Depth = 0
	identity.Name = *name
	identity.Metadata = *metadata
	identity.Created = fields.TimestampFrom(time.Now())

	// Check no newline in name
	if name.ContainsString("\n") {
		return nil, fmt.Errorf("Newline in username is illegal")
	}

	// get public key
	pubkey, err := signer.PublicKey()
	if err != nil {
		return nil, err
	}
	qKey, err := fields.NewQualifiedKey(fields.KeyTypeOpenPGP, pubkey)
	if err != nil {
		return nil, err
	}
	identity.PublicKey = *qKey
	identity.Author = *fields.NullHash()
	idDesc, err := fields.NewHashDescriptor(fields.HashTypeSHA512, int(fields.HashDigestLengthSHA512_256))
	if err != nil {
		return nil, err
	}
	identity.IDDesc = *idDesc

	// we've defined all pre-signature fields, it's time to sign the data
	signedDataBytes, err := identity.MarshalSignedData()
	if err != nil {
		return nil, err
	}
	signature, err := signer.Sign(signedDataBytes)
	if err != nil {
		return nil, err
	}

	qs, err := fields.NewQualifiedSignature(fields.SignatureTypeOpenPGP, signature)
	if err != nil {
		return nil, err
	}
	identity.Trailer.Signature = *qs

	// determine the node's final hash ID
	id, err := computeID(identity)
	if err != nil {
		return nil, err
	}
	identity.id = fields.Blob(id)

	return identity, nil
}

// Builder creates nodes in the forest on behalf of the given user.
type Builder struct {
	User *Identity
	Signer
}

// As creates a Builder that can write new nodes on behalf of the provided user.
// It is intended to be able to be used fluently, like:
//
// community, err := forest.As(user, privkey).NewCommunity(name, metatdata)
func As(user *Identity, signer Signer) *Builder {
	return &Builder{
		User:   user,
		Signer: signer,
	}
}

// NewCommunity creates a community node (signed by the given identity with the given privkey).
func (n *Builder) NewCommunity(name string, metadata []byte) (*Community, error) {
	qname, err := fields.NewQualifiedContent(fields.ContentTypeUTF8String, []byte(name))
	if err != nil {
		return nil, fmt.Errorf("Failed to create qualified content of type %d from %s", fields.ContentTypeUTF8String, name)
	}
	qmeta, err := fields.NewQualifiedContent(fields.ContentTypeTwig, metadata)
	if err != nil {
		return nil, fmt.Errorf("Failed to create qualified content of type %d from %s", fields.ContentTypeTwig, metadata)
	}
	return n.NewCommunityQualified(qname, qmeta)
}

func (n *Builder) NewCommunityQualified(name *fields.QualifiedContent, metadata *fields.QualifiedContent) (*Community, error) {
	c := newCommunity()
	c.Version = fields.CurrentVersion
	c.Type = fields.NodeTypeCommunity
	c.Parent = *fields.NullHash()
	c.Depth = 0
	c.Name = *name
	c.Metadata = *metadata
	c.Author = *n.User.ID()
	c.Created = fields.TimestampFrom(time.Now())
	idDesc, err := fields.NewHashDescriptor(fields.HashTypeSHA512, int(fields.HashDigestLengthSHA512_256))
	if err != nil {
		return nil, err
	}
	c.IDDesc = *idDesc

	// Check no newline in name
	if name.ContainsString("\n") {
		return nil, fmt.Errorf("Newline in community name is illegal")
	}

	// we've defined all pre-signature fields, it's time to sign the data
	signedDataBytes, err := c.MarshalSignedData()
	if err != nil {
		return nil, err
	}
	signature, err := n.Sign(signedDataBytes)
	if err != nil {
		return nil, err
	}
	qs, err := fields.NewQualifiedSignature(fields.SignatureTypeOpenPGP, signature)
	if err != nil {
		return nil, err
	}
	c.Trailer.Signature = *qs

	// determine the node's final hash ID
	id, err := computeID(c)
	if err != nil {
		return nil, err
	}
	c.id = fields.Blob(id)

	return c, nil
}

// NewReply creates a reply node as a child of the given community or reply
func (n *Builder) NewReply(parent interface{}, content string, metadata []byte) (*Reply, error) {
	qcontent, err := fields.NewQualifiedContent(fields.ContentTypeUTF8String, []byte(content))
	if err != nil {
		return nil, fmt.Errorf("Failed to create qualified content of type %d from %s", fields.ContentTypeUTF8String, content)
	}
	qmeta, err := fields.NewQualifiedContent(fields.ContentTypeTwig, metadata)
	if err != nil {
		return nil, fmt.Errorf("Failed to create qualified content of type %d from %s", fields.ContentTypeTwig, metadata)
	}
	return n.NewReplyQualified(parent, qcontent, qmeta)
}

func (n *Builder) NewReplyQualified(parent interface{}, content, metadata *fields.QualifiedContent) (*Reply, error) {
	r := newReply()
	r.Version = fields.CurrentVersion
	r.Type = fields.NodeTypeReply
	r.Created = fields.TimestampFrom(time.Now())
	switch concreteParent := parent.(type) {
	case *Community:
		r.CommunityID = *concreteParent.ID()
		r.ConversationID = *fields.NullHash()
		r.Parent = *concreteParent.ID()
		r.Depth = concreteParent.Depth + 1
	case *Reply:
		r.CommunityID = concreteParent.CommunityID
		// if parent is root of a conversation
		if concreteParent.Depth == 1 && concreteParent.ConversationID.Equals(fields.NullHash()) {
			r.ConversationID = *concreteParent.ID()
		} else {
			r.ConversationID = concreteParent.ConversationID
		}
		r.Parent = *concreteParent.ID()
		r.Depth = concreteParent.Depth + 1
	default:
		return nil, fmt.Errorf("parent must be either a community or reply node")

	}
	r.Content = *content
	r.Metadata = *metadata
	r.Author = *n.User.ID()
	idDesc, err := fields.NewHashDescriptor(fields.HashTypeSHA512, int(fields.HashDigestLengthSHA512_256))
	if err != nil {
		return nil, err
	}
	r.IDDesc = *idDesc

	// we've defined all pre-signature fields, it's time to sign the data
	signedDataBytes, err := r.MarshalSignedData()
	if err != nil {
		return nil, err
	}
	signature, err := n.Sign(signedDataBytes)
	if err != nil {
		return nil, err
	}
	qs, err := fields.NewQualifiedSignature(fields.SignatureTypeOpenPGP, signature)
	if err != nil {
		return nil, err
	}
	r.Trailer.Signature = *qs

	// determine the node's final hash ID
	id, err := computeID(r)
	if err != nil {
		return nil, err
	}
	r.id = fields.Blob(id)

	return r, nil
}
