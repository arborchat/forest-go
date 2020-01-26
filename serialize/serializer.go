package serialize

import (
	"bytes"
	"encoding"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// ProgressiveBinaryUnmarshaler is a type that fully describes how to unmarshal itself
// from a stream of bytes.
type ProgressiveBinaryUnmarshaler interface {
	encoding.BinaryUnmarshaler
	// BytesConsumed can be called after UnmarshalBinary to determine how many bytes of the input to
	// UnmarshalBinary were consumed in the creation of this type.
	BytesConsumed() int
}

/*
Tag-based serialization algorithm

- iterate fields of given struct searching for the `arbor` tag.
  - if field is tagged, parse tags for these comma-delimited fields:
    - "order=n": where n is an integer. For a given struct, all of these tags must have unique values. This is the sort order that will be applied when serializing and deserializing the fields.
    - "recurse=string": where string is one of always, never, serialize, deserialize. The default is "never". This field controls whether the serialization logic will descend into a struct field or will rely on that field's existing {Un,}MarshalBinary implementation.
    - "signature": this indicates that the field is a signature and should be skipped when serializing for the purpose of signing the data
  - sort fields by tag order
  - on each tag (ordered):
    - if tagged "recurse", recurse on struct and use returned binary
    - else if implments encoding.BinaryMarshaler, use that
*/

// recurseOption represents a specific strategy for serializing a field within a struct that is also a struct.
type recurseOption uint8

const (
	recurseNever recurseOption = iota
	recurseAlways
	recurseSerialize
	recurseDeserialize

	orderPrefix             = "order"
	recursePrefix           = "recurse"
	recurseValueAlways      = "always"
	recurseValueNever       = "never"
	recurseValueSerialize   = "serialize"
	recurseValueDeserialize = "deserialize"
	signaturePrefix         = "signature"
)

// define a type for the important info about a field
type serialEntry struct {
	// a reflect.Value that holds an implementation of a marshaling interface or a struct (if recurse is true)
	value reflect.Value
	// when should the field be handled by a recursive serilalization approach
	recurse recurseOption
	// whether the value is a signature field that should be skipped when
	// serializing unsigned data
	signature bool
	// the 0-based order in which this field should be serialized relative
	// to the other fields in the containing struct
	order int
}

// satisfyChecker checks that the given value implements a specific interface
type satisfyChecker func(reflect.Value) bool

func ensureIsEncodingBinaryMarshaler(in reflect.Value) bool {
	_, ok := in.Interface().(encoding.BinaryMarshaler)
	return ok
}

func ensureIsProgressiveBinaryUnmarshaler(in reflect.Value) bool {
	_, ok := in.Interface().(ProgressiveBinaryUnmarshaler)
	return ok
}

// ensure that the given reflect.Value implements a specific interface (checks using the
// test function). If it does not, try a pointer to it. If that fails, error.
func ensureSatisfies(field reflect.Value, satisfies satisfyChecker) (reflect.Value, error) {
	if !satisfies(field) {
		if !field.CanAddr() {
			return field, fmt.Errorf("Value does not implement encoding.BinaryMarshaler, and cannot take address")
		}
		// see whether a pointer to the field satisfies the interface
		if !satisfies(field.Addr()) {
			return field, fmt.Errorf("Neither value not pointer to value implement encoding.BinaryMarshaler")
		}
		field = field.Addr()
	}
	return field, nil
}

// transform a field into a populated serialEntry describing how it should
// be {de,}serialized.
func getEntry(field reflect.Value, tag string) (*serialEntry, error) {
	tagFields := strings.Split(tag, ",")
	entry := &serialEntry{value: field}
	for _, element := range tagFields {
		switch {
		case strings.HasPrefix(element, orderPrefix):
			parts := strings.Split(element, "=")
			entry.order, _ = strconv.Atoi(parts[1])
		case strings.HasPrefix(element, recursePrefix):
			parts := strings.Split(element, "=")
			switch parts[1] {
			case recurseValueAlways:
				entry.recurse = recurseAlways
			case recurseValueSerialize:
				entry.recurse = recurseSerialize
			case recurseValueDeserialize:
				entry.recurse = recurseDeserialize
			case recurseValueNever:
				fallthrough
			default:
				// default to never
				entry.recurse = recurseNever
			}
		case strings.HasPrefix(element, signaturePrefix):
			entry.signature = true
		}
	}
	return entry, nil
}

// convert the given reflect.Value (of a struct) into a slice of serialEntry
// structs describing how to {de,serialize} its fields. The interfaceTest function is
// used to ensure that the reflect.Value elements satisfy a specific interface.
func getSerializationFields(value reflect.Value) ([]*serialEntry, error) {
	const arborTag = "arbor"
	// dereference a pointer if we've been given one
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	// ensure input is a struct
	if value.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected a struct, got Kind %d", value.Kind())
	}

	// find all of the relevant fields in this struct and extract
	// their information into a slice
	structFields := make([]*serialEntry, value.NumField())
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		tag, present := value.Type().Field(i).Tag.Lookup(arborTag)
		if !present {
			continue // skip if untagged
		}
		entry, err := getEntry(field, tag)
		if err != nil {
			return nil, err
		}
		structFields[entry.order] = entry
	}
	return structFields, nil
}

// SerializationConfig configures how a node is serialized
type SerializationConfig struct {
	SkipSignatures bool
}

// ArborSerialize serializes the given reflect.Value (corresponding to a struct) into
// binary with the default configuration.
func ArborSerialize(value reflect.Value) ([]byte, error) {
	return ArborSerializeConfig(value, SerializationConfig{
		SkipSignatures: false,
	})
}

// ArborSerializeConfig serializes the given reflect.Value (corresponding to a struct) into
// binary with the provided configuration.
func ArborSerializeConfig(value reflect.Value, config SerializationConfig) ([]byte, error) {
	fields, err := getSerializationFields(value)
	if err != nil {
		return nil, err
	}
	// serialize all fields in the order specified by their tags
	var serialized bytes.Buffer
	for _, field := range fields {
		if field == nil {
			break
		}
		if field.signature && config.SkipSignatures {
			continue
		}
		if field.recurse == recurseAlways || field.recurse == recurseSerialize {
			data, err := ArborSerializeConfig(field.value, config)
			if err != nil {
				return nil, err
			}
			_, err = serialized.Write(data)
			if err != nil {
				return nil, err
			}
			continue
		}
		// ensure supports Marshaling
		field.value, err = ensureSatisfies(field.value, ensureIsEncodingBinaryMarshaler)
		if err != nil {
			return nil, err
		}
		marshaler, ok := field.value.Interface().(encoding.BinaryMarshaler)
		if !ok {
			return nil, fmt.Errorf("Tagged non-recursive field does not implement encoding.BinaryMarshaler")
		}
		data, err := marshaler.MarshalBinary()
		if err != nil {
			return nil, err
		}
		_, err = serialized.Write(data)
		if err != nil {
			return nil, err
		}
	}
	return serialized.Bytes(), nil
}

// ArborDeserialize unpacks the given bytes into the given reflect.Value
// (corresponding to a struct). It returns any bytes that were not needed
// to deserialize the struct.
func ArborDeserialize(value reflect.Value, data []byte) (unused []byte, err error) {
	structEntries, err := getSerializationFields(value)
	if err != nil {
		return nil, err
	}
	// serialize all fields in the order specified by their tags
	for _, field := range structEntries {
		if field == nil {
			break
		}
		if field.recurse == recurseAlways || field.recurse == recurseDeserialize {
			data, err = ArborDeserialize(field.value, data)
			if err != nil {
				return nil, err
			}
			continue
		}
		// ensure supports Unmarshaling
		field.value, err = ensureSatisfies(field.value, ensureIsProgressiveBinaryUnmarshaler)
		if err != nil {
			return nil, err
		}
		unmarshaler, ok := field.value.Interface().(ProgressiveBinaryUnmarshaler)
		if !ok {
			return nil, fmt.Errorf("Tagged non-recursive field does not implement ProgressiveBinaryUnmarshaler")
		}
		err := unmarshaler.UnmarshalBinary(data)
		if err != nil {
			return nil, err
		}
		bytesConsumed := unmarshaler.BytesConsumed()
		if bytesConsumed > len(data) {
			return nil, fmt.Errorf("field %v.BytesConsumed() returned %d, but only %d bytes in slice", field.value, bytesConsumed, len(data))
		}
		data = data[bytesConsumed:]
	}
	return data, nil
}
