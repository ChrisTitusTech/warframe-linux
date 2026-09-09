package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

const syncStatusTable = `CREATE TABLE IF NOT EXISTS sync_status (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	last_completed_at TEXT,
	wfcd_commit_sha TEXT
)`

var wfcdCommitAPIURL = "https://api.github.com/repos/WFCD/warframe-items/commits?path=data/json&sha=master&per_page=1"

type SyncStatus struct {
	Exists             bool
	HasLastCompletedAt bool
	LastCompletedAt    time.Time
	WFCDCommitSHA      string
}

func EnsureSyncStatus(db *sql.DB) error {
	_, err := db.Exec(syncStatusTable)
	return err
}

func ReadSyncStatus(db *sql.DB) (SyncStatus, error) {
	var tableName string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'sync_status'`).Scan(&tableName)
	if err == sql.ErrNoRows {
		return SyncStatus{}, nil
	}
	if err != nil {
		return SyncStatus{}, err
	}

	var lastCompletedAt sql.NullString
	var wfcdCommitSHA sql.NullString
	err = db.QueryRow(`SELECT last_completed_at, wfcd_commit_sha FROM sync_status WHERE id = 1`).Scan(&lastCompletedAt, &wfcdCommitSHA)
	if err == sql.ErrNoRows {
		return SyncStatus{}, nil
	}
	if err != nil {
		return SyncStatus{}, err
	}

	status := SyncStatus{
		Exists:        true,
		WFCDCommitSHA: wfcdCommitSHA.String,
	}
	if lastCompletedAt.Valid && lastCompletedAt.String != "" {
		parsed, err := time.Parse(time.RFC3339, lastCompletedAt.String)
		if err != nil {
			return SyncStatus{}, err
		}
		status.HasLastCompletedAt = true
		status.LastCompletedAt = parsed.UTC()
	}
	return status, nil
}

func WriteSyncStatus(ctx context.Context, db *sql.DB, status SyncStatus) error {
	if err := EnsureSyncStatus(db); err != nil {
		return err
	}

	lastCompletedAt := sql.NullString{}
	if status.HasLastCompletedAt {
		lastCompletedAt = sql.NullString{String: status.LastCompletedAt.UTC().Format(time.RFC3339), Valid: true}
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO sync_status(id, last_completed_at, wfcd_commit_sha)
		VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			last_completed_at = excluded.last_completed_at,
			wfcd_commit_sha = excluded.wfcd_commit_sha
	`, lastCompletedAt, status.WFCDCommitSHA)
	return err
}

func LatestWFCDCommitSHA(ctx context.Context) (string, error) {
	body, err := fetchBody(ctx, wfcdCommitAPIURL)
	if err != nil {
		return "", err
	}

	var commits []struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(body, &commits); err != nil {
		return "", err
	}
	if len(commits) == 0 || commits[0].SHA == "" {
		return "", fmt.Errorf("no WFCD commits returned")
	}
	return commits[0].SHA, nil
}
