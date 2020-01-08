package forest_test

import (
	"testing"

	forest "git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/fields"
	"git.sr.ht/~whereswaldon/forest-go/testkeys"
	"git.sr.ht/~whereswaldon/forest-go/testutil"
)

func TestIdentityNewline(t *testing.T) {
	signer := testkeys.Signer(t, testkeys.PrivKey1)
	_, err := forest.NewIdentity(signer, "newline-in\nusername", []byte{})
	if err == nil {
		t.Error("Failed to error with newline in username")
	}
}

func TestIdentityValidatesSelf(t *testing.T) {
	identity, _ := testutil.MakeIdentityOrSkip(t)
	if correct, err := forest.ValidateID(identity, *identity.ID()); err != nil || !correct {
		t.Error("ID validation failed on unmodified node", err)
	}
	if correct, err := forest.ValidateSignature(identity, identity); err != nil || !correct {
		t.Error("Signature validation failed on unmodified node", err)
	}
}

func TestIdentityValidationFailsWhenTampered(t *testing.T) {
	identity, _ := testutil.MakeIdentityOrSkip(t)
	identity.Name.Blob = fields.Blob([]byte("whatever"))
	if correct, err := forest.ValidateID(identity, *identity.ID()); err == nil && correct {
		t.Error("ID validation succeeded on modified node", err)
	}
	if correct, err := forest.ValidateSignature(identity, identity); err == nil && correct {
		t.Error("Signature validation succeeded on modified node", err)
	}
}

func TestIdentitySerialize(t *testing.T) {
	identity, _ := testutil.MakeIdentityOrSkip(t)
	buf, err := identity.MarshalBinary()
	if err != nil {
		t.Error("Failed to serialize identity", err)
	}
	id2, err := forest.UnmarshalIdentity(buf)
	if err != nil {
		t.Error("Failed to deserialize identity", err)
	}
	if !identity.Equals(id2) {
		t.Errorf("Deserialized identity should be the same as what went in, expected %v, got %v", identity, id2)
	}
}
