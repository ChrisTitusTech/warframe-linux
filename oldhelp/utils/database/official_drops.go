package database

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

var officialDropsURL = "https://www.warframe.com/droptables"

type nightmareDrop struct {
	Name     string
	Rotation string
	Chance   float64
}

type vaultDrop struct {
	Name   string
	Chance float64
}

func EnsureOfficialDrops(db *sql.DB) error {
	if err := ensureOfficialDropsTable(db, "nightmare_mod_drops", `CREATE TABLE IF NOT EXISTS nightmare_mod_drops (
		item_gameRef TEXT PRIMARY KEY,
		scraped_name TEXT NOT NULL,
		rotation TEXT NOT NULL,
		chance REAL NOT NULL
	)`); err != nil {
		return err
	}
	return ensureOfficialDropsTable(db, "vault_mod_drops", `CREATE TABLE IF NOT EXISTS vault_mod_drops (
		item_gameRef TEXT PRIMARY KEY,
		scraped_name TEXT NOT NULL,
		chance REAL NOT NULL
	)`)
}

func ensureOfficialDropsTable(db *sql.DB, tableName, createSQL string) error {
	exists, err := tableExists(db, tableName)
	if err != nil {
		return err
	}
	if !exists {
		_, err := db.Exec(createSQL)
		return err
	}

	chanceType, err := chanceColumnType(db, tableName)
	if err != nil {
		return err
	}
	if strings.EqualFold(chanceType, "REAL") {
		return nil
	}

	oldName := tableName + "_old"
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, oldName)); err != nil {
		return err
	}
	if _, err := tx.Exec(fmt.Sprintf(`ALTER TABLE "%s" RENAME TO "%s"`, tableName, oldName)); err != nil {
		return err
	}
	if _, err := tx.Exec(createSQL); err != nil {
		return err
	}

	copySQL := fmt.Sprintf(`INSERT INTO "%s"(item_gameRef, scraped_name, rotation, chance)
		SELECT item_gameRef, scraped_name, rotation, CAST(chance AS REAL)
		FROM "%s"`, tableName, oldName)
	if tableName == "vault_mod_drops" {
		copySQL = fmt.Sprintf(`INSERT INTO "%s"(item_gameRef, scraped_name, chance)
			SELECT item_gameRef, scraped_name, CAST(chance AS REAL)
			FROM "%s"`, tableName, oldName)
	}
	if _, err := tx.Exec(copySQL); err != nil {
		return err
	}
	if _, err := tx.Exec(fmt.Sprintf(`DROP TABLE "%s"`, oldName)); err != nil {
		return err
	}

	return tx.Commit()
}

func tableExists(db *sql.DB, tableName string) (bool, error) {
	var exists int
	err := db.QueryRow(`SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?`, tableName).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func chanceColumnType(db *sql.DB, tableName string) (string, error) {
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info("%s")`, tableName))
	if err != nil {
		return "", err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal any
			pk         int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &pk); err != nil {
			return "", err
		}
		if name == "chance" {
			return columnType, nil
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("table %s missing chance column", tableName)
}

func SyncOfficialModDrops(ctx context.Context, db *sql.DB) error {
	body, err := fetchBody(ctx, officialDropsURL)
	if err != nil {
		return err
	}

	nightmare, vault, err := parseOfficialModDrops(body)
	if err != nil {
		return err
	}

	modRefs, err := loadModRefs(ctx, db)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM nightmare_mod_drops`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM vault_mod_drops`); err != nil {
		return err
	}

	nightmareStmt, err := tx.PrepareContext(ctx, `INSERT INTO nightmare_mod_drops(item_gameRef, scraped_name, rotation, chance) VALUES (?,?,?,?)`)
	if err != nil {
		return err
	}
	defer nightmareStmt.Close()

	vaultStmt, err := tx.PrepareContext(ctx, `INSERT INTO vault_mod_drops(item_gameRef, scraped_name, chance) VALUES (?,?,?)`)
	if err != nil {
		return err
	}
	defer vaultStmt.Close()

	for _, drop := range nightmare {
		gameRef, ok := modRefs[drop.Name]
		if !ok {
			continue
		}
		if _, err := nightmareStmt.ExecContext(ctx, gameRef, drop.Name, drop.Rotation, drop.Chance); err != nil {
			return err
		}
	}

	for _, drop := range vault {
		gameRef, ok := modRefs[drop.Name]
		if !ok {
			continue
		}
		if _, err := vaultStmt.ExecContext(ctx, gameRef, drop.Name, drop.Chance); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func loadModRefs(ctx context.Context, db *sql.DB) (map[string]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT name, gameRef FROM mods`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refs := make(map[string]string)
	for rows.Next() {
		var name, gameRef string
		if err := rows.Scan(&name, &gameRef); err != nil {
			return nil, err
		}
		refs[name] = gameRef
	}
	return refs, rows.Err()
}

func parseOfficialModDrops(body []byte) ([]nightmareDrop, []vaultDrop, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}

	var (
		nightmare       []nightmareDrop
		vault           []vaultDrop
		currentSection  string
		currentRotation string
	)

	for _, row := range collectRows(doc) {
		id := nodeAttr(row, "id")
		if id != "" {
			switch id {
			case "nightmaremoderewards", "derelictvault":
				currentSection = id
				currentRotation = ""
			default:
				currentSection = ""
				currentRotation = ""
			}
			continue
		}

		if currentSection == "" {
			continue
		}

		cells := rowCells(row)
		if len(cells) == 0 {
			continue
		}

		switch currentSection {
		case "nightmaremoderewards":
			if strings.HasPrefix(cells[0], "Rotation ") {
				currentRotation = normalizeRotation(cells[0])
				continue
			}
			if len(cells) < 2 || currentRotation == "" {
				continue
			}
			chance, err := normalizeChance(cells[1])
			if err != nil {
				return nil, nil, fmt.Errorf("invalid nightmare chance for %q: %w", cells[0], err)
			}
			nightmare = append(nightmare, nightmareDrop{Name: cells[0], Rotation: currentRotation, Chance: chance})
		case "derelictvault":
			if len(cells) < 2 {
				continue
			}
			chance, err := normalizeChance(cells[1])
			if err != nil {
				return nil, nil, fmt.Errorf("invalid vault chance for %q: %w", cells[0], err)
			}
			vault = append(vault, vaultDrop{Name: cells[0], Chance: chance})
		}
	}

	if len(nightmare) == 0 {
		return nil, nil, fmt.Errorf("official drops parse found no nightmare rows")
	}
	if len(vault) == 0 {
		return nil, nil, fmt.Errorf("official drops parse found no vault rows")
	}
	return nightmare, vault, nil
}

func collectRows(root *html.Node) []*html.Node {
	var rows []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			rows = append(rows, n)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return rows
}

func rowCells(row *html.Node) []string {
	var cells []string
	for child := row.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		if child.Data != "th" && child.Data != "td" {
			continue
		}
		text := normalizeText(nodeText(child))
		if text == "" {
			continue
		}
		cells = append(cells, text)
	}
	return cells
}

func nodeAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func nodeText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		b.WriteString(nodeText(child))
		if child.Type == html.ElementNode && child.Data == "br" {
			b.WriteByte(' ')
		}
	}
	return b.String()
}

func normalizeText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func normalizeRotation(s string) string {
	s = strings.TrimSpace(strings.TrimPrefix(s, "Rotation "))
	if len(s) == 1 {
		return s
	}
	return strings.TrimSpace(s)
}

func normalizeChance(s string) (float64, error) {
	trimmed := strings.TrimSpace(s)
	start := strings.LastIndexByte(trimmed, '(')
	end := strings.LastIndexByte(trimmed, ')')
	if start >= 0 && end > start {
		inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed[start+1:end], "("), "%"))
		if f, err := strconv.ParseFloat(inner, 64); err == nil {
			return f, nil
		}
	}
	trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, "%"))
	f, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid chance %q", s)
	}
	return f, nil
}
