package fetchdata

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gjrud/warframe-helper/utils/database"
	"github.com/gjrud/warframe-helper/utils/viewBuilder"
)

func setUserHomeEnv(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

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
			cmd, ok := rv.Index(i).Interface().(tea.Cmd)
			if !ok {
				t.Fatalf("slice element %d = %T, want tea.Cmd", i, rv.Index(i).Interface())
			}
			msgs = append(msgs, runCmd(t, cmd)...)
		}
		return msgs
	}

	return []tea.Msg{msg}
}

func setupSyncHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	setUserHomeEnv(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".warframe-helper"), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	return filepath.Join(home, ".warframe-helper", "warframe.db")
}

func insertSyndicateItems(t *testing.T, db *sql.DB, itemType string, refs ...string) {
	t.Helper()

	for _, ref := range refs {
		if _, err := db.Exec(`INSERT INTO syndicate_items(item_gameRef, syndicate_id, item_type) VALUES (?, 1, ?)`, ref, itemType); err != nil {
			t.Fatalf("insert syndicate item %q error = %v", ref, err)
		}
	}
}

func readDatabaseFixture(t *testing.T, name string) string {
	t.Helper()

	body, err := os.ReadFile(filepath.Join("..", "..", "testdata", "database", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(body)
}

func withDefaultTransport(t *testing.T, fn roundTripperFunc) {
	t.Helper()

	prev := http.DefaultTransport
	http.DefaultTransport = fn
	t.Cleanup(func() { http.DefaultTransport = prev })
}

func httpResponse(req *http.Request, body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

func mustCountRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()

	var count int
	if err := db.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatalf("QueryRow(%q) error = %v", query, err)
	}
	return count
}

func mustDucats(t *testing.T, db *sql.DB, table string, gameRef string) int {
	t.Helper()

	var ducats int
	if err := db.QueryRow(`SELECT COALESCE(ducats, 0) FROM `+table+` WHERE gameRef = ?`, gameRef).Scan(&ducats); err != nil {
		t.Fatalf("ducat lookup %s/%s error = %v", table, gameRef, err)
	}
	return ducats
}

func openStatusDB(t *testing.T, path string) *sql.DB {
	t.Helper()

	db, err := database.OpenDB(path)
	if err != nil {
		t.Fatalf("OpenDB() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func updatedModel(t *testing.T, sub viewBuilder.SubView) Model {
	t.Helper()

	m, ok := sub.(Model)
	if !ok {
		t.Fatalf("Update() returned %T, want fetchdata.Model", sub)
	}
	return m
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

type integrationModel struct {
	inner Model
	stop  func(Model) bool
}

func (m *integrationModel) Init() tea.Cmd {
	return m.inner.Init()
}

func (m *integrationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	sub, cmd := m.inner.Update(msg)
	next, ok := sub.(Model)
	if !ok {
		panic("fetchdata update returned non-Model")
	}
	m.inner = next
	if m.stop != nil && m.stop(m.inner) {
		return m, tea.Quit
	}
	return m, cmd
}

func (m *integrationModel) View() tea.View {
	return tea.NewView(m.inner.View())
}

func runProgram(t *testing.T, model *integrationModel, send func(*tea.Program)) Model {
	t.Helper()

	p := tea.NewProgram(model, tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignals())
	errCh := make(chan error, 1)
	go func() {
		_, err := p.Run()
		errCh <- err
	}()
	if send != nil {
		send(p)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("program.Run() error = %v", err)
	}
	return model.inner
}

func TestShortSHA(t *testing.T) {
	if got := shortSHA("abcdef123456"); got != "abcdef1" {
		t.Fatalf("shortSHA(long) = %q, want %q", got, "abcdef1")
	}
	if got := shortSHA("abcdef1"); got != "abcdef1" {
		t.Fatalf("shortSHA(exact) = %q, want %q", got, "abcdef1")
	}
	if got := shortSHA("abc123"); got != "abc123" {
		t.Fatalf("shortSHA(short) = %q, want %q", got, "abc123")
	}
	if got := shortSHA(""); got != "" {
		t.Fatalf("shortSHA(empty) = %q, want empty string", got)
	}
}

func TestSyndicateResultMsg(t *testing.T) {
	tests := []struct {
		name string
		msg  any
		want any
	}{
		{
			name: "alert when empty",
			msg:  syndicateResultMsg("Steel Meridian", 5, 0),
			want: viewBuilder.AlertMsg{Text: "Steel Meridian syndicate rows: 5 -> 0 (-5)"},
		},
		{
			name: "warn when reduced",
			msg:  syndicateResultMsg("Steel Meridian", 5, 3),
			want: viewBuilder.WarnMsg{Text: "Steel Meridian syndicate rows: 5 -> 3 (-2)"},
		},
		{
			name: "step when same or larger",
			msg:  syndicateResultMsg("Steel Meridian", 5, 8),
			want: viewBuilder.StepMsg{Text: "Steel Meridian syndicate rows: 5 -> 8 (+3)"},
		},
	}

	for _, tt := range tests {
		if !reflect.DeepEqual(tt.msg, tt.want) {
			t.Fatalf("%s: got %#v, want %#v", tt.name, tt.msg, tt.want)
		}
	}
}

func TestStatusLine(t *testing.T) {
	t.Run("checking", func(t *testing.T) {
		m := Model{status: statusChecking}
		if got := m.statusLine(); got != "Checking upstream status..." {
			t.Fatalf("statusLine() = %q, want checking message", got)
		}
	})

	t.Run("up to date", func(t *testing.T) {
		m := Model{status: statusUpToDate, storedSHA: "abcdef123456"}
		want := viewBuilder.SuccessStyle.Render("Database up to date: abcdef1")
		if got := m.statusLine(); got != want {
			t.Fatalf("statusLine() = %q, want %q", got, want)
		}
	})

	t.Run("stale", func(t *testing.T) {
		m := Model{status: statusStale, storedSHA: "abcdef123456", latestSHA: "123456789abc"}
		want := viewBuilder.WarningStyle.Render("Database stale: abcdef1 -> 1234567")
		if got := m.statusLine(); got != want {
			t.Fatalf("statusLine() = %q, want %q", got, want)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		m := Model{status: statusUnknown}
		want := viewBuilder.WarningStyle.Render("Database status unknown")
		if got := m.statusLine(); got != want {
			t.Fatalf("statusLine() = %q, want %q", got, want)
		}
	})

	t.Run("missing", func(t *testing.T) {
		m := Model{status: statusMissing}
		want := viewBuilder.ErrorStyle.Render("Database not present")
		if got := m.statusLine(); got != want {
			t.Fatalf("statusLine() = %q, want %q", got, want)
		}
	})
}

func TestLastSyncLine(t *testing.T) {
	t.Run("unknown", func(t *testing.T) {
		m := Model{}
		if got := m.lastSyncLine(); got != "Last sync: unknown" {
			t.Fatalf("lastSyncLine() = %q, want %q", got, "Last sync: unknown")
		}
	})

	t.Run("utc formatted", func(t *testing.T) {
		loc := time.FixedZone("UTC+2", 2*60*60)
		m := Model{
			hasLastSync: true,
			lastSync:    time.Date(2026, time.April, 17, 12, 34, 0, 0, loc),
		}
		if got := m.lastSyncLine(); got != "Last sync: 2026-04-17 10:34 UTC" {
			t.Fatalf("lastSyncLine() = %q, want %q", got, "Last sync: 2026-04-17 10:34 UTC")
		}
	})
}

func TestView(t *testing.T) {
	t.Run("menu view shows status choices and last sync", func(t *testing.T) {
		m := Model{
			base:        viewBuilder.NewBaseModel(),
			status:      statusStale,
			storedSHA:   "abcdef123456",
			latestSHA:   "123456789abc",
			hasLastSync: true,
			lastSync:    time.Date(2026, time.April, 17, 12, 34, 0, 0, time.UTC),
		}

		view := m.View()
		if !strings.Contains(view, "Database stale: abcdef1 -> 1234567") {
			t.Fatalf("View() = %q, want stale status", view)
		}
		if !strings.Contains(view, "Last sync: 2026-04-17 12:34 UTC") {
			t.Fatalf("View() = %q, want last sync", view)
		}
		if !strings.Contains(view, "Update") {
			t.Fatalf("View() = %q, want menu option", view)
		}
	})

	t.Run("progress view shows running steps", func(t *testing.T) {
		base := viewBuilder.NewBaseModel()
		base.Chosen = true
		base.Steps = []string{"Resolving WFCD snapshot"}

		view := (Model{base: base}).View()
		if !strings.Contains(view, "Resolving WFCD snapshot") {
			t.Fatalf("View() = %q, want progress step", view)
		}
	})
}

func TestDBPath(t *testing.T) {
	home := t.TempDir()
	setUserHomeEnv(t, home)

	got, err := dbPath()
	if err != nil {
		t.Fatalf("dbPath() error = %v", err)
	}

	want := filepath.Join(home, ".warframe-helper", "warframe.db")
	if got != want {
		t.Fatalf("dbPath() = %q, want %q", got, want)
	}
}

func TestLoadStatusCmd(t *testing.T) {
	t.Run("missing database reports missing local state", func(t *testing.T) {
		setUserHomeEnv(t, t.TempDir())

		msg, ok := loadStatusCmd()().(localStatusMsg)
		if !ok {
			t.Fatalf("loadStatusCmd()() = %T, want localStatusMsg", loadStatusCmd()())
		}
		if msg.Exists {
			t.Fatal("missing database unexpectedly marked existing")
		}
		if msg.Err != nil {
			t.Fatalf("missing database err = %v, want nil", msg.Err)
		}
		if msg.Status != (database.SyncStatus{}) {
			t.Fatalf("missing database status = %#v, want zero value", msg.Status)
		}
	})

	t.Run("invalid database reports error", func(t *testing.T) {
		home := t.TempDir()
		setUserHomeEnv(t, home)

		path := filepath.Join(home, ".warframe-helper", "warframe.db")
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		if err := os.WriteFile(path, []byte("not sqlite"), 0644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		msg, ok := loadStatusCmd()().(localStatusMsg)
		if !ok {
			t.Fatalf("loadStatusCmd()() = %T, want localStatusMsg", loadStatusCmd()())
		}
		if !msg.Exists {
			t.Fatal("invalid database not marked existing")
		}
		if msg.Err == nil {
			t.Fatal("invalid database err = nil, want error")
		}
	})

	t.Run("valid status loads stored sha and sync time", func(t *testing.T) {
		home := t.TempDir()
		setUserHomeEnv(t, home)

		path := filepath.Join(home, ".warframe-helper", "warframe.db")
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}

		db := openStatusDB(t, path)
		wantTime := time.Date(2026, time.April, 17, 12, 34, 0, 0, time.FixedZone("UTC+2", 2*60*60))
		if err := database.WriteSyncStatus(context.Background(), db, database.SyncStatus{
			HasLastCompletedAt: true,
			LastCompletedAt:    wantTime,
			WFCDCommitSHA:      "abcdef123456",
		}); err != nil {
			t.Fatalf("WriteSyncStatus() error = %v", err)
		}

		msg, ok := loadStatusCmd()().(localStatusMsg)
		if !ok {
			t.Fatalf("loadStatusCmd()() = %T, want localStatusMsg", loadStatusCmd()())
		}
		if !msg.Exists {
			t.Fatal("valid database not marked existing")
		}
		if msg.Err != nil {
			t.Fatalf("valid database err = %v, want nil", msg.Err)
		}
		wantStatus := database.SyncStatus{
			Exists:             true,
			HasLastCompletedAt: true,
			LastCompletedAt:    time.Date(2026, time.April, 17, 10, 34, 0, 0, time.UTC),
			WFCDCommitSHA:      "abcdef123456",
		}
		if msg.Status != wantStatus {
			t.Fatalf("loaded status = %#v, want %#v", msg.Status, wantStatus)
		}
	})
}

func TestModelUpdateLocalStatus(t *testing.T) {
	lastSync := time.Date(2026, time.April, 17, 12, 34, 0, 0, time.UTC)

	t.Run("missing local status becomes missing", func(t *testing.T) {
		m := Model{
			status:      statusStale,
			storedSHA:   "old",
			latestSHA:   "new",
			hasLastSync: true,
			lastSync:    lastSync,
		}

		updated, cmd := m.Update(localStatusMsg{})
		got := updatedModel(t, updated)

		if cmd != nil {
			t.Fatal("missing local status returned cmd")
		}
		if got.status != statusMissing {
			t.Fatalf("status = %v, want %v", got.status, statusMissing)
		}
		if got.storedSHA != "" || got.latestSHA != "" || got.hasLastSync || !got.lastSync.IsZero() {
			t.Fatalf("missing local status did not clear state: %+v", got)
		}
	})

	t.Run("invalid local status becomes unknown", func(t *testing.T) {
		m := Model{
			status:      statusStale,
			storedSHA:   "old",
			latestSHA:   "new",
			hasLastSync: true,
			lastSync:    lastSync,
		}

		updated, cmd := m.Update(localStatusMsg{Exists: true, Err: os.ErrPermission})
		got := updatedModel(t, updated)

		if cmd != nil {
			t.Fatal("invalid local status returned cmd")
		}
		if got.status != statusUnknown {
			t.Fatalf("status = %v, want %v", got.status, statusUnknown)
		}
		if got.storedSHA != "" || got.latestSHA != "" || got.hasLastSync || !got.lastSync.IsZero() {
			t.Fatalf("invalid local status did not clear state: %+v", got)
		}
	})

	t.Run("valid local status triggers upstream check", func(t *testing.T) {
		wantTime := time.Date(2026, time.April, 17, 12, 34, 0, 0, time.UTC)
		updated, cmd := (Model{}).Update(localStatusMsg{Exists: true, Status: database.SyncStatus{
			Exists:             true,
			HasLastCompletedAt: true,
			LastCompletedAt:    wantTime,
			WFCDCommitSHA:      "abcdef123456",
		}})
		got := updatedModel(t, updated)

		if cmd == nil {
			t.Fatal("valid local status returned nil cmd")
		}
		if got.status != statusChecking {
			t.Fatalf("status = %v, want %v", got.status, statusChecking)
		}
		if got.storedSHA != "abcdef123456" {
			t.Fatalf("storedSHA = %q, want %q", got.storedSHA, "abcdef123456")
		}
		if !got.hasLastSync || !got.lastSync.Equal(wantTime) {
			t.Fatalf("last sync = %v / %v, want true / %v", got.hasLastSync, got.lastSync, wantTime)
		}
	})
}

func TestModelUpdateUpstreamStatus(t *testing.T) {
	t.Run("matching sha becomes up to date", func(t *testing.T) {
		updated, cmd := (Model{storedSHA: "abcdef123456"}).Update(upstreamStatusMsg{SHA: "abcdef123456"})
		got := updatedModel(t, updated)

		if cmd != nil {
			t.Fatal("matching upstream status returned cmd")
		}
		if got.status != statusUpToDate {
			t.Fatalf("status = %v, want %v", got.status, statusUpToDate)
		}
		if got.latestSHA != "abcdef123456" {
			t.Fatalf("latestSHA = %q, want %q", got.latestSHA, "abcdef123456")
		}
	})

	t.Run("different sha becomes stale", func(t *testing.T) {
		updated, cmd := (Model{storedSHA: "abcdef123456"}).Update(upstreamStatusMsg{SHA: "123456789abc"})
		got := updatedModel(t, updated)

		if cmd != nil {
			t.Fatal("different upstream status returned cmd")
		}
		if got.status != statusStale {
			t.Fatalf("status = %v, want %v", got.status, statusStale)
		}
		if got.latestSHA != "123456789abc" {
			t.Fatalf("latestSHA = %q, want %q", got.latestSHA, "123456789abc")
		}
	})

	t.Run("error becomes unknown", func(t *testing.T) {
		updated, cmd := (Model{storedSHA: "abcdef123456", latestSHA: "stale"}).Update(upstreamStatusMsg{Err: os.ErrDeadlineExceeded})
		got := updatedModel(t, updated)

		if cmd != nil {
			t.Fatal("errored upstream status returned cmd")
		}
		if got.status != statusUnknown {
			t.Fatalf("status = %v, want %v", got.status, statusUnknown)
		}
		if got.latestSHA != "" {
			t.Fatalf("latestSHA = %q, want empty", got.latestSHA)
		}
	})
}

func TestModelUpdateSyncComplete(t *testing.T) {
	base := viewBuilder.NewBaseModel()
	chooseBase(t, &base, 1)
	base.Steps = []string{"Saving sync status"}
	updated, cmd := (Model{base: base, status: statusChecking}).Update(syncCompleteMsg{
		At:  time.Date(2026, time.April, 17, 12, 34, 0, 0, time.FixedZone("UTC+2", 2*60*60)),
		SHA: "abcdef123456",
	})
	got := updatedModel(t, updated)

	if cmd != nil {
		t.Fatal("syncCompleteMsg returned cmd")
	}
	if got.status != statusUpToDate {
		t.Fatalf("status = %v, want %v", got.status, statusUpToDate)
	}
	if got.storedSHA != "abcdef123456" || got.latestSHA != "abcdef123456" {
		t.Fatalf("sha state = %q / %q, want both %q", got.storedSHA, got.latestSHA, "abcdef123456")
	}
	if !got.hasLastSync || !got.lastSync.Equal(time.Date(2026, time.April, 17, 10, 34, 0, 0, time.UTC)) {
		t.Fatalf("last sync = %v / %v, want UTC timestamp", got.hasLastSync, got.lastSync)
	}
	if !reflect.DeepEqual(got.base.Steps, []string{"Saving sync status", "Done"}) {
		t.Fatalf("steps = %#v, want done appended", got.base.Steps)
	}
	if !got.base.Done {
		t.Fatal("syncCompleteMsg did not mark base done")
	}
	if got.base.Cancel != nil {
		t.Fatal("syncCompleteMsg did not clear cancel")
	}
}

func TestModelUpdatePassesChoiceEnterToBase(t *testing.T) {
	updated, cmd := (Model{base: viewBuilder.NewBaseModel()}).Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := updatedModel(t, updated)

	if !got.base.Chosen {
		t.Fatal("enter did not mark model chosen")
	}
	if got.base.Cancel == nil {
		t.Fatal("enter did not store cancel func")
	}
	if cmd == nil {
		t.Fatal("enter returned nil cmd")
	}
}

func TestSyndicateStepCmd(t *testing.T) {
	t.Run("returns step and alert when work removes all rows", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "warframe.db")
		db := openStatusDB(t, path)
		if err := database.EnsureSyndicates(db); err != nil {
			t.Fatalf("EnsureSyndicates() error = %v", err)
		}
		insertSyndicateItems(t, db, "mods", "mods-a", "mods-b")

		failed := false
		msgs := runCmd(t, syndicateStepCmd(context.Background(), db, "Mods", "mods", "Syncing syndicate data for mods", &failed, func() error {
			_, err := db.Exec(`DELETE FROM syndicate_items WHERE item_type = ?`, "mods")
			return err
		}))

		want := []tea.Msg{
			viewBuilder.StepMsg{Text: "Syncing syndicate data for mods"},
			viewBuilder.AlertMsg{Text: "Mods syndicate rows: 2 -> 0 (-2)"},
		}
		if !reflect.DeepEqual(msgs, want) {
			t.Fatalf("msgs = %#v, want %#v", msgs, want)
		}
		if failed {
			t.Fatal("failed flipped true for successful work")
		}
	})

	t.Run("count error returns error without running work", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "warframe.db")
		db := openStatusDB(t, path)

		failed := false
		called := false
		msgs := runCmd(t, syndicateStepCmd(context.Background(), db, "Mods", "mods", "Syncing syndicate data for mods", &failed, func() error {
			called = true
			return nil
		}))

		if len(msgs) != 1 {
			t.Fatalf("msgs = %#v, want single error", msgs)
		}
		if _, ok := msgs[0].(viewBuilder.ErrorMsg); !ok {
			t.Fatalf("msgs[0] = %T, want viewBuilder.ErrorMsg", msgs[0])
		}
		if !failed {
			t.Fatal("count error did not flip failed")
		}
		if called {
			t.Fatal("work ran after count error")
		}
	})
}

func TestRunSyncCanceledBeforeWork(t *testing.T) {
	path := setupSyncHome(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	msgs := runCmd(t, runSync(ctx))
	if len(msgs) == 0 {
		t.Fatal("runSync() returned no messages")
	}
	for i, msg := range msgs {
		if _, ok := msg.(viewBuilder.CanceledMsg); !ok {
			t.Fatalf("msgs[%d] = %T, want viewBuilder.CanceledMsg", i, msg)
		}
	}

	db := openStatusDB(t, path)
	status, err := database.ReadSyncStatus(db)
	if err != nil {
		t.Fatalf("ReadSyncStatus() error = %v", err)
	}
	if status != (database.SyncStatus{}) {
		t.Fatalf("status = %#v, want zero value", status)
	}
}

func TestRunSyncSetupFailureWhenHomeIsFile(t *testing.T) {
	homeFile := filepath.Join(t.TempDir(), "home.txt")
	if err := os.WriteFile(homeFile, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	setUserHomeEnv(t, homeFile)

	msgs := runCmd(t, runSync(context.Background()))
	if len(msgs) != 1 {
		t.Fatalf("msgs len = %d, want 1", len(msgs))
	}
	errMsg, ok := msgs[0].(viewBuilder.ErrorMsg)
	if !ok {
		t.Fatalf("msgs[0] = %T, want viewBuilder.ErrorMsg", msgs[0])
	}
	text := strings.ToLower(errMsg.Text)
	if !strings.Contains(text, "unable to open database file") || (!strings.Contains(text, "not a directory") && !strings.Contains(text, "system cannot find the path specified")) {
		t.Fatalf("error text = %q, want open-database not-directory error", errMsg.Text)
	}
}

func TestRunSyncSuccess(t *testing.T) {
	path := setupSyncHome(t)
	officialDropsHTML := readDatabaseFixture(t, "official_drops_valid.html")
	withDefaultTransport(t, roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Host == "api.github.com" && req.URL.Path == "/repos/WFCD/warframe-items/commits":
			return httpResponse(req, `[{"sha":"abc123456789"}]`), nil
		case req.URL.Host == "api.warframe.market" && req.URL.Path == "/v2/items":
			return httpResponse(req, `[
				{"gameRef":"/Lotus/Mods/TestModBlueprint","slug":"test-mod","tags":["mod"],"i18n":{"en":{"name":"Test Mod"}}},
				{"gameRef":"/Lotus/Mods/TestModComponent","slug":"test-mod-component","tags":["mod"],"i18n":{"en":{"name":"Test Mod Component"}}},
				{"gameRef":"/Lotus/Mods/Nightmare/Constitution","slug":"constitution","tags":["mod"],"i18n":{"en":{"name":"Constitution"}}},
				{"gameRef":"/Lotus/Mods/Nightmare/HammerShot","slug":"hammer-shot","tags":["mod"],"i18n":{"en":{"name":"Hammer Shot"}}},
				{"gameRef":"/Lotus/Upgrades/Mods/Randomized/BlindRage","slug":"blind-rage","tags":["mod"],"i18n":{"en":{"name":"Blind Rage"}}},
				{"gameRef":"/Lotus/Arcanes/Test","slug":"arcane-test","tags":["arcane_enhancement"],"i18n":{"en":{"name":"Arcane Test"}}},
				{"gameRef":"/Lotus/Weapons/TestWeaponBlueprint","slug":"test-weapon","tags":["primary"],"i18n":{"en":{"name":"Test Weapon"}}},
				{"gameRef":"/Lotus/Weapons/TestWeaponComponent","slug":"test-weapon-component","tags":["primary"],"i18n":{"en":{"name":"Test Weapon Component"}}},
				{"gameRef":"/Lotus/Warframes/TestFrameBlueprint","slug":"test-frame","tags":["warframe","blueprint"],"i18n":{"en":{"name":"Test Frame"}}},
				{"gameRef":"/Lotus/Warframes/TestFrameComponent","slug":"test-frame-component","tags":["warframe","component"],"i18n":{"en":{"name":"Test Frame Component"}}},
				{"gameRef":"/Lotus/Archwing/Test","slug":"test-archwing","tags":["archwing"],"i18n":{"en":{"name":"Test Archwing"}}},
				{"gameRef":"/Lotus/Companions/Test","slug":"test-companion","tags":["sentinel"],"i18n":{"en":{"name":"Test Companion"}}}
			]`), nil
		case req.URL.Host == "raw.githubusercontent.com" && strings.HasSuffix(req.URL.Path, "/Mods.json"):
			return httpResponse(req, `[
				{"uniqueName":"/Lotus/Mods/TestModBlueprint","drops":[{"chance":100,"location":"Cephalon Suda Offering"}]}
			]`), nil
		case req.URL.Host == "raw.githubusercontent.com" && strings.HasSuffix(req.URL.Path, "/Arcanes.json"):
			return httpResponse(req, `[
				{"uniqueName":"/Lotus/Arcanes/Test","drops":[{"chance":100,"location":"The Perrin Sequence Offering"}]}
			]`), nil
		case req.URL.Host == "raw.githubusercontent.com" && strings.HasSuffix(req.URL.Path, "/Primary.json"):
			return httpResponse(req, `[
				{"uniqueName":"/Lotus/Weapons/TestWeaponBlueprint","drops":[{"chance":100,"location":"Steel Meridian Offering"}],"components":[{"tradable":true,"ducats":65,"uniqueName":"/Lotus/Weapons/TestWeaponBlueprint"}]}
			]`), nil
		case req.URL.Host == "raw.githubusercontent.com" && (strings.HasSuffix(req.URL.Path, "/Secondary.json") || strings.HasSuffix(req.URL.Path, "/Melee.json") || strings.HasSuffix(req.URL.Path, "/Arch-Gun.json") || strings.HasSuffix(req.URL.Path, "/Arch-Melee.json")):
			return httpResponse(req, `[]`), nil
		case req.URL.Host == "raw.githubusercontent.com" && strings.HasSuffix(req.URL.Path, "/Warframes.json"):
			return httpResponse(req, `[
				{"components":[{"tradable":true,"ducats":100,"uniqueName":"/Lotus/Warframes/TestFrameBlueprint"}]}
			]`), nil
		case req.URL.Host == "raw.githubusercontent.com" && strings.HasSuffix(req.URL.Path, "/Archwing.json"):
			return httpResponse(req, `[
				{"components":[{"tradable":true,"ducats":45,"uniqueName":"/Lotus/Archwing/Test"}]}
			]`), nil
		case req.URL.Host == "raw.githubusercontent.com" && strings.HasSuffix(req.URL.Path, "/Sentinels.json"):
			return httpResponse(req, `[
				{"components":[{"tradable":true,"ducats":65,"uniqueName":"/Lotus/Companions/Test"}]}
			]`), nil
		case req.URL.Host == "www.warframe.com" && req.URL.Path == "/droptables":
			return httpResponse(req, officialDropsHTML), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s", req.URL.String())
		}
	}))

	before := time.Now().UTC()
	msgs := runCmd(t, runSync(context.Background()))
	after := time.Now().UTC()
	if len(msgs) == 0 {
		t.Fatal("runSync() returned no messages")
	}

	last, ok := msgs[len(msgs)-1].(syncCompleteMsg)
	if !ok {
		t.Fatalf("last msg = %T, want syncCompleteMsg", msgs[len(msgs)-1])
	}
	if last.SHA != "abc123456789" {
		t.Fatalf("syncCompleteMsg.SHA = %q, want %q", last.SHA, "abc123456789")
	}
	if last.At.Before(before) || last.At.After(after) {
		t.Fatalf("syncCompleteMsg.At = %v, want between %v and %v", last.At, before, after)
	}

	texts := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		switch msg := msg.(type) {
		case viewBuilder.StepMsg:
			texts = append(texts, msg.Text)
		case viewBuilder.WarnMsg:
			texts = append(texts, "warn: "+msg.Text)
		case viewBuilder.AlertMsg:
			texts = append(texts, "alert: "+msg.Text)
		}
	}
	wantTexts := []string{
		"Resolving WFCD snapshot",
		"Fetching item catalog from Warframe Market",
		"Syncing syndicate data for mods",
		"Mods syndicate rows: 0 -> 2 (+2)",
		"Syncing syndicate data for arcanes",
		"Arcanes syndicate rows: 0 -> 1 (+1)",
		"Syncing syndicate and ducat data for weapons",
		"Weapons syndicate rows: 0 -> 2 (+2)",
		"Syncing ducat data for warframes, archwing, companions",
		"Syncing official drops for nightmare and vault mods",
		"Saving sync status",
	}
	if !reflect.DeepEqual(texts, wantTexts) {
		t.Fatalf("step texts = %#v, want %#v", texts, wantTexts)
	}

	db := openStatusDB(t, path)
	status, err := database.ReadSyncStatus(db)
	if err != nil {
		t.Fatalf("ReadSyncStatus() error = %v", err)
	}
	if !status.Exists || !status.HasLastCompletedAt || status.WFCDCommitSHA != "abc123456789" {
		t.Fatalf("status = %#v, want completed status with saved sha", status)
	}
	if !status.LastCompletedAt.Equal(last.At.Truncate(time.Second)) {
		t.Fatalf("stored completed at = %v, want %v", status.LastCompletedAt, last.At.Truncate(time.Second))
	}

	if got := mustCountRows(t, db, `SELECT COUNT(*) FROM syndicate_items WHERE item_type = ?`, "mods"); got != 2 {
		t.Fatalf("mods syndicate rows = %d, want 2", got)
	}
	if got := mustCountRows(t, db, `SELECT COUNT(*) FROM syndicate_items WHERE item_type = ?`, "arcanes"); got != 1 {
		t.Fatalf("arcanes syndicate rows = %d, want 1", got)
	}
	if got := mustCountRows(t, db, `SELECT COUNT(*) FROM syndicate_items WHERE item_type = ?`, "weapons"); got != 2 {
		t.Fatalf("weapons syndicate rows = %d, want 2", got)
	}
	if got := mustCountRows(t, db, `SELECT COUNT(*) FROM nightmare_mod_drops`); got != 2 {
		t.Fatalf("nightmare_mod_drops rows = %d, want 2", got)
	}
	if got := mustCountRows(t, db, `SELECT COUNT(*) FROM vault_mod_drops`); got != 1 {
		t.Fatalf("vault_mod_drops rows = %d, want 1", got)
	}
	if got := mustDucats(t, db, "weapons", "/Lotus/Weapons/TestWeaponBlueprint"); got != 65 {
		t.Fatalf("weapon ducats = %d, want 65", got)
	}
	if got := mustDucats(t, db, "weapons", "/Lotus/Weapons/TestWeaponComponent"); got != 65 {
		t.Fatalf("weapon component ducats = %d, want 65", got)
	}
	if got := mustDucats(t, db, "warframes", "/Lotus/Warframes/TestFrameComponent"); got != 100 {
		t.Fatalf("warframe component ducats = %d, want 100", got)
	}
}

func TestProgramInitMissingDatabaseShowsMissingStatus(t *testing.T) {
	setUserHomeEnv(t, t.TempDir())
	model := &integrationModel{
		inner: NewModel().(Model),
		stop: func(m Model) bool {
			return m.status == statusMissing
		},
	}

	got := runProgram(t, model, nil)
	if got.status != statusMissing {
		t.Fatalf("status = %v, want %v", got.status, statusMissing)
	}
	if view := got.View(); !strings.Contains(view, "Database not present") {
		t.Fatalf("View() = %q, want missing database text", view)
	}
	if !strings.Contains(got.View(), "Last sync: unknown") {
		t.Fatalf("View() = %q, want unknown last sync", got.View())
	}
}

func TestProgramInjectedUpstreamStatusShowsStaleView(t *testing.T) {
	lastSync := time.Date(2026, time.April, 17, 12, 34, 0, 0, time.UTC)
	model := &integrationModel{
		inner: Model{
			base:        viewBuilder.NewBaseModel(),
			status:      statusChecking,
			storedSHA:   "abcdef123456",
			hasLastSync: true,
			lastSync:    lastSync,
		},
		stop: func(m Model) bool {
			return m.status == statusStale
		},
	}

	got := runProgram(t, model, func(p *tea.Program) {
		go p.Send(upstreamStatusMsg{SHA: "123456789abc"})
	})
	if got.status != statusStale {
		t.Fatalf("status = %v, want %v", got.status, statusStale)
	}
	if got.latestSHA != "123456789abc" {
		t.Fatalf("latestSHA = %q, want %q", got.latestSHA, "123456789abc")
	}
	view := got.View()
	if !strings.Contains(view, "Database stale: abcdef1 -> 1234567") {
		t.Fatalf("View() = %q, want stale sha text", view)
	}
	if !strings.Contains(view, "Last sync: 2026-04-17 12:34 UTC") {
		t.Fatalf("View() = %q, want last sync text", view)
	}
}
