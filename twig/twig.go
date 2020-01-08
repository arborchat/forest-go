/*
Package twig implements the twig key-value data format.

Twig is a simple text key-value format. Keys and values are separated by NULL
bytes (bytes of value 0). Keys and values may not contain a NULL byte. In values,
all other characters are allowed. In keys, all characters other than the keyDelimiter (currently '/') and the NULL byte  are allowed.

Keys have an additional constraint. Each key must contain a "name" and a "version"
number. These describe the semantics of the data stored for that key, and the
precise meaning is left to the user.

The key name may not be empty.

In practice, twig keys look like:

    anexample/235
    heres one with spaces/9
*/
package twig

import (
	"bytes"
	"fmt"
	"strings"
)

// Key represents a key within the twig data
type Key struct {
	Name    string
	Version uint
}

const keyDelimiter = "/"
const keyVersionFormat = "%d"
const keyFormat = "%s" + keyDelimiter + keyVersionFormat

// FromString converts a string into a Key struct by separating the name and version
func FromString(s string) (Key, error) {
	key := Key{}
	parts := strings.SplitN(s, keyDelimiter, 2)
	if len(parts) < 2 {
		return key, fmt.Errorf("failed reading key, need 2 values, got %d", len(parts))
	}
	key.Name = parts[0]
	if len(key.Name) < 1 {
		return key, fmt.Errorf("key name must be non-empty: %s", s)
	}
	numRead, err := fmt.Sscanf(parts[1], keyVersionFormat, &key.Version)
	if err != nil {
		return key, fmt.Errorf("failed scanning twig key %s: %w", s, err)
	} else if numRead < 1 {
		return key, fmt.Errorf("failed scanning twig key %s, only got %d values but needed %d", s, numRead, 1)
	}
	return key, nil
}

// String converts a key back into its string representation
func (k Key) String() string {
	return fmt.Sprintf(keyFormat, k.Name, k.Version)
}

// Data represents a collection of twig key-value pairs. It provides
// methods for converting them to/from binary
type Data struct {
	Values map[Key][]byte
}

// New allocates an empty twig key-value Data
func New() *Data {
	return &Data{Values: make(map[Key][]byte)}
}

// UnmarshalBinary populates a Data from raw binary in Twig format
func (d *Data) UnmarshalBinary(b []byte) error {
	components := bytes.Split(b, []byte{0})
	if len(components)%2 != 0 {
		return fmt.Errorf("key with no value")
	}
	for i := 0; i < len(components); i += 2 {
		key, err := FromString(string(components[i]))
		if err != nil {
			return fmt.Errorf("failed parsing key: %w", err)
		}
		d.Values[key] = components[i+1]
	}
	return nil
}

// MarshalBinary converts this Data into twig binary form.
func (d *Data) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	for key, value := range d.Values {
		// gotta check here because the Values map is exported and could be
		// modified underneath us
		if len(key.Name) == 0 {
			return nil, fmt.Errorf("twig key cannot have empty name")
		}
		buf.WriteString(key.String())
		buf.WriteByte(0)
		buf.Write(value)
		buf.WriteByte(0)
	}
	// hide the final NULL byte
	return buf.Bytes()[:buf.Len()-1], nil
}
