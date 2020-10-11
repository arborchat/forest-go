package store

import (
	"fmt"

	"git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/fields"
)

// Walk traverses the subtree rooted at start in a breadth-first fashion invoking the
// visitor function on each node id in the subtree. The traversal stops either
// when the visitor function returns non-nil or when the entire subtree
// rooted at start has been visited.
//
// If the visitor function returns an error, it will be returned wrapped and
// can be checked for using the errors.Is or errors.As standard library
// functions.
func Walk(s forest.Store, start *fields.QualifiedHash, visitor func(*fields.QualifiedHash) error) error {
	if s == nil {
		return fmt.Errorf("store cannot be nil")
	}
	if start == nil {
		return fmt.Errorf("start cannot be nil")
	}
	if visitor == nil {
		return fmt.Errorf("visitor cannot be nil")
	}

	childQueue := []*fields.QualifiedHash{start}
	var current *fields.QualifiedHash
	for len(childQueue) > 0 {
		current, childQueue = childQueue[0], childQueue[1:]
		err := visitor(current)
		if err != nil {
			return fmt.Errorf("visitor function errored on %s: %w", current, err)
		}
		children, err := s.Children(current)
		if err != nil {
			return fmt.Errorf("failed visiting children of %s: %w", current, err)
		}
		childQueue = append(childQueue, children...)
	}
	return nil
}

// WalkNodes traverses the subtree rooted at start in a breadth-first fashion invoking the
// visitor function on each node in the subtree.
func WalkNodes(s forest.Store, start forest.Node, visitor func(forest.Node) error) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed walking nodes: %w", err)
		}
	}()
	var (
		node forest.Node
		has  bool
	)
	return Walk(s, start.ID(), func(id *fields.QualifiedHash) error {
		node, has, err = s.Get(id)
		if err != nil {
			return err
		} else if !has {
			return fmt.Errorf("tried to visit nonexistent node: %s", id)
		}
		return visitor(node)
	})
}
