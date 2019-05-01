package forest

import (
	"bytes"
	"encoding"
	"io"

	"git.sr.ht/~whereswaldon/forest-go/fields"
)

// generic node
type commonNode struct {
	// the ID is deterministically computed from the rest of the values
	id                 fields.Value
	Type               fields.NodeType
	SchemaVersion      fields.Version
	Parent             fields.QualifiedHash
	IDDesc             fields.HashDescriptor
	Depth              fields.TreeDepth
	Metadata           fields.QualifiedContent
	SignatureAuthority fields.QualifiedHash
	Signature          fields.QualifiedSignature
}

// Compute and return the commonNode's ID as a fields.Qualified Hash
func (n commonNode) ID() *fields.QualifiedHash {
	return &fields.QualifiedHash{
		Descriptor: n.IDDesc,
		Value:      n.id,
	}
}

func (n *commonNode) presignSerializationOrder() []fields.BidirectionalBinaryMarshaler {
	order := []fields.BidirectionalBinaryMarshaler{
		&n.SchemaVersion,
		&n.Type,
	}
	order = append(order, &n.Parent)
	order = append(order, n.IDDesc.SerializationOrder()...)
	order = append(order, &n.Depth)
	order = append(order, &n.Metadata)
	order = append(order, &n.SignatureAuthority)
	return order
}

func (n *commonNode) postsignSerializationOrder() []fields.BidirectionalBinaryMarshaler {
	return []fields.BidirectionalBinaryMarshaler{&n.Signature}
}

// unmarshalBinaryPreamble does the unmarshaling work for all of the common
// node fields before the node-specific fields and returns the unused data.
func (n *commonNode) unmarshalBinaryPreamble(b []byte) ([]byte, error) {
	return fields.UnmarshalAll(b, fields.AsUnmarshaler(n.presignSerializationOrder())...)
}

// unmarshalBinarySignature does the unmarshaling work for the signature field after the
// node-specific fields and returns the unused data.
func (n *commonNode) unmarshalBinarySignature(b []byte) ([]byte, error) {
	return fields.UnmarshalAll(b, fields.AsUnmarshaler(n.postsignSerializationOrder())...)
}

// GetSignature returns the signature for the node, which must correspond to the Signature Authority for
// the node in order to be valid.
func (n *commonNode) GetSignature() *fields.QualifiedSignature {
	return &n.Signature
}

// SignatureIdentityHash returns the node identitifer for the Identity that signed this node.
func (n *commonNode) SignatureIdentityHash() *fields.QualifiedHash {
	return &n.SignatureAuthority
}

func (n commonNode) IsIdentity() bool {
	return n.Type == fields.NodeTypeIdentity
}

func (n commonNode) HashDescriptor() *fields.HashDescriptor {
	return &n.IDDesc
}

func (n *commonNode) Equals(n2 *commonNode) bool {
	return n.Type.Equals(&n2.Type) &&
		n.SchemaVersion.Equals(&n2.SchemaVersion) &&
		n.Parent.Equals(&n2.Parent) &&
		n.IDDesc.Equals(&n2.IDDesc) &&
		n.Depth.Equals(&n2.Depth) &&
		n.Metadata.Equals(&n2.Metadata) &&
		n.SignatureAuthority.Equals(&n2.SignatureAuthority) &&
		n.Signature.Equals(&n2.Signature)
}

// concrete nodes

// Identity nodes represent a user. They associate a username with a public key that the user
// will sign messages with.
type Identity struct {
	commonNode
	Name      fields.QualifiedContent
	PublicKey fields.QualifiedKey
}

func newIdentity() *Identity {
	i := new(Identity)
	// define how to serialize this node type's fields
	return i
}

func (i *Identity) nodeSpecificSerializationOrder() []fields.BidirectionalBinaryMarshaler {
	return []fields.BidirectionalBinaryMarshaler{&i.Name, &i.PublicKey}
}

func (i *Identity) SerializationOrder() []fields.BidirectionalBinaryMarshaler {
	order := i.commonNode.presignSerializationOrder()
	order = append(order, i.nodeSpecificSerializationOrder()...)
	order = append(order, i.commonNode.postsignSerializationOrder()...)
	return order
}

// MarshalSignedData writes all data that should be signed in the correct order for signing. This
// can be used both to generate and validate message signatures.
func (i Identity) MarshalSignedData() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := fields.MarshalAllInto(buf, fields.AsMarshaler(i.presignSerializationOrder())...); err != nil {
		return nil, err
	}
	if err := fields.MarshalAllInto(buf, fields.AsMarshaler(i.nodeSpecificSerializationOrder())...); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (i Identity) MarshalBinary() ([]byte, error) {
	signed, err := i.MarshalSignedData()
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(signed)
	if err := fields.MarshalAllInto(buf, fields.AsMarshaler(i.postsignSerializationOrder())...); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func UnmarshalIdentity(b []byte) (*Identity, error) {
	i := newIdentity()
	if err := i.UnmarshalBinary(b); err != nil {
		return nil, err
	}
	return i, nil
}

func (i *Identity) UnmarshalBinary(b []byte) error {
	_, err := fields.UnmarshalAll(b, fields.AsUnmarshaler(i.SerializationOrder())...)
	if err != nil {
		return err
	}
	idBytes, err := computeID(i)
	if err != nil {
		return err
	}
	i.id = fields.Value(idBytes)
	return nil
}

func marshalTextWithPrefix(w io.Writer, prefix string, target encoding.TextMarshaler) error {
	b, err := target.MarshalText()
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(prefix)); err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	return nil
}

/*func (i *Identity) MarshalText() ([]byte, error) {
	buf := new(bytes.Buffer)
	if _, err := buf.WriteString("identity {"); err != nil {
		return nil, err
	}
	id := i.ID()
	if err := marshalTextWithPrefix(buf, "\n\tID: ", id); err != nil {
		return nil, err
	}
	if err := marshalTextWithPrefix(buf, "\n\tParent: ", i.Parent); err != nil {
		return nil, err
	}
	if err := marshalTextWithPrefix(buf, "\n\tName: ", i.Name); err != nil {
		return nil, err
	}
	if err := marshalTextWithPrefix(buf, "\n\tPublicKey: ", i.PublicKey); err != nil {
		return nil, err
	}
	if err := marshalTextWithPrefix(buf, "\n\tMetadata: ", i.Metadata); err != nil {
		return nil, err
	}
	if err := marshalTextWithPrefix(buf, "\n\tSignatureAuthority: ", i.SignatureAuthority); err != nil {
		return nil, err
	}
	if err := marshalTextWithPrefix(buf, "\n\tSignature: ", i.Signature()); err != nil {
		return nil, err
	}
	if _, err := buf.WriteString("\n}"); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}*/

func (i *Identity) Equals(i2 *Identity) bool {
	return i.commonNode.Equals(&i2.commonNode) &&
		i.Name.Equals(&i2.Name) &&
		i.PublicKey.Equals(&i2.PublicKey)
}

type Community struct {
	commonNode
	Name fields.QualifiedContent
}

func newCommunity() *Community {
	c := new(Community)
	// define how to serialize this node type's fields
	return c
}

func (c *Community) nodeSpecificSerializationOrder() []fields.BidirectionalBinaryMarshaler {
	return []fields.BidirectionalBinaryMarshaler{&c.Name}
}

func (c *Community) SerializationOrder() []fields.BidirectionalBinaryMarshaler {
	order := c.commonNode.presignSerializationOrder()
	order = append(order, c.nodeSpecificSerializationOrder()...)
	order = append(order, c.commonNode.postsignSerializationOrder()...)
	return order
}

func (c Community) MarshalSignedData() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := fields.MarshalAllInto(buf, fields.AsMarshaler(c.presignSerializationOrder())...); err != nil {
		return nil, err
	}
	if err := fields.MarshalAllInto(buf, fields.AsMarshaler(c.nodeSpecificSerializationOrder())...); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c Community) MarshalBinary() ([]byte, error) {
	signed, err := c.MarshalSignedData()
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(signed)
	if err := fields.MarshalAllInto(buf, fields.AsMarshaler(c.postsignSerializationOrder())...); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func UnmarshalCommunity(b []byte) (*Community, error) {
	c := newCommunity()
	if err := c.UnmarshalBinary(b); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Community) UnmarshalBinary(b []byte) error {
	_, err := fields.UnmarshalAll(b, fields.AsUnmarshaler(c.SerializationOrder())...)
	if err != nil {
		return err
	}
	idBytes, err := computeID(c)
	if err != nil {
		return err
	}
	c.id = fields.Value(idBytes)
	return nil
}

func (c *Community) Equals(c2 *Community) bool {
	return c.commonNode.Equals(&c2.commonNode) &&
		c.Name.Equals(&c2.Name)
}

type Conversation struct {
	commonNode
	Content fields.QualifiedContent
}

func newConversation() *Conversation {
	c := new(Conversation)
	// define how to serialize this node type's fields
	return c
}

func (c *Conversation) nodeSpecificSerializationOrder() []fields.BidirectionalBinaryMarshaler {
	return []fields.BidirectionalBinaryMarshaler{&c.Content}
}

func (c *Conversation) SerializationOrder() []fields.BidirectionalBinaryMarshaler {
	order := c.commonNode.presignSerializationOrder()
	order = append(order, c.nodeSpecificSerializationOrder()...)
	order = append(order, c.commonNode.postsignSerializationOrder()...)
	return order
}

func (c Conversation) MarshalSignedData() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := fields.MarshalAllInto(buf, fields.AsMarshaler(c.presignSerializationOrder())...); err != nil {
		return nil, err
	}
	if err := fields.MarshalAllInto(buf, fields.AsMarshaler(c.nodeSpecificSerializationOrder())...); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c Conversation) MarshalBinary() ([]byte, error) {
	signed, err := c.MarshalSignedData()
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(signed)
	if err := fields.MarshalAllInto(buf, fields.AsMarshaler(c.postsignSerializationOrder())...); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func UnmarshalConversation(b []byte) (*Conversation, error) {
	c := newConversation()
	if err := c.UnmarshalBinary(b); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Conversation) UnmarshalBinary(b []byte) error {
	_, err := fields.UnmarshalAll(b, fields.AsUnmarshaler(c.SerializationOrder())...)
	if err != nil {
		return err
	}
	idBytes, err := computeID(c)
	if err != nil {
		return err
	}
	c.id = fields.Value(idBytes)
	return nil
}

func (c *Conversation) Equals(c2 *Conversation) bool {
	return c.commonNode.Equals(&c2.commonNode) &&
		c.Content.Equals(&c2.Content)
}

type Reply struct {
	commonNode
	CommunityID fields.QualifiedHash
	Content     fields.QualifiedContent
}

func newReply() *Reply {
	r := new(Reply)
	// define how to serialize this node type's fields
	return r
}

func (r *Reply) nodeSpecificSerializationOrder() []fields.BidirectionalBinaryMarshaler {
	return []fields.BidirectionalBinaryMarshaler{&r.CommunityID, &r.Content}
}

func (r *Reply) SerializationOrder() []fields.BidirectionalBinaryMarshaler {
	order := r.commonNode.presignSerializationOrder()
	order = append(order, r.nodeSpecificSerializationOrder()...)
	order = append(order, r.commonNode.postsignSerializationOrder()...)
	return order
}

func (r Reply) MarshalSignedData() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := fields.MarshalAllInto(buf, fields.AsMarshaler(r.presignSerializationOrder())...); err != nil {
		return nil, err
	}
	if err := fields.MarshalAllInto(buf, fields.AsMarshaler(r.nodeSpecificSerializationOrder())...); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (r Reply) MarshalBinary() ([]byte, error) {
	signed, err := r.MarshalSignedData()
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(signed)
	if err := fields.MarshalAllInto(buf, fields.AsMarshaler(r.postsignSerializationOrder())...); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func UnmarshalReply(b []byte) (*Reply, error) {
	r := newReply()
	if err := r.UnmarshalBinary(b); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Reply) UnmarshalBinary(b []byte) error {
	_, err := fields.UnmarshalAll(b, fields.AsUnmarshaler(r.SerializationOrder())...)
	if err != nil {
		return err
	}
	idBytes, err := computeID(r)
	if err != nil {
		return err
	}
	r.id = fields.Value(idBytes)
	return nil
}

func (r *Reply) Equals(r2 *Reply) bool {
	return r.commonNode.Equals(&r2.commonNode) &&
		r.Content.Equals(&r2.Content)
}
