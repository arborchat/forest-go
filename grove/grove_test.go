package grove_test

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	forest "git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/fields"
	"git.sr.ht/~whereswaldon/forest-go/grove"
	"git.sr.ht/~whereswaldon/forest-go/testkeys"
)

type truncatableFile interface {
	grove.File
	Truncate(size int64) error
}

// fakeFile implements the grove.File interface, but is entirely in-memory.
// This helps speed testing.
type fakeFile struct {
	data []byte
	*bytes.Buffer
	name    string
	mode    os.FileMode
	modtime time.Time
}

var _ os.FileInfo = &fakeFile{}
var _ truncatableFile = &fakeFile{}

func newFakeFile(name string, content []byte) *fakeFile {
	return &fakeFile{
		name:    name,
		mode:    os.FileMode(0660),
		modtime: time.Now(),
		data:    content,
		Buffer:  bytes.NewBuffer(content),
	}
}

func (f *fakeFile) Name() string {
	return f.name
}

func (f *fakeFile) Size() int64 {
	return int64(f.Buffer.Len())
}

func (f *fakeFile) Mode() os.FileMode {
	return f.mode
}

func (f *fakeFile) ModTime() time.Time {
	return f.modtime
}

func (f *fakeFile) IsDir() bool {
	return false
}

func (f *fakeFile) Sys() interface{} {
	return nil
}

// needed to implement Close so that fakeFile is a io.ReadWriteCloser
func (f *fakeFile) Close() error {
	return nil
}

func (f *fakeFile) Readdir(n int) ([]os.FileInfo, error) {
	return []os.FileInfo{}, nil
}

// ResetBuffer creates a new Buffer with the file's data. This is useful
// for ensuring that a given fakeFile can be read more than once. Calling
// this method effectively resets the contents of the file to be correct
// after the file has been read (reading the file will empty it).
func (f *fakeFile) ResetBuffer() {
	f.Buffer = bytes.NewBuffer(f.data)
}

// needed to implement truncatableFile
func (f *fakeFile) Truncate(size int64) error {
	f.Buffer.Truncate(int(size))
	f.modtime = time.Now()
	return nil
}

// errFile implements the grove.File interface and wraps another grove.File.
// If the errFile's error field is set to nil, it is a transparent wrapper
// for the underlying File. If the field is set to a non-nil error value,
// this will be returned from all operations that can return an error.
type errFile struct {
	error
	wrappedFile truncatableFile
}

var _ truncatableFile = &errFile{}
var _ os.FileInfo = &errFile{}

func NewErrFile(file truncatableFile) *errFile {
	return &errFile{
		wrappedFile: file,
	}
}

func (e *errFile) Name() string {
	return e.wrappedFile.Name()
}

func (e *errFile) Size() int64 {
	if fake, implements := e.wrappedFile.(os.FileInfo); implements {
		return fake.Size()
	}
	return 0
}

func (e *errFile) Mode() os.FileMode {
	if fake, implements := e.wrappedFile.(os.FileInfo); implements {
		return fake.Mode()
	}
	return os.FileMode(0660)
}

func (e *errFile) ModTime() time.Time {
	if fake, implements := e.wrappedFile.(os.FileInfo); implements {
		return fake.ModTime()
	}
	return time.Now()
}

func (e *errFile) IsDir() bool {
	if fake, implements := e.wrappedFile.(os.FileInfo); implements {
		return fake.IsDir()
	}
	return false
}

func (e *errFile) Sys() interface{} {
	if fake, implements := e.wrappedFile.(os.FileInfo); implements {
		return fake.Sys()
	}
	return nil
}

func (e *errFile) Read(b []byte) (int, error) {
	if e.error != nil {
		return 0, e.error
	}
	return e.wrappedFile.Read(b)
}

func (e *errFile) Write(b []byte) (int, error) {
	if e.error != nil {
		return 0, e.error
	}
	return e.wrappedFile.Write(b)
}

func (e *errFile) Close() error {
	if e.error != nil {
		return e.error
	}
	return e.wrappedFile.Close()
}

func (e *errFile) Readdir(n int) ([]os.FileInfo, error) {
	if e.error != nil {
		return nil, e.error
	}
	return e.wrappedFile.Readdir(n)
}

func (e *errFile) Truncate(size int64) error {
	if e.error != nil {
		return e.error
	}
	return e.wrappedFile.Truncate(size)
}

// fakeFS implements grove.FS, but is entirely in-memory.
type fakeFS struct {
	*bytes.Buffer
	files map[string]truncatableFile
}

var _ grove.FS = fakeFS{}

func newFakeFS() fakeFS {
	return fakeFS{
		files: make(map[string]truncatableFile),
	}
}

func (r fakeFS) Name() string {
	return ""
}

func (r fakeFS) Close() error {
	return nil
}

func (r fakeFS) Readdir(n int) ([]os.FileInfo, error) {
	count := n
	if count <= 0 {
		count = len(r.files)
	}
	info := make([]os.FileInfo, 0, count)
	for _, file := range r.files {
		info = append(info, file.(os.FileInfo))
	}
	return info, nil
}

// Open opens the given path as an absolute path relative to the root
// of the fakeFS
func (r fakeFS) Open(path string) (grove.File, error) {
	if path == "" {
		return r, nil
	}
	file, exists := r.files[path]
	if !exists {
		return nil, os.ErrNotExist
	}
	return file, nil
}

// Create makes the given path as an absolute path relative to the root
// of the fakeFS
func (r fakeFS) Create(path string) (grove.File, error) {
	// mimic os.Create(), so creating a file that already exists truncates
	// the current one
	file, exists := r.files[path]
	if exists {
		file.Truncate(0)
	} else {
		file = newFakeFile(path, []byte{})
		r.files[path] = file
	}

	return file, nil
}

// OpenFile opens the given path as an absolute path relative to the root
// of the fakeFS
func (r fakeFS) OpenFile(path string, flag int, perm os.FileMode) (grove.File, error) {
	return r.Open(path)
}

// errFS is a testing type that wraps an ordinary FS with the ability to
// return a specific error on any function call.
type errFS struct {
	fs grove.FS
	error
}

var _ grove.FS = errFS{}

func newErrFS(fs grove.FS) *errFS {
	return &errFS{
		fs: fs,
	}
}

// Open opens the given path as an absolute path relative to the root
// of the errFS
func (r errFS) Open(path string) (grove.File, error) {
	if r.error != nil {
		return nil, r.error
	}
	return r.fs.Open(path)
}

// Create makes the given path as an absolute path relative to the root
// of the errFS
func (r errFS) Create(path string) (grove.File, error) {
	if r.error != nil {
		return nil, r.error
	}
	return r.fs.Create(path)
}

// OpenFile opens the given path as an absolute path relative to the root
// of the errFS
func (r errFS) OpenFile(path string, flag int, perm os.FileMode) (grove.File, error) {
	if r.error != nil {
		return nil, r.error
	}
	return r.fs.OpenFile(path, flag, perm)
}

type testNodeBuilder struct {
	*testing.T
	*forest.Builder
	*forest.Community
}

func NewNodeBuilder(t *testing.T) *testNodeBuilder {
	signer := testkeys.Signer(t, testkeys.PrivKey1)
	id, err := forest.NewIdentity(signer, "node-builder", "")
	if err != nil {
		t.Errorf("Failed to create identity: %v", err)
		return nil
	}
	builder := forest.As(id, signer)
	community, err := builder.NewCommunity("nodes-built-for-testing", "")
	if err != nil {
		t.Errorf("Failed to create community: %v", err)
		return nil
	}
	return &testNodeBuilder{
		T:         t,
		Builder:   builder,
		Community: community,
	}
}

// newReplyFile creates a fakeFile that contains the binary data for a reply
// node that is a direct child of the given community and constructed by the
// given builder. It returns the reply node as a convenience for testing.
func (tnb *testNodeBuilder) newReplyFile(content string) (*forest.Reply, *fakeFile) {
	reply, err := tnb.NewReply(tnb.Community, content, "")
	if err != nil {
		tnb.T.Errorf("Failed generating test reply node: %v", err)
	}
	b, err := reply.MarshalBinary()
	if err != nil {
		tnb.T.Errorf("Failed marshalling test reply node: %v", err)
	}
	return reply, newFakeFile(reply.ID().String(), b)
}

func TestCreateEmptyGrove(t *testing.T) {
	fs := newFakeFS()
	grove, err := grove.NewWithFS(fs)
	if err != nil {
		t.Fatalf("Failed to create grove with fake fs: %v", err)
	}
	if grove == nil {
		t.Fatalf("Grove constructor did not err, but returned nil grove")
	}
}

func TestCreateGroveFromNil(t *testing.T) {
	_, err := grove.NewWithFS(nil)
	if err == nil {
		t.Fatalf("Created grove with nil fs, should have errored")
	}
}

func TestGroveGet(t *testing.T) {
	fs := newFakeFS()
	fakeNodeBuilder := NewNodeBuilder(t)
	reply, replyFile := fakeNodeBuilder.newReplyFile("test content")
	g, err := grove.NewWithFS(fs)
	if err != nil {
		t.Errorf("Failed constructing grove: %v", err)
	}

	// no nodes in fs, make sure we get nothing
	if node, present, err := g.Get(reply.ID()); err != nil {
		t.Errorf("Failed looking for %v (not present): %v", reply.ID(), err)
	} else if present {
		t.Errorf("Grove indicated that a node was present when it was not added")
	} else if node != nil {
		t.Errorf("Grove returned a node when the requested node was not present")
	}

	// add node to fs, now should be discoverable
	fs.files[replyFile.Name()] = replyFile

	// no nodes in fs, make sure we get nothing
	if node, present, err := g.Get(reply.ID()); err != nil {
		t.Errorf("Failed looking for %v (present): %v", reply.ID(), err)
	} else if !present {
		t.Errorf("Grove indicated that a node was not present when it should have been")
	} else if node == nil {
		t.Errorf("Grove did not return a node when the requested node was present")
	}
}

func TestGroveGetErrorReadingFile(t *testing.T) {
	fs := newFakeFS()
	fakeNodeBuilder := NewNodeBuilder(t)
	reply, replyFile := fakeNodeBuilder.newReplyFile("test content")
	errReplyFile := NewErrFile(replyFile)
	g, err := grove.NewWithFS(fs)
	if err != nil {
		t.Errorf("Failed constructing grove: %v", err)
	}

	// add node to fs, now should be discoverable
	fs.files[errReplyFile.Name()] = errReplyFile
	errReplyFile.error = os.ErrClosed

	// no nodes in fs, make sure we get nothing
	if node, present, err := g.Get(reply.ID()); err == nil {
		t.Errorf("Expected error reading file to be propagated upward, got nil")
	} else if present {
		t.Errorf("Grove indicated that a node was present when it could not be read")
	} else if node != nil {
		t.Errorf("Grove returned a node when the requested node was unreadable")
	}
}

func TestGroveGetErrorUnmarshallingFile(t *testing.T) {
	fs := newFakeFS()
	fakeNodeBuilder := NewNodeBuilder(t)
	reply, replyFile := fakeNodeBuilder.newReplyFile("test content")
	replyFile.Reset()
	_, err := replyFile.Write([]byte("this is not an arbor node"))
	if err != nil {
		t.Skipf("Unable to write test data into node file: %v", err)
	}
	g, err := grove.NewWithFS(fs)
	if err != nil {
		t.Errorf("Failed constructing grove: %v", err)
	}

	// add node to fs, now should be discoverable
	fs.files[replyFile.Name()] = replyFile

	// no nodes in fs, make sure we get nothing
	if node, present, err := g.Get(reply.ID()); err == nil {
		t.Errorf("Expected error unmarshalling file to be propagated upward, got nil")
	} else if present {
		t.Errorf("Grove indicated that a node was present when it could not be unmarshalled")
	} else if node != nil {
		t.Errorf("Grove returned a node when the requested node was unparsable")
	}
}

func TestGroveGetErrorOpeningFile(t *testing.T) {
	fs := newFakeFS()
	eFS := newErrFS(fs)
	fakeNodeBuilder := NewNodeBuilder(t)
	reply, _ := fakeNodeBuilder.newReplyFile("test content")
	g, err := grove.NewWithFS(eFS)
	if err != nil {
		t.Errorf("Failed constructing grove: %v", err)
	}
	eFS.error = os.ErrPermission

	// no nodes in fs, make sure we get nothing
	if node, present, err := g.Get(reply.ID()); err == nil {
		t.Errorf("Expected error accessing file to be propagated upward, got nil")
	} else if present {
		t.Errorf("Grove indicated that a node was present when it could not be opened")
	} else if node != nil {
		t.Errorf("Grove returned a node when the requested node was inaccessible")
	}
}

func TestGroveAdd(t *testing.T) {
	fs := newFakeFS()
	fakeNodeBuilder := NewNodeBuilder(t)
	reply, _ := fakeNodeBuilder.newReplyFile("test content")

	g, err := grove.NewWithFS(fs)
	if err != nil {
		t.Errorf("Failed constructing grove: %v", err)
	}

	if err := g.Add(reply); err != nil {
		t.Errorf("Expected Add() to succeed: %v", err)
	}
}

func TestGroveAddFailToWrite(t *testing.T) {
	fs := newFakeFS()
	fakeNodeBuilder := NewNodeBuilder(t)
	reply, replyFile := fakeNodeBuilder.newReplyFile("test content")
	eFile := NewErrFile(replyFile)

	g, err := grove.NewWithFS(fs)
	if err != nil {
		t.Errorf("Failed constructing grove: %v", err)
	}

	fs.files[eFile.Name()] = eFile
	eFile.error = os.ErrClosed

	if err := g.Add(reply); err == nil {
		t.Errorf("Expected Add() to fail when writing to file fails")
	}
}

func TestGroveAddShouldntTruncateExisting(t *testing.T) {
	fs := newFakeFS()
	fakeNodeBuilder := NewNodeBuilder(t)
	reply, replyFile := fakeNodeBuilder.newReplyFile("test content")
	fs.files[replyFile.Name()] = replyFile
	originalModTime := replyFile.ModTime()

	g, err := grove.NewWithFS(fs)
	if err != nil {
		t.Errorf("Failed constructing grove: %v", err)
	}

	if err := g.Add(reply); err != nil {
		t.Errorf("Failed adding node to grove which already contains it: %v", err)
	}

	if !replyFile.ModTime().Equal(originalModTime) {
		t.Errorf("Reply file should not have been modified by Add() operation, but modtime is different")
	}
}

func TestGroveAddFailToCreate(t *testing.T) {
	fs := newFakeFS()
	efs := newErrFS(fs)
	efs.error = os.ErrPermission
	fakeNodeBuilder := NewNodeBuilder(t)
	reply, _ := fakeNodeBuilder.newReplyFile("test content")
	g, err := grove.NewWithFS(efs)
	if err != nil {
		t.Errorf("Failed constructing grove: %v", err)
	}

	if err := g.Add(reply); err == nil {
		t.Errorf("Expected Add() to fail when creating file fails")
	}
}

type errNode struct {
	error
}

var _ forest.Node = errNode{}

func (e errNode) TreeDepth() fields.TreeDepth {
	return 0
}

func (e errNode) ID() *fields.QualifiedHash {
	return &fields.QualifiedHash{}
}

func (e errNode) ParentID() *fields.QualifiedHash {
	return &fields.QualifiedHash{}
}

func (e errNode) Equals(interface{}) bool {
	return false
}

func (e errNode) MarshalBinary() ([]byte, error) {
	return nil, e.error
}

func (e errNode) UnmarshalBinary([]byte) error {
	return e.error
}

func (e errNode) ValidateDeep(forest.Store) error {
	return e.error
}

func (e errNode) ValidateShallow() error {
	return e.error
}

func TestGroveAddFailToSerialize(t *testing.T) {
	fs := newFakeFS()
	efs := newErrFS(fs)
	efs.error = os.ErrPermission
	eNode := errNode{
		fmt.Errorf("I can't be serialized"),
	}
	g, err := grove.NewWithFS(efs)
	if err != nil {
		t.Errorf("Failed constructing grove: %v", err)
	}

	if err := g.Add(eNode); err == nil {
		t.Errorf("Expected Add() to fail when serializing node fails")
	}
}

func TestGroveChildren(t *testing.T) {
	fs := newFakeFS()
	fakeNodeBuilder := NewNodeBuilder(t)
	reply, replyFile := fakeNodeBuilder.newReplyFile("test content")
	_, replyFile1 := fakeNodeBuilder.newReplyFile("test content")
	_, replyFile2 := fakeNodeBuilder.newReplyFile("test content")
	g, err := grove.NewWithFS(fs)
	if err != nil {
		t.Errorf("Failed constructing grove: %v", err)
	}

	// add node to fs, now should be discoverable
	fs.files[replyFile.Name()] = replyFile

	identity := fakeNodeBuilder.Builder.User
	identityData, err := identity.MarshalBinary()
	idFileName, _ := identity.ID().MarshalString()
	idFile := newFakeFile(idFileName, identityData)
	fs.files[idFile.Name()] = idFile

	community := fakeNodeBuilder.Community
	communityData, err := community.MarshalBinary()
	communityFileName, _ := community.ID().MarshalString()
	communityFile := newFakeFile(communityFileName, communityData)
	fs.files[communityFile.Name()] = communityFile

	if children, err := g.Children(identity.ID()); err != nil {
		t.Errorf("Expected looking for identity children to succeed: %v", err)
	} else if len(children) > 0 {
		t.Errorf("Expected no child nodes for identity, found %d", len(children))
	}

	// reset fakeFiles so they can be read again
	replyFile.ResetBuffer()
	idFile.ResetBuffer()
	communityFile.ResetBuffer()

	if children, err := g.Children(community.ID()); err != nil {
		t.Errorf("Expected looking for community children to succeed: %v", err)
	} else if len(children) < 1 {
		t.Errorf("Expected child nodes for community, found none")
	} else if !children[0].Equals(reply.ID()) {
		t.Errorf("Expected child of community node to be reply node")
	}

	// reset fakeFiles so they can be read again
	replyFile.ResetBuffer()
	idFile.ResetBuffer()
	communityFile.ResetBuffer()

	fs.files[replyFile1.Name()] = replyFile1
	fs.files[replyFile2.Name()] = replyFile2

	if children, err := g.Children(community.ID()); err != nil {
		t.Errorf("Expected looking for community children to succeed: %v", err)
	} else if len(children) < 3 {
		t.Errorf("Expected 3 child nodes for community, found none")
	}
}

func TestGroveChildrenOpenRootFails(t *testing.T) {
	fs := newFakeFS()
	efs := newErrFS(fs)
	efs.error = os.ErrPermission
	fakeNodeBuilder := NewNodeBuilder(t)
	reply, _ := fakeNodeBuilder.newReplyFile("test content")
	g, err := grove.NewWithFS(efs)
	if err != nil {
		t.Errorf("Failed constructing grove: %v", err)
	}

	if children, err := g.Children(reply.ID()); err == nil {
		t.Errorf("Expected error opening root grove dir to cause Children() to fail, but did not error")
	} else if len(children) > 0 {
		t.Errorf("Expected no child nodes to be returned when opening root grove dir fails, but found %d", len(children))
	}
}

func TestGroveChildrenOpenNodeFails(t *testing.T) {
	fs := newFakeFS()
	fakeNodeBuilder := NewNodeBuilder(t)
	reply, replyFile := fakeNodeBuilder.newReplyFile("test content")
	eReplyFile := NewErrFile(replyFile)
	eReplyFile.error = os.ErrPermission
	g, err := grove.NewWithFS(fs)
	if err != nil {
		t.Errorf("Failed constructing grove: %v", err)
	}

	// add node to fs, now should be discoverable
	fs.files[eReplyFile.Name()] = eReplyFile

	if children, err := g.Children(reply.ID()); err == nil {
		t.Errorf("Expected permission error when reading node file to be propagated upward, but Children() did not error")
	} else if len(children) > 0 {
		t.Errorf("Expected no child nodes for when reading a node failed, found %d", len(children))
	}
}

func TestGroveChildrenParseNodeFails(t *testing.T) {
	fs := newFakeFS()
	fakeNodeBuilder := NewNodeBuilder(t)
	reply, replyFile := fakeNodeBuilder.newReplyFile("test content")
	replyFile.Buffer.Truncate(1)
	g, err := grove.NewWithFS(fs)
	if err != nil {
		t.Errorf("Failed constructing grove: %v", err)
	}

	// add node to fs, now should be discoverable
	fs.files[replyFile.Name()] = replyFile

	if children, err := g.Children(reply.ID()); err == nil {
		t.Errorf("Expected error when parsing node file to be propagated upward, but Children() did not error")
	} else if len(children) > 0 {
		t.Errorf("Expected no child nodes for when parsing a node failed, found %d", len(children))
	}
}

func TestGroveRecent(t *testing.T) {
	fs := newFakeFS()
	fakeNodeBuilder := NewNodeBuilder(t)
	reply, replyFile := fakeNodeBuilder.newReplyFile("test content")
	reply1, replyFile1 := fakeNodeBuilder.newReplyFile("test content")
	reply2, replyFile2 := fakeNodeBuilder.newReplyFile("test content")
	g, err := grove.NewWithFS(fs)
	if err != nil {
		t.Errorf("Failed constructing grove: %v", err)
	}

	// add node to fs, now should be discoverable
	fs.files[replyFile.Name()] = replyFile
	fs.files[replyFile1.Name()] = replyFile1
	fs.files[replyFile2.Name()] = replyFile2

	identity := fakeNodeBuilder.Builder.User
	identityData, err := identity.MarshalBinary()
	idFileName, _ := identity.ID().MarshalString()
	idFile := newFakeFile(idFileName, identityData)
	fs.files[idFile.Name()] = idFile

	community := fakeNodeBuilder.Community
	communityData, err := community.MarshalBinary()
	communityFileName, _ := community.ID().MarshalString()
	communityFile := newFakeFile(communityFileName, communityData)
	fs.files[communityFile.Name()] = communityFile

	if replies, err := g.Recent(fields.NodeTypeReply, 5); err != nil {
		t.Errorf("Expected recent replies to succeed: %v", err)
	} else if len(replies) != 3 {
		t.Errorf("Expected 3 replies, found %d", len(replies))
	} else {
		for i, r := range replies {
			asReply, isReply := r.(*forest.Reply)
			if !isReply {
				t.Errorf("Reply node is not actually a reply")
			}
			if i > 0 {
				lastReply := replies[i-1].(*forest.Reply)
				if lastReply.Created < asReply.Created {
					t.Errorf("Nodes are not in descending order of Created field")
				}

			}
			switch i {
			case 0:
				if !reply2.ID().Equals(r.ID()) {
					t.Errorf("First node returned should have been last one created")
				}
			case 1:
				if !reply1.ID().Equals(r.ID()) {
					t.Errorf("Second node created should have been second node returned")
				}
			case 2:
				if !reply.ID().Equals(r.ID()) {
					t.Errorf("Last node returned should have been the first one created")
				}
			}

		}
	}

	// reset fakeFiles so they can be read again
	resetAll := func() {
		replyFile.ResetBuffer()
		replyFile1.ResetBuffer()
		replyFile2.ResetBuffer()
		idFile.ResetBuffer()
		communityFile.ResetBuffer()
	}
	resetAll()

	if replies, err := g.Recent(fields.NodeTypeReply, 2); err != nil {
		t.Errorf("Did not expect valid query for recent replies to err: %v", err)
	} else if len(replies) != 2 {
		t.Errorf("Expected %d nodes, got %d", 2, len(replies))
	}

	resetAll()

	if ids, err := g.Recent(fields.NodeTypeIdentity, 2); err != nil {
		t.Errorf("Did not expect valid query for recent identities to err: %v", err)
	} else if len(ids) != 1 {
		t.Errorf("Expected %d nodes, got %d", 1, len(ids))
	} else {
		asIdentity, isIdentity := ids[0].(*forest.Identity)
		if !isIdentity {
			t.Errorf("Recent() request for identities returned a different node type")
		}
		if !asIdentity.ID().Equals(identity.ID()) {
			t.Errorf("Recent() didn't return the correct identity")
		}
	}

	resetAll()

	if communities, err := g.Recent(fields.NodeTypeCommunity, 2); err != nil {
		t.Errorf("Did not expect valid query for recent identities to err: %v", err)
	} else if len(communities) != 1 {
		t.Errorf("Expected %d nodes, got %d", 1, len(communities))
	} else {
		asCommunity, isCommunity := communities[0].(*forest.Community)
		if !isCommunity {
			t.Errorf("Recent() request for communities returned a different node type")
		}
		if !asCommunity.ID().Equals(community.ID()) {
			t.Errorf("Recent() didn't return the correct community")
		}
	}

}

func TestGroveRecentOpenNodeFails(t *testing.T) {
	fs := newFakeFS()
	fakeNodeBuilder := NewNodeBuilder(t)
	_, replyFile := fakeNodeBuilder.newReplyFile("test content")
	eReplyFile := NewErrFile(replyFile)
	eReplyFile.error = os.ErrPermission
	g, err := grove.NewWithFS(fs)
	if err != nil {
		t.Errorf("Failed constructing grove: %v", err)
	}

	// add node to fs, now should be discoverable
	fs.files[eReplyFile.Name()] = eReplyFile

	if replies, err := g.Recent(fields.NodeTypeReply, 5); err == nil {
		t.Errorf("Expected permission error when reading node file to be propagated upward, but Recent() did not error")
	} else if len(replies) > 0 {
		t.Errorf("Expected no recent nodes for when reading a node failed, found %d", len(replies))
	}
}
