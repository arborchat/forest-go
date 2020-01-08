package twig

import (
	"bytes"
	"fmt"
)

type Key struct {
	Name    string
	Version uint
}

const keyFormat = "%s/%d"

func FromString(s string) (Key, error) {
	key := Key{}
	numRead, err := fmt.Scanf(keyFormat, &key.Name, &key.Version)
	if err != nil {
		return key, fmt.Errorf("failed scanning twig key %s: %w", s, err)
	} else if numRead < 2 {
		return key, fmt.Errorf("failed scanning twig key %s, only got %d values but needed %d", s, numRead, 2)
	}
	return key, nil
}

func (k Key) String() string {
	return fmt.Sprintf(keyFormat, k.Name, k.Version)
}

type Data struct {
	Values map[Key][]byte
}

func New() *Data {
	return &Data{Values: make(map[Key][]byte)}
}

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

func (d *Data) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	for key, value := range d.Values {
		buf.WriteString(key.String())
		buf.WriteByte(0)
		buf.Write(value)
		buf.WriteByte(0)
	}
	// hide the final NULL byte
	return buf.Bytes()[:buf.Len()-1], nil
}
