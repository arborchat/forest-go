package fields_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
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
	}

	for _, row := range inputs {
		t.Run(row.Name, func(t *testing.T) {
			if err := row.Content.Validate(); err != nil && !row.ShouldError {
				t.Fatalf("should have errored")
			} else if err == nil && row.ShouldError {
				t.Fatalf("should not have errored")
			}
		})
	}
}
func TestQualifiedKey(t *testing.T) {
	rsaKey, _ := openpgp.NewEntity("testkey", "", "", nil)
	buf := new(bytes.Buffer)
	rsaKey.Serialize(buf)
	rsaKeyBytes := buf.Bytes()
	ecdsaKey, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	pgpEcdsaKey := packet.NewECDSAPublicKey(time.Now(), &ecdsaKey.PublicKey)
	buf2 := new(bytes.Buffer)
	pgpEcdsaKey.Serialize(buf2)
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
			}
		})
	}
}
