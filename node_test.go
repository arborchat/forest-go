package forest_test

import (
	"testing"

	forest "git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/testutil"
)

func TestUnmarshalNode(t *testing.T) {
	id, _, community, reply := testutil.MakeReplyOrSkip(t)
	for _, node := range []forest.Node{id, community, reply} {
		bin, err := node.MarshalBinary()
		if err != nil {
			t.Skip("Failed to marshal node into binary", err)
		}
		out, err := forest.UnmarshalBinaryNode(bin)
		if err != nil {
			t.Errorf("Failed to unmarshal valid binary node: %v", err)
		}
		if !out.Equals(node) {
			t.Errorf("Unmarshaled node is not the same as original")
		}
	}
}
