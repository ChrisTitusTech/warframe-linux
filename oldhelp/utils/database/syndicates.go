package database

import (
	"context"
	"database/sql"
	"strings"
)

var syndicates = []string{
	"Steel Meridian",
	"Arbiters of Hexis",
	"Cephalon Suda",
	"The Perrin Sequence",
	"Red Veil",
	"New Loka",
	"Conclave",
	"Cephalon Simaris",
	"Ostron",
	"The Quills",
	"Solaris United",
	"Vox Solaris",
	"Ventkids",
	"Entrati",
	"Necraloid",
	"Kahl's Garrison",
	"The Holdfasts",
	"Cavia",
	"The Hex",
	"Nightcap",
}

func EnsureSyndicates(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS syndicates (
		id INTEGER PRIMARY KEY,
		name TEXT UNIQUE NOT NULL
	)`)
	if err != nil {
		return err
	}

	var oldCol string
	db.QueryRow(`SELECT name FROM pragma_table_info('syndicate_items') WHERE name = 'item_uniqueName'`).Scan(&oldCol)
	if oldCol != "" {
		if _, err := db.Exec(`DROP TABLE syndicate_items`); err != nil {
			return err
		}
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS syndicate_items (
		item_gameRef TEXT NOT NULL,
		syndicate_id INTEGER NOT NULL,
		item_type TEXT NOT NULL,
		PRIMARY KEY (item_gameRef, syndicate_id, item_type)
	)`)
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	for _, s := range syndicates {
		if _, err := tx.Exec(`INSERT INTO syndicates(name) VALUES (?) ON CONFLICT(name) DO NOTHING`, s); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func CountSyndicateItems(ctx context.Context, db *sql.DB, itemType string) (int, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM syndicate_items WHERE item_type = ?`, itemType).Scan(&count)
	return count, err
}

func extractSyndicateNames(item map[string]any) []string {
	drops, ok := item["drops"].([]any)
	if !ok {
		return nil
	}

	var found []string
	for _, d := range drops {
		drop, ok := d.(map[string]any)
		if !ok {
			continue
		}
		chance, ok := drop["chance"].(float64)
		if !ok || chance != 100 {
			continue
		}
		location, ok := drop["location"].(string)
		if !ok {
			continue
		}

		for _, s := range syndicates {
			if strings.Contains(location, s) {
				found = append(found, s)
				break
			}
		}
	}
	return found
}

func SyncSyndicateItems(ctx context.Context, db *sql.DB, raw []map[string]any, itemType string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM syndicate_items WHERE item_type = ?`, itemType); err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO syndicate_items(item_gameRef, syndicate_id, item_type)
		SELECT ?, id, ? FROM syndicates WHERE name = ?
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range raw {
		uniqueName, ok := item["uniqueName"].(string)
		if !ok {
			continue
		}
		syndicateNames := extractSyndicateNames(item)
		if len(syndicateNames) == 0 {
			continue
		}
		for _, gameRef := range gameRefCandidates(uniqueName) {
			for _, s := range syndicateNames {
				if _, err := stmt.ExecContext(ctx, gameRef, itemType, s); err != nil {
					return err
				}
			}
		}
	}

	return tx.Commit()
}
