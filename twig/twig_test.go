package twig_test

import (
	"bytes"
	"testing"

	"git.sr.ht/~whereswaldon/forest-go/twig"
)

func TestKeys(t *testing.T) {
	table := []struct {
		CaseName  string
		KeyString string
		Valid     bool
		twig.Key
	}{
		{"empty key", "", false, twig.Key{}},
		{"no fields key", "/", false, twig.Key{}},
		{"only version key", "/0", false, twig.Key{}},
		{"only long version key", "/340", false, twig.Key{}},
		{"only name key", "a/", false, twig.Key{}},
		{"only long name key", "alongversion/", false, twig.Key{}},
		{"version and name key", "a/1", true, twig.Key{Name: "a", Version: 1}},
		{"long version and long name key", "alongversion/231", true, twig.Key{Name: "alongversion", Version: 231}},
		{"delimiter in name", "a/a/1", true, twig.Key{Name: "a/a", Version: 1}},
		{"delimiter in long name", "along/version/1", true, twig.Key{Name: "along/version", Version: 1}},
		{"whitespace in name", " /1", true, twig.Key{Name: " ", Version: 1}},
		{"whitespace in long name", "along \t\nversion/1", true, twig.Key{Name: "along \t\nversion", Version: 1}},
	}
	for _, row := range table {
		t.Run(row.CaseName, func(t *testing.T) {
			key, err := twig.FromString(row.KeyString)
			if row.Valid {
				if err != nil {
					t.Fatalf("should not have errored, got: %v", err)
				}
				if key.Name != row.Key.Name {
					t.Fatalf("expected name %s, got %s", row.Key.Name, key.Name)
				} else if key.Version != row.Key.Version {
					t.Fatalf("expected version %d, got %d", row.Key.Version, key.Version)
				}
				if row.KeyString != key.String() {
					t.Fatalf("expected key.String() to produce %s, got %s", row.KeyString, key.String())
				}
			} else {
				if err == nil {
					t.Fatalf("should have errored")
				}
			}
		})
	}
}

func TestDataMarshal(t *testing.T) {
	data := twig.New()
	data.Values[twig.Key{Name: "example", Version: 11}] = []byte("hello")
	data.Values[twig.Key{Name: "another", Version: 423}] = []byte("")
	asBin, err := data.MarshalBinary()
	if err != nil {
		t.Fatalf("Failed to marshal valid Data: %v", err)
	}
	data2 := twig.New()
	if err := data2.UnmarshalBinary(asBin); err != nil {
		t.Fatalf("Failed to unmarshal valid Data: %v", err)
	}
	for key, value := range data.Values {
		if !bytes.Equal(value, data2.Values[key]) {
			t.Fatalf("different data at key %s", key)
		}
	}
}

func TestDataMarshalBadKey(t *testing.T) {
	data := twig.New()
	data.Values[twig.Key{Name: "", Version: 423}] = []byte("")
	asBin, err := data.MarshalBinary()
	if err == nil {
		t.Fatalf("Should have failed to marshal illegal empty key")
	} else if asBin != nil {
		t.Fatalf("Should have returned nil slice when failing to marshal")
	}
}

func TestDataMarshalNoBytes(t *testing.T) {
	data := twig.New()
	asBin, err := data.MarshalBinary()
	if err != nil {
		t.Fatalf("Should not error on marshallign empty twig store: %v", nil)
	} else if len(asBin) != 0 {
		t.Fatalf("Empty data store should return an empty.")
	}
}

func TestDataSet(t *testing.T) {
	data := twig.New()
	var err error
	data, err = data.Set("foo", 1, []byte("bar"))
	if err != nil {
		t.Fatalf("failed to set legal key value pair")
	}
	data, err = data.Set("baz", 1, []byte{1, 2, 3, 4, 0, 5})
	if err == nil {
		t.Fatalf("successfully set illegal key value pair containing null byte")
	}
}
