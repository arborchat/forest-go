package fields_test

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"testing"
	"time"

	"git.sr.ht/~whereswaldon/forest-go/fields"
	"git.sr.ht/~whereswaldon/forest-go/twig"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/packet"
)

func TestQualifiedContent(t *testing.T) {
	validTwig := twig.New()
	validTwig.Values[twig.Key{Name: "key", Version: 1}] = []byte("value")
	validTwig.Values[twig.Key{Name: "key2", Version: 99}] = []byte("value2")
	validTwigBytes, _ := validTwig.MarshalBinary()
	inputs := []struct {
		Content     fields.QualifiedContent
		Name        string
		ShouldError bool
	}{
		{
			Content: fields.QualifiedContent{
				Descriptor: fields.ContentDescriptor{
					Type:   fields.ContentTypeUTF8String,
					Length: 8,
				},
				Blob: []byte("example!"),
			},
			Name:        "valid utf8",
			ShouldError: false,
		},
		{
			Content: fields.QualifiedContent{
				Descriptor: fields.ContentDescriptor{
					Type:   fields.ContentTypeUTF8String,
					Length: 8,
				},
				Blob: []byte("exampl"),
			},
			Name:        "too short utf8",
			ShouldError: true,
		},
		{
			Content: fields.QualifiedContent{
				Descriptor: fields.ContentDescriptor{
					Type:   fields.ContentTypeUTF8String,
					Length: 2,
				},
				Blob: []byte("exampl"),
			},
			Name:        "too long utf8",
			ShouldError: true,
		},
		{
			Content: fields.QualifiedContent{
				Descriptor: fields.ContentDescriptor{
					Type:   fields.ContentTypeUTF8String,
					Length: 2,
				},
				Blob: []byte{0xff, 0xfe, 0xfd},
			},
			Name:        "invalid utf8 bytes",
			ShouldError: true,
		},
		{
			Content: fields.QualifiedContent{
				Descriptor: fields.ContentDescriptor{
					Type:   0,
					Length: 2,
				},
				Blob: []byte("hello"),
			},
			Name:        "undefined content type",
			ShouldError: true,
		},
		{
			Content: fields.QualifiedContent{
				Descriptor: fields.ContentDescriptor{
					Type:   fields.ContentTypeTwig,
					Length: fields.ContentLength(len(validTwigBytes)),
				},
				Blob: validTwigBytes,
			},
			Name:        "valid twig",
			ShouldError: false,
		},
		{
			Content: fields.QualifiedContent{
				Descriptor: fields.ContentDescriptor{
					Type:   fields.ContentTypeTwig,
					Length: fields.ContentLength(len(validTwigBytes)) + 1,
				},
				Blob: append(validTwigBytes, 0),
			},
			Name:        "invalid twig",
			ShouldError: true,
		},
		{
			Content: fields.QualifiedContent{
				Descriptor: fields.ContentDescriptor{
					Type:   fields.ContentTypeTwig,
					Length: fields.ContentLength(0),
				},
				Blob: []byte{},
			},
			Name:        "valid empty twig",
			ShouldError: false,
		},
	}

	for _, row := range inputs {
		t.Run(row.Name, func(t *testing.T) {
			if err := row.Content.Validate(); err != nil && !row.ShouldError {
				t.Fatalf("should not have errored: %v", err)
			} else if err == nil && row.ShouldError {
				t.Fatalf("should have errored: %v", err)
			} else if err != nil {
				t.Logf("Recieved expected error: %v", err)
			}
		})
	}
}

func newECDSAKey() *packet.PrivateKey {
	ecdsaKey, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	pgpEcdsaKey := packet.NewECDSAPrivateKey(time.Now(), ecdsaKey)
	return pgpEcdsaKey
}

func TestQualifiedKey(t *testing.T) {
	rsaKey, _ := openpgp.NewEntity("testkey", "", "", nil)
	buf := new(bytes.Buffer)
	rsaKey.Serialize(buf)
	rsaKeyBytes := buf.Bytes()
	pgpEcdsaKey := newECDSAKey()
	buf2 := new(bytes.Buffer)
	pgpEcdsaKey.PublicKey.Serialize(buf2)
	pgpEcdsaKeyBytes := buf2.Bytes()
	inputs := []struct {
		Content     fields.QualifiedKey
		Name        string
		ShouldError bool
	}{
		{
			Content: fields.QualifiedKey{
				Descriptor: fields.KeyDescriptor{
					Type:   fields.KeyTypeOpenPGPRSA,
					Length: fields.ContentLength(len(rsaKeyBytes)),
				},
				Blob: rsaKeyBytes,
			},
			Name:        "valid openpgp RSA key",
			ShouldError: false,
		},
		{
			Content: fields.QualifiedKey{
				Descriptor: fields.KeyDescriptor{
					Type:   fields.KeyTypeOpenPGPRSA,
					Length: fields.ContentLength(len(pgpEcdsaKeyBytes)),
				},
				Blob: pgpEcdsaKeyBytes,
			},
			Name:        "invalid openpgp RSA key (is ECDSA key)",
			ShouldError: true,
		},
	}

	for _, row := range inputs {
		t.Run(row.Name, func(t *testing.T) {
			if err := row.Content.Validate(); err != nil && !row.ShouldError {
				t.Fatalf("should have errored")
			} else if err == nil && row.ShouldError {
				t.Fatalf("should not have errored")
			} else if err != nil {
				t.Logf("Recieved expected error: %v", err)
			}

		})
	}
}

func TestQualifiedSignature(t *testing.T) {
	signingData := "I should be signed"
	// make an RSA signature to test with
	rsaKey, _ := openpgp.NewEntity("testkey", "", "", nil)
	privRSAKey := rsaKey.PrivateKey
	rsaSignature := new(packet.Signature)
	rsaSignature.Hash = crypto.SHA256
	rsaSignature.PubKeyAlgo = packet.PubKeyAlgoRSA
	rsaHash := sha256.New()
	rsaHash.Write([]byte(signingData))
	if err := rsaSignature.Sign(rsaHash, privRSAKey, nil); err != nil {
		t.Fatalf("failed to sign RSA test data: %v", err)
	}
	rsaBuf := new(bytes.Buffer)
	rsaSignature.Serialize(rsaBuf)
	rsaSignatureBytes := rsaBuf.Bytes()

	// make an ECDSA signature to test with
	ecdsaKey := newECDSAKey()
	ecdsaSignature := new(packet.Signature)
	ecdsaSignature.Hash = crypto.SHA256
	ecdsaSignature.PubKeyAlgo = packet.PubKeyAlgoECDSA
	hash := sha256.New()
	hash.Write([]byte(signingData))
	if err := ecdsaSignature.Sign(hash, ecdsaKey, nil); err != nil {
		t.Fatalf("Failed to ECDSA sign test data: %v", err)
	}
	buf := new(bytes.Buffer)
	ecdsaSignature.Serialize(buf)
	ecdsaSignatureBytes := buf.Bytes()

	inputs := []struct {
		Content     fields.QualifiedSignature
		Name        string
		ShouldError bool
	}{
		{
			Content: fields.QualifiedSignature{
				Descriptor: fields.SignatureDescriptor{
					Type:   fields.SignatureTypeOpenPGPRSA,
					Length: fields.ContentLength(len(ecdsaSignatureBytes)),
				},
				Blob: ecdsaSignatureBytes,
			},
			Name:        "invalid openpgp RSA sig (is ECDSA sig)",
			ShouldError: true,
		},
		{
			Content: fields.QualifiedSignature{
				Descriptor: fields.SignatureDescriptor{
					Type:   fields.SignatureTypeOpenPGPRSA,
					Length: fields.ContentLength(len(rsaSignatureBytes)),
				},
				Blob: rsaSignatureBytes,
			},
			Name:        "valid openpgp RSA sig",
			ShouldError: false,
		},
	}

	for _, row := range inputs {
		t.Run(row.Name, func(t *testing.T) {
			if err := row.Content.Validate(); err != nil && !row.ShouldError {
				t.Fatalf("should have errored")
			} else if err == nil && row.ShouldError {
				t.Fatalf("should not have errored")
			} else {
				t.Logf("Received expected error: %v", err)
			}
		})
	}
}
