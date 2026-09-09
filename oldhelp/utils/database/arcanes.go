package database

import (
	"context"
	"database/sql"
)

func EnrichArcanes(ctx context.Context, db *sql.DB, wfcdRef string) error {
	raw, err := FetchWFCDRaw(ctx, wfcdRef, arcanesWFCDFile)
	if err != nil {
		return err
	}
	return SyncSyndicateItems(ctx, db, raw, "arcanes")
}
