package forest_test

import (
	"bytes"
	"testing"

	forest "git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/fields"
	"golang.org/x/crypto/openpgp"
)

func getKey(privKey, passphrase string) (*openpgp.Entity, error) {
	privKeyBuf := bytes.NewBuffer([]byte(privKey))
	keyEntities, err := openpgp.ReadArmoredKeyRing(privKeyBuf)
	if err != nil {
		return nil, err
	}

	keyEntity := keyEntities[0]

	if keyEntity.PrivateKey.Encrypted {
		keyEntity.PrivateKey.Decrypt([]byte(passphrase))
	}

	return keyEntity, nil
}

func MakeIdentityFromKeyOrSkip(t *testing.T, privKey, passphrase string) (*forest.Identity, forest.Signer) {
	privkey, err := getKey(privKey, passphrase)
	if err != nil {
		t.Skip("Failed to create private key", err)
	}
	signer, err := forest.NewNativeSigner(privkey)
	identity, err := forest.NewIdentity(signer, "test-username", "")
	if err != nil {
		t.Error("Failed to create Identity with valid parameters", err)
	}
	return identity, signer
}

func MakeIdentityOrSkip(t *testing.T) (*forest.Identity, forest.Signer) {
	return MakeIdentityFromKeyOrSkip(t, privKey1, testKeyPassphrase)
}

func TestIdentityValidatesSelf(t *testing.T) {
	identity, _ := MakeIdentityOrSkip(t)
	if correct, err := forest.ValidateID(identity, *identity.ID()); err != nil || !correct {
		t.Error("ID validation failed on unmodified node", err)
	}
	if correct, err := forest.ValidateSignature(identity, identity); err != nil || !correct {
		t.Error("Signature validation failed on unmodified node", err)
	}
}

func TestIdentityValidationFailsWhenTampered(t *testing.T) {
	identity, _ := MakeIdentityOrSkip(t)
	identity.Name.Blob = fields.Blob([]byte("whatever"))
	if correct, err := forest.ValidateID(identity, *identity.ID()); err == nil && correct {
		t.Error("ID validation succeeded on modified node", err)
	}
	if correct, err := forest.ValidateSignature(identity, identity); err == nil && correct {
		t.Error("Signature validation succeeded on modified node", err)
	}
}

func TestIdentitySerialize(t *testing.T) {
	identity, _ := MakeIdentityOrSkip(t)
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

const testKeyPassphrase = "pleasedonotusethisasapassword"
const privKey1 = `-----BEGIN PGP PRIVATE KEY BLOCK-----

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
LPMVs+cyXrCxG6HDCOQ=
=x5Z7
-----END PGP PRIVATE KEY BLOCK-----`

const privKey2 = `-----BEGIN PGP PRIVATE KEY BLOCK-----

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
gDeRQt5NvnI2Qn1RA10=
=Ui/x
-----END PGP PRIVATE KEY BLOCK-----`
