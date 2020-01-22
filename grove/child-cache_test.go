package grove_test

import (
	"sort"
	"strings"
	"testing"

	"git.sr.ht/~whereswaldon/forest-go/grove"
	"git.sr.ht/~whereswaldon/forest-go/testutil"
)

func TestChildCache(t *testing.T) {
	ids := testutil.RandomQualifiedHashSlice(10)
	parent, children := ids[0], ids[1:]
	cache := grove.NewChildCache()
	childrenOut, inCache := cache.Get(parent)
	if inCache {
		t.Fatalf("%s should not be in cache before it is added", parent)
	}
	if len(childrenOut) > 0 {
		t.Fatalf("expected no children, found %d", len(childrenOut))
	}
	cache.Add(parent, children...)
	childrenOut, inCache = cache.Get(parent)
	if !inCache {
		t.Fatalf("%s should be in cache after it is added", parent)
	}
	if len(childrenOut) != len(children) {
		t.Fatalf("Expected %d children, got %d", len(children), len(childrenOut))
	}
	sort.Slice(children, func(i, j int) bool {
		return strings.Compare(children[i].String(), children[j].String()) < 0
	})
	sort.Slice(childrenOut, func(i, j int) bool {
		return strings.Compare(childrenOut[i].String(), childrenOut[j].String()) < 0
	})
	for i := range children {
		if !children[i].Equals(childrenOut[i]) {
			t.Fatalf("Child mismatch at element %d, %s != %s", i, children[i], childrenOut[i])
		}
	}
	childrenOut, inCache = cache.Get(children[0])
	if inCache {
		t.Fatalf("id added only as a child should not be a cache hit")
	}
	if len(childrenOut) > 0 {
		t.Fatalf("id added only as a child should not have children")
	}
}

func TestChildCacheAddDuplicate(t *testing.T) {
	parent := testutil.RandomQualifiedHash()
	id1 := testutil.RandomQualifiedHash()
	id2 := *id1 // copy the data

	cache := grove.NewChildCache()

	cache.Add(parent, id1)
	cache.Add(parent, &id2)
	results, hit := cache.Get(parent)
	if !hit {
		t.Fatalf("should have hit")
	}
	if len(results) != 1 {
		t.Fatalf("expected %d results, got %d", 1, len(results))
	}
}
