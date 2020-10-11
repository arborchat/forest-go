package store

import (
	"fmt"
	"sort"

	forest "git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/fields"
)

type MemoryStore struct {
	Items    map[string]forest.Node
	ChildMap map[string][]string
}

var _ forest.Store = &MemoryStore{}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		Items:    make(map[string]forest.Node),
		ChildMap: make(map[string][]string),
	}
}

func (m *MemoryStore) CopyInto(other forest.Store) error {
	for _, node := range m.Items {
		if err := other.Add(node); err != nil {
			return err
		}
	}
	return nil
}

func (m *MemoryStore) Get(id *fields.QualifiedHash) (forest.Node, bool, error) {
	return m.GetID(id.String())
}

func (m *MemoryStore) GetIdentity(id *fields.QualifiedHash) (forest.Node, bool, error) {
	return m.Get(id)
}

func (m *MemoryStore) GetCommunity(id *fields.QualifiedHash) (forest.Node, bool, error) {
	return m.Get(id)
}

func (m *MemoryStore) GetConversation(communityID, conversationID *fields.QualifiedHash) (forest.Node, bool, error) {
	return m.Get(conversationID)
}

func (m *MemoryStore) GetReply(communityID, conversationID, replyID *fields.QualifiedHash) (forest.Node, bool, error) {
	return m.Get(replyID)
}

func (m *MemoryStore) GetID(id string) (forest.Node, bool, error) {
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

func (m *MemoryStore) Add(node forest.Node) error {
	id := node.ID().String()
	return m.AddID(id, node)
}

func (m *MemoryStore) AddID(id string, node forest.Node) error {
	// safe to ignore error because we know it can't happen
	if _, has, _ := m.GetID(id); has {
		return nil
	}
	m.Items[id] = node
	parentID := node.ParentID().String()
	m.ChildMap[parentID] = append(m.ChildMap[parentID], id)
	return nil
}

func (m *MemoryStore) RemoveSubtree(id *fields.QualifiedHash) error {
	children, err := m.Children(id)
	if err != nil {
		return fmt.Errorf("failed looking up children of %s: %w", id, err)
	}
	for _, child := range children {
		if err := m.RemoveSubtree(child); err != nil {
			return fmt.Errorf("failed removing children of %s: %w", child, err)
		}
	}
	child, _, err := m.Get(id)
	if err != nil {
		return fmt.Errorf("failed looking up child %s during removal: %w", id, err)
	}
	idString := id.String()
	parentIdString := child.ParentID().String()
	delete(m.Items, idString)
	siblings := m.ChildMap[parentIdString]
	for i := range siblings {
		if siblings[i] != idString {
			continue
		}
		for k := i + 1; k < len(siblings); k++ {
			siblings[i] = siblings[k]
		}
		m.ChildMap[parentIdString] = siblings[:len(siblings)-1]
		break
	}
	return nil
}

// Recent returns a slice of len `quantity` (or fewer) nodes of the given type.
// These nodes are the most recent (by creation time) nodes of that type known
// to the store.
func (m *MemoryStore) Recent(nodeType fields.NodeType, quantity int) ([]forest.Node, error) {
	// highly inefficient implementation, but it should work for now
	candidates := make([]forest.Node, 0, quantity)
	for _, node := range m.Items {
		switch n := node.(type) {
		case *forest.Identity:
			if nodeType == fields.NodeTypeIdentity {
				candidates = append(candidates, n)
				sort.SliceStable(candidates, func(i, j int) bool {
					return candidates[i].(*forest.Identity).Created > candidates[j].(*forest.Identity).Created
				})
			}
		case *forest.Community:
			if nodeType == fields.NodeTypeCommunity {
				candidates = append(candidates, n)
				sort.SliceStable(candidates, func(i, j int) bool {
					return candidates[i].(*forest.Community).Created > candidates[j].(*forest.Community).Created
				})
			}
		case *forest.Reply:
			if nodeType == fields.NodeTypeReply {
				candidates = append(candidates, n)
				sort.SliceStable(candidates, func(i, j int) bool {
					return candidates[i].(*forest.Reply).Created > candidates[j].(*forest.Reply).Created
				})
			}
		}
	}
	if len(candidates) > quantity {
		candidates = candidates[:quantity]
	}
	return candidates, nil
}
