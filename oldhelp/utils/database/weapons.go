package database

import (
	"context"
	"database/sql"

	"golang.org/x/sync/errgroup"
)

func EnrichWeapons(ctx context.Context, db *sql.DB, wfcdRef string) error {
	results := make([][]map[string]any, len(weaponsWFCDFiles))
	g, gctx := errgroup.WithContext(ctx)
	for i, file := range weaponsWFCDFiles {
		g.Go(func() error {
			var err error
			results[i], err = FetchWFCDRaw(gctx, wfcdRef, file)
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	var all []map[string]any
	for _, r := range results {
		all = append(all, r...)
	}
	if err := SyncSyndicateItems(ctx, db, all, "weapons"); err != nil {
		return err
	}
	return SyncDucats(ctx, db, all, "weapons")
}
