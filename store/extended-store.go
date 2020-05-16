package store

import (
	forest "git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/fields"
)

// ExtendedStore provides a superset of the functionality of the Store interface,
// implementing methods for subscribing to changes and querying higher-level
// structural information like ancestry and descendants.
type ExtendedStore interface {
	forest.Store
	SubscribeToNewMessages(handler func(n forest.Node)) Subscription
	UnsubscribeToNewMessages(Subscription)
	AddAs(forest.Node, Subscription) (err error)
	AncestryOf(id *fields.QualifiedHash) ([]*fields.QualifiedHash, error)
	DescendantsOf(id *fields.QualifiedHash) ([]*fields.QualifiedHash, error)
	LeavesOf(id *fields.QualifiedHash) ([]*fields.QualifiedHash, error)
}
