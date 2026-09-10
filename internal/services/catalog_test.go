package services

import "testing"

func TestListIsIsolatedBetweenCallsAndInstances(t *testing.T) {
	catalog := NewMockCatalog()
	items := catalog.List()
	original := items[0]
	items[0].Name = "changed by caller"
	for _, source := range []*Catalog{catalog, NewMockCatalog()} {
		if got := source.List()[0]; got != original {
			t.Fatalf("caller mutated catalog: got %+v, want %+v", got, original)
		}
	}
}
