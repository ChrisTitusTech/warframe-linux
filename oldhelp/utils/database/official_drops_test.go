package database

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readDatabaseFixture(t *testing.T, name string) []byte {
	t.Helper()

	path := filepath.Join("..", "..", "testdata", "database", name)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return body
}

func createModsTable(t *testing.T, db *sql.DB) {
	t.Helper()

	if _, err := db.Exec(`CREATE TABLE mods (gameRef TEXT PRIMARY KEY, slug TEXT NOT NULL, name TEXT NOT NULL, ducats INTEGER)`); err != nil {
		t.Fatalf("create mods table error = %v", err)
	}
}

func insertMod(t *testing.T, db *sql.DB, gameRef, name string) {
	t.Helper()

	if _, err := db.Exec(`INSERT INTO mods(gameRef, slug, name) VALUES (?,?,?)`, gameRef, strings.ToLower(strings.ReplaceAll(name, " ", "-")), name); err != nil {
		t.Fatalf("insert mod %q error = %v", name, err)
	}
}

func TestEnsureOfficialDrops(t *testing.T) {
	t.Run("creates tables when missing", func(t *testing.T) {
		db := openTestDB(t)

		if err := EnsureOfficialDrops(db); err != nil {
			t.Fatalf("EnsureOfficialDrops() error = %v", err)
		}

		for _, table := range []string{"nightmare_mod_drops", "vault_mod_drops"} {
			exists, err := tableExists(db, table)
			if err != nil {
				t.Fatalf("tableExists(%q) error = %v", table, err)
			}
			if !exists {
				t.Fatalf("tableExists(%q) = false, want true", table)
			}
			chanceType, err := chanceColumnType(db, table)
			if err != nil {
				t.Fatalf("chanceColumnType(%q) error = %v", table, err)
			}
			if chanceType != "REAL" {
				t.Fatalf("chanceColumnType(%q) = %q, want %q", table, chanceType, "REAL")
			}
		}
	})

	t.Run("migrates legacy text chance columns and preserves rows", func(t *testing.T) {
		db := openTestDB(t)

		if _, err := db.Exec(`CREATE TABLE nightmare_mod_drops (item_gameRef TEXT PRIMARY KEY, scraped_name TEXT NOT NULL, rotation TEXT NOT NULL, chance TEXT NOT NULL)`); err != nil {
			t.Fatalf("create nightmare legacy table error = %v", err)
		}
		if _, err := db.Exec(`INSERT INTO nightmare_mod_drops(item_gameRef, scraped_name, rotation, chance) VALUES ('/Lotus/Mods/Nightmare/Constitution', 'Constitution', 'A', '2.53')`); err != nil {
			t.Fatalf("insert nightmare legacy row error = %v", err)
		}
		if _, err := db.Exec(`CREATE TABLE vault_mod_drops (item_gameRef TEXT PRIMARY KEY, scraped_name TEXT NOT NULL, chance TEXT NOT NULL)`); err != nil {
			t.Fatalf("create vault legacy table error = %v", err)
		}
		if _, err := db.Exec(`INSERT INTO vault_mod_drops(item_gameRef, scraped_name, chance) VALUES ('/Lotus/Upgrades/Mods/Randomized/BlindRage', 'Blind Rage', '4.35')`); err != nil {
			t.Fatalf("insert vault legacy row error = %v", err)
		}

		if err := EnsureOfficialDrops(db); err != nil {
			t.Fatalf("EnsureOfficialDrops() error = %v", err)
		}

		var nightmareChance float64
		if err := db.QueryRow(`SELECT chance FROM nightmare_mod_drops WHERE item_gameRef = '/Lotus/Mods/Nightmare/Constitution'`).Scan(&nightmareChance); err != nil {
			t.Fatalf("scan migrated nightmare row error = %v", err)
		}
		if nightmareChance != 2.53 {
			t.Fatalf("migrated nightmare chance = %v, want %v", nightmareChance, 2.53)
		}

		var vaultChance float64
		if err := db.QueryRow(`SELECT chance FROM vault_mod_drops WHERE item_gameRef = '/Lotus/Upgrades/Mods/Randomized/BlindRage'`).Scan(&vaultChance); err != nil {
			t.Fatalf("scan migrated vault row error = %v", err)
		}
		if vaultChance != 4.35 {
			t.Fatalf("migrated vault chance = %v, want %v", vaultChance, 4.35)
		}
	})
}

func TestChanceColumnType(t *testing.T) {
	t.Run("returns column type", func(t *testing.T) {
		db := openTestDB(t)
		if err := EnsureOfficialDrops(db); err != nil {
			t.Fatalf("EnsureOfficialDrops() error = %v", err)
		}

		got, err := chanceColumnType(db, "nightmare_mod_drops")
		if err != nil {
			t.Fatalf("chanceColumnType() error = %v", err)
		}
		if got != "REAL" {
			t.Fatalf("chanceColumnType() = %q, want %q", got, "REAL")
		}
	})

	t.Run("errors when chance column missing", func(t *testing.T) {
		db := openTestDB(t)
		if _, err := db.Exec(`CREATE TABLE not_drops (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`); err != nil {
			t.Fatalf("create not_drops table error = %v", err)
		}

		_, err := chanceColumnType(db, "not_drops")
		if err == nil || !strings.Contains(err.Error(), "missing chance column") {
			t.Fatalf("chanceColumnType() error = %v, want missing chance column", err)
		}
	})
}

func TestLoadModRefs(t *testing.T) {
	t.Run("returns name to gameRef map", func(t *testing.T) {
		db := openTestDB(t)
		createModsTable(t, db)
		insertMod(t, db, "/Lotus/Mods/Nightmare/Constitution", "Constitution")
		insertMod(t, db, "/Lotus/Upgrades/Mods/Randomized/BlindRage", "Blind Rage")

		got, err := loadModRefs(context.Background(), db)
		if err != nil {
			t.Fatalf("loadModRefs() error = %v", err)
		}

		if got["Constitution"] != "/Lotus/Mods/Nightmare/Constitution" {
			t.Fatalf("Constitution ref = %q", got["Constitution"])
		}
		if got["Blind Rage"] != "/Lotus/Upgrades/Mods/Randomized/BlindRage" {
			t.Fatalf("Blind Rage ref = %q", got["Blind Rage"])
		}
	})

	t.Run("errors when mods table missing", func(t *testing.T) {
		db := openTestDB(t)

		if _, err := loadModRefs(context.Background(), db); err == nil {
			t.Fatal("loadModRefs() error = nil, want query error")
		}
	})
}

func TestParseOfficialModDrops(t *testing.T) {
	t.Run("parses nightmare and vault sections", func(t *testing.T) {
		nightmare, vault, err := parseOfficialModDrops(readDatabaseFixture(t, "official_drops_valid.html"))
		if err != nil {
			t.Fatalf("parseOfficialModDrops() error = %v", err)
		}

		wantNightmare := []nightmareDrop{
			{Name: "Constitution", Rotation: "A", Chance: 2.53},
			{Name: "Hammer Shot", Rotation: "A", Chance: 1.01},
			{Name: "Blaze", Rotation: "C", Chance: 5.64},
		}
		if len(nightmare) != len(wantNightmare) {
			t.Fatalf("len(nightmare) = %d, want %d", len(nightmare), len(wantNightmare))
		}
		for i := range wantNightmare {
			if nightmare[i] != wantNightmare[i] {
				t.Fatalf("nightmare[%d] = %#v, want %#v", i, nightmare[i], wantNightmare[i])
			}
		}

		wantVault := []vaultDrop{
			{Name: "Blind Rage", Chance: 4.35},
			{Name: "Overextended", Chance: 4.35},
		}
		if len(vault) != len(wantVault) {
			t.Fatalf("len(vault) = %d, want %d", len(vault), len(wantVault))
		}
		for i := range wantVault {
			if vault[i] != wantVault[i] {
				t.Fatalf("vault[%d] = %#v, want %#v", i, vault[i], wantVault[i])
			}
		}
	})

	t.Run("errors on invalid nightmare chance", func(t *testing.T) {
		_, _, err := parseOfficialModDrops(readDatabaseFixture(t, "official_drops_invalid_chance.html"))
		if err == nil || !strings.Contains(err.Error(), `invalid nightmare chance for "Constitution"`) {
			t.Fatalf("parseOfficialModDrops() error = %v, want invalid nightmare chance", err)
		}
	})

	t.Run("errors when vault section missing rows", func(t *testing.T) {
		_, _, err := parseOfficialModDrops(readDatabaseFixture(t, "official_drops_missing_vault.html"))
		if err == nil || !strings.Contains(err.Error(), "found no vault rows") {
			t.Fatalf("parseOfficialModDrops() error = %v, want no vault rows", err)
		}
	})
}

func TestNormalizeChance(t *testing.T) {
	t.Run("parses plain percent", func(t *testing.T) {
		got, err := normalizeChance("4.35%")
		if err != nil {
			t.Fatalf("normalizeChance() error = %v", err)
		}
		if got != 4.35 {
			t.Fatalf("normalizeChance() = %v, want %v", got, 4.35)
		}
	})

	t.Run("parses parenthesized percent", func(t *testing.T) {
		got, err := normalizeChance("2.53% (2.53%)")
		if err != nil {
			t.Fatalf("normalizeChance() error = %v", err)
		}
		if got != 2.53 {
			t.Fatalf("normalizeChance() = %v, want %v", got, 2.53)
		}
	})

	t.Run("errors on invalid string", func(t *testing.T) {
		_, err := normalizeChance("abc")
		if err == nil || !strings.Contains(err.Error(), `invalid chance "abc"`) {
			t.Fatalf("normalizeChance() error = %v, want invalid chance", err)
		}
	})
}

func TestSyncOfficialModDrops(t *testing.T) {
	t.Run("fetches configured url and syncs parsed rows", func(t *testing.T) {
		db := openTestDB(t)
		createModsTable(t, db)
		insertMod(t, db, "/Lotus/Mods/Nightmare/Constitution", "Constitution")
		insertMod(t, db, "/Lotus/Mods/Nightmare/HammerShot", "Hammer Shot")
		insertMod(t, db, "/Lotus/Upgrades/Mods/Randomized/BlindRage", "Blind Rage")

		if err := EnsureOfficialDrops(db); err != nil {
			t.Fatalf("EnsureOfficialDrops() error = %v", err)
		}
		if _, err := db.Exec(`INSERT INTO nightmare_mod_drops(item_gameRef, scraped_name, rotation, chance) VALUES ('old-nightmare', 'Old Nightmare', 'A', 99.9)`); err != nil {
			t.Fatalf("insert old nightmare row error = %v", err)
		}
		if _, err := db.Exec(`INSERT INTO vault_mod_drops(item_gameRef, scraped_name, chance) VALUES ('old-vault', 'Old Vault', 99.9)`); err != nil {
			t.Fatalf("insert old vault row error = %v", err)
		}

		var requestedPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestedPath = r.URL.Path
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(readDatabaseFixture(t, "official_drops_valid.html"))
		}))
		defer server.Close()

		oldURL := officialDropsURL
		officialDropsURL = server.URL + "/custom-drops"
		defer func() { officialDropsURL = oldURL }()

		if err := SyncOfficialModDrops(context.Background(), db); err != nil {
			t.Fatalf("SyncOfficialModDrops() error = %v", err)
		}
		if requestedPath != "/custom-drops" {
			t.Fatalf("requested path = %q, want %q", requestedPath, "/custom-drops")
		}

		var nightmareCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM nightmare_mod_drops`).Scan(&nightmareCount); err != nil {
			t.Fatalf("nightmare count error = %v", err)
		}
		if nightmareCount != 2 {
			t.Fatalf("nightmare count = %d, want %d", nightmareCount, 2)
		}

		var vaultCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM vault_mod_drops`).Scan(&vaultCount); err != nil {
			t.Fatalf("vault count error = %v", err)
		}
		if vaultCount != 1 {
			t.Fatalf("vault count = %d, want %d", vaultCount, 1)
		}

		var rotation string
		var chance float64
		if err := db.QueryRow(`SELECT rotation, chance FROM nightmare_mod_drops WHERE item_gameRef = '/Lotus/Mods/Nightmare/Constitution'`).Scan(&rotation, &chance); err != nil {
			t.Fatalf("query synced nightmare row error = %v", err)
		}
		if rotation != "A" || chance != 2.53 {
			t.Fatalf("Constitution row = (%q, %v), want (%q, %v)", rotation, chance, "A", 2.53)
		}

		var blindRageChance float64
		if err := db.QueryRow(`SELECT chance FROM vault_mod_drops WHERE item_gameRef = '/Lotus/Upgrades/Mods/Randomized/BlindRage'`).Scan(&blindRageChance); err != nil {
			t.Fatalf("query synced vault row error = %v", err)
		}
		if blindRageChance != 4.35 {
			t.Fatalf("Blind Rage chance = %v, want %v", blindRageChance, 4.35)
		}

		var oldRows int
		if err := db.QueryRow(`SELECT COUNT(*) FROM nightmare_mod_drops WHERE item_gameRef = 'old-nightmare'`).Scan(&oldRows); err != nil {
			t.Fatalf("old nightmare row count error = %v", err)
		}
		if oldRows != 0 {
			t.Fatalf("old nightmare rows = %d, want 0", oldRows)
		}
	})

	t.Run("aborts on fetch error", func(t *testing.T) {
		db := openTestDB(t)
		if err := EnsureOfficialDrops(db); err != nil {
			t.Fatalf("EnsureOfficialDrops() error = %v", err)
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusBadGateway)
		}))
		defer server.Close()

		oldURL := officialDropsURL
		officialDropsURL = server.URL
		defer func() { officialDropsURL = oldURL }()

		if err := SyncOfficialModDrops(context.Background(), db); err == nil || !strings.Contains(err.Error(), "unexpected status") {
			t.Fatalf("SyncOfficialModDrops() error = %v, want unexpected status", err)
		}
	})

	t.Run("aborts on parse error without deleting old rows", func(t *testing.T) {
		db := openTestDB(t)
		createModsTable(t, db)
		insertMod(t, db, "/Lotus/Mods/Nightmare/Constitution", "Constitution")
		if err := EnsureOfficialDrops(db); err != nil {
			t.Fatalf("EnsureOfficialDrops() error = %v", err)
		}
		if _, err := db.Exec(`INSERT INTO nightmare_mod_drops(item_gameRef, scraped_name, rotation, chance) VALUES ('keep-nightmare', 'Keep Nightmare', 'B', 7.7)`); err != nil {
			t.Fatalf("insert keep nightmare row error = %v", err)
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(readDatabaseFixture(t, "official_drops_invalid_chance.html"))
		}))
		defer server.Close()

		oldURL := officialDropsURL
		officialDropsURL = server.URL
		defer func() { officialDropsURL = oldURL }()

		if err := SyncOfficialModDrops(context.Background(), db); err == nil || !strings.Contains(err.Error(), "invalid nightmare chance") {
			t.Fatalf("SyncOfficialModDrops() error = %v, want parse error", err)
		}

		var kept int
		if err := db.QueryRow(`SELECT COUNT(*) FROM nightmare_mod_drops WHERE item_gameRef = 'keep-nightmare'`).Scan(&kept); err != nil {
			t.Fatalf("kept nightmare row count error = %v", err)
		}
		if kept != 1 {
			t.Fatalf("kept nightmare rows = %d, want 1", kept)
		}
	})

	t.Run("aborts on insert error and rolls back deletes", func(t *testing.T) {
		db := openTestDB(t)
		createModsTable(t, db)
		insertMod(t, db, "/Lotus/Mods/Nightmare/Constitution", "Constitution")
		insertMod(t, db, "/Lotus/Mods/Nightmare/HammerShot", "Hammer Shot")
		insertMod(t, db, "/Lotus/Upgrades/Mods/Randomized/BlindRage", "Blind Rage")
		insertMod(t, db, "/Lotus/Upgrades/Mods/Randomized/Overextended", "Overextended")
		if err := EnsureOfficialDrops(db); err != nil {
			t.Fatalf("EnsureOfficialDrops() error = %v", err)
		}
		if _, err := db.Exec(`INSERT INTO nightmare_mod_drops(item_gameRef, scraped_name, rotation, chance) VALUES ('keep-nightmare', 'Keep Nightmare', 'B', 7.7)`); err != nil {
			t.Fatalf("insert keep nightmare row error = %v", err)
		}

		if _, err := db.Exec(`DROP TABLE vault_mod_drops`); err != nil {
			t.Fatalf("drop vault_mod_drops error = %v", err)
		}
		if _, err := db.Exec(`CREATE TABLE vault_mod_drops (item_gameRef TEXT PRIMARY KEY, scraped_name TEXT NOT NULL)`); err != nil {
			t.Fatalf("recreate invalid vault_mod_drops error = %v", err)
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(readDatabaseFixture(t, "official_drops_valid.html"))
		}))
		defer server.Close()

		oldURL := officialDropsURL
		officialDropsURL = server.URL
		defer func() { officialDropsURL = oldURL }()

		if err := SyncOfficialModDrops(context.Background(), db); err == nil {
			t.Fatal("SyncOfficialModDrops() error = nil, want insert error")
		}

		var kept int
		if err := db.QueryRow(`SELECT COUNT(*) FROM nightmare_mod_drops WHERE item_gameRef = 'keep-nightmare'`).Scan(&kept); err != nil {
			t.Fatalf("kept nightmare row count error = %v", err)
		}
		if kept != 1 {
			t.Fatalf("kept nightmare rows = %d, want 1", kept)
		}
	})
}
