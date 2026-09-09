package database

import (
	"context"
	"database/sql"

	"golang.org/x/sync/errgroup"
)

func EnrichWarframes(ctx context.Context, db *sql.DB, wfcdRef string) error {
	raw, err := FetchWFCDRaw(ctx, wfcdRef, warframesWFCDFile)
	if err != nil {
		return err
	}
	return SyncDucats(ctx, db, raw, "warframes")
}

func EnrichArchwing(ctx context.Context, db *sql.DB, wfcdRef string) error {
	raw, err := FetchWFCDRaw(ctx, wfcdRef, archwingWFCDFile)
	if err != nil {
		return err
	}
	return SyncDucats(ctx, db, raw, "archwing")
}

func EnrichCompanions(ctx context.Context, db *sql.DB, wfcdRef string) error {
	raw, err := FetchWFCDRaw(ctx, wfcdRef, sentinelsWFCDFile)
	if err != nil {
		return err
	}
	return SyncDucats(ctx, db, raw, "companions")
}

func EnrichDucatCategories(ctx context.Context, db *sql.DB, wfcdRef string) error {
	results := make([][]map[string]any, len(ducatWFCDSources))
	g, gctx := errgroup.WithContext(ctx)
	for i, src := range ducatWFCDSources {
		g.Go(func() error {
			var err error
			results[i], err = FetchWFCDRaw(gctx, wfcdRef, src.file)
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	for i, src := range ducatWFCDSources {
		if err := SyncDucats(ctx, db, results[i], src.table); err != nil {
			return err
		}
	}
	return nil
}

func SyncDucats(ctx context.Context, db *sql.DB, raw []map[string]any, tableName string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `UPDATE `+tableName+` SET ducats = ? WHERE gameRef = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range raw {
		components, ok := item["components"].([]any)
		if !ok {
			continue
		}
		for _, c := range components {
			comp, ok := c.(map[string]any)
			if !ok {
				continue
			}
			tradable, _ := comp["tradable"].(bool)
			if !tradable {
				continue
			}
			ducats, ok := comp["ducats"].(float64)
			if !ok || ducats <= 0 {
				continue
			}
			uniqueName, ok := comp["uniqueName"].(string)
			if !ok {
				continue
			}
			for _, ref := range gameRefCandidates(uniqueName) {
				if _, err := stmt.ExecContext(ctx, int(ducats), ref); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit()
}
