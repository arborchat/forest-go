package forest

import (
	"encoding"
	"fmt"
	"reflect"
	"time"

	"git.sr.ht/~whereswaldon/forest-go/fields"
	"git.sr.ht/~whereswaldon/forest-go/serialize"
	"git.sr.ht/~whereswaldon/forest-go/twig"
)

const MaxNameLength = 256

type Validator interface {
	Validate() error
}

type Node interface {
	AuthorID() *fields.QualifiedHash
	CreatedAt() time.Time
	Equals(interface{}) bool
	ID() *fields.QualifiedHash
	ParentID() *fields.QualifiedHash
	TreeDepth() fields.TreeDepth
	TwigMetadata() (*twig.Data, error)
	ValidateDeep(Store) error
	ValidateShallow() error
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}

// NodeTypeOf returns the NodeType of the provided binary-marshaled node.
// If the provided bytes are not a forest node or the type cannot be determined,
// an error will be returned and the first return value must be ignored.
func NodeTypeOf(b []byte) (fields.NodeType, error) {
	_, t, err := VersionAndNodeTypeOf(b)
	return t, err
}

func VersionAndNodeTypeOf(b []byte) (fields.Version, fields.NodeType, error) {
	var schema SchemaInfo
	_, err := serialize.ArborDeserialize(reflect.ValueOf(&schema), b)
	return schema.Version, schema.Type, err
}

// UnmarshalBinaryNode unmarshals a node of any type. If it does not return an
// error, the concrete type of the first return parameter will be one of the
// node structs declared in this package (e.g. Identity, Community, etc...)
func UnmarshalBinaryNode(b []byte) (Node, error) {
	v, t, err := VersionAndNodeTypeOf(b)
	if err != nil {
		return nil, err
	}
	if v > fields.CurrentVersion {
		return nil, fmt.Errorf("Unable to unmarshal node of version %d, only supports <= %d", v, fields.CurrentVersion)
	}
	switch t {
	case fields.NodeTypeIdentity:
		return UnmarshalIdentity(b)
	case fields.NodeTypeCommunity:
		return UnmarshalCommunity(b)
	case fields.NodeTypeReply:
		return UnmarshalReply(b)
	default:
		return nil, fmt.Errorf("Unable to unmarshal node of type %d, unknown type", t)
	}
}

type SchemaInfo struct {
	Version fields.Version  `arbor:"order=0"`
	Type    fields.NodeType `arbor:"order=1"`
}

// generic node
type CommonNode struct {
	// the ID is deterministically computed from the rest of the values
	id         fields.Blob
	SchemaInfo `arbor:"order=0,recurse=always"`
	Parent     fields.QualifiedHash    `arbor:"order=1,recurse=serialize"`
	IDDesc     fields.HashDescriptor   `arbor:"order=2,recurse=always"`
	Depth      fields.TreeDepth        `arbor:"order=3"`
	Created    fields.Timestamp        `arbor:"order=4"`
	Metadata   fields.QualifiedContent `arbor:"order=5,recurse=serialize"`
	Author     fields.QualifiedHash    `arbor:"order=6,recurse=serialize"`
}

// Compute and return the CommonNode's ID as a fields.Qualified Hash
func (n CommonNode) ID() *fields.QualifiedHash {
	return &fields.QualifiedHash{
		Descriptor: n.IDDesc,
		Blob:       n.id,
	}
}

func (n CommonNode) CreatedAt() time.Time {
	return n.Created.Time()
}

func (n CommonNode) ParentID() *fields.QualifiedHash {
	return &fields.QualifiedHash{n.Parent.Descriptor, n.Parent.Blob}
}

func (n CommonNode) TreeDepth() fields.TreeDepth {
	return n.Depth
}

// SignatureIdentityHash returns the node identitifer for the Identity that signed this node.
func (n *CommonNode) SignatureIdentityHash() *fields.QualifiedHash {
	return &n.Author
}

// AuthorID returns the Author of a common node. It wraps SignatureIdentityHash to satisfy
// the Node interface.
func (n *CommonNode) AuthorID() *fields.QualifiedHash {
	return n.SignatureIdentityHash()
}

func (n CommonNode) IsIdentity() bool {
	return n.Type == fields.NodeTypeIdentity
}

func (n CommonNode) HashDescriptor() *fields.HashDescriptor {
	return &n.IDDesc
}

func (n *CommonNode) Equals(n2 *CommonNode) bool {
	if n == n2 {
		return true
	}
	if n == nil || n2 == nil {
		return false
	}
	return n.Type.Equals(&n2.Type) &&
		n.Version.Equals(&n2.Version) &&
		n.Parent.Equals(&n2.Parent) &&
		n.IDDesc.Equals(&n2.IDDesc) &&
		n.Depth.Equals(&n2.Depth) &&
		n.Created.Equals(&n2.Created) &&
		n.Metadata.Equals(&n2.Metadata) &&
		n.Author.Equals(&n2.Author)
}

// ValidateShallow checks all fields for internal validity. It does not check
// the existence or validity of nodes referenced from this node. If the node
// validates, ValidateShallow returns `nil`.
func (n *CommonNode) ValidateShallow() error {
	if _, validType := fields.ValidNodeTypes[n.Type]; !validType {
		return fmt.Errorf("%d is not a valid node type", n.Type)
	}
	if n.Version > fields.CurrentVersion {
		return fmt.Errorf("%d is higher than than the supported version %d", n.Version, fields.CurrentVersion)
	}
	id := n.ID()
	needsValidation := []Validator{id, &n.Parent, &n.Metadata, &n.Author}
	for _, nv := range needsValidation {
		if err := nv.Validate(); err != nil {
			return err
		}
	}
	if n.Metadata.Descriptor.Type != fields.ContentTypeTwig {
		return fmt.Errorf("Metadata must be twig, got content type %d", n.Metadata.Descriptor.Type)
	}
	if _, err := n.TwigMetadata(); err != nil {
		return fmt.Errorf("Twig metadata failed to validate: %v", err)
	}
	return nil
}

// ValidateDeep checks for the existence of all referenced nodes within the provided store.
func (n *CommonNode) ValidateDeep(store Store) error {
	// ensure known parent
	if !n.Parent.Equals(fields.NullHash()) {
		if _, has, err := store.Get(&n.Parent); !has {
			return fmt.Errorf("Unknown parent %v", n.Parent)
		} else if err != nil {
			return err
		}
	}
	// ensure known author
	if !n.Author.Equals(fields.NullHash()) {
		if _, has, err := store.Get(&n.Author); !has {
			return fmt.Errorf("Unknown Author %v", n.Author)
		} else if err != nil {
			return err
		}
	}
	return nil
}

// TwigMetadata returns the metadata of this node parsed into a *twig.Data
func (n *CommonNode) TwigMetadata() (*twig.Data, error) {
	if n.Metadata.Descriptor.Type != fields.ContentTypeTwig {
		return nil, fmt.Errorf("metadata is not twig on this node")
	}
	data := twig.New()
	if err := data.UnmarshalBinary(n.Metadata.Blob); err != nil {
		return nil, fmt.Errorf("failed converting metadata into twig: %w", err)
	}
	return data, nil
}

// Trailer is the final set of fields in every arbor node
type Trailer struct {
	Signature fields.QualifiedSignature `arbor:"order=0,recurse=serialize,signature"`
}

// GetSignature returns the signature for the node, which must correspond to the Signature Authority for
// the node in order to be valid.
func (t *Trailer) GetSignature() *fields.QualifiedSignature {
	return &t.Signature
}

func (t *Trailer) Equals(t2 *Trailer) bool {
	return t.Signature.Equals(&t2.Signature)
}

// concrete nodes

// Identity nodes represent a user. They associate a username with a public key that the user
// will sign messages with.
type Identity struct {
	CommonNode `arbor:"order=0,recurse=always"`
	Name       fields.QualifiedContent `arbor:"order=1,recurse=serialize"`
	PublicKey  fields.QualifiedKey     `arbor:"order=2,recurse=serialize"`
	Trailer    `arbor:"order=3,recurse=always"`
}

func newIdentity() *Identity {
	i := new(Identity)
	// define how to serialize this node type's fields
	return i
}

// MarshalSignedData writes all data that should be signed in the correct order for signing. This
// can be used both to generate and validate message signatures.
func (i *Identity) MarshalSignedData() ([]byte, error) {
	return serialize.ArborSerializeConfig(reflect.ValueOf(i), serialize.SerializationConfig{
		SkipSignatures: true,
	})
}

func (i *Identity) MarshalBinary() ([]byte, error) {
	return serialize.ArborSerialize(reflect.ValueOf(i))
}

func UnmarshalIdentity(b []byte) (*Identity, error) {
	i := &Identity{}
	if err := i.UnmarshalBinary(b); err != nil {
		return nil, err
	}
	return i, nil
}

func (i *Identity) UnmarshalBinary(b []byte) error {
	_, err := serialize.ArborDeserialize(reflect.ValueOf(i), b)
	if err != nil {
		return err
	}
	i.id, err = computeID(i)
	return err
}

func (i *Identity) Equals(other interface{}) bool {
	i2, valid := other.(*Identity)
	if !valid {
		return false
	}
	return i.CommonNode.Equals(&i2.CommonNode) &&
		i.Name.Equals(&i2.Name) &&
		i.PublicKey.Equals(&i2.PublicKey) &&
		i.Trailer.Equals(&i2.Trailer)
}

// ValidateShallow checks all fields for internal validity. It does not check
// the existence or validity of nodes referenced from this node.
func (i *Identity) ValidateShallow() error {
	if err := i.CommonNode.ValidateShallow(); err != nil {
		return err
	}
	needsValidation := []Validator{&i.Name, &i.PublicKey}
	for _, nv := range needsValidation {
		if err := nv.Validate(); err != nil {
			return err
		}
	}
	if i.Name.Descriptor.Length > MaxNameLength {
		return fmt.Errorf("Name is longer than maximum of %d", MaxNameLength)
	}
	if i.Depth != fields.TreeDepth(0) {
		return fmt.Errorf("Identity depth must be 0, got %d", i.Depth)
	}
	if !i.Parent.Equals(fields.NullHash()) {
		return fmt.Errorf("Identity parent must be null hash, got %v", i.Parent)
	}
	if !i.Author.Equals(fields.NullHash()) {
		return fmt.Errorf("Identity author must be null hash, got %v", i.Author)
	}
	return nil
}

// ValidateDeep checks all referenced nodes for existence within the store.
func (i *Identity) ValidateDeep(store Store) error {
	return nil
}

type Community struct {
	CommonNode `arbor:"order=0,recurse=always"`
	Name       fields.QualifiedContent `arbor:"order=1,recurse=serialize"`
	Trailer    `arbor:"order=2,recurse=always"`
}

func newCommunity() *Community {
	c := new(Community)
	// define how to serialize this node type's fields
	return c
}

func (c *Community) MarshalSignedData() ([]byte, error) {
	return serialize.ArborSerializeConfig(reflect.ValueOf(c), serialize.SerializationConfig{
		SkipSignatures: true,
	})
}

func (c *Community) MarshalBinary() ([]byte, error) {
	return serialize.ArborSerialize(reflect.ValueOf(c))
}

func UnmarshalCommunity(b []byte) (*Community, error) {
	c := &Community{}
	if err := c.UnmarshalBinary(b); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Community) UnmarshalBinary(b []byte) error {
	_, err := serialize.ArborDeserialize(reflect.ValueOf(c), b)
	if err != nil {
		return err
	}
	c.id, err = computeID(c)
	return err
}

func (c *Community) Equals(other interface{}) bool {
	c2, valid := other.(*Community)
	if !valid {
		return false
	}
	return c.CommonNode.Equals(&c2.CommonNode) &&
		c.Name.Equals(&c2.Name) &&
		c.Trailer.Equals(&c2.Trailer)
}

// ValidateShallow checks all fields for internal validity. It does not check
// the existence or validity of nodes referenced from this node.
func (c *Community) ValidateShallow() error {
	if err := c.CommonNode.ValidateShallow(); err != nil {
		return err
	}
	needsValidation := []Validator{&c.Name}
	for _, nv := range needsValidation {
		if err := nv.Validate(); err != nil {
			return err
		}
	}
	if c.Name.Descriptor.Length > MaxNameLength {
		return fmt.Errorf("Name is longer than maximum of %d", MaxNameLength)
	}
	if c.Depth != fields.TreeDepth(0) {
		return fmt.Errorf("Community depth must be 0, got %d", c.Depth)
	}
	if !c.Parent.Equals(fields.NullHash()) {
		return fmt.Errorf("Community parent must be null hash, got %v", c.Parent)
	}
	if c.Author.Equals(fields.NullHash()) {
		return fmt.Errorf("Community author must not be null hash")
	}
	return nil
}

// ValidateDeep checks all referenced nodes for existence within the store.
func (c *Community) ValidateDeep(store Store) error {
	if _, has, err := store.Get(&c.Author); !has {
		return fmt.Errorf("Missing author node %v", c.Author)
	} else if err != nil {
		return err
	}
	return nil
}

type Reply struct {
	CommonNode     `arbor:"order=0,recurse=always"`
	CommunityID    fields.QualifiedHash    `arbor:"order=1,recurse=serialize"`
	ConversationID fields.QualifiedHash    `arbor:"order=2,recurse=serialize"`
	Content        fields.QualifiedContent `arbor:"order=3,recurse=serialize"`
	Trailer        `arbor:"order=4,recurse=always"`
}

func newReply() *Reply {
	r := new(Reply)
	// define how to serialize this node type's fields
	return r
}

func (r *Reply) MarshalSignedData() ([]byte, error) {
	return serialize.ArborSerializeConfig(reflect.ValueOf(r), serialize.SerializationConfig{
		SkipSignatures: true,
	})
}

func (r *Reply) MarshalBinary() ([]byte, error) {
	return serialize.ArborSerialize(reflect.ValueOf(r))
}

func UnmarshalReply(b []byte) (*Reply, error) {
	r := &Reply{}
	if err := r.UnmarshalBinary(b); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Reply) UnmarshalBinary(b []byte) error {
	_, err := serialize.ArborDeserialize(reflect.ValueOf(r), b)
	if err != nil {
		return err
	}
	r.id, err = computeID(r)
	return err
}

func (r *Reply) Equals(other interface{}) bool {
	r2, valid := other.(*Reply)
	if !valid {
		return false
	}
	return r.CommonNode.Equals(&r2.CommonNode) &&
		r.Content.Equals(&r2.Content) &&
		r.Trailer.Equals(&r2.Trailer)
}

// ValidateShallow checks all fields for internal validity. It does not check
// the existence or validity of nodes referenced from this node.
func (r *Reply) ValidateShallow() error {
	if err := r.CommonNode.ValidateShallow(); err != nil {
		return err
	}
	needsValidation := []Validator{&r.Content, &r.CommunityID, &r.ConversationID}
	for _, nv := range needsValidation {
		if err := nv.Validate(); err != nil {
			return err
		}
	}
	if r.Depth < fields.TreeDepth(1) {
		return fmt.Errorf("Reply depth must be at least 1, got %d", r.Depth)
	} else if r.Depth == fields.TreeDepth(1) && !r.ConversationID.Equals(fields.NullHash()) {
		return fmt.Errorf("Reply conversation id at depth 1 must be null hash")
	} else if r.Depth > fields.TreeDepth(1) && r.ConversationID.Equals(fields.NullHash()) {
		return fmt.Errorf("Reply conversation id at depth > 1 must be null hash, got %v", r.ConversationID)
	}
	if r.Parent.Equals(fields.NullHash()) {
		return fmt.Errorf("Reply parent must not be null hash")
	}
	if r.Author.Equals(fields.NullHash()) {
		return fmt.Errorf("Reply author must not be null hash")
	}
	if r.CommunityID.Equals(fields.NullHash()) {
		return fmt.Errorf("Reply community id must not be null hash")
	}
	return nil
}

// ValidateDeep checks all referenced nodes for existence within the store.
func (r *Reply) ValidateDeep(store Store) error {
	needed := []*fields.QualifiedHash{&r.Author, &r.Parent, &r.CommunityID}
	if r.Depth > fields.TreeDepth(1) {
		needed = append(needed, &r.ConversationID)
	}
	for _, neededNode := range needed {
		if _, has, err := store.Get(neededNode); !has {
			return fmt.Errorf("Missing required node %v", neededNode)
		} else if err != nil {
			return err
		}
	}
	return nil
}
