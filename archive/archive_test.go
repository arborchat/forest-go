package archive_test

import (
	"testing"

	"git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/archive"
	"git.sr.ht/~whereswaldon/forest-go/fields"
	"git.sr.ht/~whereswaldon/forest-go/testkeys"
)

type setup struct {
	identity, community, r1, r2, r3a, r3b forest.Node
}

func testStore(t *testing.T) (forest.Store, setup) {
	store := forest.NewMemoryStore()
	setup := setup{}
	signer := testkeys.Signer(t, testkeys.PrivKey1)
	identity, err := forest.NewIdentity(signer, "archivetest", []byte{})
	if err != nil {
		t.Fatalf("failed making test identity: %v", err)
	}
	store.Add(identity)
	setup.identity = identity
	builder := forest.As(identity, signer)
	community, err := builder.NewCommunity("archivetest1", []byte{})
	if err != nil {
		t.Fatalf("failed making test community: %v", err)
	}
	store.Add(community)
	setup.community = community
	setup.r1, err = builder.NewReply(community, "r1", []byte{})
	if err != nil {
		t.Fatalf("failing creating reply: %v", err)
	}
	store.Add(setup.r1)
	setup.r2, err = builder.NewReply(setup.r1, "r2", []byte{})
	if err != nil {
		t.Fatalf("failing creating reply: %v", err)
	}
	store.Add(setup.r2)
	setup.r3a, err = builder.NewReply(setup.r2, "r3a", []byte{})
	if err != nil {
		t.Fatalf("failing creating reply: %v", err)
	}
	store.Add(setup.r3a)
	setup.r3b, err = builder.NewReply(setup.r2, "r3b", []byte{})
	if err != nil {
		t.Fatalf("failing creating reply: %v", err)
	}
	store.Add(setup.r3b)
	return store, setup
}

func TestAncestryOf(t *testing.T) {
	store, setup := testStore(t)
	arch := archive.New(store)
	ancestry, err := arch.AncestryOf(setup.r3a.ID())
	if err != nil {
		t.Fatalf("failed fetching ancestry of %s: %v", setup.r3a.ID(), err)
	}
	r3aAncestors := 3
	if len(ancestry) != r3aAncestors {
		t.Fatalf("expected %d nodes of history for %s, got %d", r3aAncestors, setup.r3a.ID(), len(ancestry))
	}
	switch {
	case !ancestry[0].Equals(setup.r2.ID()):
		fallthrough
	case !ancestry[1].Equals(setup.r1.ID()):
		fallthrough
	case !ancestry[2].Equals(setup.community.ID()):
		t.Fatalf("incorrect ancestry: %v", ancestry)
	}
}

func contains(ids []*fields.QualifiedHash, id *fields.QualifiedHash) bool {
	for _, element := range ids {
		if element.Equals(id) {
			return true
		}
	}
	return false
}

func TestDescendantsOf(t *testing.T) {
	store, setup := testStore(t)
	arch := archive.New(store)
	descendants, err := arch.DescendantsOf(setup.r1.ID())
	if err != nil {
		t.Fatalf("failed fetching descendants of %s: %v", setup.r1.ID(), err)
	}
	switch {
	case contains(descendants, setup.community.ID()):
		t.Fatalf("community is not descendant of conversation")
	case contains(descendants, setup.identity.ID()):
		t.Fatalf("identity is not descendant of conversation")
	case contains(descendants, setup.r1.ID()):
		t.Fatalf("r1 is not a descendant of itself")
	case !contains(descendants, setup.r2.ID()):
		fallthrough
	case !contains(descendants, setup.r3a.ID()):
		fallthrough
	case !contains(descendants, setup.r3b.ID()):
		t.Fatalf("descendants is missing nodes")
	}
}

func TestLeavesOf(t *testing.T) {
	store, setup := testStore(t)
	arch := archive.New(store)
	leaves, err := arch.LeavesOf(setup.r1.ID())
	if err != nil {
		t.Fatalf("failed fetching leaves of %s: %v", setup.r1.ID(), err)
	}
	switch {
	case contains(leaves, setup.community.ID()):
		t.Fatalf("community is not descendant of conversation")
	case contains(leaves, setup.identity.ID()):
		t.Fatalf("identity is not descendant of conversation")
	case contains(leaves, setup.r1.ID()):
		t.Fatalf("r1 is not a leaf of itself")
	case contains(leaves, setup.r2.ID()):
		t.Fatalf("r2 is not a leaf of r1")
	case !contains(leaves, setup.r3a.ID()):
		fallthrough
	case !contains(leaves, setup.r3b.ID()):
		t.Fatalf("leaves is missing nodes")
	}
}
