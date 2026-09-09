package database

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEnrichWeapons(t *testing.T) {
	db := openTestDB(t)
	if err := EnsureSyndicates(db); err != nil {
		t.Fatalf("EnsureSyndicates() error = %v", err)
	}
	mustExec(t, db, `CREATE TABLE weapons (gameRef TEXT PRIMARY KEY, slug TEXT NOT NULL, name TEXT NOT NULL, ducats INTEGER)`)
	for _, ref := range []string{"/Lotus/Weapons/TestBlueprint", "/Lotus/Weapons/TestComponent", "/Lotus/Weapons/Secondary"} {
		mustExec(t, db, `INSERT INTO weapons(gameRef, slug, name) VALUES (?, ?, ?)`, ref, strings.TrimPrefix(ref, "/Lotus/Weapons/"), ref)
	}

	responses := map[string]string{
		"/ref-3/data/json/Primary.json":    `[{"uniqueName":"/Lotus/Weapons/TestBlueprint","drops":[{"chance":100,"location":"Steel Meridian Offering"}],"components":[{"tradable":true,"ducats":65,"uniqueName":"/Lotus/Weapons/TestBlueprint"}]}]`,
		"/ref-3/data/json/Secondary.json":  `[{"uniqueName":"/Lotus/Weapons/Secondary","drops":[{"chance":100,"location":"Red Veil Offering"}],"components":[{"tradable":true,"ducats":25,"uniqueName":"/Lotus/Weapons/Secondary"}]}]`,
		"/ref-3/data/json/Melee.json":      `[]`,
		"/ref-3/data/json/Arch-Gun.json":   `[]`,
		"/ref-3/data/json/Arch-Melee.json": `[]`,
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := responses[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer ts.Close()
	withDatabaseHTTPClient(t, ts.Client())
	withWFCDBaseURL(t, ts.URL+"/%s/data/json/%s")

	if err := EnrichWeapons(context.Background(), db, "ref-3"); err != nil {
		t.Fatalf("EnrichWeapons() error = %v", err)
	}

	links := mustSyndicateLinks(t, db, "weapons")
	wantLinks := map[string][]string{
		"/Lotus/Weapons/TestBlueprint": {"Steel Meridian"},
		"/Lotus/Weapons/TestComponent": {"Steel Meridian"},
		"/Lotus/Weapons/Secondary":     {"Red Veil"},
	}
	if fmt.Sprint(links) != fmt.Sprint(wantLinks) {
		t.Fatalf("weapon links = %#v, want %#v", links, wantLinks)
	}

	ducats := mustIntsByGameRef(t, db, "weapons")
	wantDucats := map[string]int{
		"/Lotus/Weapons/TestBlueprint": 65,
		"/Lotus/Weapons/TestComponent": 65,
		"/Lotus/Weapons/Secondary":     25,
	}
	if fmt.Sprint(ducats) != fmt.Sprint(wantDucats) {
		t.Fatalf("weapon ducats = %#v, want %#v", ducats, wantDucats)
	}
}
