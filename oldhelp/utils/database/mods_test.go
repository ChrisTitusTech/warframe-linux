package database

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEnrichMods(t *testing.T) {
	db := openTestDB(t)
	if err := EnsureSyndicates(db); err != nil {
		t.Fatalf("EnsureSyndicates() error = %v", err)
	}
	mustExec(t, db, `CREATE TABLE mods (gameRef TEXT PRIMARY KEY, slug TEXT NOT NULL, name TEXT NOT NULL, ducats INTEGER)`)
	mustExec(t, db, `INSERT INTO mods(gameRef, slug, name) VALUES (?, ?, ?)`, "/Lotus/Mods/TestModBlueprint", "test-mod", "Test Mod")
	mustExec(t, db, `INSERT INTO mods(gameRef, slug, name) VALUES (?, ?, ?)`, "/Lotus/Mods/TestModComponent", "test-mod-component", "Test Mod Component")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ref-1/data/json/Mods.json" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/ref-1/data/json/Mods.json")
		}
		_, _ = w.Write([]byte(`[
			{"uniqueName":"/Lotus/Mods/TestModBlueprint","drops":[{"chance":100,"location":"Cephalon Suda Offering"}]},
			{"uniqueName":"/Lotus/Mods/NoDrop","drops":[{"chance":25,"location":"Red Veil Offering"}]}
		]`))
	}))
	defer ts.Close()
	withDatabaseHTTPClient(t, ts.Client())
	withWFCDBaseURL(t, ts.URL+"/%s/data/json/%s")

	if err := EnrichMods(context.Background(), db, "ref-1"); err != nil {
		t.Fatalf("EnrichMods() error = %v", err)
	}

	got := mustSyndicateLinks(t, db, "mods")
	want := map[string][]string{
		"/Lotus/Mods/TestModBlueprint": {"Cephalon Suda"},
		"/Lotus/Mods/TestModComponent": {"Cephalon Suda"},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("mod links = %#v, want %#v", got, want)
	}
}
