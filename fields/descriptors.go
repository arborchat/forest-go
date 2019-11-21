package fields

import (
	"bytes"
	"encoding"
	"fmt"
	"strings"
)

const sizeofDescriptor = sizeofgenericType + sizeofContentLength

const descriptorTextSeparator = "_"

func marshalTextDescriptor(descriptorType encoding.TextMarshaler, length encoding.TextMarshaler) ([]byte, error) {
	buf := new(bytes.Buffer)
	b, err := descriptorType.MarshalText()
	if err != nil {
		return nil, err
	}
	_, _ = buf.Write(b)
	_, _ = buf.Write([]byte(descriptorTextSeparator))
	b, err = length.MarshalText()
	if err != nil {
		return nil, err
	}
	_, _ = buf.Write(b)
	return buf.Bytes(), nil
}

func unmarshalTextDelimited(b []byte, delimiter string, descriptorType, length encoding.TextUnmarshaler) error {
	parts := strings.Split(string(b), delimiter)
	if len(parts) < 2 {
		return fmt.Errorf("too few \"%s\"-delimited parts (expected %d, got %d)", descriptorTextSeparator, 2, len(parts))
	} else if len(parts) > 2 {
		// ensure that occurrences of our delimiter after the first one are ignored
		parts[1] = strings.Join(parts[1:], delimiter)
		parts = parts[:2]
	}

	if err := descriptorType.UnmarshalText([]byte(parts[0])); err != nil {
		return fmt.Errorf("failed unmarshaling descriptor type: %v", err)
	}
	if err := length.UnmarshalText([]byte(parts[1])); err != nil {
		return fmt.Errorf("failed unmarshaling descriptor length: %v", err)
	}
	return nil
}

// concrete descriptors
type HashDescriptor struct {
	Type   HashType      `arbor:"order=0"`
	Length ContentLength `arbor:"order=1"`
}

const sizeofHashDescriptor = sizeofDescriptor

func NewHashDescriptor(t HashType, length int) (*HashDescriptor, error) {
	cLength, err := NewContentLength(length)
	if err != nil {
		return nil, err
	}
	return &HashDescriptor{t, *cLength}, nil
}

func (d *HashDescriptor) Equals(other *HashDescriptor) bool {
	return d.Type.Equals(&other.Type) && d.Length.Equals(&other.Length)
}

func (d *HashDescriptor) MarshalText() ([]byte, error) {
	return marshalTextDescriptor(d.Type, d.Length)
}

func (d *HashDescriptor) UnmarshalText(b []byte) error {
	return unmarshalTextDelimited(b, descriptorTextSeparator, &d.Type, &d.Length)
}
func (d *HashDescriptor) Validate() error {
	validLengths, validType := ValidHashTypes[d.Type]
	if !validType {
		return fmt.Errorf("%d is not a valid hash type", d.Type)
	}
	found := false
	for _, length := range validLengths {
		if length == d.Length {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("%d is not a valid hash length for hash type %d", d.Length, d.Type)
	}
	return nil
}

type ContentDescriptor struct {
	Type   ContentType   `arbor:"order=0"`
	Length ContentLength `arbor:"order=1"`
}

const sizeofContentDescriptor = sizeofDescriptor

func NewContentDescriptor(t ContentType, length int) (*ContentDescriptor, error) {
	cLength, err := NewContentLength(length)
	if err != nil {
		return nil, err
	}
	return &ContentDescriptor{t, *cLength}, nil
}

func (d *ContentDescriptor) Equals(other *ContentDescriptor) bool {
	return d.Type.Equals(&other.Type) && d.Length.Equals(&other.Length)
}

func (d *ContentDescriptor) MarshalText() ([]byte, error) {
	return marshalTextDescriptor(d.Type, d.Length)
}

func (d *ContentDescriptor) Validate() error {
	_, validType := ValidContentTypes[d.Type]
	if !validType {
		return fmt.Errorf("%d is not a valid content type", d.Type)
	}
	return nil
}

type SignatureDescriptor struct {
	Type   SignatureType `arbor:"order=0"`
	Length ContentLength `arbor:"order=1"`
}

const sizeofSignatureDescriptor = sizeofDescriptor

func NewSignatureDescriptor(t SignatureType, length int) (*SignatureDescriptor, error) {
	cLength, err := NewContentLength(length)
	if err != nil {
		return nil, err
	}
	return &SignatureDescriptor{t, *cLength}, nil
}

func (d *SignatureDescriptor) Equals(other *SignatureDescriptor) bool {
	return d.Type.Equals(&other.Type) && d.Length.Equals(&other.Length)
}

func (d *SignatureDescriptor) MarshalText() ([]byte, error) {
	return marshalTextDescriptor(d.Type, d.Length)
}

func (d *SignatureDescriptor) Validate() error {
	_, validType := ValidSignatureTypes[d.Type]
	if !validType {
		return fmt.Errorf("%d is not a valid signature type", d.Type)
	}
	return nil
}

type KeyDescriptor struct {
	Type   KeyType       `arbor:"order=0"`
	Length ContentLength `arbor:"order=1"`
}

const sizeofKeyDescriptor = sizeofDescriptor

func NewKeyDescriptor(t KeyType, length int) (*KeyDescriptor, error) {
	cLength, err := NewContentLength(length)
	if err != nil {
		return nil, err
	}
	return &KeyDescriptor{t, *cLength}, nil
}

func (d *KeyDescriptor) Equals(other *KeyDescriptor) bool {
	return d.Type.Equals(&other.Type) && d.Length.Equals(&other.Length)
}

func (d *KeyDescriptor) MarshalText() ([]byte, error) {
	return marshalTextDescriptor(d.Type, d.Length)
}

func (d *KeyDescriptor) Validate() error {
	_, validType := ValidKeyTypes[d.Type]
	if !validType {
		return fmt.Errorf("%d is not a valid key type", d.Type)
	}
	return nil
}
