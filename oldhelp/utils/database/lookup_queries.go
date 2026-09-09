package database

import "strings"

var syndicateTradableTables = []string{"mods", "arcanes", "weapons"}

func unionQuery(tables []string, build func(string) string) string {
	parts := make([]string, len(tables))
	for i, table := range tables {
		parts[i] = build(table)
	}
	return strings.Join(parts, "\nUNION ALL\n")
}

func SyndicateTradableItemsQuery() string {
	return unionQuery(syndicateTradableTables, func(table string) string {
		return "SELECT t.gameRef, t.slug, t.name, '" + table + "', GROUP_CONCAT(s.name, ', ') FROM " + table + " t " +
			"JOIN syndicate_items si ON si.item_gameRef = t.gameRef " +
			"JOIN syndicates s ON s.id = si.syndicate_id " +
			"GROUP BY t.gameRef, t.name"
	})
}

func TradableItemsQuery() string {
	return unionQuery(allTables, func(table string) string {
		return "SELECT gameRef, slug, name, '" + table + "' FROM " + table
	})
}

func DucatItemsQuery() string {
	return unionQuery(allTables, func(table string) string {
		return "SELECT gameRef, slug, name, ducats FROM " + table + " WHERE ducats IS NOT NULL AND ducats > 0"
	})
}

func NightmareItemsQuery() string {
	return `SELECT m.gameRef, m.slug, m.name, n.rotation, n.chance
		FROM mods m
		JOIN nightmare_mod_drops n ON n.item_gameRef = m.gameRef`
}

func VaultItemsQuery() string {
	return `SELECT m.gameRef, m.slug, m.name, v.chance
		FROM mods m
		JOIN vault_mod_drops v ON v.item_gameRef = m.gameRef`
}
