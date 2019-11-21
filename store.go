package forest

import (
	"fmt"
	"sort"

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

type MemoryStore struct {
	Items    map[string]Node
	ChildMap map[string][]string
}

var _ Store = &MemoryStore{}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		Items:    make(map[string]Node),
		ChildMap: make(map[string][]string),
	}
}

func (m *MemoryStore) CopyInto(other Store) error {
	for _, node := range m.Items {
		if err := other.Add(node); err != nil {
			return err
		}
	}
	return nil
}

func (m *MemoryStore) Get(id *fields.QualifiedHash) (Node, bool, error) {
	return m.GetID(id.String())
}

func (m *MemoryStore) GetIdentity(id *fields.QualifiedHash) (Node, bool, error) {
	return m.Get(id)
}

func (m *MemoryStore) GetCommunity(id *fields.QualifiedHash) (Node, bool, error) {
	return m.Get(id)
}

func (m *MemoryStore) GetConversation(communityID, conversationID *fields.QualifiedHash) (Node, bool, error) {
	return m.Get(conversationID)
}

func (m *MemoryStore) GetReply(communityID, conversationID, replyID *fields.QualifiedHash) (Node, bool, error) {
	return m.Get(replyID)
}

func (m *MemoryStore) GetID(id string) (Node, bool, error) {
	item, has := m.Items[id]
	return item, has, nil
}

func (m *MemoryStore) Children(id *fields.QualifiedHash) ([]*fields.QualifiedHash, error) {
	idString := id.String()
	children, any := m.ChildMap[idString]
	if !any {
		return []*fields.QualifiedHash{}, nil
	}
	childIDs := make([]*fields.QualifiedHash, len(children))
	for i, childStr := range children {
		childIDs[i] = &fields.QualifiedHash{}
		if err := childIDs[i].UnmarshalText([]byte(childStr)); err != nil {
			return nil, fmt.Errorf("failed to transform key back into node id: %w", err)
		}
	}
	return childIDs, nil
}

func (m *MemoryStore) Add(node Node) error {
	id := node.ID().String()
	return m.AddID(id, node)
}

func (m *MemoryStore) AddID(id string, node Node) error {
	// safe to ignore error because we know it can't happen
	if _, has, _ := m.GetID(id); has {
		return nil
	}
	m.Items[id] = node
	parentID := node.ParentID().String()
	m.ChildMap[parentID] = append(m.ChildMap[parentID], id)
	return nil
}

// Recent returns a slice of len `quantity` (or fewer) nodes of the given type.
// These nodes are the most recent (by creation time) nodes of that type known
// to the store.
func (m *MemoryStore) Recent(nodeType fields.NodeType, quantity int) ([]Node, error) {
	// highly inefficient implementation, but it should work for now
	candidates := make([]Node, 0, quantity)
	for _, node := range m.Items {
		switch n := node.(type) {
		case *Identity:
			if nodeType == fields.NodeTypeIdentity {
				candidates = append(candidates, n)
				sort.SliceStable(candidates, func(i, j int) bool {
					return candidates[i].(*Identity).Created > candidates[j].(*Identity).Created
				})
			}
		case *Community:
			if nodeType == fields.NodeTypeCommunity {
				candidates = append(candidates, n)
				sort.SliceStable(candidates, func(i, j int) bool {
					return candidates[i].(*Community).Created > candidates[j].(*Community).Created
				})
			}
		case *Reply:
			if nodeType == fields.NodeTypeReply {
				candidates = append(candidates, n)
				sort.SliceStable(candidates, func(i, j int) bool {
					return candidates[i].(*Reply).Created > candidates[j].(*Reply).Created
				})
			}
		}
	}
	if len(candidates) > quantity {
		candidates = candidates[:quantity]
	}
	return candidates, nil
}

// CacheStore combines two other stores into one logical store. It is
// useful when store implementations have different performance
// characteristics and one is dramatically faster than the other. Once
// a CacheStore is created, the individual stores within it should not
// be directly modified.
type CacheStore struct {
	Cache, Back Store
}

var _ Store = &CacheStore{}

// NewCacheStore creates a single logical store from the given two stores.
// All items from `cache` are automatically copied into `base` during
// the construction of the CacheStore, and from then on (assuming
// neither store is modified directly outside of CacheStore) all elements
// added are guaranteed to be added to `base`. It is recommended to use
// fast in-memory implementations as the `cache` layer and disk or
// network-based implementations as the `base` layer.
func NewCacheStore(cache, back Store) (*CacheStore, error) {
	if err := cache.CopyInto(back); err != nil {
		return nil, err
	}
	return &CacheStore{cache, back}, nil
}

// Get returns the requested node if it is present in either the Cache or the Back Store.
// If the cache is missed by the backing store is hit, the node will automatically be
// added to the cache.
func (m *CacheStore) Get(id *fields.QualifiedHash) (Node, bool, error) {
	return m.getUsingFuncs(id, m.Cache.Get, m.Back.Get)
}

func (m *CacheStore) CopyInto(other Store) error {
	return m.Back.CopyInto(other)
}

// Add inserts the given node into both stores of the CacheStore
func (m *CacheStore) Add(node Node) error {
	if err := m.Back.Add(node); err != nil {
		return err
	}
	if err := m.Cache.Add(node); err != nil {
		return err
	}
	return nil
}

func (m *CacheStore) getUsingFuncs(id *fields.QualifiedHash, getter1, getter2 func(*fields.QualifiedHash) (Node, bool, error)) (Node, bool, error) {
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

func (m *CacheStore) GetIdentity(id *fields.QualifiedHash) (Node, bool, error) {
	return m.getUsingFuncs(id, m.Cache.GetIdentity, m.Back.GetIdentity)
}

func (m *CacheStore) GetCommunity(id *fields.QualifiedHash) (Node, bool, error) {
	return m.getUsingFuncs(id, m.Cache.GetCommunity, m.Back.GetCommunity)
}

func (m *CacheStore) GetConversation(communityID, conversationID *fields.QualifiedHash) (Node, bool, error) {
	return m.getUsingFuncs(communityID, // this id is irrelevant
		func(*fields.QualifiedHash) (Node, bool, error) {
			return m.Cache.GetConversation(communityID, conversationID)
		},
		func(*fields.QualifiedHash) (Node, bool, error) {
			return m.Back.GetConversation(communityID, conversationID)
		})
}

func (m *CacheStore) GetReply(communityID, conversationID, replyID *fields.QualifiedHash) (Node, bool, error) {
	return m.getUsingFuncs(communityID, // this id is irrelevant
		func(*fields.QualifiedHash) (Node, bool, error) {
			return m.Cache.GetReply(communityID, conversationID, replyID)
		},
		func(*fields.QualifiedHash) (Node, bool, error) {
			return m.Back.GetReply(communityID, conversationID, replyID)
		})
}

func (m *CacheStore) Children(id *fields.QualifiedHash) ([]*fields.QualifiedHash, error) {
	return m.Back.Children(id)
}

func (m *CacheStore) Recent(nodeType fields.NodeType, quantity int) ([]Node, error) {
	return m.Back.Recent(nodeType, quantity)
}
