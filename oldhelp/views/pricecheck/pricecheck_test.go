package pricecheck

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/gjrud/warframe-helper/utils/database"
	"github.com/gjrud/warframe-helper/utils/pricechecker"
	"github.com/gjrud/warframe-helper/utils/viewBuilder"
)

func setUserHomeEnv(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
}

var pricecheckTables = []string{"mods", "arcanes", "warframes", "weapons", "archwing", "companions", "misc"}

func runCmd(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()

	if cmd == nil {
		return nil
	}
	return expandMsg(t, cmd())
}

func expandMsg(t *testing.T, msg tea.Msg) []tea.Msg {
	t.Helper()

	if msg == nil {
		return nil
	}

	rv := reflect.ValueOf(msg)
	if rv.IsValid() && rv.Kind() == reflect.Slice {
		var msgs []tea.Msg
		for i := 0; i < rv.Len(); i++ {
			if rv.Index(i).IsNil() {
				continue
			}
			sub, ok := rv.Index(i).Interface().(tea.Cmd)
			if !ok {
				t.Fatalf("slice message item %d = %T, want tea.Cmd", i, rv.Index(i).Interface())
			}
			msgs = append(msgs, runCmd(t, sub)...)
		}
		return msgs
	}

	return []tea.Msg{msg}
}

func sequenceCmds(t *testing.T, cmd tea.Cmd) []tea.Cmd {
	t.Helper()

	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}

	rv := reflect.ValueOf(msg)
	if !rv.IsValid() || rv.Kind() != reflect.Slice {
		t.Fatalf("sequence cmd() = %T, want slice", msg)
	}

	cmds := make([]tea.Cmd, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		if rv.Index(i).IsNil() {
			continue
		}
		sub, ok := rv.Index(i).Interface().(tea.Cmd)
		if !ok {
			t.Fatalf("sequence element %d = %T, want tea.Cmd", i, rv.Index(i).Interface())
		}
		cmds = append(cmds, sub)
	}
	return cmds
}

func runSequenceSteps(t *testing.T, cmd tea.Cmd, afterStep func(int, []tea.Msg)) []tea.Msg {
	t.Helper()

	var msgs []tea.Msg
	for i, stepCmd := range sequenceCmds(t, cmd) {
		stepMsgs := runCmd(t, stepCmd)
		msgs = append(msgs, stepMsgs...)
		if afterStep != nil {
			afterStep(i, stepMsgs)
		}
	}
	return msgs
}

func setupPricecheckHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	setUserHomeEnv(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".warframe-helper"), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	return home
}

func openPricecheckDB(t *testing.T, path string) *sql.DB {
	t.Helper()

	db, err := database.OpenDB(path)
	if err != nil {
		t.Fatalf("OpenDB() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	for _, tableName := range pricecheckTables {
		if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS ` + tableName + ` (gameRef TEXT PRIMARY KEY, slug TEXT NOT NULL, name TEXT NOT NULL, ducats INTEGER)`); err != nil {
			t.Fatalf("create table %s error = %v", tableName, err)
		}
	}
	if err := database.EnsureSyndicates(db); err != nil {
		t.Fatalf("EnsureSyndicates() error = %v", err)
	}
	if err := database.EnsureOfficialDrops(db); err != nil {
		t.Fatalf("EnsureOfficialDrops() error = %v", err)
	}
	return db
}

func writeInventoryFixture(t *testing.T, home string, body string) {
	t.Helper()

	path := filepath.Join(home, ".warframe-helper", "inventory.json")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func viewLines(view string) []string {
	return strings.Split(strings.TrimSuffix(ansi.Strip(view), "\n"), "\n")
}

func insertMarketItem(t *testing.T, db *sql.DB, tableName string, item pricechecker.GameObject) {
	t.Helper()

	if _, err := db.Exec(`INSERT INTO `+tableName+`(gameRef, slug, name, ducats) VALUES (?, ?, ?, ?)`, item.GameRef, item.Slug, item.Name, item.Ducats); err != nil {
		t.Fatalf("insert item %s/%s error = %v", tableName, item.GameRef, err)
	}
}

func insertSyndicateLink(t *testing.T, db *sql.DB, itemGameRef string, itemType string, syndicateName string) {
	t.Helper()

	if _, err := db.Exec(`INSERT INTO syndicate_items(item_gameRef, syndicate_id, item_type) SELECT ?, id, ? FROM syndicates WHERE name = ?`, itemGameRef, itemType, syndicateName); err != nil {
		t.Fatalf("insert syndicate link error = %v", err)
	}
}

func insertNightmareDrop(t *testing.T, db *sql.DB, itemGameRef string, scrapedName string, rotation string, chance float64) {
	t.Helper()

	if _, err := db.Exec(`INSERT INTO nightmare_mod_drops(item_gameRef, scraped_name, rotation, chance) VALUES (?, ?, ?, ?)`, itemGameRef, scrapedName, rotation, chance); err != nil {
		t.Fatalf("insert nightmare drop error = %v", err)
	}
}

func insertVaultDrop(t *testing.T, db *sql.DB, itemGameRef string, scrapedName string, chance float64) {
	t.Helper()

	if _, err := db.Exec(`INSERT INTO vault_mod_drops(item_gameRef, scraped_name, chance) VALUES (?, ?, ?)`, itemGameRef, scrapedName, chance); err != nil {
		t.Fatalf("insert vault drop error = %v", err)
	}
}

func chooseBase(t *testing.T, base *viewBuilder.BaseModel, numOptions int) {
	t.Helper()

	cmd := base.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter}, numOptions, func(int, context.Context) tea.Cmd { return nil })
	if cmd != nil {
		_ = cmd()
	}
	if !base.Chosen || base.Cancel == nil {
		t.Fatalf("base not chosen after enter: %+v", *base)
	}
}

func TestFetchNextItem(t *testing.T) {
	t.Run("canceled context returns canceled message", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		msg := fetchNextItem(ctx, fetchItemsMsg{items: []pricechecker.GameObject{{Name: "A"}}})()
		if _, ok := msg.(viewBuilder.CanceledMsg); !ok {
			t.Fatalf("msg = %T, want CanceledMsg", msg)
		}
	})

	t.Run("fetch failure appends failed item", func(t *testing.T) {
		input := fetchItemsMsg{
			mode:  pricecheckMode,
			items: []pricechecker.GameObject{{Name: "A", GameRef: "a", Category: "Mod"}, {Name: "B", GameRef: "b"}},
		}

		msg := fetchNextItem(context.Background(), input)()
		got, ok := msg.(fetchItemsMsg)
		if !ok {
			t.Fatalf("msg = %T, want fetchItemsMsg", msg)
		}
		if got.index != 1 {
			t.Fatalf("index = %d, want 1", got.index)
		}
		want := []pricechecker.OutputItem{{Name: "A", GameRef: "a", Category: "Mod", FetchFailed: true}}
		if !reflect.DeepEqual(got.results, want) {
			t.Fatalf("results = %#v, want %#v", got.results, want)
		}
	})

	t.Run("non-final failure preserves prior results and advances index", func(t *testing.T) {
		prior := []pricechecker.OutputItem{{Name: "Existing", AvgPrice: 9.5, Count: 3}}
		msg := fetchNextItem(context.Background(), fetchItemsMsg{
			mode:    pricecheckMode,
			items:   []pricechecker.GameObject{{Name: "A", GameRef: "a", Slug: "alpha"}, {Name: "B", GameRef: "b"}, {Name: "C", GameRef: "c", Slug: "charlie"}},
			index:   1,
			results: prior,
		})()
		got, ok := msg.(fetchItemsMsg)
		if !ok {
			t.Fatalf("msg = %T, want fetchItemsMsg", msg)
		}
		if got.index != 2 {
			t.Fatalf("index = %d, want 2", got.index)
		}
		want := []pricechecker.OutputItem{{Name: "Existing", AvgPrice: 9.5, Count: 3}, {Name: "B", GameRef: "b", FetchFailed: true}}
		if !reflect.DeepEqual(got.results, want) {
			t.Fatalf("results = %#v, want %#v", got.results, want)
		}
	})

	t.Run("last item returns sorted results", func(t *testing.T) {
		msg := fetchNextItem(context.Background(), fetchItemsMsg{
			mode:    ducatMode,
			items:   []pricechecker.GameObject{{Name: "Cheap", GameRef: "cheap", Ducats: 10}, {Name: "Broken", GameRef: "broken", Ducats: 50}},
			index:   1,
			results: []pricechecker.OutputItem{{Name: "Cheap", AvgPrice: 20, Ducats: 10}},
		})()
		got, ok := msg.(ResultsMsg)
		if !ok {
			t.Fatalf("msg = %T, want ResultsMsg", msg)
		}
		want := []pricechecker.OutputItem{
			{Name: "Cheap", AvgPrice: 20, Ducats: 10},
			{Name: "Broken", GameRef: "broken", Ducats: 50, FetchFailed: true},
		}
		if !reflect.DeepEqual(got.Items, want) {
			t.Fatalf("items = %#v, want %#v", got.Items, want)
		}
	})
}

func TestFailedItem(t *testing.T) {
	input := pricechecker.GameObject{
		Name:       "Arcane Example",
		GameRef:    "arcane_example",
		Category:   "Arcane",
		Syndicates: "Steel Meridian",
		Ducats:     45,
		Rotation:   "C",
		Chance:     2.75,
	}

	got := failedItem(input)

	if got.Name != input.Name || got.GameRef != input.GameRef || got.Category != input.Category {
		t.Fatalf("failedItem() lost identifying fields: %#v", got)
	}
	if got.Syndicates != input.Syndicates || got.Ducats != input.Ducats {
		t.Fatalf("failedItem() lost metadata fields: %#v", got)
	}
	if got.Rotation != input.Rotation || got.Chance != input.Chance {
		t.Fatalf("failedItem() lost drop fields: %#v", got)
	}
	if !got.FetchFailed {
		t.Fatal("failedItem() should mark output as failed")
	}
}

func TestStartFetch(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		msg := startFetch(vaultMode, nil)
		got, ok := msg.(ResultsMsg)
		if !ok {
			t.Fatalf("startFetch() returned %T, want ResultsMsg", msg)
		}
		if got.Mode != vaultMode {
			t.Fatalf("ResultsMsg mode = %v, want %v", got.Mode, vaultMode)
		}
		if len(got.Items) != 0 {
			t.Fatalf("ResultsMsg items len = %d, want 0", len(got.Items))
		}
	})

	t.Run("non-empty", func(t *testing.T) {
		items := []pricechecker.GameObject{{Name: "Forma Blueprint"}, {Name: "Orokin Cell"}}

		msg := startFetch(syndicateMode, items)
		got, ok := msg.(fetchItemsMsg)
		if !ok {
			t.Fatalf("startFetch() returned %T, want fetchItemsMsg", msg)
		}
		if got.mode != syndicateMode {
			t.Fatalf("fetchItemsMsg mode = %v, want %v", got.mode, syndicateMode)
		}
		if !reflect.DeepEqual(got.items, items) {
			t.Fatalf("fetchItemsMsg items = %#v, want %#v", got.items, items)
		}
		if got.index != 0 {
			t.Fatalf("fetchItemsMsg index = %d, want 0", got.index)
		}
		if len(got.results) != 0 || cap(got.results) != len(items) {
			t.Fatalf("fetchItemsMsg results len/cap = %d/%d, want 0/%d", len(got.results), cap(got.results), len(items))
		}
	})
}

func TestRenderRows(t *testing.T) {
	item := pricechecker.OutputItem{
		Name:       "Primed Example",
		Category:   "Mod",
		AvgPrice:   12.3,
		Count:      7,
		Ducats:     65,
		Rotation:   "B",
		Chance:     4.56,
		Syndicates: "New Loka",
	}

	tests := []struct {
		name     string
		mode     resultMode
		wantCols []table.Column
		wantRow  table.Row
	}{
		{name: "pricecheck", mode: pricecheckMode, wantCols: resultCols, wantRow: table.Row{"Primed Example", "Mod", "12.3", "7"}},
		{name: "syndicate", mode: syndicateMode, wantCols: syndicateResultCols, wantRow: table.Row{"Primed Example", "Mod", "12.3", "7", "New Loka"}},
		{name: "ducat", mode: ducatMode, wantCols: ducatResultCols, wantRow: table.Row{"Primed Example", "65", "12.3"}},
		{name: "nightmare", mode: nightmareMode, wantCols: nightmareResultCols, wantRow: table.Row{"Primed Example", "12.3", "7", "B", "4.56"}},
		{name: "vault", mode: vaultMode, wantCols: vaultResultCols, wantRow: table.Row{"Primed Example", "12.3", "7"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols, rows := renderRows(tt.mode, []pricechecker.OutputItem{item})
			if !reflect.DeepEqual(cols, tt.wantCols) {
				t.Fatalf("columns = %#v, want %#v", cols, tt.wantCols)
			}
			if len(rows) != 1 {
				t.Fatalf("rows len = %d, want 1", len(rows))
			}
			if !reflect.DeepEqual(rows[0], tt.wantRow) {
				t.Fatalf("row = %#v, want %#v", rows[0], tt.wantRow)
			}
		})
	}

	t.Run("failed item styling", func(t *testing.T) {
		cols, rows := renderRows(pricecheckMode, []pricechecker.OutputItem{{
			Name:        "Broken Item",
			Category:    "Mod",
			FetchFailed: true,
		}})
		if !reflect.DeepEqual(cols, resultCols) {
			t.Fatalf("columns = %#v, want %#v", cols, resultCols)
		}
		want := table.Row{
			viewBuilder.ErrorStyle.Render("Broken Item"),
			viewBuilder.ErrorStyle.Render("Mod"),
			viewBuilder.ErrorStyle.Render("ERR"),
			viewBuilder.ErrorStyle.Render("ERR"),
		}
		if !reflect.DeepEqual(rows[0], want) {
			t.Fatalf("failed row = %#v, want %#v", rows[0], want)
		}
	})
}

func TestUpdateResultsMsg(t *testing.T) {
	m := NewModel().(Model)
	chooseBase(t, &m.base, len(options))
	m.base.Steps = []string{"Loading"}
	m.fetching = true
	m.table.SetRows([]table.Row{{"z"}, {"a"}})
	m.table.SetCursor(1)

	results := []pricechecker.OutputItem{{Name: "Forma Blueprint", AvgPrice: 9.5, Count: 14}}

	updated, cmd := m.Update(ResultsMsg{Mode: vaultMode, Items: results})
	got := updated.(Model)

	if !got.base.Done {
		t.Fatal("base.Done = false, want true")
	}
	if got.base.Cancel != nil {
		t.Fatal("base.Cancel should be cleared")
	}
	if got.fetching {
		t.Fatal("fetching = true, want false")
	}
	if !reflect.DeepEqual(got.base.Steps, []string{"Loading", "Done"}) {
		t.Fatalf("steps = %#v, want appended Done", got.base.Steps)
	}
	if !reflect.DeepEqual(got.table.Columns(), vaultResultCols) {
		t.Fatalf("columns = %#v, want %#v", got.table.Columns(), vaultResultCols)
	}
	if got.table.Width() != viewBuilder.ColsWidth(vaultResultCols) {
		t.Fatalf("table width = %d, want %d", got.table.Width(), viewBuilder.ColsWidth(vaultResultCols))
	}
	if !reflect.DeepEqual(got.table.Rows(), []table.Row{{"Forma Blueprint", "9.5", "14"}}) {
		t.Fatalf("rows = %#v", got.table.Rows())
	}
	if got.table.Cursor() != 0 {
		t.Fatalf("cursor = %d, want 0", got.table.Cursor())
	}
	if got.progress.Percent() != 1.0 {
		t.Fatalf("progress percent = %v, want 1.0", got.progress.Percent())
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want animation cmd")
	}
	_ = cmd()
}

func TestUpdateCanceledMsg(t *testing.T) {
	m := NewModel().(Model)
	chooseBase(t, &m.base, len(options))
	m.base.Done = true
	m.base.Quitting = true
	m.base.Steps = []string{"Loading", "Done"}
	m.fetching = true
	m.table.SetColumns(vaultResultCols)
	m.table.SetWidth(123)
	m.table.SetRows([]table.Row{{"keep me"}})

	updated, cmd := m.Update(viewBuilder.CanceledMsg{})
	got := updated.(Model)

	if cmd != nil {
		t.Fatal("cmd should be nil on cancel")
	}
	if got.base.Chosen || got.base.Done || got.base.Quitting {
		t.Fatalf("base reset failed: %#v", got.base)
	}
	if got.base.Cancel != nil || got.base.Steps != nil {
		t.Fatalf("cancel/steps not cleared: cancel=%v steps=%#v", got.base.Cancel != nil, got.base.Steps)
	}
	if got.fetching {
		t.Fatal("fetching = true, want false")
	}
	if len(got.table.Rows()) != 0 {
		t.Fatalf("rows len = %d, want 0", len(got.table.Rows()))
	}
	if !reflect.DeepEqual(got.table.Columns(), resultCols) {
		t.Fatalf("columns = %#v, want %#v", got.table.Columns(), resultCols)
	}
	if got.table.Width() != viewBuilder.ColsWidth(resultCols) {
		t.Fatalf("table width = %d, want %d", got.table.Width(), viewBuilder.ColsWidth(resultCols))
	}
}

func TestUpdateAvailableSizeMsg(t *testing.T) {
	tests := []struct {
		name       string
		height     int
		width      int
		wantHeight int
		startWidth int
		wantWidth  int
	}{
		{name: "updates table and progress width", height: 3, width: 9, wantHeight: 2, startWidth: 0, wantWidth: 6},
		{name: "zero width keeps progress width", height: 0, width: 0, wantHeight: 0, startWidth: 11, wantWidth: 11},
		{name: "negative width keeps progress width", height: 1, width: -4, wantHeight: 0, startWidth: 11, wantWidth: 11},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel().(Model)
			if tt.startWidth > 0 {
				m.progress.SetWidth(tt.startWidth)
			}

			updated, cmd := m.Update(viewBuilder.AvailableSizeMsg{Height: tt.height, Width: tt.width})
			got := updated.(Model)

			if cmd != nil {
				t.Fatal("cmd should be nil for size update")
			}
			if got.table.Height() != tt.wantHeight {
				t.Fatalf("table height = %d, want %d", got.table.Height(), tt.wantHeight)
			}
			if got.progress.Width() != tt.wantWidth {
				t.Fatalf("progress width = %d, want %d", got.progress.Width(), tt.wantWidth)
			}
		})
	}
}

func TestUpdateFetchItemsMsg(t *testing.T) {
	tests := []struct {
		name        string
		items       []pricechecker.GameObject
		index       int
		wantPercent float64
	}{
		{name: "no items keeps zero progress", wantPercent: 0},
		{name: "first item boundary", items: []pricechecker.GameObject{{Name: "A"}, {Name: "B"}, {Name: "C"}, {Name: "D"}}, index: 0, wantPercent: 0},
		{name: "middle item boundary", items: []pricechecker.GameObject{{Name: "A"}, {Name: "B"}, {Name: "C"}, {Name: "D"}}, index: 2, wantPercent: 0.5},
		{name: "final item boundary", items: []pricechecker.GameObject{{Name: "A"}, {Name: "B"}, {Name: "C"}, {Name: "D"}}, index: 3, wantPercent: 0.75},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel().(Model)
			m.ctx = context.Background()
			msg := fetchItemsMsg{
				mode:  pricecheckMode,
				items: tt.items,
				index: tt.index,
			}

			updated, cmd := m.Update(msg)
			got := updated.(Model)

			if !got.fetching {
				t.Fatal("fetching = false, want true")
			}
			if got.progress.Percent() != tt.wantPercent {
				t.Fatalf("progress percent = %v, want %v", got.progress.Percent(), tt.wantPercent)
			}
			if len(got.table.Rows()) != 0 {
				t.Fatalf("rows len = %d, want 0", len(got.table.Rows()))
			}
			if cmd == nil {
				t.Fatal("fetchItemsMsg should return non-nil cmd")
			}
		})
	}
}

func TestUpdateFetchItemsMsgReturnsBatchWithFetchAndAnimation(t *testing.T) {
	m := NewModel().(Model)
	m.ctx = context.Background()

	_, cmd := m.Update(fetchItemsMsg{
		mode:  pricecheckMode,
		items: []pricechecker.GameObject{{Name: "A"}},
	})
	if cmd == nil {
		t.Fatal("fetchItemsMsg should return non-nil cmd")
	}

	msg := cmd()
	v := reflect.ValueOf(msg)
	if !v.IsValid() || v.Kind() != reflect.Slice {
		t.Fatalf("cmd() = %T, want slice batch", msg)
	}
	if v.Len() != 2 {
		t.Fatalf("batch len = %d, want 2", v.Len())
	}
	for i := 0; i < v.Len(); i++ {
		if _, ok := v.Index(i).Interface().(tea.Cmd); !ok {
			t.Fatalf("batch item %d = %T, want tea.Cmd", i, v.Index(i).Interface())
		}
	}
}

func TestRunOperation(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*testing.T, *sql.DB, string)
		choice int
		want   []tea.Msg
	}{
		{
			name:   "inventory price check uses home database",
			choice: 0,
			setup: func(t *testing.T, db *sql.DB, home string) {
				insertMarketItem(t, db, "mods", pricechecker.GameObject{Name: "Owned Mod", GameRef: "owned-ref", Slug: "owned-mod"})
				writeInventoryFixture(t, home, `{"RawUpgrades":[{"ItemType":"owned-ref","ItemCount":1}]}`)
			},
			want: []tea.Msg{
				viewBuilder.StepMsg{Text: "Reading inventory file"},
				viewBuilder.StepMsg{Text: "Loading database items"},
				viewBuilder.StepMsg{Text: "Fetching market prices"},
				startFetch(pricecheckMode, []pricechecker.GameObject{{Name: "Owned Mod", GameRef: "owned-ref", Slug: "owned-mod", Category: "mods"}}),
			},
		},
		{
			name:   "ducat scan uses home database",
			choice: 1,
			setup: func(t *testing.T, db *sql.DB, home string) {
				insertMarketItem(t, db, "weapons", pricechecker.GameObject{Name: "Prime Part", GameRef: "ducat-ref", Slug: "prime-part", Ducats: 45})
				writeInventoryFixture(t, home, `{"MiscItems":[{"ItemType":"ducat-ref","ItemCount":1}]}`)
			},
			want: []tea.Msg{
				viewBuilder.StepMsg{Text: "Reading inventory file"},
				viewBuilder.StepMsg{Text: "Loading database items"},
				viewBuilder.StepMsg{Text: "Fetching market prices"},
				startFetch(ducatMode, []pricechecker.GameObject{{Name: "Prime Part", GameRef: "ducat-ref", Slug: "prime-part", Ducats: 45}}),
			},
		},
		{
			name:   "syndicate scan uses home database",
			choice: 2,
			setup: func(t *testing.T, db *sql.DB, _ string) {
				insertMarketItem(t, db, "mods", pricechecker.GameObject{Name: "Syndicate Mod", GameRef: "synd-ref", Slug: "synd-mod"})
				insertSyndicateLink(t, db, "synd-ref", "mods", "Steel Meridian")
			},
			want: []tea.Msg{
				viewBuilder.StepMsg{Text: "Loading syndicate items"},
				viewBuilder.StepMsg{Text: "Fetching market prices"},
				startFetch(syndicateMode, []pricechecker.GameObject{{Name: "Syndicate Mod", GameRef: "synd-ref", Slug: "synd-mod", Category: "mods", Syndicates: "Steel Meridian"}}),
			},
		},
		{
			name:   "nightmare scan uses home database",
			choice: 3,
			setup: func(t *testing.T, db *sql.DB, _ string) {
				insertMarketItem(t, db, "mods", pricechecker.GameObject{Name: "Nightmare Mod", GameRef: "nightmare-ref", Slug: "nightmare-mod"})
				insertNightmareDrop(t, db, "nightmare-ref", "Nightmare Mod", "C", 4.25)
			},
			want: []tea.Msg{
				viewBuilder.StepMsg{Text: "Loading nightmare mods"},
				viewBuilder.StepMsg{Text: "Fetching market prices"},
				startFetch(nightmareMode, []pricechecker.GameObject{{Name: "Nightmare Mod", GameRef: "nightmare-ref", Slug: "nightmare-mod", Rotation: "C", Chance: 4.25}}),
			},
		},
		{
			name:   "vault scan uses home database",
			choice: 4,
			setup: func(t *testing.T, db *sql.DB, _ string) {
				insertMarketItem(t, db, "mods", pricechecker.GameObject{Name: "Vault Mod", GameRef: "vault-ref", Slug: "vault-mod"})
				insertVaultDrop(t, db, "vault-ref", "Vault Mod", 1.75)
			},
			want: []tea.Msg{
				viewBuilder.StepMsg{Text: "Loading vault mods"},
				viewBuilder.StepMsg{Text: "Fetching market prices"},
				startFetch(vaultMode, []pricechecker.GameObject{{Name: "Vault Mod", GameRef: "vault-ref", Slug: "vault-mod", Chance: 1.75}}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := setupPricecheckHome(t)
			dbPath := filepath.Join(home, ".warframe-helper", "warframe.db")
			db := openPricecheckDB(t, dbPath)
			tt.setup(t, db, home)

			msgs := runSequenceSteps(t, runOperation(tt.choice, context.Background()), nil)
			if !reflect.DeepEqual(msgs, tt.want) {
				t.Fatalf("msgs = %#v, want %#v", msgs, tt.want)
			}
		})
	}
}

func TestRunOperationRequiresInventoryFile(t *testing.T) {
	home := setupPricecheckHome(t)
	dbPath := filepath.Join(home, ".warframe-helper", "warframe.db")
	_ = openPricecheckDB(t, dbPath)

	for _, choice := range []int{0, 1} {
		t.Run(fmt.Sprintf("choice %d", choice), func(t *testing.T) {
			msgs := runCmd(t, runOperation(choice, context.Background()))
			want := []tea.Msg{viewBuilder.ErrorMsg{Text: filepath.Join(home, ".warframe-helper", "inventory.json") + " is required for this choice"}}
			if !reflect.DeepEqual(msgs, want) {
				t.Fatalf("msgs = %#v, want %#v", msgs, want)
			}
		})
	}
}

func TestRunOperationErrorsAndCancel(t *testing.T) {
	t.Run("owned query failure stops before fetch", func(t *testing.T) {
		home := setupPricecheckHome(t)
		writeInventoryFixture(t, home, `{"RawUpgrades":[{"ItemType":"owned-ref","ItemCount":1}]}`)

		msgs := runSequenceSteps(t, runOperation(0, context.Background()), nil)
		wantPrefix := []tea.Msg{
			viewBuilder.StepMsg{Text: "Reading inventory file"},
			viewBuilder.StepMsg{Text: "Loading database items"},
		}
		if len(msgs) != 3 {
			t.Fatalf("msgs len = %d, want 3", len(msgs))
		}
		if !reflect.DeepEqual(msgs[:2], wantPrefix) {
			t.Fatalf("msgs prefix = %#v, want %#v", msgs[:2], wantPrefix)
		}
		if _, ok := msgs[2].(viewBuilder.ErrorMsg); !ok {
			t.Fatalf("msgs[2] = %T, want ErrorMsg", msgs[2])
		}
	})

	t.Run("non-owned query failure stops before fetch", func(t *testing.T) {
		setupPricecheckHome(t)

		msgs := runSequenceSteps(t, runOperation(4, context.Background()), nil)
		if len(msgs) != 2 {
			t.Fatalf("msgs len = %d, want 2", len(msgs))
		}
		if !reflect.DeepEqual(msgs[:1], []tea.Msg{viewBuilder.StepMsg{Text: "Loading vault mods"}}) {
			t.Fatalf("msgs prefix = %#v", msgs[:1])
		}
		if _, ok := msgs[1].(viewBuilder.ErrorMsg); !ok {
			t.Fatalf("msgs[1] = %T, want ErrorMsg", msgs[1])
		}
	})

	t.Run("owned cancel after load returns canceled messages", func(t *testing.T) {
		home := setupPricecheckHome(t)
		dbPath := filepath.Join(home, ".warframe-helper", "warframe.db")
		db := openPricecheckDB(t, dbPath)
		insertMarketItem(t, db, "mods", pricechecker.GameObject{Name: "Owned", GameRef: "owned-ref", Slug: "owned-ref"})
		writeInventoryFixture(t, home, `{"RawUpgrades":[{"ItemType":"owned-ref","ItemCount":1}]}`)
		ctx, cancel := context.WithCancel(context.Background())
		msgs := runSequenceSteps(t, runOperation(0, ctx), func(step int, _ []tea.Msg) {
			if step == 1 {
				cancel()
			}
		})

		want := []tea.Msg{
			viewBuilder.StepMsg{Text: "Reading inventory file"},
			viewBuilder.StepMsg{Text: "Loading database items"},
			viewBuilder.CanceledMsg{},
			viewBuilder.CanceledMsg{},
		}
		if !reflect.DeepEqual(msgs, want) {
			t.Fatalf("msgs = %#v, want %#v", msgs, want)
		}
	})

	t.Run("non-owned cancel after load returns canceled messages", func(t *testing.T) {
		home := setupPricecheckHome(t)
		dbPath := filepath.Join(home, ".warframe-helper", "warframe.db")
		db := openPricecheckDB(t, dbPath)
		insertMarketItem(t, db, "mods", pricechecker.GameObject{Name: "A", GameRef: "vault-a", Slug: "vault-a"})
		insertVaultDrop(t, db, "vault-a", "A", 2.5)
		ctx, cancel := context.WithCancel(context.Background())
		msgs := runSequenceSteps(t, runOperation(4, ctx), func(step int, _ []tea.Msg) {
			if step == 0 {
				cancel()
			}
		})

		want := []tea.Msg{
			viewBuilder.StepMsg{Text: "Loading vault mods"},
			viewBuilder.CanceledMsg{},
			viewBuilder.CanceledMsg{},
		}
		if !reflect.DeepEqual(msgs, want) {
			t.Fatalf("msgs = %#v, want %#v", msgs, want)
		}
	})
}

func TestUpdateQuitFromChosenDoneStateResetsToMenu(t *testing.T) {
	m := NewModel().(Model)
	m.base.Chosen = true
	m.base.Done = true
	m.base.Steps = []string{"Loading", "Done"}
	m.table.SetColumns(vaultResultCols)
	m.table.SetRows([]table.Row{{"keep me"}})

	updated, cmd := m.Update(tea.KeyPressMsg{Text: "q"})
	got := updated.(Model)

	if cmd != nil {
		t.Fatal("quit should return nil cmd")
	}
	if got.base.Chosen || got.base.Done || got.base.Cancel != nil {
		t.Fatalf("base not reset: %#v", got.base)
	}
	if got.base.Steps != nil {
		t.Fatalf("steps = %#v, want nil", got.base.Steps)
	}
	if len(got.table.Rows()) != 0 {
		t.Fatalf("rows len = %d, want 0", len(got.table.Rows()))
	}
	if !reflect.DeepEqual(got.table.Columns(), resultCols) {
		t.Fatalf("columns = %#v, want %#v", got.table.Columns(), resultCols)
	}
	if got.table.Width() != viewBuilder.ColsWidth(resultCols) {
		t.Fatalf("table width = %d, want %d", got.table.Width(), viewBuilder.ColsWidth(resultCols))
	}
}

func TestUpdateDoneStateNavigationSupportsUpAndDown(t *testing.T) {
	m := NewModel().(Model)
	m.base.Chosen = true
	m.base.Done = true
	m.table.SetRows([]table.Row{{"first", "1", "1"}, {"second", "2", "2"}, {"third", "3", "3"}})

	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	got := updated.(Model)
	if got.table.Cursor() != 1 {
		t.Fatalf("cursor after down = %d, want 1", got.table.Cursor())
	}
	if cmd != nil {
		_ = cmd()
	}

	updated, cmd = got.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	got = updated.(Model)
	if got.table.Cursor() != 0 {
		t.Fatalf("cursor after up = %d, want 0", got.table.Cursor())
	}
	if cmd != nil {
		_ = cmd()
	}
}

func TestViewBranches(t *testing.T) {
	t.Run("menu", func(t *testing.T) {
		setupPricecheckHome(t)
		m := NewModel().(Model)
		got := m.View()
		wantLines := []string{
			"[ ] Inventory (requires inventory.json)",
			"[ ] Ducats (requires inventory.json)",
			"[x] Syndicates",
			"[ ] Nightmare Mods",
			"[ ] Corrupted Mods",
		}
		if lines := viewLines(got); !reflect.DeepEqual(lines, wantLines) {
			t.Fatalf("View() lines = %#v, want %#v", lines, wantLines)
		}
		if !strings.Contains(got, disabledChoiceStyle.Render("[ ] Inventory"+inventoryRequiredSuffix)) {
			t.Fatalf("View() missing disabled inventory styling: %q", got)
		}
		if !strings.Contains(got, disabledChoiceStyle.Render("[ ] Ducats"+inventoryRequiredSuffix)) {
			t.Fatalf("View() missing disabled ducats styling: %q", got)
		}
	})

	t.Run("menu with inventory file uses normal choices", func(t *testing.T) {
		home := setupPricecheckHome(t)
		writeInventoryFixture(t, home, `{"RawUpgrades":[]}`)
		m := NewModel().(Model)
		if got := m.View(); got != viewBuilder.ChoicesView(0, options) {
			t.Fatalf("View() = %q, want normal choices view", got)
		}
	})

	t.Run("progress without fetch bar", func(t *testing.T) {
		m := NewModel().(Model)
		m.base.Chosen = true
		m.base.Steps = []string{"Loading"}
		got := m.View()
		want := viewBuilder.ProgressView(m.base.Steps, m.base.Spinner.View(), false, false)
		if got != want {
			t.Fatalf("View() = %q, want %q", got, want)
		}
	})

	t.Run("progress with fetch bar", func(t *testing.T) {
		m := NewModel().(Model)
		m.base.Chosen = true
		m.base.Steps = []string{"Loading", "Fetching market prices"}
		m.fetching = true
		got := m.View()
		want := viewBuilder.ProgressView(m.base.Steps, m.base.Spinner.View(), false, false) + "\n" + m.progress.View() + "\n"
		if got != want {
			t.Fatalf("View() = %q, want %q", got, want)
		}
	})

	t.Run("done returns table", func(t *testing.T) {
		m := NewModel().(Model)
		m.base.Chosen = true
		m.base.Done = true
		m.table.SetRows([]table.Row{{"Forma Blueprint", "9.5", "14"}})
		if got := m.View(); got != m.table.View() {
			t.Fatalf("View() = %q, want table view", got)
		}
	})
}

func TestMenuNavigationSkipsInventoryRequiredChoices(t *testing.T) {
	setupPricecheckHome(t)
	m := NewModel().(Model)

	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd != nil {
		t.Fatal("down should not return cmd")
	}
	got := updated.(Model)
	if got.base.Choice != 3 {
		t.Fatalf("choice after down = %d, want 3", got.base.Choice)
	}

	updated, cmd = got.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if cmd != nil {
		t.Fatal("up should not return cmd")
	}
	got = updated.(Model)
	if got.base.Choice != 2 {
		t.Fatalf("choice after up = %d, want 2", got.base.Choice)
	}
}

func TestMenuEnterStartsFirstEnabledChoiceWhenInventoryMissing(t *testing.T) {
	home := setupPricecheckHome(t)
	dbPath := filepath.Join(home, ".warframe-helper", "warframe.db")
	db := openPricecheckDB(t, dbPath)
	insertMarketItem(t, db, "mods", pricechecker.GameObject{Name: "Syndicate Mod", GameRef: "synd-ref", Slug: "synd-mod"})
	insertSyndicateLink(t, db, "synd-ref", "mods", "Steel Meridian")

	m := NewModel().(Model)
	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := updated.(Model)
	if !got.base.Chosen {
		t.Fatal("enter should choose first enabled option")
	}
	if got.base.Choice != 2 {
		t.Fatalf("choice after enter = %d, want 2", got.base.Choice)
	}
	msgs := runSequenceSteps(t, cmd, nil)
	want := []tea.Msg{
		viewBuilder.StepMsg{Text: "Loading syndicate items"},
		viewBuilder.StepMsg{Text: "Fetching market prices"},
		startFetch(syndicateMode, []pricechecker.GameObject{{Name: "Syndicate Mod", GameRef: "synd-ref", Slug: "synd-mod", Category: "mods", Syndicates: "Steel Meridian"}}),
	}
	if !reflect.DeepEqual(msgs, want) {
		t.Fatalf("msgs = %#v, want %#v", msgs, want)
	}
}
