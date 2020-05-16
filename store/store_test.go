package store_test

import (
	"testing"

	forest "git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/fields"
	"git.sr.ht/~whereswaldon/forest-go/store"
	"git.sr.ht/~whereswaldon/forest-go/testutil"
)

func TestMemoryStore(t *testing.T) {
	s := store.NewMemoryStore()
	testStandardStoreInterface(t, s, "MemoryStore")
}

func testStandardStoreInterface(t *testing.T, s forest.Store, storeImplName string) {
	// create three test nodes, one of each type
	identity, _, community, reply := testutil.MakeReplyOrSkip(t)
	nodes := []forest.Node{identity, community, reply}

	// create a set of functions that perform different "Get" operations on nodes
	getFuncs := map[string]func(*fields.QualifiedHash) (forest.Node, bool, error){
		"get":       s.Get,
		"identity":  s.GetIdentity,
		"community": s.GetCommunity,
		"conversation": func(id *fields.QualifiedHash) (forest.Node, bool, error) {
			return s.GetConversation(community.ID(), id)
		},
		"reply": func(id *fields.QualifiedHash) (forest.Node, bool, error) {
			return s.GetReply(community.ID(), reply.ID(), id)
		},
	}

	// ensure no getter functions succeed on an empty store
	for _, i := range nodes {
		for _, get := range getFuncs {
			if node, has, err := get(i.ID()); has {
				t.Errorf("Empty %s should not contain element %v", storeImplName, i.ID())
			} else if err != nil {
				t.Errorf("Empty %s Get() should not err with %s", storeImplName, err)
			} else if node != nil {
				t.Errorf("Empty %s Get() should return none-nil node %v", storeImplName, node)
			}
		}
	}

	// add each node
	for _, i := range nodes {
		if err := s.Add(i); err != nil {
			t.Errorf("%s Add() should not err on Add(): %s", storeImplName, err)
		}
	}

	// map each node to the getters that should be successful in fetching it
	nodesToGetters := []struct {
		forest.Node
		funcs []string
	}{
		{identity, []string{"get", "identity"}},
		{community, []string{"get", "community"}},
		{reply, []string{"get", "conversation", "reply"}},
	}

	// ensure all getters work for each node
	for _, getterConfig := range nodesToGetters {
		currentNode := getterConfig.Node
		for _, getterName := range getterConfig.funcs {
			if node, has, err := getFuncs[getterName](currentNode.ID()); !has {
				t.Errorf("%s should contain element %v", storeImplName, currentNode.ID())
			} else if err != nil {
				t.Errorf("%s Has() should not err with %s", storeImplName, err)
			} else if !currentNode.Equals(node) {
				t.Errorf("%s Get() should return a node equal to the one that was Add()ed. Got %v, expected %v", storeImplName, node, currentNode)
			}
		}
	}

	// map nodes to the children that they ought to have within the store
	nodesToChildren := []struct {
		forest.Node
		children []*fields.QualifiedHash
	}{
		{identity, []*fields.QualifiedHash{}},
		{community, []*fields.QualifiedHash{reply.ID()}},
		{reply, []*fields.QualifiedHash{}},
	}

	// check each node has its proper children
	for _, childConfig := range nodesToChildren {
		if children, err := s.Children(childConfig.ID()); err != nil {
			t.Errorf("%s should not error fetching children of %v", storeImplName, childConfig.ID())
		} else {
			for _, child := range childConfig.children {
				if !containsID(children, child) {
					t.Errorf("%s should have %v as a child of %v", storeImplName, child, childConfig.ID())
				}
			}
		}
	}

	// add some more nodes so that we can test the Recent method
	identity2, _, community2, reply2 := testutil.MakeReplyOrSkip(t)
	for _, i := range []forest.Node{identity2, community2, reply2} {
		if err := s.Add(i); err != nil {
			t.Errorf("%s Add() should not err on Add(): %s", storeImplName, err)
		}
	}
	// try recent on each node type and ensure that it returns the right
	// number and order of results
	type recentRun struct {
		fields.NodeType
		atZero forest.Node
		atOne  forest.Node
	}
	for _, run := range []recentRun{
		{fields.NodeTypeIdentity, identity2, identity},
		{fields.NodeTypeCommunity, community2, community},
		{fields.NodeTypeReply, reply2, reply},
	} {
		recentNodes, err := s.Recent(run.NodeType, 1)
		if err != nil {
			t.Errorf("Recent failed on valid input: %v", err)
		} else if len(recentNodes) < 1 {
			t.Errorf("Recent on store with data returned too few results")
		} else if !recentNodes[0].Equals(run.atZero) {
			t.Errorf("Expected most recent node to be the newly created one")
		}
		recentNodes, err = s.Recent(run.NodeType, 2)
		if err != nil {
			t.Errorf("Recent failed on valid input: %v", err)
		} else if len(recentNodes) < 2 {
			t.Errorf("Recent on store with data returned too few results")
		} else if !recentNodes[0].Equals(run.atZero) {
			t.Errorf("Expected most recent node to be the newly created one")
		} else if !recentNodes[1].Equals(run.atOne) {
			t.Errorf("Expected first node to be the older one")
		}
		recentNodes, err = s.Recent(run.NodeType, 3)
		if err != nil {
			t.Errorf("Recent failed on valid input: %v", err)
		} else if len(recentNodes) > 2 {
			t.Errorf("Recent on store with only two matching nodes returned more than 2 results")
		}
	}
}

func containsID(ids []*fields.QualifiedHash, id *fields.QualifiedHash) bool {
	for _, current := range ids {
		if current.Equals(id) {
			return true
		}
	}
	return false
}

func TestCacheStore(t *testing.T) {
	s1 := store.NewMemoryStore()
	s2 := store.NewMemoryStore()
	c, err := store.NewCacheStore(s1, s2)
	if err != nil {
		t.Errorf("Unexpected error constructing CacheStore: %v", err)
	}
	testStandardStoreInterface(t, c, "CacheStore")
}

func TestCacheStoreDownPropagation(t *testing.T) {
	s1 := store.NewMemoryStore()
	id, _, com, rep := testutil.MakeReplyOrSkip(t)
	nodes := []forest.Node{id, com, rep}
	subrange := nodes[:len(nodes)-1]
	for _, node := range subrange {
		if err := s1.Add(node); err != nil {
			t.Skipf("Failed adding %v to %v", node, s1)
		}
	}
	s2 := store.NewMemoryStore()
	if _, err := store.NewCacheStore(s1, s2); err != nil {
		t.Errorf("Unexpected error when constructing CacheStore: %v", err)
	}

	for _, node := range subrange {
		if n2, has, err := s2.Get(node.ID()); err != nil {
			t.Errorf("Unexpected error getting node from cache base layer: %s", err)
		} else if !has {
			t.Errorf("Expected cache base layer to contain %v", node.ID())
		} else if !n2.Equals(node) {
			t.Errorf("Expected cache base layer to contain the same value for ID %v", node.ID())
		}
	}
}

func TestCacheStoreUpPropagation(t *testing.T) {
	base := store.NewMemoryStore()
	id, _, com, rep := testutil.MakeReplyOrSkip(t)
	nodes := []forest.Node{id, com, rep}
	subrange := nodes[:len(nodes)-1]
	for _, node := range subrange {
		if err := base.Add(node); err != nil {
			t.Skipf("Failed adding %v to %v", node, base)
		}
	}
	cache := store.NewMemoryStore()
	combined, err := store.NewCacheStore(cache, base)
	if err != nil {
		t.Errorf("Unexpected error when constructing CacheStore: %v", err)
	}

	for _, node := range subrange {
		if _, has, err := cache.Get(node.ID()); err != nil {
			t.Errorf("Unexpected error getting node from cache layer: %s", err)
		} else if has {
			t.Errorf("Expected cache layer not to contain %v", node.ID())
		}
		if n2, has, err := combined.Get(node.ID()); err != nil {
			t.Errorf("Unexpected error getting node from cache store: %s", err)
		} else if !has {
			t.Errorf("Expected cache store to contain %v", node.ID())
		} else if !n2.Equals(node) {
			t.Errorf("Expected cache store to contain the same value for ID %v", node.ID())
		}
		if n2, has, err := cache.Get(node.ID()); err != nil {
			t.Errorf("Unexpected error getting node from cache layer: %s", err)
		} else if !has {
			t.Errorf("Expected cache layer to contain %v after warming cache", node.ID())
		} else if !n2.Equals(node) {
			t.Errorf("Expected cache layer to contain the same value for ID %v after warming cache", node.ID())
		}
	}
}
