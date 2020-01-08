package twig_test

import (
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
		{"only name key", "a/", false, twig.Key{}},
		{"version and name key", "a/1", true, twig.Key{Name: "a", Version: 1}},
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
			} else {
				if err == nil {
					t.Fatalf("should have errored")
				}
			}
		})
	}
}
