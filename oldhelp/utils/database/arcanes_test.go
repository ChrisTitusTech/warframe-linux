package database

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEnrichArcanes(t *testing.T) {
	db := openTestDB(t)
	if err := EnsureSyndicates(db); err != nil {
		t.Fatalf("EnsureSyndicates() error = %v", err)
	}
	mustExec(t, db, `CREATE TABLE arcanes (gameRef TEXT PRIMARY KEY, slug TEXT NOT NULL, name TEXT NOT NULL, ducats INTEGER)`)
	mustExec(t, db, `INSERT INTO arcanes(gameRef, slug, name) VALUES (?, ?, ?)`, "/Lotus/Arcanes/Test", "test", "Test")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"uniqueName":"/Lotus/Arcanes/Test","drops":[{"chance":100,"location":"The Perrin Sequence Offering"}]}
		]`))
	}))
	defer ts.Close()
	withDatabaseHTTPClient(t, ts.Client())
	withWFCDBaseURL(t, ts.URL+"/%s/data/json/%s")

	if err := EnrichArcanes(context.Background(), db, "ref-2"); err != nil {
		t.Fatalf("EnrichArcanes() error = %v", err)
	}

	got := mustSyndicateLinks(t, db, "arcanes")
	want := map[string][]string{"/Lotus/Arcanes/Test": {"The Perrin Sequence"}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("arcane links = %#v, want %#v", got, want)
	}
}
