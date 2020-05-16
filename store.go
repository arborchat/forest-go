package forest

import (
	"git.sr.ht/~whereswaldon/forest-go/fields"
)

type Store interface {
	CopyInto(Store) error
	Get(*fields.QualifiedHash) (Node, bool, error)
	GetIdentity(*fields.QualifiedHash) (Node, bool, error)
	GetCommunity(*fields.QualifiedHash) (Node, bool, error)
	GetConversation(communityID, conversationID *fields.QualifiedHash) (Node, bool, error)
	GetReply(communityID, conversationID, replyID *fields.QualifiedHash) (Node, bool, error)
	Children(*fields.QualifiedHash) ([]*fields.QualifiedHash, error)
	Recent(nodeType fields.NodeType, quantity int) ([]Node, error)
	// Add inserts a node into the store. It is *not* an error to insert a node which is already
	// stored. Implementations must not return an error in this case.
	Add(Node) error
}
