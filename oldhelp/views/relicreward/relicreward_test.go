package relicreward

import (
	"context"
	"database/sql"
	"image"
	"image/color"
	"image/png"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"github.com/gjrud/warframe-helper/utils/database"
	"github.com/gjrud/warframe-helper/utils/pricechecker"
	"github.com/gjrud/warframe-helper/utils/relicdetect"
	"github.com/gjrud/warframe-helper/utils/viewBuilder"
)

func setUserHomeEnv(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
}

func containsAnyFold(s string, parts ...string) bool {
	s = strings.ToLower(s)
	for _, part := range parts {
		if strings.Contains(s, strings.ToLower(part)) {
			return true
		}
	}
	return false
}

const (
	rewardRefWidth  = 2560
	rewardRefHeight = 1440
	rewardTmplX     = 380
	rewardTmplY     = 62
	rewardTmplW     = 735
	rewardTmplH     = 60
)

var relicTables = []string{"mods", "arcanes", "warframes", "weapons", "archwing", "companions", "misc"}

type stubCapturer struct {
	imgs []image.Image
	err  error
}

func (s stubCapturer) CaptureAll() ([]image.Image, error) {
	return s.imgs, s.err
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

func setupRelicHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	setUserHomeEnv(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".warframe-helper"), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	return home
}

func openRelicDB(t *testing.T, path string) *sql.DB {
	t.Helper()

	db, err := database.OpenDB(path)
	if err != nil {
		t.Fatalf("OpenDB() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	for _, tableName := range relicTables {
		if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS ` + tableName + ` (gameRef TEXT PRIMARY KEY, slug TEXT NOT NULL, name TEXT NOT NULL, ducats INTEGER)`); err != nil {
			t.Fatalf("create table %s error = %v", tableName, err)
		}
	}
	return db
}

func insertTradableItem(t *testing.T, db *sql.DB, tableName string, item pricechecker.GameObject) {
	t.Helper()

	if _, err := db.Exec(`INSERT INTO `+tableName+`(gameRef, slug, name, ducats) VALUES (?, ?, ?, ?)`, item.GameRef, item.Slug, item.Name, item.Ducats); err != nil {
		t.Fatalf("insert item %s/%s error = %v", tableName, item.GameRef, err)
	}
}

func fillNRGBA(img *image.NRGBA, r image.Rectangle, c color.NRGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
}

func writeRewardTemplate(t *testing.T, path string, c color.NRGBA) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, rewardTmplW, rewardTmplH))
	fillNRGBA(img, img.Bounds(), c)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create(%q) error = %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
}

func loadRewardTemplate(t *testing.T, c color.NRGBA) relicdetect.Template {
	t.Helper()

	path := filepath.Join(t.TempDir(), "reward_template.png")
	writeRewardTemplate(t, path, c)
	tmpl, err := relicdetect.LoadTemplate(path)
	if err != nil {
		t.Fatalf("LoadTemplate() error = %v", err)
	}
	return tmpl
}

func newRewardScreen(templateColor, bg color.NRGBA) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, rewardRefWidth, rewardRefHeight))
	fillNRGBA(img, img.Bounds(), bg)
	fillNRGBA(img, image.Rect(rewardTmplX, rewardTmplY, rewardTmplX+rewardTmplW, rewardTmplY+rewardTmplH), templateColor)
	return img
}

func writeExecutable(t *testing.T, dir string, name string, body string) {
	t.Helper()

	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func setBinPath(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "bin")
	for name, body := range files {
		writeExecutable(t, dir, name, body)
	}
	t.Setenv("PATH", dir)
	return dir
}

func chooseBase(t *testing.T, base *viewBuilder.BaseModel, numOptions int) context.Context {
	t.Helper()

	var chosenCtx context.Context
	cmd := base.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter}, numOptions, func(_ int, ctx context.Context) tea.Cmd {
		chosenCtx = ctx
		return nil
	})
	if cmd != nil {
		_ = cmd()
	}
	if !base.Chosen || base.Cancel == nil || chosenCtx == nil {
		t.Fatalf("base not chosen after enter: %+v", *base)
	}
	return chosenCtx
}

func startBlockingProxy(t *testing.T, done <-chan struct{}) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				<-done
			}(conn)
		}
	}()

	return "http://" + ln.Addr().String()
}

func TestRunInitHomeDirFailureReturnsError(t *testing.T) {
	setUserHomeEnv(t, "")

	got, ok := runInit(context.Background())().(viewBuilder.ErrorMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want viewBuilder.ErrorMsg", got)
	}
	if !containsAnyFold(got.Text, "$HOME is not defined", "%userprofile% is not defined") {
		t.Fatalf("error text = %q, want home-dir env error", got.Text)
	}
}

func TestRunInitSuccessReturnsScanInitMsg(t *testing.T) {
	home := setupRelicHome(t)
	t.Setenv("SWAYSOCK", "")
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	setBinPath(t, map[string]string{
		"scrot": "#!/bin/sh\nexit 0\n",
	})

	dbPath := filepath.Join(home, ".warframe-helper", "warframe.db")
	db := openRelicDB(t, dbPath)
	dbItems := []pricechecker.GameObject{{Name: "Forma Blueprint", GameRef: "forma_blueprint", Slug: "forma-blueprint", Category: "mods"}}
	insertTradableItem(t, db, "mods", dbItems[0])

	templatePath := filepath.Join(home, ".warframe-helper", "reward_template.png")
	writeRewardTemplate(t, templatePath, color.NRGBA{R: 12, G: 34, B: 56, A: 255})

	msgs := runSequenceSteps(t, runInit(context.Background()), nil)
	if len(msgs) != 4 {
		t.Fatalf("msgs len = %d, want 4", len(msgs))
	}
	if !reflect.DeepEqual(msgs[:3], []tea.Msg{
		viewBuilder.StepMsg{Text: "Loading template"},
		viewBuilder.StepMsg{Text: "Loading database items"},
		viewBuilder.StepMsg{Text: "Probing screen capture"},
	}) {
		t.Fatalf("step msgs = %#v", msgs[:3])
	}

	got, ok := msgs[3].(scanInitMsg)
	if !ok {
		t.Fatalf("last msg = %T, want scanInitMsg", msgs[3])
	}
	if !reflect.DeepEqual(got.prepared, relicdetect.PrepareItems(dbItems)) {
		t.Fatalf("prepared = %#v, want %#v", got.prepared, relicdetect.PrepareItems(dbItems))
	}
	if got.capturer == nil {
		t.Fatal("capturer = nil, want non-nil")
	}
	wantPath := filepath.Join(home, ".warframe-helper", "wf-words.txt")
	if got.wordListPath != wantPath {
		t.Fatalf("wordListPath = %q, want %q", got.wordListPath, wantPath)
	}
	b, err := os.ReadFile(got.wordListPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", got.wordListPath, err)
	}
	if string(b) != "Blueprint\nForma\n" {
		t.Fatalf("word list contents = %q, want %q", string(b), "Blueprint\nForma\n")
	}
}

func TestRunInitWriteWordListFailureLeavesEmptyPath(t *testing.T) {
	home := setupRelicHome(t)
	t.Setenv("SWAYSOCK", "")
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	setBinPath(t, map[string]string{
		"scrot": "#!/bin/sh\nexit 0\n",
	})

	dbPath := filepath.Join(home, ".warframe-helper", "warframe.db")
	db := openRelicDB(t, dbPath)
	insertTradableItem(t, db, "mods", pricechecker.GameObject{Name: "Forma Blueprint", GameRef: "forma_blueprint", Slug: "forma-blueprint"})

	templatePath := filepath.Join(home, ".warframe-helper", "reward_template.png")
	writeRewardTemplate(t, templatePath, color.NRGBA{R: 1, G: 2, B: 3, A: 255})

	wordListPath := filepath.Join(home, ".warframe-helper", "wf-words.txt")
	if err := os.Mkdir(wordListPath, 0755); err != nil {
		t.Fatalf("Mkdir(%q) error = %v", wordListPath, err)
	}

	msgs := runSequenceSteps(t, runInit(context.Background()), nil)
	got, ok := msgs[len(msgs)-1].(scanInitMsg)
	if !ok {
		t.Fatalf("last msg = %T, want scanInitMsg", msgs[len(msgs)-1])
	}
	if got.wordListPath != "" {
		t.Fatalf("wordListPath = %q, want empty", got.wordListPath)
	}
}

func TestRunInitCanceledBetweenStepsReturnsCanceled(t *testing.T) {
	home := setupRelicHome(t)
	t.Setenv("SWAYSOCK", "")
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	setBinPath(t, map[string]string{
		"scrot": "#!/bin/sh\nexit 0\n",
	})

	dbPath := filepath.Join(home, ".warframe-helper", "warframe.db")
	db := openRelicDB(t, dbPath)
	insertTradableItem(t, db, "mods", pricechecker.GameObject{Name: "Forma Blueprint", GameRef: "forma_blueprint", Slug: "forma-blueprint"})
	writeRewardTemplate(t, filepath.Join(home, ".warframe-helper", "reward_template.png"), color.NRGBA{R: 7, G: 8, B: 9, A: 255})

	ctx, cancel := context.WithCancel(context.Background())
	msgs := runSequenceSteps(t, runInit(ctx), func(step int, _ []tea.Msg) {
		if step == 0 {
			cancel()
		}
	})

	want := []tea.Msg{
		viewBuilder.StepMsg{Text: "Loading template"},
		viewBuilder.CanceledMsg{},
		viewBuilder.CanceledMsg{},
		viewBuilder.CanceledMsg{},
		viewBuilder.CanceledMsg{},
	}
	if !reflect.DeepEqual(msgs, want) {
		t.Fatalf("msgs = %#v, want %#v", msgs, want)
	}
}

func TestRunInitFailures(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, home string)
		wantPrefix []tea.Msg
		wantErrs   []string
		skipOS     string
	}{
		{
			name: "ensure template failure",
			setup: func(t *testing.T, home string) {
				homeFile := filepath.Join(t.TempDir(), "home.txt")
				if err := os.WriteFile(homeFile, []byte("not a directory"), 0644); err != nil {
					t.Fatalf("WriteFile() error = %v", err)
				}
				setUserHomeEnv(t, homeFile)
			},
			wantPrefix: []tea.Msg{viewBuilder.StepMsg{Text: "Loading template"}},
			wantErrs:   []string{"not a directory", "system cannot find the path specified"},
		},
		{
			name: "load template failure",
			setup: func(t *testing.T, home string) {
				path := filepath.Join(home, ".warframe-helper", "reward_template.png")
				if err := os.WriteFile(path, []byte("not png"), 0644); err != nil {
					t.Fatalf("WriteFile() error = %v", err)
				}
			},
			wantPrefix: []tea.Msg{viewBuilder.StepMsg{Text: "Loading template"}},
			wantErrs:   []string{"unexpected EOF"},
		},
		{
			name: "query items failure",
			setup: func(t *testing.T, home string) {
				writeRewardTemplate(t, filepath.Join(home, ".warframe-helper", "reward_template.png"), color.NRGBA{R: 3, G: 4, B: 5, A: 255})
				path := filepath.Join(home, ".warframe-helper", "warframe.db")
				if err := os.WriteFile(path, []byte("not sqlite"), 0644); err != nil {
					t.Fatalf("WriteFile() error = %v", err)
				}
			},
			wantPrefix: []tea.Msg{
				viewBuilder.StepMsg{Text: "Loading template"},
				viewBuilder.StepMsg{Text: "Loading database items"},
			},
			wantErrs: []string{"database"},
		},
		{
			name: "probe capture failure",
			setup: func(t *testing.T, home string) {
				writeRewardTemplate(t, filepath.Join(home, ".warframe-helper", "reward_template.png"), color.NRGBA{R: 6, G: 7, B: 8, A: 255})
				db := openRelicDB(t, filepath.Join(home, ".warframe-helper", "warframe.db"))
				insertTradableItem(t, db, "mods", pricechecker.GameObject{Name: "Forma Blueprint", GameRef: "forma_blueprint", Slug: "forma-blueprint"})
				t.Setenv("PATH", "")
				t.Setenv("SWAYSOCK", "")
				t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
			},
			wantPrefix: []tea.Msg{
				viewBuilder.StepMsg{Text: "Loading template"},
				viewBuilder.StepMsg{Text: "Loading database items"},
				viewBuilder.StepMsg{Text: "Probing screen capture"},
			},
			wantErrs: []string{"no screenshot tool found"},
			skipOS:   "windows",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipOS == runtime.GOOS {
				t.Skipf("%s-specific test", tt.skipOS)
			}
			home := setupRelicHome(t)
			tt.setup(t, home)

			msgs := runSequenceSteps(t, runInit(context.Background()), nil)
			if len(msgs) != len(tt.wantPrefix)+1 {
				t.Fatalf("msgs len = %d, want %d", len(msgs), len(tt.wantPrefix)+1)
			}
			if !reflect.DeepEqual(msgs[:len(tt.wantPrefix)], tt.wantPrefix) {
				t.Fatalf("msgs prefix = %#v, want %#v", msgs[:len(tt.wantPrefix)], tt.wantPrefix)
			}
			errMsg, ok := msgs[len(msgs)-1].(viewBuilder.ErrorMsg)
			if !ok {
				t.Fatalf("last msg = %T, want viewBuilder.ErrorMsg", msgs[len(msgs)-1])
			}
			if !containsAnyFold(errMsg.Text, tt.wantErrs...) {
				t.Fatalf("error text = %q, want one of %#v", errMsg.Text, tt.wantErrs)
			}
		})
	}
}

func TestBuildRewardRowsSortsByScore(t *testing.T) {
	items := []pricechecker.OutputItem{
		{Name: "Low Score", GameRef: "low_score", AvgPrice: 10, Count: 2},
		{Name: "Top Score", GameRef: "top_score", AvgPrice: 15, Count: 4},
		{Name: "Fetch Failed", GameRef: "fetch_failed", FetchFailed: true},
	}

	got := buildRewardRows(items, []bool{false, false, false})
	want := []table.Row{
		{"Top Score", "15.0", "4"},
		{"Low Score", "10.0", "2"},
		{
			viewBuilder.ErrorStyle.Render("Fetch Failed"),
			viewBuilder.ErrorStyle.Render("ERR"),
			viewBuilder.ErrorStyle.Render("ERR"),
		},
	}

	if !reflect.DeepEqual(got.rows, want) {
		t.Fatalf("rows = %#v, want %#v", got.rows, want)
	}
}

func TestBuildRewardRowsUnmatchedRendering(t *testing.T) {
	got := buildRewardRows([]pricechecker.OutputItem{{Name: "Unknown Reward"}}, []bool{true})
	want := []table.Row{{"Unknown Reward", "N/A", "-"}}

	if !reflect.DeepEqual(got.rows, want) {
		t.Fatalf("rows = %#v, want %#v", got.rows, want)
	}
}

func TestBuildRewardRowsFailedRendering(t *testing.T) {
	got := buildRewardRows([]pricechecker.OutputItem{{Name: "Bugged Reward", FetchFailed: true}}, []bool{false})
	want := []table.Row{{
		viewBuilder.ErrorStyle.Render("Bugged Reward"),
		viewBuilder.ErrorStyle.Render("ERR"),
		viewBuilder.ErrorStyle.Render("ERR"),
	}}

	if !reflect.DeepEqual(got.rows, want) {
		t.Fatalf("rows = %#v, want %#v", got.rows, want)
	}
}

func TestFetchNextRewardUnmatchedBuildsRowsWithoutFetching(t *testing.T) {
	msg := fetchRewardMsg{
		toFetch:   []pricechecker.GameObject{{Name: "Matched Reward", GameRef: "matched_reward"}},
		rawNames:  []string{"Unknown Reward"},
		approx:    []bool{true},
		items:     nil,
		itemFlags: nil,
	}

	cmd := fetchNextReward(context.Background(), msg)
	if cmd == nil {
		t.Fatal("fetchNextReward() returned nil cmd")
	}

	got, ok := cmd().(rewardReadyMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want rewardReadyMsg", cmd())
	}

	want := rewardReadyMsg{rows: []table.Row{{"Unknown Reward", "N/A", "-"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cmd() = %#v, want %#v", got, want)
	}
}

func TestFetchNextRewardCanceledDuringSleep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := fetchNextReward(ctx, fetchRewardMsg{
		toFetch:  []pricechecker.GameObject{{Name: "Reward", GameRef: "reward"}},
		rawNames: []string{""},
		approx:   []bool{false},
	})

	if got, ok := cmd().(viewBuilder.CanceledMsg); !ok {
		t.Fatalf("cmd() = %T, want viewBuilder.CanceledMsg", got)
	}
}

func TestFetchNextRewardFailureBuildsFetchFailedRow(t *testing.T) {
	cmd := fetchNextReward(context.Background(), fetchRewardMsg{
		toFetch:  []pricechecker.GameObject{{Name: "Reward", GameRef: "reward"}},
		rawNames: []string{""},
		approx:   []bool{false},
	})

	got, ok := cmd().(rewardReadyMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want rewardReadyMsg", got)
	}
	want := rewardReadyMsg{rows: []table.Row{{
		viewBuilder.ErrorStyle.Render("Reward"),
		viewBuilder.ErrorStyle.Render("ERR"),
		viewBuilder.ErrorStyle.Render("ERR"),
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cmd() = %#v, want %#v", got, want)
	}
}

func TestFetchNextRewardFetchFailureWithCanceledContextReturnsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	proxyURL := startBlockingProxy(t, ctx.Done())
	t.Setenv("HTTPS_PROXY", proxyURL)
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("NO_PROXY", "")
	go func() {
		time.Sleep(450 * time.Millisecond)
		cancel()
	}()

	cmd := fetchNextReward(ctx, fetchRewardMsg{
		toFetch:  []pricechecker.GameObject{{Name: "Reward", GameRef: "reward", Slug: "reward"}},
		rawNames: []string{""},
		approx:   []bool{false},
	})

	if got, ok := cmd().(viewBuilder.CanceledMsg); !ok {
		t.Fatalf("cmd() = %T, want viewBuilder.CanceledMsg", got)
	}
}

func TestScanCmdCanceledDuringSleep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := scanCmd(ctx, stubCapturer{}, relicdetect.Template{}, "")
	if got, ok := cmd().(viewBuilder.CanceledMsg); !ok {
		t.Fatalf("cmd() = %T, want viewBuilder.CanceledMsg", got)
	}
}

func TestScanCmdCaptureErrorReturnsErrorMsg(t *testing.T) {
	cmd := scanCmd(context.Background(), stubCapturer{err: os.ErrNotExist}, relicdetect.Template{}, "")

	got, ok := cmd().(viewBuilder.ErrorMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want viewBuilder.ErrorMsg", got)
	}
	if !strings.Contains(got.Text, os.ErrNotExist.Error()) {
		t.Fatalf("error text = %q, want substring %q", got.Text, os.ErrNotExist.Error())
	}
}

func TestScanCmdWithoutRewardMatchReturnsTick(t *testing.T) {
	tmpl := loadRewardTemplate(t, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	img := newRewardScreen(color.NRGBA{R: 0, G: 0, B: 0, A: 255}, color.NRGBA{R: 0, G: 0, B: 0, A: 255})
	cmd := scanCmd(context.Background(), stubCapturer{imgs: []image.Image{img}}, tmpl, "")

	if got, ok := cmd().(scanTickMsg); !ok {
		t.Fatalf("cmd() = %T, want scanTickMsg", got)
	}
}

func TestModelUpdateAvailableSizeMsgSetsTableHeight(t *testing.T) {
	m := NewModel().(Model)

	updated, cmd := m.Update(viewBuilder.AvailableSizeMsg{Height: 3})
	got := updated.(Model)

	if cmd != nil {
		t.Fatal("cmd should be nil for size update")
	}
	if got.table.Height() != 2 {
		t.Fatalf("table height = %d, want 2", got.table.Height())
	}
}

func TestModelUpdateScanInitMsgSetsStateAndStartsScan(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	capturer := stubCapturer{}
	tmpl := relicdetect.Template{}
	prepared := relicdetect.PrepareItems([]pricechecker.GameObject{{Name: "Forma Blueprint", GameRef: "forma_blueprint"}})
	m := NewModel().(Model)
	m.ctx = ctx

	updated, cmd := m.Update(scanInitMsg{tmpl: tmpl, prepared: prepared, capturer: capturer, wordListPath: "words.txt"})
	got := updated.(Model)

	if !reflect.DeepEqual(got.template, tmpl) {
		t.Fatalf("template = %#v, want %#v", got.template, tmpl)
	}
	if !reflect.DeepEqual(got.prepared, prepared) {
		t.Fatalf("prepared = %#v, want %#v", got.prepared, prepared)
	}
	if _, ok := got.capturer.(stubCapturer); !ok {
		t.Fatal("capturer not stored")
	}
	if got.wordListPath != "words.txt" {
		t.Fatalf("wordListPath = %q, want %q", got.wordListPath, "words.txt")
	}
	if cmd == nil {
		t.Fatal("scanInitMsg should return scan cmd")
	}
	if _, ok := cmd().(viewBuilder.CanceledMsg); !ok {
		t.Fatalf("cmd() = %T, want viewBuilder.CanceledMsg", cmd())
	}
}

func TestModelUpdateRewardReadyMsgLoadsRows(t *testing.T) {
	m := NewModel().(Model)
	rows := []table.Row{{"A", "1.0", "2"}, {"B", "0.5", "1"}}

	updated, cmd := m.Update(rewardReadyMsg{rows: rows})
	got := updated.(Model)

	if !reflect.DeepEqual(got.table.Rows(), rows) {
		t.Fatalf("rows = %#v, want %#v", got.table.Rows(), rows)
	}
	if got.table.Cursor() != 0 {
		t.Fatalf("cursor = %d, want 0", got.table.Cursor())
	}
	if cmd == nil {
		t.Fatal("rewardReadyMsg should return cooldown cmd")
	}
}

func TestModelUpdateOCRDoneMsgBuildsRewardRowsForUnmatchedItem(t *testing.T) {
	m := NewModel().(Model)
	m.prepared = relicdetect.PrepareItems([]pricechecker.GameObject{{Name: "Forma Blueprint", GameRef: "forma_blueprint"}})

	updated, cmd := m.Update(ocrDoneMsg{names: []string{"Mystery Reward"}})
	if _, ok := updated.(Model); !ok {
		t.Fatal("updated model has wrong type")
	}
	if cmd == nil {
		t.Fatal("ocrDoneMsg should return fetch cmd")
	}

	got, ok := cmd().(rewardReadyMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want rewardReadyMsg", got)
	}
	want := rewardReadyMsg{rows: []table.Row{{"Mystery Reward", "N/A", "-"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cmd() = %#v, want %#v", got, want)
	}
}

func TestModelUpdateFetchRewardMsgReturnsRewardRows(t *testing.T) {
	m := NewModel().(Model)

	updated, cmd := m.Update(fetchRewardMsg{
		toFetch:  []pricechecker.GameObject{{Name: "Unused", GameRef: "unused"}},
		rawNames: []string{"Unknown Reward"},
		approx:   []bool{true},
	})
	if _, ok := updated.(Model); !ok {
		t.Fatal("updated model has wrong type")
	}
	if cmd == nil {
		t.Fatal("fetchRewardMsg should return fetch cmd")
	}

	got, ok := cmd().(rewardReadyMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want rewardReadyMsg", got)
	}
	want := rewardReadyMsg{rows: []table.Row{{"Unknown Reward", "N/A", "-"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cmd() = %#v, want %#v", got, want)
	}
}

func TestModelUpdateCooldownDoneMsgClearsRows(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	m := NewModel().(Model)
	m.ctx = ctx
	m.capturer = stubCapturer{}
	m.table.SetRows([]table.Row{{"keep", "1", "1"}})

	updated, cmd := m.Update(cooldownDoneMsg{})
	got := updated.(Model)

	if len(got.table.Rows()) != 0 {
		t.Fatalf("rows len = %d, want 0", len(got.table.Rows()))
	}
	if cmd == nil {
		t.Fatal("cooldownDoneMsg should return scan cmd")
	}
	if _, ok := cmd().(viewBuilder.CanceledMsg); !ok {
		t.Fatalf("cmd() = %T, want viewBuilder.CanceledMsg", cmd())
	}
}

func TestModelUpdateCanceledMsgResetsToMenu(t *testing.T) {
	m := NewModel().(Model)
	m.base.Chosen = true
	m.base.Done = true
	m.base.Steps = []string{"Scanning"}
	m.table.SetRows([]table.Row{{"keep", "1", "1"}})
	m.capturer = stubCapturer{}

	updated, cmd := m.Update(viewBuilder.CanceledMsg{})
	got := updated.(Model)

	if cmd != nil {
		t.Fatal("canceled should return nil cmd")
	}
	if got.base.Chosen || got.base.Done {
		t.Fatalf("base not reset: %#v", got.base)
	}
	if len(got.table.Rows()) != 0 {
		t.Fatalf("rows len = %d, want 0", len(got.table.Rows()))
	}
	if got.capturer != nil {
		t.Fatal("capturer should be cleared")
	}
}

func TestModelUpdateChosenQuitResetsToMenu(t *testing.T) {
	m := NewModel().(Model)
	ctx := chooseBase(t, &m.base, len(options))
	m.base.Done = true
	m.base.Steps = []string{"Scanning", "Done"}
	m.table.SetRows([]table.Row{{"keep", "1", "1"}})

	updated, cmd := m.Update(tea.KeyPressMsg{Text: "q"})
	got := updated.(Model)

	if cmd != nil {
		t.Fatal("quit should return nil cmd")
	}
	if ctx.Err() == nil {
		t.Fatal("quit should cancel context")
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
	if got.capturer != nil {
		t.Fatal("capturer should be cleared")
	}
}

func TestModelViewMenuChoices(t *testing.T) {
	m := NewModel().(Model)

	if got, want := m.View(), viewBuilder.ChoicesView(m.base.Choice, options); got != want {
		t.Fatalf("View() = %q, want %q", got, want)
	}
}

func TestModelViewProgressWhileInitializing(t *testing.T) {
	m := NewModel().(Model)
	m.base.Chosen = true
	m.base.Steps = []string{"Loading template"}

	if got, want := m.View(), viewBuilder.ProgressView(m.base.Steps, m.base.Spinner.View(), false, m.base.Quitting); got != want {
		t.Fatalf("View() = %q, want %q", got, want)
	}
}

func TestModelViewTableWhenRowsLoaded(t *testing.T) {
	m := NewModel().(Model)
	m.base.Chosen = true
	m.capturer = stubCapturer{}
	m.table.SetRows([]table.Row{{"Forma Blueprint", "12.5", "3"}})

	if got, want := m.View(), m.table.View(); got != want {
		t.Fatalf("View() = %q, want %q", got, want)
	}
}

func TestModelViewScanningWhenCapturerReadyWithoutRows(t *testing.T) {
	m := NewModel().(Model)
	m.base.Chosen = true
	m.capturer = stubCapturer{}

	if got := m.View(); !strings.HasSuffix(got, " Scanning...") {
		t.Fatalf("View() = %q, want suffix %q", got, " Scanning...")
	}
}
