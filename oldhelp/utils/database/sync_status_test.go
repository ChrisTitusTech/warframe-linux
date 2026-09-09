package database

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "warframe.db")
	db, err := OpenDB(path)
	if err != nil {
		t.Fatalf("OpenDB() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db

}

func TestEnsureSyncStatusCreatesTable(t *testing.T) {
	db := openTestDB(t)

	if err := EnsureSyncStatus(db); err != nil {
		t.Fatalf("EnsureSyncStatus() error = %v", err)
	}

	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'sync_status'`).Scan(&name)
	if err != nil {
		t.Fatalf("sync_status table lookup error = %v", err)
	}
	if name != "sync_status" {
		t.Fatalf("table name = %q, want %q", name, "sync_status")
	}

}

func TestReadSyncStatus(t *testing.T) {
	t.Run("missing table returns zero value", func(t *testing.T) {
		db := openTestDB(t)

		got, err := ReadSyncStatus(db)
		if err != nil {
			t.Fatalf("ReadSyncStatus() error = %v", err)
		}
		if got != (SyncStatus{}) {
			t.Fatalf("ReadSyncStatus() = %#v, want zero value", got)
		}
	})

	t.Run("empty table returns zero value", func(t *testing.T) {
		db := openTestDB(t)
		if err := EnsureSyncStatus(db); err != nil {
			t.Fatalf("EnsureSyncStatus() error = %v", err)
		}

		got, err := ReadSyncStatus(db)
		if err != nil {
			t.Fatalf("ReadSyncStatus() error = %v", err)
		}
		if got != (SyncStatus{}) {
			t.Fatalf("ReadSyncStatus() = %#v, want zero value", got)
		}
	})

	t.Run("malformed timestamp returns error", func(t *testing.T) {
		db := openTestDB(t)
		if err := EnsureSyncStatus(db); err != nil {
			t.Fatalf("EnsureSyncStatus() error = %v", err)
		}
		_, err := db.Exec(`INSERT INTO sync_status(id, last_completed_at, wfcd_commit_sha) VALUES (1, ?, ?)`, "not-a-time", "abcdef")
		if err != nil {
			t.Fatalf("insert malformed row error = %v", err)
		}

		if _, err := ReadSyncStatus(db); err == nil {
			t.Fatal("ReadSyncStatus() error = nil, want parse error")
		}
	})
}

func TestWriteSyncStatusCreatesSchemaAndRoundTripsUTC(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	input := SyncStatus{
		HasLastCompletedAt: true,
		LastCompletedAt:    time.Date(2026, time.April, 17, 12, 34, 56, 0, time.FixedZone("UTC+2", 2*60*60)),
		WFCDCommitSHA:      "abcdef123456",
	}

	if err := WriteSyncStatus(ctx, db, input); err != nil {
		t.Fatalf("WriteSyncStatus() error = %v", err)
	}

	var rawTimestamp string
	if err := db.QueryRow(`SELECT last_completed_at FROM sync_status WHERE id = 1`).Scan(&rawTimestamp); err != nil {
		t.Fatalf("raw timestamp lookup error = %v", err)
	}
	if rawTimestamp != "2026-04-17T10:34:56Z" {
		t.Fatalf("raw timestamp = %q, want %q", rawTimestamp, "2026-04-17T10:34:56Z")
	}

	got, err := ReadSyncStatus(db)
	if err != nil {
		t.Fatalf("ReadSyncStatus() error = %v", err)
	}

	want := SyncStatus{
		Exists:             true,
		HasLastCompletedAt: true,
		LastCompletedAt:    time.Date(2026, time.April, 17, 10, 34, 56, 0, time.UTC),
		WFCDCommitSHA:      "abcdef123456",
	}
	if got != want {
		t.Fatalf("round trip status = %#v, want %#v", got, want)
	}
}

func TestWriteSyncStatusClearsTimestamp(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := WriteSyncStatus(ctx, db, SyncStatus{HasLastCompletedAt: true, LastCompletedAt: time.Unix(10, 0), WFCDCommitSHA: "sha1"}); err != nil {
		t.Fatalf("first WriteSyncStatus() error = %v", err)
	}
	if err := WriteSyncStatus(ctx, db, SyncStatus{WFCDCommitSHA: "sha2"}); err != nil {
		t.Fatalf("second WriteSyncStatus() error = %v", err)
	}
	got, err := ReadSyncStatus(db)
	if err != nil {
		t.Fatalf("ReadSyncStatus() error = %v", err)
	}
	if got.HasLastCompletedAt || got.WFCDCommitSHA != "sha2" {
		t.Fatalf("status after clear = %#v", got)
	}
}

func TestLatestWFCDCommitSHA(t *testing.T) {
	t.Run("reads configured endpoint", func(t *testing.T) {
		var gotPath string
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path + "?" + r.URL.RawQuery
			_, _ = w.Write([]byte(`[{"sha":"abc123"}]`))
		}))
		defer ts.Close()
		withDatabaseHTTPClient(t, ts.Client())
		withWFCDCommitAPIURL(t, ts.URL+"/custom/commits?x=1")

		got, err := LatestWFCDCommitSHA(context.Background())
		if err != nil {
			t.Fatalf("LatestWFCDCommitSHA() error = %v", err)
		}
		if got != "abc123" {
			t.Fatalf("LatestWFCDCommitSHA() = %q, want %q", got, "abc123")
		}
		if gotPath != "/custom/commits?x=1" {
			t.Fatalf("request path = %q, want %q", gotPath, "/custom/commits?x=1")
		}
	})

	t.Run("errors on empty commits", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`[]`))
		}))
		defer ts.Close()
		withDatabaseHTTPClient(t, ts.Client())
		withWFCDCommitAPIURL(t, ts.URL)

		_, err := LatestWFCDCommitSHA(context.Background())
		if err == nil || !strings.Contains(err.Error(), "no WFCD commits returned") {
			t.Fatalf("LatestWFCDCommitSHA() error = %v, want empty result error", err)
		}
	})
}
