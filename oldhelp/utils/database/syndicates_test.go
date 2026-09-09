package database

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"testing"
)

func mustSyndicateLinks(t *testing.T, db *sql.DB, itemType string) map[string][]string {
	t.Helper()
	rows, err := db.Query(`
		SELECT si.item_gameRef, s.name
		FROM syndicate_items si
		JOIN syndicates s ON s.id = si.syndicate_id
		WHERE si.item_type = ?
		ORDER BY si.item_gameRef, s.name
	`, itemType)
	if err != nil {
		t.Fatalf("query syndicate links error = %v", err)
	}
	defer rows.Close()

	got := map[string][]string{}
	for rows.Next() {
		var ref, name string
		if err := rows.Scan(&ref, &name); err != nil {
			t.Fatalf("rows.Scan() error = %v", err)
		}
		got[ref] = append(got[ref], name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err() = %v", err)
	}
	for ref := range got {
		sort.Strings(got[ref])
	}
	return got
}

func TestExtractSyndicateNames(t *testing.T) {
	item := map[string]any{
		"drops": []any{
			map[string]any{"chance": float64(100), "location": "Steel Meridian Offering"},
			map[string]any{"chance": float64(100), "location": "The Perrin Sequence Medallion"},
			map[string]any{"chance": float64(50), "location": "Red Veil Offering"},
			map[string]any{"chance": float64(100), "location": 123},
			"bad",
		},
	}
	got := extractSyndicateNames(item)
	want := []string{"Steel Meridian", "The Perrin Sequence"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("extractSyndicateNames() = %#v, want %#v", got, want)
	}
}

func TestSyncSyndicateItems(t *testing.T) {
	db := openTestDB(t)
	if err := EnsureSyndicates(db); err != nil {
		t.Fatalf("EnsureSyndicates() error = %v", err)
	}
	mustExec(t, db, `INSERT INTO syndicate_items(item_gameRef, syndicate_id, item_type) SELECT 'stale', id, 'mods' FROM syndicates WHERE name = 'Cephalon Suda'`)

	raw := []map[string]any{
		{
			"uniqueName": "/Lotus/Mods/TestModBlueprint",
			"drops": []any{
				map[string]any{"chance": float64(100), "location": "Cephalon Suda Offering"},
				map[string]any{"chance": float64(100), "location": "Red Veil Offering"},
			},
		},
		{
			"uniqueName": "/Lotus/Mods/IgnoreMe",
			"drops":      []any{map[string]any{"chance": float64(20), "location": "Steel Meridian Offering"}},
		},
		{},
	}

	if err := SyncSyndicateItems(context.Background(), db, raw, "mods"); err != nil {
		t.Fatalf("SyncSyndicateItems() error = %v", err)
	}

	got := mustSyndicateLinks(t, db, "mods")
	want := map[string][]string{
		"/Lotus/Mods/TestModBlueprint": {"Cephalon Suda", "Red Veil"},
		"/Lotus/Mods/TestModComponent": {"Cephalon Suda", "Red Veil"},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("syndicate links = %#v, want %#v", got, want)
	}
}

func TestEnsureSyndicatesDropsLegacyTable(t *testing.T) {
	db := openTestDB(t)
	mustExec(t, db, `CREATE TABLE syndicate_items (item_uniqueName TEXT, syndicate_id INTEGER, item_type TEXT)`)

	if err := EnsureSyndicates(db); err != nil {
		t.Fatalf("EnsureSyndicates() error = %v", err)
	}

	var colCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('syndicate_items') WHERE name = 'item_gameRef'`).Scan(&colCount); err != nil {
		t.Fatalf("pragma_table_info lookup error = %v", err)
	}
	if colCount != 1 {
		t.Fatalf("item_gameRef column count = %d, want 1", colCount)
	}
}
