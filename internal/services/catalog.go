package services

// Item is a display-only fixture, not a provider or inventory record.
type Item struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

type Catalog struct {
	items []Item
}

func NewMockCatalog() *Catalog {
	return &Catalog{items: []Item{
		{ID: "mock:frame", Name: "Example Warframe", Category: "Warframe"},
		{ID: "mock:weapon", Name: "Example Rifle", Category: "Primary weapon"},
		{ID: "mock:relic", Name: "Example Relic", Category: "Relic"},
	}}
}

// List returns a copy so callers cannot modify the shared fixture.
func (c *Catalog) List() []Item {
	return append([]Item{}, c.items...)
}
