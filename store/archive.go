package store

import (
	"fmt"

	"git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/fields"
)

// Subscription is an identifier for a particular handler function within
// a SubscriberStore. It can be provided to delete a handler function or to
// suppress notifications to the corresponding handler.
type Subscription uint

// the zero subscription is never used
const neverAssigned = 0
const firstSubscription = 1

// Archive is a wrapper type that extends the store.ExtendedStore interface
// on top of an existing forest.Store. It is safe for concurrent use.
type Archive struct {
	store                                 forest.Store
	requests                              chan func()
	nextSubscriberKey                     Subscription
	postAddSubscribers, preAddSubscribers map[Subscription]func(forest.Node)
}

var _ ExtendedStore = &Archive{}

// NewArchive creates a thread-safe storage structure for
// forest nodes by wrapping an existing store implementation
func NewArchive(store forest.Store) *Archive {
	m := &Archive{
		store:              store,
		requests:           make(chan func()),
		nextSubscriberKey:  firstSubscription,
		postAddSubscribers: make(map[Subscription]func(forest.Node)),
		preAddSubscribers:  make(map[Subscription]func(forest.Node)),
	}
	go func() {
		for function := range m.requests {
			function()
		}
	}()
	return m
}

// SubscribeToNewMessages establishes the given function as a handler to be
// invoked on each node added to the store. The returned subscription ID
// can be used to unsubscribe later, as well as to supress notifications
// with AddAs().
//
// Handler functions are invoked synchronously on the same goroutine that invokes
// Add() or AddAs(), and should not block. If long-running code is needed in a
// handler, launch a new goroutine.
func (m *Archive) SubscribeToNewMessages(handler func(n forest.Node)) (subscriptionID Subscription) {
	return m.subscribeInMap(m.postAddSubscribers, handler)
}

// PresubscribeToNewMessages establishes the given function as a handler to be
// invoked on each node added to the store. The returned subscription ID
// can be used to unsubscribe later, as well as to supress notifications
// with AddAs(). The handler function will be invoked *before* nodes are
// inserted into the store instead of after (like a normal Subscribe).
//
// Handler functions are invoked synchronously on the same goroutine that invokes
// Add() or AddAs(), and should not block. If long-running code is needed in a
// handler, launch a new goroutine.
func (m *Archive) PresubscribeToNewMessages(handler func(n forest.Node)) (subscriptionID Subscription) {
	return m.subscribeInMap(m.preAddSubscribers, handler)
}

func (m *Archive) subscribeInMap(targetMap map[Subscription]func(forest.Node), handler func(n forest.Node)) (subscriptionID Subscription) {
	done := make(chan struct{})
	m.requests <- func() {
		defer close(done)
		subscriptionID = m.nextSubscriberKey
		m.nextSubscriberKey++
		// handler unsigned overflow
		// TODO: ensure subscription reuse can't occur
		if m.nextSubscriberKey == neverAssigned {
			m.nextSubscriberKey = firstSubscription
		}
		targetMap[subscriptionID] = handler
	}
	<-done
	return
}

// UnsubscribeToNewMessages removes the handler for a given subscription from
// the store.
func (m *Archive) UnsubscribeToNewMessages(subscriptionID Subscription) {
	m.unsubscribeInMap(m.postAddSubscribers, subscriptionID)
}

// UnpresubscribeToNewMessages removes the handler for a given subscription from
// the store.
func (m *Archive) UnpresubscribeToNewMessages(subscriptionID Subscription) {
	m.unsubscribeInMap(m.preAddSubscribers, subscriptionID)
}

// executeAsync runs the provided closure in a thread-safe way and blocks until it
// completes
func (m *Archive) executeAsync(f func()) {
	done := make(chan struct{})
	m.requests <- func() {
		defer close(done)
		f()
	}
	<-done
}

func (m *Archive) unsubscribeInMap(targetMap map[Subscription]func(forest.Node), subscriptionID Subscription) {
	m.executeAsync(func() {
		if _, subscribed := targetMap[subscriptionID]; subscribed {
			delete(targetMap, subscriptionID)
		}
	})
}

func (m *Archive) CopyInto(s forest.Store) (err error) {
	m.executeAsync(func() {
		err = m.store.CopyInto(s)
	})
	return
}

func (m *Archive) Get(id *fields.QualifiedHash) (node forest.Node, present bool, err error) {
	m.executeAsync(func() {
		node, present, err = m.store.Get(id)
	})
	return
}

func (m *Archive) GetIdentity(id *fields.QualifiedHash) (node forest.Node, present bool, err error) {
	m.executeAsync(func() {
		node, present, err = m.store.GetIdentity(id)
	})
	return
}

func (m *Archive) GetCommunity(id *fields.QualifiedHash) (node forest.Node, present bool, err error) {
	m.executeAsync(func() {
		node, present, err = m.store.GetCommunity(id)
	})
	return
}

func (m *Archive) GetConversation(communityID, conversationID *fields.QualifiedHash) (node forest.Node, present bool, err error) {
	m.executeAsync(func() {
		node, present, err = m.store.GetConversation(communityID, conversationID)
	})
	return
}

func (m *Archive) GetReply(communityID, conversationID, replyID *fields.QualifiedHash) (node forest.Node, present bool, err error) {
	m.executeAsync(func() {
		node, present, err = m.store.GetReply(communityID, conversationID, replyID)
	})
	return
}

func (m *Archive) Children(id *fields.QualifiedHash) (ids []*fields.QualifiedHash, err error) {
	m.executeAsync(func() {
		ids, err = m.store.Children(id)
	})
	return
}

func (m *Archive) Recent(nodeType fields.NodeType, quantity int) (nodes []forest.Node, err error) {
	m.executeAsync(func() {
		nodes, err = m.store.Recent(nodeType, quantity)
	})
	return
}

// Add inserts a node into the underlying store. Importantly, this will send a notification
// of a new node to *all* subscribers. If the calling code is a subscriber, it will still
// be notified of the new node. To supress this, use AddAs() instead.
//
// Subscribers will only be notified if the node is not already present in the archive.
func (m *Archive) Add(node forest.Node) (err error) {
	return m.AddAs(node, neverAssigned)
}

// AddAs allows adding a node to the underlying store without being notified
// of it as a new node. The addedByID (subscription id returned from SubscribeToNewMessages)
// will not be notified of the new nodes, but all other subscribers will be.
//
// Subscribers will only be notified if the node is not already present in the archive.
func (m *Archive) AddAs(node forest.Node, addedByID Subscription) (err error) {
	if _, has, _ := m.Get(node.ID()); has {
		return
	}
	m.executeAsync(func() {
		m.notifySubscribed(m.preAddSubscribers, node, addedByID)
		if err = m.store.Add(node); err == nil {
			m.notifySubscribed(m.postAddSubscribers, node, addedByID)
		}
	})
	return
}

// notifySubscribed runs all of the subscription handlers with
// the provided node as input to each handler.
func (m *Archive) notifySubscribed(targetMap map[Subscription]func(forest.Node), node forest.Node, ignore Subscription) {
	for subscriptionID, handler := range targetMap {
		if subscriptionID != ignore {
			handler(node)
		}
	}
}

// Shut down the worker gorountine that powers this store. Subsequent
// calls to methods on this MessageStore have undefined behavior
func (m *Archive) Destroy() {
	close(m.requests)
}

// AncestryOf returns the IDs of all known ancestors of the node with the given `id`. The ancestors are
// returned sorted by descending depth, so the root of the ancestry tree is the final node in the slice.
func (a *Archive) AncestryOf(id *fields.QualifiedHash) ([]*fields.QualifiedHash, error) {
	node, present, err := a.Get(id)
	if err != nil {
		return nil, fmt.Errorf("failed looking up %s: %w", id, err)
	} else if !present {
		return []*fields.QualifiedHash{}, nil
	}
	ancestors := make([]*fields.QualifiedHash, 0, node.TreeDepth())
	next := node.ParentID()
	for !next.Equals(fields.NullHash()) {
		parent, present, err := a.Get(next)
		if err != nil {
			return nil, fmt.Errorf("failed looking up ancestor %s: %w", next, err)
		} else if !present {
			return ancestors, nil
		}
		ancestors = append(ancestors, next)
		next = parent.ParentID()
	}
	return ancestors, nil
}

// DescendantsOf returns the IDs of all known descendants of the node with the given `id`. The order
// in which the descendants are returned is undefined.
func (a *Archive) DescendantsOf(id *fields.QualifiedHash) ([]*fields.QualifiedHash, error) {
	descendants := make([]*fields.QualifiedHash, 0)

	err := Walk(a, id, func(id *fields.QualifiedHash) error {
		descendants = append(descendants, id)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed traversing descendants: %w", err)
	}
	return descendants, nil
}

// LeavesOf returns the leaf nodes of the tree rooted at `id`. The order of the returned
// leaves is undefined.
func (a *Archive) LeavesOf(id *fields.QualifiedHash) ([]*fields.QualifiedHash, error) {
	leaves := make([]*fields.QualifiedHash, 0)

	err := Walk(a, id, func(id *fields.QualifiedHash) error {
		children, err := a.Children(id)
		if err != nil {
			return fmt.Errorf("failed looking up children of %s: %w", id, err)
		}
		if len(children) == 0 {
			leaves = append(leaves, id)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed traversing descendants: %w", err)
	}
	return leaves, nil
}

func (a *Archive) RemoveSubtree(id *fields.QualifiedHash) error {
	var err error
	a.executeAsync(func() {
		err = a.store.RemoveSubtree(id)
	})
	return err
}
