package store_test

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"testing"

	"git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/fields"
	"git.sr.ht/~whereswaldon/forest-go/store"
	"git.sr.ht/~whereswaldon/forest-go/testutil"
)

func prep(t *testing.T) (s forest.Store, root *fields.QualifiedHash, ids []*fields.QualifiedHash) {
	id, signer, community, reply := testutil.MakeReplyOrSkip(t)
	builder := forest.Builder{
		User:   id,
		Signer: signer,
	}

	s = store.NewMemoryStore()
	s.Add(id)

	nodes := []forest.Node{community, reply}
	for i := 0; i < 10; i++ {
		parent := nodes[rand.Intn(len(nodes))]
		n, err := builder.NewReply(parent, strconv.Itoa(i), []byte{})
		if err != nil {
			t.Errorf("failed generating test node: %v", err)
		}
		nodes = append(nodes, n)
	}
	ids = []*fields.QualifiedHash{}
	for _, node := range nodes {
		ids = append(ids, node.ID())
		s.Add(node)
	}

	return s, community.ID(), ids
}

func TestWalk(t *testing.T) {
	s, root, ids := prep(t)

	reachedIDs := []*fields.QualifiedHash{}
	store.Walk(s, root, func(id *fields.QualifiedHash) error {
		reachedIDs = append(reachedIDs, id)
		return nil
	})

	sortIds(ids)
	sortIds(reachedIDs)
	if !sameIds(ids, reachedIDs) {
		t.Errorf("failed to reach all nodes in walk: expected %v, got %v", ids, reachedIDs)
	}
	t.Log(ids)
	t.Log(reachedIDs)
}

func TestWalkTerminate(t *testing.T) {
	s, root, _ := prep(t)

	count := 0
	stop := fmt.Errorf("stop")
	reachedIDs := []*fields.QualifiedHash{}
	if err := store.Walk(s, root, func(id *fields.QualifiedHash) error {
		if count >= 5 {
			return stop
		}
		count++
		reachedIDs = append(reachedIDs, id)
		return nil
	}); err != nil {
		if !errors.Is(err, stop) {
			t.Errorf("should have returned wrapped stop error")
		}
	}

	if len(reachedIDs) != count {
		t.Errorf("visited an unexpected number of nodes, expected %d, visited %d", count, len(reachedIDs))
	}
}

func sortIds(ids []*fields.QualifiedHash) {
	sort.Slice(ids, func(i, j int) bool {
		return strings.Compare(ids[i].String(), ids[j].String()) < 0
	})
}

func sameIds(a, b []*fields.QualifiedHash) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Equals(b[i]) {
			return false
		}
	}
	return true
}

func TestWalkNils(t *testing.T) {
	_, _, community, _ := testutil.MakeReplyOrSkip(t)
	s := store.NewMemoryStore()
	if err := store.Walk(nil, community.ID(), func(*fields.QualifiedHash) error {
		return nil
	}); err == nil {
		t.Errorf("Walk should error with nil store")
	}
	if err := store.Walk(s, nil, func(*fields.QualifiedHash) error {
		return nil
	}); err == nil {
		t.Errorf("Walk should error with root")
	}
	if err := store.Walk(s, community.ID(), nil); err == nil {
		t.Errorf("Walk should error with nil visitor")
	}
}
