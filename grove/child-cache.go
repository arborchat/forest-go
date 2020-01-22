package grove

import "git.sr.ht/~whereswaldon/forest-go/fields"

// ChildCache provides a simple API for keeping track of which node IDs
// are known to be children of which other node IDs.
type ChildCache struct {
	Elements map[string]map[*fields.QualifiedHash]struct{}
}

// NewChildCache creates a new empty child cache
func NewChildCache() *ChildCache {
	return &ChildCache{
		Elements: make(map[string]map[*fields.QualifiedHash]struct{}),
	}
}

// Add inserts the given children as child elements of the given parent.
func (c *ChildCache) Add(parent *fields.QualifiedHash, children ...*fields.QualifiedHash) {
	submap, inMap := c.Elements[parent.String()]
	if !inMap {
		submap = make(map[*fields.QualifiedHash]struct{})
		c.Elements[parent.String()] = submap
	}
	for _, child := range children {
		submap[child] = struct{}{}
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
	for child := range submap {
		out = append(out, child)
	}
	return out, true
}
