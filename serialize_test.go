package forest_test

import (
	"encoding/json"
	"reflect"
	"testing"

	forest "git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/serialize"
	"git.sr.ht/~whereswaldon/forest-go/testutil"
)

func TestNewArborSerializerSymmetry(t *testing.T) {
	identity, _, community, reply := testutil.MakeReplyOrSkip(t)
	identity2, _, community2, reply2 := testutil.MakeReplyOrSkip(t)
	nodes := []forest.Node{identity, community, reply}
	outNodes := []forest.Node{identity2, community2, reply2}
	for i, node := range nodes {
		node2 := outNodes[i]
		data, err := serialize.ArborSerialize(reflect.ValueOf(node))
		if err != nil {
			t.Errorf("Failed to serialize tagged node: %s", err)
		}
		excess, err := serialize.ArborDeserialize(reflect.ValueOf(node2), data)
		if err != nil {
			t.Errorf("Failed to deserialize tagged node: %s", err)
		}
		if len(excess) != 0 {
			t.Errorf("Expected 0 bytes of excess data, got %d", len(excess))
		}
		if !node.Equals(node2) {
			json1, _ := json.Marshal(node)
			json2, _ := json.Marshal(node2)
			t.Errorf("Expected %s and %s to be equal", json1, json2)
		}
	}
}
