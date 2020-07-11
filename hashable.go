package forest

import (
	"crypto/sha512"
	"encoding"
	"fmt"
	"hash"

	"git.sr.ht/~whereswaldon/forest-go/fields"
)

type Hashable interface {
	HashDescriptor() *fields.HashDescriptor
	encoding.BinaryMarshaler
}

// computeID determines the correct value of the ID of any hashable entity
func computeID(h Hashable) ([]byte, error) {
	// map from HashType and Length to the function that creates an instance of that hash
	// algorithm
	hashType2Func := map[fields.HashType]map[fields.ContentLength]func() hash.Hash{
		fields.HashTypeSHA512: map[fields.ContentLength]func() hash.Hash{
			fields.HashDigestLengthSHA512_256: sha512.New512_256,
		},
	}
	hd := h.HashDescriptor()
	if hd.Type == fields.HashTypeNullHash {
		return []byte{}, nil
	}
	binaryContent, err := h.MarshalBinary()
	if err != nil {
		return nil, err
	}
	hashCategory, found := hashType2Func[fields.HashType(hd.Type)]
	if !found {
		return nil, fmt.Errorf("Unknown HashType %d", hd.Type)
	}
	hashFunc, found := hashCategory[hd.Length]
	if !found {
		return nil, fmt.Errorf("Invalid hash length %d for hash type %d", hd.Length, hd.Type)
	}
	hasher := hashFunc()
	_, _ = hasher.Write(binaryContent) // never errors
	return hasher.Sum(nil), nil
}

// ValidateID returns whether the ID of this commonNode matches the data. The first
// return value indicates the result of the comparison. If there is an error,
// the first return value will always be false and the second will indicate
// what went wrong when computing the hash.
func ValidateID(h Hashable, expected fields.QualifiedHash) (bool, error) {
	id, err := computeID(h)
	if err != nil {
		return false, err
	}
	computedID := fields.QualifiedHash{
		Descriptor: *h.HashDescriptor(),
		Blob:       fields.Blob(id),
	}
	return expected.Equals(&computedID), nil
}
