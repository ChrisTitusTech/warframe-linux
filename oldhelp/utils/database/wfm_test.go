package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func withWFMItemsURL(t *testing.T, url string) {
	t.Helper()
	prev := wfmItemsURL
	wfmItemsURL = url
	t.Cleanup(func() { wfmItemsURL = prev })
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("Exec(%q) error = %v", query, err)
	}
}

func mustTableCount(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil {
		t.Fatalf("count %s error = %v", table, err)
	}
	return count
}

func mustStringMap(t *testing.T, db *sql.DB, query string, args ...any) map[string]string {
	t.Helper()
	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatalf("Query(%q) error = %v", query, err)
	}
	defer rows.Close()

	got := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			t.Fatalf("rows.Scan() error = %v", err)
		}
		got[k] = v
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err() = %v", err)
	}
	return got
}

func TestRouteByTags(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want string
	}{
		{"mods", []string{"mod"}, "mods"},
		{"arcanes enhancement", []string{"arcane_enhancement"}, "arcanes"},
		{"arcanes helmet", []string{"arcane_helmet"}, "arcanes"},
		{"archwing", []string{"archwing"}, "archwing"},
		{"companions sentinel", []string{"sentinel"}, "companions"},
		{"companions kubrow", []string{"kubrow"}, "companions"},
		{"weapons", []string{"primary"}, "weapons"},
		{"warframes blueprint", []string{"warframe", "blueprint"}, "warframes"},
		{"warframes component", []string{"warframe", "component"}, "warframes"},
		{"misc", []string{"resource"}, "misc"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := routeByTags(tc.tags); got != tc.want {
				t.Fatalf("routeByTags(%v) = %q, want %q", tc.tags, got, tc.want)
			}
		})
	}
}

func TestFetchWFMItems(t *testing.T) {
	t.Run("accepts direct array", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`[{"gameRef":"/Lotus/A","slug":"a","tags":["mod"],"i18n":{"en":{"name":"Alpha"}}}]`))
		}))
		defer ts.Close()
		withDatabaseHTTPClient(t, ts.Client())

		got, err := fetchWFMItems(context.Background(), ts.URL)
		if err != nil {
			t.Fatalf("fetchWFMItems() error = %v", err)
		}
		if len(got) != 1 || got[0].I18n.En.Name != "Alpha" {
			t.Fatalf("fetchWFMItems() = %#v", got)
		}
	})

	t.Run("accepts wrapped data", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"data":[{"gameRef":"/Lotus/B","slug":"b","tags":["primary"],"i18n":{"en":{"name":"Beta"}}}]}`))
		}))
		defer ts.Close()
		withDatabaseHTTPClient(t, ts.Client())

		got, err := fetchWFMItems(context.Background(), ts.URL)
		if err != nil {
			t.Fatalf("fetchWFMItems() error = %v", err)
		}
		if len(got) != 1 || got[0].Slug != "b" {
			t.Fatalf("fetchWFMItems() = %#v", got)
		}
	})

	t.Run("accepts wrapped payload", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"payload":[{"gameRef":"/Lotus/C","slug":"c","tags":["sentinel"],"i18n":{"en":{"name":"Gamma"}}}]}`))
		}))
		defer ts.Close()
		withDatabaseHTTPClient(t, ts.Client())

		got, err := fetchWFMItems(context.Background(), ts.URL)
		if err != nil {
			t.Fatalf("fetchWFMItems() error = %v", err)
		}
		if len(got) != 1 || got[0].GameRef != "/Lotus/C" {
			t.Fatalf("fetchWFMItems() = %#v", got)
		}
	})

	t.Run("errors on unknown format", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"items":[]}`))
		}))
		defer ts.Close()
		withDatabaseHTTPClient(t, ts.Client())

		_, err := fetchWFMItems(context.Background(), ts.URL)
		if err == nil || !strings.Contains(err.Error(), "unrecognized WFM response format") {
			t.Fatalf("fetchWFMItems() error = %v, want format error", err)
		}
	})
}

func TestSyncFromWFM(t *testing.T) {
	t.Run("creates tables routes items and updates duplicates", func(t *testing.T) {
		db := openTestDB(t)
		requestCount := 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			_, _ = w.Write([]byte(`[
				{"gameRef":"/Lotus/Mods/ConditionOverload","slug":"condition-overload-old","tags":["mod"],"i18n":{"en":{"name":"Condition Overload Old"}}},
				{"gameRef":"/Lotus/Mods/ConditionOverload","slug":"condition-overload","tags":["mod"],"i18n":{"en":{"name":"Condition Overload"}}},
				{"gameRef":"/Lotus/Arcanes/ArcaneEnergize","slug":"arcane-energize","tags":["arcane_enhancement"],"i18n":{"en":{"name":"Arcane Energize"}}},
				{"gameRef":"/Lotus/Types/Items/Research/DojoColors/WarframeResearchItem","slug":"warframe-research-item","tags":["warframe","blueprint"],"i18n":{"en":{"name":"Warframe Research Item"}}},
				{"gameRef":"/Lotus/Weapons/Tenno/Rifle/Braton","slug":"braton","tags":["primary"],"i18n":{"en":{"name":"Braton"}}},
				{"gameRef":"/Lotus/Powersuits/Archwing/Itzal","slug":"itzal","tags":["archwing"],"i18n":{"en":{"name":"Itzal"}}},
				{"gameRef":"/Lotus/Types/Sentinels/Carrier","slug":"carrier","tags":["sentinel"],"i18n":{"en":{"name":"Carrier"}}},
				{"gameRef":"/Lotus/Types/Items/Misc/ResourceItem","slug":"resource-item","tags":["resource"],"i18n":{"en":{"name":"Resource Item"}}}
			]`))
		}))
		defer ts.Close()
		withDatabaseHTTPClient(t, ts.Client())

		if err := SyncFromWFM(context.Background(), db, ts.URL); err != nil {
			t.Fatalf("SyncFromWFM() error = %v", err)
		}
		if requestCount != 1 {
			t.Fatalf("requestCount = %d, want 1", requestCount)
		}

		gotMods := mustStringMap(t, db, `SELECT gameRef, slug FROM mods`)
		if len(gotMods) != 1 || gotMods["/Lotus/Mods/ConditionOverload"] != "condition-overload" {
			t.Fatalf("mods slugs = %#v", gotMods)
		}

		var modName string
		if err := db.QueryRow(`SELECT name FROM mods WHERE gameRef = ?`, "/Lotus/Mods/ConditionOverload").Scan(&modName); err != nil {
			t.Fatalf("mod name lookup error = %v", err)
		}
		if modName != "Condition Overload" {
			t.Fatalf("mod name = %q, want %q", modName, "Condition Overload")
		}

		counts := map[string]int{}
		for _, table := range allTables {
			counts[table] = mustTableCount(t, db, table)
		}
		wantCounts := map[string]int{"mods": 1, "arcanes": 1, "warframes": 1, "weapons": 1, "archwing": 1, "companions": 1, "misc": 1}
		if fmt.Sprint(counts) != fmt.Sprint(wantCounts) {
			t.Fatalf("table counts = %#v, want %#v", counts, wantCounts)
		}
	})

	t.Run("uses default url when override empty", func(t *testing.T) {
		db := openTestDB(t)
		var gotPath string
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			_, _ = w.Write([]byte(`[{"gameRef":"/Lotus/Mods/Default","slug":"default","tags":["mod"],"i18n":{"en":{"name":"Default"}}}]`))
		}))
		defer ts.Close()
		withDatabaseHTTPClient(t, ts.Client())
		withWFMItemsURL(t, ts.URL+"/market")

		if err := SyncFromWFM(context.Background(), db, ""); err != nil {
			t.Fatalf("SyncFromWFM() error = %v", err)
		}
		if gotPath != "/market" {
			t.Fatalf("request path = %q, want %q", gotPath, "/market")
		}
	})

	t.Run("rolls back on transaction lock failure", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "warframe.db")
		db, err := OpenDB(path)
		if err != nil {
			t.Fatalf("OpenDB() error = %v", err)
		}
		lockerDB, err := OpenDB(path)
		if err != nil {
			t.Fatalf("OpenDB(locker) error = %v", err)
		}
		db.SetMaxOpenConns(1)
		lockerDB.SetMaxOpenConns(1)
		t.Cleanup(func() { _ = db.Close() })
		t.Cleanup(func() { _ = lockerDB.Close() })

		mustExec(t, db, `CREATE TABLE mods (gameRef TEXT PRIMARY KEY, slug TEXT NOT NULL, name TEXT NOT NULL, ducats INTEGER)`)
		mustExec(t, db, `INSERT INTO mods(gameRef, slug, name, ducats) VALUES ('legacy', 'legacy-slug', 'Legacy', 5)`)
		mustExec(t, db, `PRAGMA busy_timeout = 10`)

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`[{"gameRef":"/Lotus/Mods/New","slug":"new","tags":["mod"],"i18n":{"en":{"name":"New"}}}]`))
		}))
		defer ts.Close()
		withDatabaseHTTPClient(t, ts.Client())

		if _, err := lockerDB.Exec(`BEGIN EXCLUSIVE`); err != nil {
			t.Fatalf("BEGIN EXCLUSIVE error = %v", err)
		}

		err = SyncFromWFM(context.Background(), db, ts.URL)
		if err == nil {
			t.Fatal("SyncFromWFM() error = nil, want lock failure")
		}
		if _, rollbackErr := lockerDB.Exec(`ROLLBACK`); rollbackErr != nil {
			t.Fatalf("ROLLBACK error = %v", rollbackErr)
		}

		got := mustStringMap(t, db, `SELECT gameRef, slug FROM mods`)
		if len(got) != 1 || got["legacy"] != "legacy-slug" {
			t.Fatalf("mods after rollback = %#v", got)
		}
	})
}
