/*
Package grove implements an on-disk storage format for arbor forest
nodes. This hierarchical storage format is called a "grove", and
the management type implemented by this package satisfies the
forest.Store interface.

Note: this package is not yet complete.
*/
package grove

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/fields"
)

// File represents a type that supports file-like operations. *os.File
// implements this interface, and will likely be used most of the time.
// This interface exists mostly to simply testing.
type File interface {
	io.ReadWriteCloser
	Name() string
	Readdir(n int) ([]os.FileInfo, error)
}

// FS represents a type that acts as a filesystem. It can create and
// open files at specific paths
type FS interface {
	Open(path string) (File, error)
	Create(path string) (File, error)
	OpenFile(path string, flag int, perm os.FileMode) (File, error)
}

// RelativeFS is a file system that acts relative to a specific path
type RelativeFS struct {
	Root string
}

// ensure RelativeFS satisfies the FS interface
var _ FS = RelativeFS{}

func (r RelativeFS) resolve(path string) string {
	return filepath.Join(r.Root, path)
}

// Open opens the given path as an absolute path relative to the root
// of the RelativeFS
func (r RelativeFS) Open(path string) (File, error) {
	return os.Open(r.resolve(path))
}

// Create makes the given path as an absolute path relative to the root
// of the RelativeFS
func (r RelativeFS) Create(path string) (File, error) {
	return os.Create(r.resolve(path))
}

// OpenFile opens the given path as an absolute path relative to the root
// of the RelativeFS
func (r RelativeFS) OpenFile(path string, flag int, perm os.FileMode) (File, error) {
	return os.OpenFile(r.resolve(path), flag, perm)
}

// Grove is an on-disk store for arbor forest nodes. It maintains internal
// in-memory caches in order to accelerate certain expensive operations.
// Because of this, it must be notified when new content appears on disk.
// The recommended way to handle this is to use file-system watching on
// the grove directory and to call Add() with any new nodes that appear
// (it is not an error to call Add() on a node already present in a store).
type Grove struct {
	FS
	NodeCache *forest.MemoryStore
	*ChildCache
}

// New constructs a Grove that stores nodes in a hierarchy rooted at
// the given path.
func New(root string) (*Grove, error) {
	return NewWithFS(RelativeFS{root})
}

// NewWithFS constructs a Grove using the given FS implementation to
// access its nodes. This is primarily useful for testing.
func NewWithFS(fs FS) (*Grove, error) {
	if fs == nil {
		return nil, fmt.Errorf("fs cannot be nil")
	}
	return &Grove{
		FS:         fs,
		NodeCache:  forest.NewMemoryStore(),
		ChildCache: NewChildCache(),
	}, nil
}

// Get searches the grove for a node with the given id. It returns the node if it was
// found, a boolean indicating whether it was found, and an error (if there was a
// problem searching for the node).
// The returned `present` will never be true unless the returned `node` holds an
// actual node struct. If the file holding a node exists on disk but was unable
// to be opened, read, or parsed, `present` will still be false.
func (g *Grove) Get(nodeID *fields.QualifiedHash) (node forest.Node, present bool, err error) {
	node, inCache, _ := g.NodeCache.Get(nodeID)
	if inCache {
		return node, true, nil
	}
	filename := nodeID.String()
	file, err := g.Open(filename)
	// if the file doesn't exist, just return false with no error
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	// if it's some other error, wrap it and return
	if err != nil {
		return nil, false, fmt.Errorf("failed opening node file \"%s\": %w", filename, err)
	}
	b, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, false, fmt.Errorf("failed reading bytes from \"%s\": %w", filename, err)
	}
	node, err = forest.UnmarshalBinaryNode(b)
	if err != nil {
		return nil, false, fmt.Errorf("failed unmarshalling node from \"%s\": %w", filename, err)
	}
	_ = g.NodeCache.Add(node)
	return node, true, nil
}

// getAllNodeFileInfo returns a slice of information about all node files
// within the grove.
func (g *Grove) getAllNodeFileInfo() ([]os.FileInfo, error) {
	// open root of grove hierarchy so we can list its nodes
	rootDir, err := g.Open("")
	if err != nil {
		return nil, fmt.Errorf("failed opening grove root dir: %w", err)
	}
	info, err := rootDir.Readdir(-1) // read whole directory at once. Inefficient
	if err != nil {
		return nil, fmt.Errorf("failed listing files in grove: %w", err)
	}
	nodeInfo := make([]os.FileInfo, 0, len(info))
	// find all files that are plausibly nodes
	for _, fileInfo := range info {
		// search for the string form of all supported hash types
		for _, hashName := range fields.HashNames {
			if strings.HasPrefix(fileInfo.Name(), hashName) {
				nodeInfo = append(nodeInfo, fileInfo)
			}
		}
	}
	return nodeInfo, nil
}

// nodeFromInfo converts the info about a file into a node extracted from
// the contents of that file (it opens, reads, and parses the file).
func (g *Grove) nodeFromInfo(info os.FileInfo) (forest.Node, error) {
	nodeFile, err := g.Open(info.Name())
	if err != nil {
		return nil, fmt.Errorf("failed opening node file %s: %w", info.Name(), err)
	}
	nodeData, err := ioutil.ReadAll(nodeFile)
	if err != nil {
		return nil, fmt.Errorf("failed reading node file %s: %w", info.Name(), err)
	}
	node, err := forest.UnmarshalBinaryNode(nodeData)
	if err != nil {
		return nil, fmt.Errorf("failed parsing node file %s: %w", info.Name(), err)
	}
	return node, nil
}

// nodesFromInfo batch-converts a slice of file info into a slice of
// forest nodes by calling nodeFromInfo on each.
func (g *Grove) nodesFromInfo(info []os.FileInfo) ([]forest.Node, error) {
	nodes := make([]forest.Node, 0, len(info))
	for _, nodeFileInfo := range info {
		node, err := g.nodeFromInfo(nodeFileInfo)
		if err != nil {
			return nil, fmt.Errorf("failed transforming fileInfo into Node: %w", err)
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// allNodes returns a slice of every node in the grove.
func (g *Grove) allNodes() ([]forest.Node, error) {
	nodeInfo, err := g.getAllNodeFileInfo()
	if err != nil {
		return nil, fmt.Errorf("failed listing node file candidates: %w", err)
	}
	nodes, err := g.nodesFromInfo(nodeInfo)
	if err != nil {
		return nil, fmt.Errorf("failed converting node files into nodes: %w", err)
	}
	for _, node := range nodes {
		_ = g.NodeCache.Add(node)
	}
	return nodes, nil
}

// Children returns the IDs of all known child nodes of the specified ID.
// Any error opening, reading, or parsing files in the grove that occurs
// during the search for child nodes will cause the entire operation to
// error.
func (g *Grove) Children(id *fields.QualifiedHash) ([]*fields.QualifiedHash, error) {
	children, inCache := g.ChildCache.Get(id)
	if inCache {
		return children, nil
	}
	if err := g.RebuildChildCache(); err != nil {
		return nil, fmt.Errorf("failed rebuilding child cache: %w", err)
	}
	children, inCache = g.ChildCache.Get(id)
	if !inCache {
		return []*fields.QualifiedHash{}, nil
	}

	return children, nil
}

// Recent returns a slice of the most recently-created nodes of the given type.
// The slice is sorted so that the most-recently-created nodes are at the beginning.
func (g *Grove) Recent(nodeType fields.NodeType, quantity int) ([]forest.Node, error) {
	nodes, err := g.allNodes()
	if err != nil {
		return nil, fmt.Errorf("failed getting all nodes from grove: %w", err)
	}
	// TODO: find a cleaner way to sort nodes by time
	sort.Slice(nodes, func(i, j int) bool {
		var a, b forest.CommonNode
		switch n := nodes[i].(type) {
		case *forest.Identity:
			a = n.CommonNode
		case *forest.Community:
			a = n.CommonNode
		case *forest.Reply:
			a = n.CommonNode
		}
		switch n := nodes[j].(type) {
		case *forest.Identity:
			b = n.CommonNode
		case *forest.Community:
			b = n.CommonNode
		case *forest.Reply:
			b = n.CommonNode
		}
		return a.Created > b.Created
	})
	rightType := make([]forest.Node, 0, quantity)
	for _, node := range nodes {
		switch node.(type) {
		case *forest.Identity:
			if nodeType == fields.NodeTypeIdentity {
				rightType = append(rightType, node)
			}
		case *forest.Community:
			if nodeType == fields.NodeTypeCommunity {
				rightType = append(rightType, node)
			}
		case *forest.Reply:
			if nodeType == fields.NodeTypeReply {
				rightType = append(rightType, node)
			}
		}
	}
	if len(rightType) > quantity {
		rightType = rightType[:quantity]
	}
	return rightType, nil
}

// RebuildChildCache must be called each time a node is inserted into the
// underlying storage without actually calling Add() on the grove. Without
// this, calls to Children() will not always include new results.
func (g *Grove) RebuildChildCache() error {
	nodes, err := g.allNodes()
	if err != nil {
		return fmt.Errorf("failed getting all nodes from grove: %w", err)
	}
	for _, node := range nodes {
		g.ChildCache.Add(node.ParentID(), node.ID())
	}
	return nil
}

// Add inserts the node into the grove. If the given node is already in the
// grove, Add will do nothing. It is not an error to insert a node more than
// once.
func (g *Grove) Add(node forest.Node) error {
	g.ChildCache.Add(node.ParentID(), node.ID())
	if _, alreadyPresent, err := g.Get(node.ID()); err != nil {
		return fmt.Errorf("failed checking whether node already in grove: %w", err)
	} else if alreadyPresent {
		return nil
	}
	data, err := node.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to serialize node: %w", err)
	}

	id := node.ID().String()
	nodeFile, err := g.Create(id)
	if err != nil {
		return fmt.Errorf("failed to create file for node %s: %w", id, err)
	}
	defer nodeFile.Close()

	_, err = nodeFile.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write data to file for node %s: %w", id, err)
	}
	return nil
}

// GetIdentity returns an Identity node with the given ID (if it is present
// in the grove). This operation may be faster than using Get, as the grove
// may be able to do less search work when it knows the type of node you're
// looking for in advance.
//
// BUG(whereswaldon): The current implementation may return nodes of the
// wrong NodeType if they match the provided ID
func (g *Grove) GetIdentity(id *fields.QualifiedHash) (forest.Node, bool, error) {
	// this naiive implementation is not efficient, but works as a short-term
	// thing.
	//
	// TODO: change the on-disk representation so that operations like this can
	// be fast (store different node types in different directories, etc...)
	return g.Get(id)
}

// GetCommunity returns an Community node with the given ID (if it is present
// in the grove). This operation may be faster than using Get, as the grove
// may be able to do less search work when it knows the type of node you're
// looking for in advance.
//
// BUG(whereswaldon): The current implementation may return nodes of the
// wrong NodeType if they match the provided ID
func (g *Grove) GetCommunity(id *fields.QualifiedHash) (forest.Node, bool, error) {
	// this naiive implementation is not efficient, but works as a short-term
	// thing.
	//
	// TODO: change the on-disk representation so that operations like this can
	// be fast (store different node types in different directories, etc...)
	return g.Get(id)
}

// GetConversation returns an Conversation node with the given ID (if it is present
// in the grove). This operation may be faster than using Get, as the grove
// may be able to do less search work when it knows the type of node you're
// looking for and its parent node in advance.
//
// BUG(whereswaldon): The current implementation may return nodes of the
// wrong NodeType if they match the provided ID
func (g *Grove) GetConversation(communityID, conversationID *fields.QualifiedHash) (forest.Node, bool, error) {
	// this naiive implementation is not efficient, but works as a short-term
	// thing.
	//
	// TODO: change the on-disk representation so that operations like this can
	// be fast (store different node types in different directories, etc...)
	return g.Get(conversationID)
}

// GetReply returns an Reply node with the given ID (if it is present
// in the grove). This operation may be faster than using Get, as the grove
// may be able to do less search work when it knows the type of node you're
// looking for and its parent community and conversation node in advance.
//
// BUG(whereswaldon): The current implementation may return nodes of the
// wrong NodeType if they match the provided ID
func (g *Grove) GetReply(communityID, conversationID, replyID *fields.QualifiedHash) (forest.Node, bool, error) {
	// this naiive implementation is not efficient, but works as a short-term
	// thing.
	//
	// TODO: change the on-disk representation so that operations like this can
	// be fast (store different node types in different directories, etc...)
	return g.Get(replyID)
}

// CopyInto copies all nodes from the store into the provided store.
//
// BUG(whereswaldon): this method is not yet implemented. It requires
// more extensive file manipulation than other Grove methods (listing
// directory contents) and has therefore been deprioritized in favor
// of the functionality that can be implemented simply. However, it is
// implementable, and should be done as soon as is feasible.
func (g *Grove) CopyInto(other forest.Store) error {
	return fmt.Errorf("method CopyInto() is not currently implemented on Grove")
}
