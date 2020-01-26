package serialize_test

import (
	"reflect"
	"testing"

	"git.sr.ht/~whereswaldon/forest-go/serialize"
)

type broken struct{}

func (b broken) BytesConsumed() int {
	return 100
}

func (b broken) UnmarshalBinary(buf []byte) error {
	return nil
}

var _ serialize.ProgressiveBinaryUnmarshaler = broken{}

// This is a regression test for the problem captured here:
// https://todo.sr.ht/~whereswaldon/arbor-dev/67
func TestSerializeBoundsCheck(t *testing.T) {
	buf := [10]byte{}
	b := struct {
		B broken `arbor:"order=0"`
	}{broken{}}
	// this (pre-fix) triggered a panic because the BytesConsumed method returns more bytes
	// than exist in the input data.
	if _, err := serialize.ArborDeserialize(reflect.ValueOf(b), buf[:]); err == nil {
		t.Fatalf("a broken implementation of ProgressiveBinaryUnmarshaler should cause an error, not be ignored or panic")
	}
}
