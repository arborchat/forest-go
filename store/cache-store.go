package store

import (
	"fmt"

	forest "git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/fields"
)

// CacheStore combines two other stores into one logical store. It is
// useful when store implementations have different performance
// characteristics and one is dramatically faster than the other. Once
// a CacheStore is created, the individual stores within it should not
// be directly modified.
type CacheStore struct {
	Cache, Back forest.Store
}

var _ forest.Store = &CacheStore{}

// NewCacheStore creates a single logical store from the given two stores.
// All items from `cache` are automatically copied into `base` during
// the construction of the CacheStore, and from then on (assuming
// neither store is modified directly outside of CacheStore) all elements
// added are guaranteed to be added to `base`. It is recommended to use
// fast in-memory implementations as the `cache` layer and disk or
// network-based implementations as the `base` layer.
func NewCacheStore(cache, back forest.Store) (*CacheStore, error) {
	if err := cache.CopyInto(back); err != nil {
		return nil, err
	}
	return &CacheStore{cache, back}, nil
}

// Get returns the requested node if it is present in either the Cache or the Back Store.
// If the cache is missed by the backing store is hit, the node will automatically be
// added to the cache.
func (m *CacheStore) Get(id *fields.QualifiedHash) (forest.Node, bool, error) {
	return m.getUsingFuncs(id, m.Cache.Get, m.Back.Get)
}

func (m *CacheStore) CopyInto(other forest.Store) error {
	return m.Back.CopyInto(other)
}

// Add inserts the given node into both stores of the CacheStore
func (m *CacheStore) Add(node forest.Node) error {
	if err := m.Back.Add(node); err != nil {
		return err
	}
	if err := m.Cache.Add(node); err != nil {
		return err
	}
	return nil
}

func (m *CacheStore) getUsingFuncs(id *fields.QualifiedHash, getter1, getter2 func(*fields.QualifiedHash) (forest.Node, bool, error)) (forest.Node, bool, error) {
	cacheNode, inCache, err := getter1(id)
	if err != nil {
		return nil, false, fmt.Errorf("failed fetching id from cache: %w", err)
	}
	if inCache {
		return cacheNode, inCache, err
	}
	backNode, inBackingStore, err := getter2(id)
	if err != nil {
		return nil, false, fmt.Errorf("failed fetching id from cache: %w", err)
	}
	if inBackingStore {
		if err := m.Cache.Add(backNode); err != nil {
			return nil, false, fmt.Errorf("failed to up-propagate node into cache: %w", err)
		}
	}
	return backNode, inBackingStore, err
}

func (m *CacheStore) GetIdentity(id *fields.QualifiedHash) (forest.Node, bool, error) {
	return m.getUsingFuncs(id, m.Cache.GetIdentity, m.Back.GetIdentity)
}

func (m *CacheStore) GetCommunity(id *fields.QualifiedHash) (forest.Node, bool, error) {
	return m.getUsingFuncs(id, m.Cache.GetCommunity, m.Back.GetCommunity)
}

func (m *CacheStore) GetConversation(communityID, conversationID *fields.QualifiedHash) (forest.Node, bool, error) {
	return m.getUsingFuncs(communityID, // this id is irrelevant
		func(*fields.QualifiedHash) (forest.Node, bool, error) {
			return m.Cache.GetConversation(communityID, conversationID)
		},
		func(*fields.QualifiedHash) (forest.Node, bool, error) {
			return m.Back.GetConversation(communityID, conversationID)
		})
}

func (m *CacheStore) GetReply(communityID, conversationID, replyID *fields.QualifiedHash) (forest.Node, bool, error) {
	return m.getUsingFuncs(communityID, // this id is irrelevant
		func(*fields.QualifiedHash) (forest.Node, bool, error) {
			return m.Cache.GetReply(communityID, conversationID, replyID)
		},
		func(*fields.QualifiedHash) (forest.Node, bool, error) {
			return m.Back.GetReply(communityID, conversationID, replyID)
		})
}

func (m *CacheStore) Children(id *fields.QualifiedHash) ([]*fields.QualifiedHash, error) {
	return m.Back.Children(id)
}

func (m *CacheStore) Recent(nodeType fields.NodeType, quantity int) ([]forest.Node, error) {
	return m.Back.Recent(nodeType, quantity)
}

func (m *CacheStore) RemoveSubtree(id *fields.QualifiedHash) error {
	if err := m.Back.RemoveSubtree(id); err != nil {
		return fmt.Errorf("cachestore failed removing from backing store: %w", err)
	}
	if err := m.Cache.RemoveSubtree(id); err != nil {
		return fmt.Errorf("cachestore failed removing from cache: %w", err)
	}
	return nil
}
