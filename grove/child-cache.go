package grove

import "git.sr.ht/~whereswaldon/forest-go/fields"

// ChildCache provides a simple API for keeping track of which node IDs
// are known to be children of which other node IDs.
type ChildCache struct {
	Elements map[string]map[string]*fields.QualifiedHash
}

// NewChildCache creates a new empty child cache
func NewChildCache() *ChildCache {
	return &ChildCache{
		Elements: make(map[string]map[string]*fields.QualifiedHash),
	}
}

// Add inserts the given children as child elements of the given parent.
func (c *ChildCache) Add(parent *fields.QualifiedHash, children ...*fields.QualifiedHash) {
	submap, inMap := c.Elements[parent.String()]
	if !inMap {
		submap = make(map[string]*fields.QualifiedHash)
		c.Elements[parent.String()] = submap
	}
	for _, child := range children {
		submap[child.String()] = child
	}

}

// Get returns all known children of the given parent. The second return value
// indicates whether or not the parent was found in the cache.
func (c *ChildCache) Get(parent *fields.QualifiedHash) ([]*fields.QualifiedHash, bool) {
	submap, inMap := c.Elements[parent.String()]
	if !inMap {
		return nil, false
	}
	out := make([]*fields.QualifiedHash, 0, len(submap))
	for _, child := range submap {
		out = append(out, child)
	}
	return out, true
}

// RemoveChild removes the provided child node from the
// list of children for the provided parent node.
func (c *ChildCache) RemoveChild(parent, child *fields.QualifiedHash) error {
	submap, inMap := c.Elements[parent.String()]
	if !inMap {
		return nil
	}
	childString := child.String()
	_, contained := submap[childString]
	if !contained {
		return nil
	}
	delete(submap, childString)
	return nil
}

// RemoveParent destroys the top-level cache entry
// for the given node.
func (c *ChildCache) RemoveParent(id *fields.QualifiedHash) {
	idString := id.String()
	_, inMap := c.Elements[idString]
	if !inMap {
		return
	}
	delete(c.Elements, idString)
}
