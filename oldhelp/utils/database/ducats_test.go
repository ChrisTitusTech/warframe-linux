package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func mustIntsByGameRef(t *testing.T, db *sql.DB, table string) map[string]int {
	t.Helper()
	rows, err := db.Query(`SELECT gameRef, COALESCE(ducats, 0) FROM ` + table)
	if err != nil {
		t.Fatalf("Query %s ducats error = %v", table, err)
	}
	defer rows.Close()

	got := map[string]int{}
	for rows.Next() {
		var ref string
		var ducats int
		if err := rows.Scan(&ref, &ducats); err != nil {
			t.Fatalf("rows.Scan() error = %v", err)
		}
		got[ref] = ducats
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err() = %v", err)
	}
	return got
}

func TestSyncDucats(t *testing.T) {
	db := openTestDB(t)
	mustExec(t, db, `CREATE TABLE weapons (gameRef TEXT PRIMARY KEY, slug TEXT NOT NULL, name TEXT NOT NULL, ducats INTEGER)`)
	mustExec(t, db, `INSERT INTO weapons(gameRef, slug, name, ducats) VALUES (?, ?, ?, ?)`, "/Lotus/Weapons/TestBlueprint", "test-blueprint", "Test Blueprint", 0)
	mustExec(t, db, `INSERT INTO weapons(gameRef, slug, name, ducats) VALUES (?, ?, ?, ?)`, "/Lotus/Weapons/TestComponent", "test-component", "Test Component", 0)
	mustExec(t, db, `INSERT INTO weapons(gameRef, slug, name, ducats) VALUES (?, ?, ?, ?)`, "/Lotus/Weapons/Other", "other", "Other", 7)

	raw := []map[string]any{
		{
			"components": []any{
				map[string]any{"tradable": true, "ducats": float64(45), "uniqueName": "/Lotus/Weapons/TestBlueprint"},
				map[string]any{"tradable": false, "ducats": float64(100), "uniqueName": "/Lotus/Weapons/Other"},
				map[string]any{"tradable": true, "ducats": float64(0), "uniqueName": "/Lotus/Weapons/Other"},
				map[string]any{"tradable": true, "ducats": float64(15)},
			},
		},
	}

	if err := SyncDucats(context.Background(), db, raw, "weapons"); err != nil {
		t.Fatalf("SyncDucats() error = %v", err)
	}

	got := mustIntsByGameRef(t, db, "weapons")
	want := map[string]int{
		"/Lotus/Weapons/TestBlueprint": 45,
		"/Lotus/Weapons/TestComponent": 45,
		"/Lotus/Weapons/Other":         7,
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("weapon ducats = %#v, want %#v", got, want)
	}
}

func TestEnrichDucatCategories(t *testing.T) {
	db := openTestDB(t)
	for _, table := range []string{"warframes", "archwing", "companions"} {
		mustExec(t, db, `CREATE TABLE `+table+` (gameRef TEXT PRIMARY KEY, slug TEXT NOT NULL, name TEXT NOT NULL, ducats INTEGER)`)
	}
	mustExec(t, db, `INSERT INTO warframes(gameRef, slug, name) VALUES (?, ?, ?)`, "/Lotus/Warframes/TestBlueprint", "wf-bp", "WF BP")
	mustExec(t, db, `INSERT INTO warframes(gameRef, slug, name) VALUES (?, ?, ?)`, "/Lotus/Warframes/TestComponent", "wf-comp", "WF Comp")
	mustExec(t, db, `INSERT INTO archwing(gameRef, slug, name) VALUES (?, ?, ?)`, "/Lotus/Archwing/Test", "aw", "AW")
	mustExec(t, db, `INSERT INTO companions(gameRef, slug, name) VALUES (?, ?, ?)`, "/Lotus/Companions/Test", "comp", "Comp")

	responses := map[string]string{
		"/ref-4/data/json/Warframes.json": `[{"components":[{"tradable":true,"ducats":100,"uniqueName":"/Lotus/Warframes/TestBlueprint"}]}]`,
		"/ref-4/data/json/Archwing.json":  `[{"components":[{"tradable":true,"ducats":45,"uniqueName":"/Lotus/Archwing/Test"}]}]`,
		"/ref-4/data/json/Sentinels.json": `[{"components":[{"tradable":true,"ducats":65,"uniqueName":"/Lotus/Companions/Test"}]}]`,
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(responses[r.URL.Path]))
	}))
	defer ts.Close()
	withDatabaseHTTPClient(t, ts.Client())
	withWFCDBaseURL(t, ts.URL+"/%s/data/json/%s")

	if err := EnrichDucatCategories(context.Background(), db, "ref-4"); err != nil {
		t.Fatalf("EnrichDucatCategories() error = %v", err)
	}

	if got := mustIntsByGameRef(t, db, "warframes"); fmt.Sprint(got) != fmt.Sprint(map[string]int{"/Lotus/Warframes/TestBlueprint": 100, "/Lotus/Warframes/TestComponent": 100}) {
		t.Fatalf("warframe ducats = %#v", got)
	}
	if got := mustIntsByGameRef(t, db, "archwing"); fmt.Sprint(got) != fmt.Sprint(map[string]int{"/Lotus/Archwing/Test": 45}) {
		t.Fatalf("archwing ducats = %#v", got)
	}
	if got := mustIntsByGameRef(t, db, "companions"); fmt.Sprint(got) != fmt.Sprint(map[string]int{"/Lotus/Companions/Test": 65}) {
		t.Fatalf("companions ducats = %#v", got)
	}
}
