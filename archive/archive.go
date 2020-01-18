/*
Package archive defines a helpful wrapper type to augment the store interface
with higher-level methods for querying ancestry and descendants of nodes in
the store.
*/
package archive

import (
	"fmt"

	"git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/fields"
)

// Archive extends a store with methods high-level structural queries.
type Archive struct {
	forest.Store
}

// New constructs a new Archive wrapping the given store
func New(store forest.Store) *Archive {
	return &Archive{store}
}

// AncestryOf returns the IDs of all known ancestors of the node with the given `id`. The ancestors are
// returned sorted by descending depth, so the root of the ancestry tree is the final node in the slice.
func (a *Archive) AncestryOf(id *fields.QualifiedHash) ([]*fields.QualifiedHash, error) {
	node, present, err := a.Store.Get(id)
	if err != nil {
		return nil, fmt.Errorf("failed looking up %s: %w", id, err)
	} else if !present {
		return []*fields.QualifiedHash{}, nil
	}
	ancestors := make([]*fields.QualifiedHash, 0, node.TreeDepth())
	next := node.ParentID()
	for !next.Equals(fields.NullHash()) {
		parent, present, err := a.Store.Get(next)
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
func (v *Archive) DescendantsOf(id *fields.QualifiedHash) ([]*fields.QualifiedHash, error) {
	descendants := make([]*fields.QualifiedHash, 0)
	directChildren := []*fields.QualifiedHash{id}

	for len(directChildren) > 0 {
		target := directChildren[0]
		directChildren = directChildren[1:]
		children, err := v.Children(target)
		if err != nil {
			return nil, fmt.Errorf("failed looking up children of %s: %w", target, err)
		}
		for _, childID := range children {
			descendants = append(descendants, childID)
			directChildren = append(directChildren, childID)
		}
	}
	return descendants, nil
}

// LeavesOf returns the leaf nodes of the tree rooted at `id`. The order of the returned
// leaves is undefined.
func (v *Archive) LeavesOf(id *fields.QualifiedHash) ([]*fields.QualifiedHash, error) {
	leaves := make([]*fields.QualifiedHash, 0)
	directChildren := []*fields.QualifiedHash{id}

	for len(directChildren) > 0 {
		target := directChildren[0]
		directChildren = directChildren[1:]
		children, err := v.Children(target)
		if err != nil {
			return nil, fmt.Errorf("failed looking up children of %s: %w", target, err)
		}
		if len(children) == 0 {
			leaves = append(leaves, target)
			continue
		}
		for _, childID := range children {
			directChildren = append(directChildren, childID)
		}
	}
	return leaves, nil
}
