package testutil

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"git.sr.ht/~whereswaldon/forest-go/fields"
)

func QualifiedContentOrSkip(t *testing.T, contentType fields.ContentType, content []byte) *fields.QualifiedContent {
	qContent, err := fields.NewQualifiedContent(contentType, content)
	if err != nil {
		t.Skip("Failed to qualify content", err)
	}
	return qContent
}

func RandomQualifiedHash() *fields.QualifiedHash {
	length := 32
	return &fields.QualifiedHash{
		Descriptor: fields.HashDescriptor{
			Type:   fields.HashTypeSHA512,
			Length: fields.ContentLength(length),
		},
		Blob: fields.Blob(RandomBytes(length)),
	}
}

func RandomQualifiedHashSlice(count int) []*fields.QualifiedHash {
	out := make([]*fields.QualifiedHash, count)
	for i := 0; i < count; i++ {
		out[i] = RandomQualifiedHash()
	}
	return out
}

func RandomBytes(length int) []byte {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return b
}

func RandomString(length int) string {
	return string(base64.StdEncoding.EncodeToString(RandomBytes(length)))
}
