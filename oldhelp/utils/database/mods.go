package database

import (
	"context"
	"database/sql"
)

func EnrichMods(ctx context.Context, db *sql.DB, wfcdRef string) error {
	raw, err := FetchWFCDRaw(ctx, wfcdRef, modsWFCDFile)
	if err != nil {
		return err
	}
	return SyncSyndicateItems(ctx, db, raw, "mods")
}
