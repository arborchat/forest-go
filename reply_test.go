package forest_test

import (
	"testing"

	forest "git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/fields"
	"git.sr.ht/~whereswaldon/forest-go/testkeys"
	"git.sr.ht/~whereswaldon/forest-go/testutil"
)

func TestNewReply(t *testing.T) {
	identity, privkey, community := testutil.MakeCommunityOrSkip(t)
	reply, err := forest.As(identity, privkey).NewReply(community, "test content", []byte{})
	if err != nil {
		t.Error("Failed to create reply with valid parameters", err)
	}
	if !reply.Parent.Equals(community.ID()) {
		t.Error("Root Reply's parent is not parent community")
	} else if !reply.ConversationID.Equals(fields.NullHash()) {
		t.Error("Root Reply's conversation is not null hash")
	} else if !reply.CommunityID.Equals(community.ID()) {
		t.Error("Root Reply's community is not owning community")
	}
}

func getReplyToReplyOrFail(t *testing.T) (identity1, identity2 *forest.Identity, reply1, reply2 *forest.Reply, community *forest.Community) {
	var privkey forest.Signer
	var err error
	identity1, privkey, community, reply1 = testutil.MakeReplyOrSkip(t)
	identity2, privkey = testutil.MakeIdentityFromKeyOrSkip(t, testkeys.PrivKey1, "")
	reply2, err = forest.As(identity2, privkey).NewReply(reply1, "other test content", []byte{})
	if err != nil {
		t.Error("Failed to create reply with valid parameters", err)
	}
	return identity1, identity2, reply1, reply2, community
}

func TestNewReplyToReply(t *testing.T) {
	_, _, reply, reply2, community := getReplyToReplyOrFail(t)

	if !reply2.Parent.Equals(reply.ID()) {
		t.Error("Reply's parent is not parent conversation")
	} else if !reply2.ConversationID.Equals(reply.ID()) {
		t.Error("Reply's conversation is not parent conversation")
	} else if !reply2.CommunityID.Equals(community.ID()) {
		t.Error("Reply's community is not owning community")
	}
}

func TestReplyValidatesSelf(t *testing.T) {
	identity, _, _, reply := testutil.MakeReplyOrSkip(t)
	validateReply(t, identity, reply)
}

func failToValidateReply(t *testing.T, author *forest.Identity, reply *forest.Reply) {
	if correct, err := forest.ValidateID(reply, *reply.ID()); err == nil && correct {
		t.Error("ID validation succeded on modified node", err)
	}
	if correct, err := forest.ValidateSignature(reply, author); err == nil && correct {
		t.Error("Signature validation succeded on modified node", err)
	}
}

func validateReply(t *testing.T, author *forest.Identity, reply *forest.Reply) {
	if correct, err := forest.ValidateID(reply, *reply.ID()); err != nil || !correct {
		t.Error("ID validation failed on unmodified node", err)
	}
	if correct, err := forest.ValidateSignature(reply, author); err != nil || !correct {
		t.Error("Signature validation failed on unmodified node", err)
	}
}

func TestReplyValidationFailsWhenTampered(t *testing.T) {
	identity, _, _, reply := testutil.MakeReplyOrSkip(t)
	reply.Content.Blob = fields.Blob([]byte("whatever"))
	failToValidateReply(t, identity, reply)
}

func ensureSerializes(t *testing.T, reply *forest.Reply) {
	buf, err := reply.MarshalBinary()
	if err != nil {
		t.Error("Failed to serialize identity", err)
	}
	c2, err := forest.UnmarshalReply(buf)
	if err != nil {
		t.Error("Failed to deserialize identity", err)
	}
	if !reply.Equals(c2) {
		t.Errorf("Deserialized identity should be the same as what went in, expected %v, got %v", reply, c2)
	}
}

func TestReplySerializes(t *testing.T) {
	_, _, _, reply := testutil.MakeReplyOrSkip(t)
	ensureSerializes(t, reply)
}

func TestReplyToReplyValidates(t *testing.T) {
	_, identity2, _, r2, _ := getReplyToReplyOrFail(t)
	validateReply(t, identity2, r2)
}

func TestReplyToReplyFailsWhenTampered(t *testing.T) {
	_, identity2, _, r2, _ := getReplyToReplyOrFail(t)
	r2.Content.Blob = fields.Blob([]byte("else"))
	failToValidateReply(t, identity2, r2)
}

func TestReplyToReplySerializes(t *testing.T) {
	_, _, _, r2, _ := getReplyToReplyOrFail(t)
	ensureSerializes(t, r2)
}
