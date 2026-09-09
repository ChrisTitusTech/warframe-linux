package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

var wfmItemsURL = "https://api.warframe.market/v2/items"

var allTables = []string{"mods", "arcanes", "warframes", "weapons", "archwing", "companions", "misc"}
var weaponTags = []string{"weapon", "primary", "secondary", "melee", "archgun", "archmelee", "kitgun"}

type wfmItem struct {
	GameRef string   `json:"gameRef"`
	Slug    string   `json:"slug"`
	Tags    []string `json:"tags"`
	I18n    struct {
		En struct {
			Name string `json:"name"`
		} `json:"en"`
	} `json:"i18n"`
}

func routeByTags(tags []string) string {
	has := func(t string) bool { return slices.Contains(tags, t) }
	switch {
	case has("mod"):
		return "mods"
	case has("arcane_enhancement") || has("arcane_helmet"):
		return "arcanes"
	case has("archwing"):
		return "archwing"
	case has("sentinel") || has("kubrow"):
		return "companions"
	case slices.ContainsFunc(weaponTags, has):
		return "weapons"
	case has("warframe") && (has("blueprint") || has("component")):
		return "warframes"
	default:
		return "misc"
	}
}

func gameRefCandidates(uniqueName string) []string {
	candidates := []string{uniqueName}
	switch {
	case strings.HasSuffix(uniqueName, "Component"):
		candidates = append(candidates, strings.TrimSuffix(uniqueName, "Component")+"Blueprint")
	case strings.HasSuffix(uniqueName, "Blueprint"):
		candidates = append(candidates, strings.TrimSuffix(uniqueName, "Blueprint")+"Component")
	}
	return candidates
}

func fetchWFMItems(ctx context.Context, url string) ([]wfmItem, error) {
	body, err := fetchBody(ctx, url)
	if err != nil {
		return nil, err
	}
	var items []wfmItem
	if json.Unmarshal(body, &items) == nil && len(items) > 0 {
		return items, nil
	}
	var wrapped struct {
		Data    []wfmItem `json:"data"`
		Payload []wfmItem `json:"payload"`
	}
	if json.Unmarshal(body, &wrapped) == nil {
		if len(wrapped.Data) > 0 {
			return wrapped.Data, nil
		}
		if len(wrapped.Payload) > 0 {
			return wrapped.Payload, nil
		}
	}
	preview := body
	if len(preview) > 300 {
		preview = preview[:300]
	}
	return nil, fmt.Errorf("unrecognized WFM response format: %s", preview)
}

func SyncFromWFM(ctx context.Context, db *sql.DB, url string) error {
	if url == "" {
		url = wfmItemsURL
	}
	items, err := fetchWFMItems(ctx, url)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, t := range allTables {
		if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS `+t); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `CREATE TABLE `+t+` (gameRef TEXT PRIMARY KEY, slug TEXT NOT NULL, name TEXT NOT NULL, ducats INTEGER)`); err != nil {
			return err
		}
	}

	groups := make(map[string][]wfmItem, len(allTables))
	for _, item := range items {
		t := routeByTags(item.Tags)
		groups[t] = append(groups[t], item)
	}

	for table, tableItems := range groups {
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO `+table+`(gameRef, slug, name) VALUES (?,?,?) ON CONFLICT(gameRef) DO UPDATE SET slug=excluded.slug, name=excluded.name`)
		if err != nil {
			return err
		}
		for _, item := range tableItems {
			if _, err := stmt.ExecContext(ctx, item.GameRef, item.Slug, item.I18n.En.Name); err != nil {
				stmt.Close()
				return err
			}
		}
		stmt.Close()
	}

	return tx.Commit()
}
