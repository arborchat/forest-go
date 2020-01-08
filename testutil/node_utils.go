/*
Package testutil provides utilities for easily making test arbor nodes and
content.
*/
package testutil

import (
	"testing"

	"git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/fields"
	"git.sr.ht/~whereswaldon/forest-go/testkeys"
)

func MakeIdentityFromKeyOrSkip(t *testing.T, privKey, passphrase string) (*forest.Identity, forest.Signer) {
	signer := testkeys.Signer(t, privKey)
	identity, err := forest.NewIdentity(signer, "test-username", []byte{})
	if err != nil {
		t.Error("Failed to create Identity with valid parameters", err)
	}
	return identity, signer
}

func MakeIdentityOrSkip(t *testing.T) (*forest.Identity, forest.Signer) {
	return MakeIdentityFromKeyOrSkip(t, testkeys.PrivKey1, "")
}

func MakeCommunityOrSkip(t *testing.T) (*forest.Identity, forest.Signer, *forest.Community) {
	identity, privkey := MakeIdentityOrSkip(t)
	community, err := forest.As(identity, privkey).NewCommunity("test community", []byte{})
	if err != nil {
		t.Error("Failed to create Community with valid parameters", err)
	}
	return identity, privkey, community
}

func MakeReplyOrSkip(t *testing.T) (*forest.Identity, forest.Signer, *forest.Community, *forest.Reply) {
	identity, privkey, community := MakeCommunityOrSkip(t)
	reply, err := forest.As(identity, privkey).NewReply(community, "more test content", []byte{})
	if err != nil {
		t.Error("Failed to create reply with valid parameters", err)
	}
	return identity, privkey, community, reply
}

func RandomIdentity(t *testing.T) *forest.Identity {
	signer := testkeys.Signer(t, testkeys.PrivKey1)
	name := RandomString(12)
	id, err := forest.NewIdentity(signer, name, []byte{})
	if err != nil {
		t.Errorf("Failed to generate test identity: %v", err)
		return nil
	}
	return id
}

func RandomNodeSlice(length int, t *testing.T) ([]*fields.QualifiedHash, []forest.Node) {
	ids := make([]*fields.QualifiedHash, length)
	nodes := make([]forest.Node, length)
	for i := 0; i < length; i++ {
		identity := RandomIdentity(t)
		ids[i] = identity.ID()
		nodes[i] = identity
	}
	return ids, nodes
}
