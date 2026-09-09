//go:build linux

package memoryscan

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	tea "charm.land/bubbletea/v2"
	"github.com/gjrud/warframe-helper/utils/viewBuilder"
	"golang.org/x/sys/unix"
)

const (
	helperAuthz       = "?accountId=123456789012345678901234&nonce=1234567890"
	helperProcessName = "Warframe.x64.ex"
)

var helperPayloads [][]byte

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func expandCmd(t *testing.T, cmd tea.Cmd) []tea.Msg {
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
		msgs := make([]tea.Msg, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			if rv.Index(i).IsNil() {
				continue
			}
			sub, ok := rv.Index(i).Interface().(tea.Cmd)
			if !ok {
				t.Fatalf("slice element %d = %T, want tea.Cmd", i, rv.Index(i).Interface())
			}
			msgs = append(msgs, expandCmd(t, sub)...)
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
		t.Fatalf("cmd() = %T, want sequence slice", msg)
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
		stepMsgs := expandCmd(t, stepCmd)
		msgs = append(msgs, stepMsgs...)
		if afterStep != nil {
			afterStep(i, stepMsgs)
		}
	}
	return msgs
}

func setupHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
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

func updateModel(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()

	next, cmd := m.Update(msg)
	updated, ok := next.(Model)
	if !ok {
		t.Fatalf("Update() model = %T, want memoryscan.Model", next)
	}
	return updated, cmd
}

func initConsentMsg(t *testing.T, m Model) consentCheckMsg {
	t.Helper()

	for _, msg := range expandCmd(t, m.Init()) {
		if consentMsg, ok := msg.(consentCheckMsg); ok {
			return consentMsg
		}
	}
	t.Fatal("Init() did not return consentCheckMsg")
	return consentCheckMsg{}
}

func startScanHelper(t *testing.T, mode string) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=^TestMemoryscanHelperProcess$")
	cmd.Env = append(os.Environ(),
		"GO_WANT_MEMORYSCAN_HELPER=1",
		"MEMORYSCAN_HELPER_MODE="+mode,
	)
	if mode == "authz" {
		cmd.Env = append(cmd.Env, "MEMORYSCAN_HELPER_AUTHZ="+helperAuthz)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe() error = %v", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe() error = %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	ready, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
		t.Fatalf("helper ready read error = %v\nstderr:\n%s", err, stderr.String())
	}
	if strings.TrimSpace(ready) != "ready" {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
		t.Fatalf("helper ready = %q, want %q\nstderr:\n%s", ready, "ready", stderr.String())
	}

	t.Cleanup(func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	})
}

func TestMemoryscanHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_MEMORYSCAN_HELPER") != "1" {
		return
	}

	name := append([]byte(helperProcessName), 0)
	if err := unix.Prctl(unix.PR_SET_NAME, uintptr(unsafe.Pointer(&name[0])), 0, 0, 0); err != nil {
		t.Fatalf("Prctl(PR_SET_NAME) error = %v", err)
	}

	switch os.Getenv("MEMORYSCAN_HELPER_MODE") {
	case "authz":
		helperPayloads = [][]byte{
			[]byte(strings.Repeat("prefix-"+os.Getenv("MEMORYSCAN_HELPER_AUTHZ")+" ", 4)),
			[]byte(strings.Repeat("suffix-"+os.Getenv("MEMORYSCAN_HELPER_AUTHZ")+" ", 4)),
		}
	case "noauthz":
		helperPayloads = [][]byte{
			bytes.Repeat([]byte("no auth token here "), 1<<15),
		}
	default:
		t.Fatalf("unknown helper mode %q", os.Getenv("MEMORYSCAN_HELPER_MODE"))
	}

	if _, err := io.WriteString(os.Stdout, "ready\n"); err != nil {
		t.Fatalf("stdout ready write error = %v", err)
	}
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		t.Fatalf("stdin wait error = %v", err)
	}
}

func TestConsentPersistence(t *testing.T) {
	home := setupHome(t)

	granted, err := hasSavedConsent()
	if err != nil {
		t.Fatalf("hasSavedConsent() error = %v", err)
	}
	if granted {
		t.Fatal("hasSavedConsent() = true, want false before save")
	}

	if err := saveConsent(); err != nil {
		t.Fatalf("saveConsent() error = %v", err)
	}

	path := filepath.Join(home, ".warframe-helper", memoryScanConsentFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	var record consentRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if record.Version != 1 {
		t.Fatalf("record.Version = %d, want 1", record.Version)
	}
	if record.Phrase != memoryScanConsentPhrase {
		t.Fatalf("record.Phrase = %q, want %q", record.Phrase, memoryScanConsentPhrase)
	}
	if record.AcceptedAt.IsZero() {
		t.Fatal("record.AcceptedAt is zero")
	}

	granted, err = hasSavedConsent()
	if err != nil {
		t.Fatalf("hasSavedConsent() after save error = %v", err)
	}
	if !granted {
		t.Fatal("hasSavedConsent() = false, want true after save")
	}

	if err := revokeConsent(); err != nil {
		t.Fatalf("revokeConsent() error = %v", err)
	}

	granted, err = hasSavedConsent()
	if err != nil {
		t.Fatalf("hasSavedConsent() after revoke error = %v", err)
	}
	if granted {
		t.Fatal("hasSavedConsent() = true, want false after revoke")
	}
}

func TestModelPromptsForConsentWhenMissing(t *testing.T) {
	setupHome(t)
	m, ok := NewModel().(Model)
	if !ok {
		t.Fatal("NewModel() did not return memoryscan.Model")
	}
	consentMsg := initConsentMsg(t, m)
	m, _ = updateModel(t, m, consentMsg)

	m, cmd := updateModel(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter returned nil cmd")
	}
	msg := cmd()

	m, cmd = updateModel(t, m, msg)
	if m.mode != viewModePrompt {
		t.Fatalf("mode = %v, want prompt", m.mode)
	}
	if cmd == nil {
		t.Fatal("consent prompt focus cmd is nil")
	}
	view := m.View()
	if !strings.Contains(view, "possible ban") {
		t.Fatalf("View() missing warning, got %q", view)
	}
	if !strings.Contains(view, memoryScanConsentPhrase) {
		t.Fatalf("View() missing consent phrase, got %q", view)
	}
}

func TestMenuViewShowsPermissionStatusWhenNotGranted(t *testing.T) {
	setupHome(t)
	m, ok := NewModel().(Model)
	if !ok {
		t.Fatal("NewModel() did not return memoryscan.Model")
	}
	consentMsg := initConsentMsg(t, m)
	m, _ = updateModel(t, m, consentMsg)

	view := m.View()
	if !strings.Contains(view, "Permission status: denied") {
		t.Fatalf("View() missing denied status, got %q", view)
	}
	if strings.Contains(view, "possible ban") {
		t.Fatalf("View() unexpectedly showed warning while denied, got %q", view)
	}
}

func TestMenuViewShowsWarningWhenPermissionGranted(t *testing.T) {
	setupHome(t)
	if err := saveConsent(); err != nil {
		t.Fatalf("saveConsent() error = %v", err)
	}
	m, ok := NewModel().(Model)
	if !ok {
		t.Fatal("NewModel() did not return memoryscan.Model")
	}
	consentMsg := initConsentMsg(t, m)
	m, _ = updateModel(t, m, consentMsg)

	view := m.View()
	if !strings.Contains(view, "Permission status: allowed") {
		t.Fatalf("View() missing allowed status, got %q", view)
	}
	if !strings.Contains(view, "possible ban") {
		t.Fatalf("View() missing warning when allowed, got %q", view)
	}
}

func TestModelWrongConsentPhraseShowsError(t *testing.T) {
	m, ok := NewModel().(Model)
	if !ok {
		t.Fatal("NewModel() did not return memoryscan.Model")
	}
	m.base.Chosen = true
	m.mode = viewModePrompt

	m, cmd := updateModel(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("wrong consent phrase returned cmd")
	}
	if m.mode != viewModePrompt {
		t.Fatalf("mode = %v, want prompt after wrong phrase", m.mode)
	}
	if m.inputErr != "Type the exact phrase to continue." {
		t.Fatalf("inputErr = %q", m.inputErr)
	}
}

func TestModelAcceptedConsentSavesAndFetchesInventory(t *testing.T) {
	home := setupHome(t)
	startScanHelper(t, "authz")
	withDefaultTransport(t, roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.URL.String(); got != "https://mobile.warframe.com/api/inventory.php"+helperAuthz {
			t.Fatalf("url = %q, want %q", got, "https://mobile.warframe.com/api/inventory.php"+helperAuthz)
		}
		return httpResponse(req, `{"items":[1,2]}`), nil
	}))

	m, ok := NewModel().(Model)
	if !ok {
		t.Fatal("NewModel() did not return memoryscan.Model")
	}
	m.base.Chosen = true
	m.mode = viewModePrompt
	m.ctx = context.Background()
	m.input.SetValue(memoryScanConsentPhrase)

	m, cmd := updateModel(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.mode != viewModeProgress {
		t.Fatalf("mode = %v, want progress after accepted phrase", m.mode)
	}
	if cmd == nil {
		t.Fatal("accepted phrase returned nil cmd")
	}

	msgs := expandCmd(t, cmd)
	want := []tea.Msg{
		viewBuilder.StepMsg{Text: "Searching Warframe PID"},
		viewBuilder.StepMsg{Text: "Scanning memory for Auth string"},
		viewBuilder.StepMsg{Text: "Fetching inventory"},
		viewBuilder.StepMsg{Text: "Saving inventory"},
		viewBuilder.DoneMsg{},
	}
	if !reflect.DeepEqual(msgs, want) {
		t.Fatalf("msgs = %#v, want %#v", msgs, want)
	}

	if granted, err := hasSavedConsent(); err != nil {
		t.Fatalf("hasSavedConsent() error = %v", err)
	} else if !granted {
		t.Fatal("hasSavedConsent() = false, want true")
	}

	path := filepath.Join(home, ".warframe-helper", "inventory.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("inventory file missing: %v", err)
	}
}

func TestModelSavedConsentBypassesPrompt(t *testing.T) {
	setupHome(t)
	if err := saveConsent(); err != nil {
		t.Fatalf("saveConsent() error = %v", err)
	}
	startScanHelper(t, "authz")
	withDefaultTransport(t, roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return httpResponse(req, `{"items":[1,2]}`), nil
	}))

	m, ok := NewModel().(Model)
	if !ok {
		t.Fatal("NewModel() did not return memoryscan.Model")
	}
	consentMsg := initConsentMsg(t, m)
	m, _ = updateModel(t, m, consentMsg)

	m, cmd := updateModel(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter returned nil cmd")
	}
	msg := cmd()

	m, cmd = updateModel(t, m, msg)
	if m.mode == viewModePrompt {
		t.Fatal("mode = prompt, want non-prompt with saved consent")
	}
	if cmd == nil {
		t.Fatal("saved consent did not start fetch command")
	}

	runMsgs := expandCmd(t, cmd)
	if len(runMsgs) != 5 {
		t.Fatalf("msgs len = %d, want 5", len(runMsgs))
	}
}

func TestQuitFromChosenStateReturnsToMenuView(t *testing.T) {
	setupHome(t)
	m, ok := NewModel().(Model)
	if !ok {
		t.Fatal("NewModel() did not return memoryscan.Model")
	}
	m.base.Chosen = true
	m.mode = viewModeProgress
	m.base.Steps = []string{"Searching Warframe PID"}

	m, cmd := updateModel(t, m, tea.KeyPressMsg{Text: "q"})
	if m.base.Chosen {
		t.Fatal("base.Chosen = true, want false after quit")
	}
	if m.mode != viewModeMenu {
		t.Fatalf("mode = %v, want menu after quit", m.mode)
	}
	if cmd == nil {
		t.Fatal("quit from chosen state returned nil cmd")
	}
	view := m.View()
	if !strings.Contains(view, "Permission status") && !strings.Contains(view, "Checking permission status") {
		t.Fatalf("View() did not return menu, got %q", view)
	}
}

func TestRevokeOptionRemovesSavedConsent(t *testing.T) {
	setupHome(t)
	if err := saveConsent(); err != nil {
		t.Fatalf("saveConsent() error = %v", err)
	}

	m, ok := NewModel().(Model)
	if !ok {
		t.Fatal("NewModel() did not return memoryscan.Model")
	}
	consentMsg := initConsentMsg(t, m)
	m, _ = updateModel(t, m, consentMsg)
	m, cmd := updateModel(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd != nil {
		t.Fatal("down navigation returned cmd")
	}
	if m.base.Choice != 1 {
		t.Fatalf("choice = %d, want 1", m.base.Choice)
	}

	m, cmd = updateModel(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("revoke option returned nil cmd")
	}

	runMsgs := expandCmd(t, cmd)
	want := []tea.Msg{
		viewBuilder.StepMsg{Text: "Revoking saved memory scan permission"},
		viewBuilder.DoneMsg{},
	}
	if !reflect.DeepEqual(runMsgs, want) {
		t.Fatalf("msgs = %#v, want %#v", runMsgs, want)
	}

	if granted, err := hasSavedConsent(); err != nil {
		t.Fatalf("hasSavedConsent() error = %v", err)
	} else if granted {
		t.Fatal("hasSavedConsent() = true, want false after revoke")
	}
}

func TestFetchInventorySuccess(t *testing.T) {
	home := setupHome(t)
	startScanHelper(t, "authz")
	withDefaultTransport(t, roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.URL.String(); got != "https://mobile.warframe.com/api/inventory.php"+helperAuthz {
			t.Fatalf("url = %q, want %q", got, "https://mobile.warframe.com/api/inventory.php"+helperAuthz)
		}
		return httpResponse(req, `{"items":[1,2]}`), nil
	}))

	msgs := expandCmd(t, fetchInventory(context.Background()))
	want := []tea.Msg{
		viewBuilder.StepMsg{Text: "Searching Warframe PID"},
		viewBuilder.StepMsg{Text: "Scanning memory for Auth string"},
		viewBuilder.StepMsg{Text: "Fetching inventory"},
		viewBuilder.StepMsg{Text: "Saving inventory"},
		viewBuilder.DoneMsg{},
	}
	if !reflect.DeepEqual(msgs, want) {
		t.Fatalf("msgs = %#v, want %#v", msgs, want)
	}

	path := filepath.Join(home, ".warframe-helper", "inventory.json")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	const wantBody = "{\n  \"items\": [\n    1,\n    2\n  ]\n}"
	if string(got) != wantBody {
		t.Fatalf("saved inventory = %q, want %q", got, wantBody)
	}
}

func TestFetchInventoryFailures(t *testing.T) {
	t.Run("userHomeDir failure", func(t *testing.T) {
		t.Setenv("HOME", "")

		msgs := expandCmd(t, fetchInventory(context.Background()))
		want := []tea.Msg{viewBuilder.ErrorMsg{Text: "$HOME is not defined"}}
		if !reflect.DeepEqual(msgs, want) {
			t.Fatalf("msgs = %#v, want %#v", msgs, want)
		}
	})

	t.Run("findProcess failure", func(t *testing.T) {
		setupHome(t)

		msgs := expandCmd(t, fetchInventory(context.Background()))
		want := []tea.Msg{
			viewBuilder.StepMsg{Text: "Searching Warframe PID"},
			viewBuilder.ErrorMsg{Text: "warframe process not found"},
		}
		if !reflect.DeepEqual(msgs, want) {
			t.Fatalf("msgs = %#v, want %#v", msgs, want)
		}
	})

	t.Run("scanAuthz failure", func(t *testing.T) {
		setupHome(t)
		startScanHelper(t, "noauthz")
		withDefaultTransport(t, roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return nil, io.ErrUnexpectedEOF
		}))

		msgs := expandCmd(t, fetchInventory(context.Background()))
		want := []tea.Msg{
			viewBuilder.StepMsg{Text: "Searching Warframe PID"},
			viewBuilder.StepMsg{Text: "Scanning memory for Auth string"},
			viewBuilder.ErrorMsg{Text: "authz not found in process memory"},
		}
		if !reflect.DeepEqual(msgs, want) {
			t.Fatalf("msgs = %#v, want %#v", msgs, want)
		}
	})

	t.Run("fetchInventory failure", func(t *testing.T) {
		setupHome(t)
		startScanHelper(t, "authz")
		withDefaultTransport(t, roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return nil, io.ErrUnexpectedEOF
		}))

		msgs := expandCmd(t, fetchInventory(context.Background()))
		wantPrefix := []tea.Msg{
			viewBuilder.StepMsg{Text: "Searching Warframe PID"},
			viewBuilder.StepMsg{Text: "Scanning memory for Auth string"},
			viewBuilder.StepMsg{Text: "Fetching inventory"},
		}
		if len(msgs) != 4 {
			t.Fatalf("msgs len = %d, want 4", len(msgs))
		}
		if !reflect.DeepEqual(msgs[:3], wantPrefix) {
			t.Fatalf("msgs prefix = %#v, want %#v", msgs[:3], wantPrefix)
		}
		errMsg, ok := msgs[3].(viewBuilder.ErrorMsg)
		if !ok {
			t.Fatalf("msgs[3] = %T, want viewBuilder.ErrorMsg", msgs[3])
		}
		if !strings.Contains(errMsg.Text, "request failed:") || !strings.Contains(errMsg.Text, "unexpected EOF") {
			t.Fatalf("fetch error = %q, want wrapped unexpected EOF", errMsg.Text)
		}
	})

	t.Run("saveInventory failure", func(t *testing.T) {
		homeFile := filepath.Join(t.TempDir(), "home.txt")
		if err := os.WriteFile(homeFile, []byte("not a directory"), 0644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		t.Setenv("HOME", homeFile)
		startScanHelper(t, "authz")
		withDefaultTransport(t, roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return httpResponse(req, `{"items":[1,2]}`), nil
		}))

		msgs := expandCmd(t, fetchInventory(context.Background()))
		wantPrefix := []tea.Msg{
			viewBuilder.StepMsg{Text: "Searching Warframe PID"},
			viewBuilder.StepMsg{Text: "Scanning memory for Auth string"},
			viewBuilder.StepMsg{Text: "Fetching inventory"},
			viewBuilder.StepMsg{Text: "Saving inventory"},
		}
		if len(msgs) != 5 {
			t.Fatalf("msgs len = %d, want 5", len(msgs))
		}
		if !reflect.DeepEqual(msgs[:4], wantPrefix) {
			t.Fatalf("msgs prefix = %#v, want %#v", msgs[:4], wantPrefix)
		}
		errMsg, ok := msgs[4].(viewBuilder.ErrorMsg)
		if !ok {
			t.Fatalf("msgs[4] = %T, want viewBuilder.ErrorMsg", msgs[4])
		}
		if !strings.Contains(errMsg.Text, "failed to create dir") || !strings.Contains(errMsg.Text, "not a directory") {
			t.Fatalf("save error = %q, want create dir not a directory", errMsg.Text)
		}
	})
}

func TestFetchInventoryCancellation(t *testing.T) {
	t.Run("canceled before start", func(t *testing.T) {
		setupHome(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		msgs := runSequenceSteps(t, fetchInventory(ctx), nil)
		want := []tea.Msg{
			viewBuilder.CanceledMsg{},
			viewBuilder.CanceledMsg{},
			viewBuilder.CanceledMsg{},
			viewBuilder.CanceledMsg{},
			viewBuilder.CanceledMsg{},
			viewBuilder.CanceledMsg{},
			viewBuilder.CanceledMsg{},
			viewBuilder.CanceledMsg{},
			viewBuilder.CanceledMsg{},
		}
		if !reflect.DeepEqual(msgs, want) {
			t.Fatalf("msgs = %#v, want %#v", msgs, want)
		}
	})

	t.Run("canceled after PID discovery", func(t *testing.T) {
		setupHome(t)
		startScanHelper(t, "noauthz")
		ctx, cancel := context.WithCancel(context.Background())

		msgs := runSequenceSteps(t, fetchInventory(ctx), func(step int, _ []tea.Msg) {
			if step == 0 {
				cancel()
			}
		})
		want := []tea.Msg{
			viewBuilder.StepMsg{Text: "Searching Warframe PID"},
			viewBuilder.CanceledMsg{},
			viewBuilder.CanceledMsg{},
			viewBuilder.CanceledMsg{},
			viewBuilder.CanceledMsg{},
			viewBuilder.CanceledMsg{},
			viewBuilder.CanceledMsg{},
			viewBuilder.CanceledMsg{},
		}
		if !reflect.DeepEqual(msgs, want) {
			t.Fatalf("msgs = %#v, want %#v", msgs, want)
		}
	})

	t.Run("canceled after Auth discovery", func(t *testing.T) {
		setupHome(t)
		startScanHelper(t, "authz")
		ctx, cancel := context.WithCancel(context.Background())

		msgs := runSequenceSteps(t, fetchInventory(ctx), func(step int, _ []tea.Msg) {
			if step == 1 {
				cancel()
			}
		})
		want := []tea.Msg{
			viewBuilder.StepMsg{Text: "Searching Warframe PID"},
			viewBuilder.StepMsg{Text: "Scanning memory for Auth string"},
			viewBuilder.CanceledMsg{},
			viewBuilder.CanceledMsg{},
			viewBuilder.CanceledMsg{},
			viewBuilder.CanceledMsg{},
			viewBuilder.CanceledMsg{},
		}
		if !reflect.DeepEqual(msgs, want) {
			t.Fatalf("msgs = %#v, want %#v", msgs, want)
		}
	})

	t.Run("canceled after API fetch", func(t *testing.T) {
		setupHome(t)
		startScanHelper(t, "authz")
		withDefaultTransport(t, roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return httpResponse(req, `{"items":[1,2]}`), nil
		}))
		ctx, cancel := context.WithCancel(context.Background())

		msgs := runSequenceSteps(t, fetchInventory(ctx), func(step int, _ []tea.Msg) {
			if step == 2 {
				cancel()
			}
		})
		want := []tea.Msg{
			viewBuilder.StepMsg{Text: "Searching Warframe PID"},
			viewBuilder.StepMsg{Text: "Scanning memory for Auth string"},
			viewBuilder.StepMsg{Text: "Fetching inventory"},
			viewBuilder.CanceledMsg{},
			viewBuilder.CanceledMsg{},
			viewBuilder.CanceledMsg{},
		}
		if !reflect.DeepEqual(msgs, want) {
			t.Fatalf("msgs = %#v, want %#v", msgs, want)
		}
	})

	t.Run("canceled after save", func(t *testing.T) {
		setupHome(t)
		startScanHelper(t, "authz")
		withDefaultTransport(t, roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return httpResponse(req, `{"items":[1,2]}`), nil
		}))
		ctx, cancel := context.WithCancel(context.Background())

		msgs := runSequenceSteps(t, fetchInventory(ctx), func(step int, _ []tea.Msg) {
			if step == 3 {
				cancel()
			}
		})
		want := []tea.Msg{
			viewBuilder.StepMsg{Text: "Searching Warframe PID"},
			viewBuilder.StepMsg{Text: "Scanning memory for Auth string"},
			viewBuilder.StepMsg{Text: "Fetching inventory"},
			viewBuilder.StepMsg{Text: "Saving inventory"},
			viewBuilder.CanceledMsg{},
		}
		if !reflect.DeepEqual(msgs, want) {
			t.Fatalf("msgs = %#v, want %#v", msgs, want)
		}
	})
}
