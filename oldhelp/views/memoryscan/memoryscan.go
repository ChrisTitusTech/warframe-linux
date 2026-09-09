package memoryscan

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/gjrud/warframe-helper/utils/memoryScanner"
	"github.com/gjrud/warframe-helper/utils/viewBuilder"
)

const (
	memoryScanConsentFile   = "memory_scan_consent.json"
	memoryScanConsentPhrase = "I understand and accept the risk involved in using this functionality"
)

var options = []string{"Scan game memory", "Revoke permission"}

const (
	scanChoice = iota
	revokeChoice
)

type consentStatus uint8

type viewMode uint8

const (
	consentStatusUnknown consentStatus = iota
	consentStatusDenied
	consentStatusAllowed
	consentStatusUnavailable
)

const (
	viewModeMenu viewMode = iota
	viewModePrompt
	viewModeProgress
)

type consentRecord struct {
	Version    int       `json:"version"`
	Phrase     string    `json:"phrase"`
	AcceptedAt time.Time `json:"accepted_at"`
}

type consentCheckMsg struct {
	Granted bool
	Err     error
	Start   bool
}

type Model struct {
	base             viewBuilder.BaseModel
	input            textinput.Model
	ctx              context.Context
	mode             viewMode
	inputErr         string
	consentStatus    consentStatus
	consentStatusErr string
}

func NewModel() viewBuilder.SubView {
	input := textinput.New()
	input.Prompt = "> "
	input.Placeholder = memoryScanConsentPhrase
	input.SetWidth(len(memoryScanConsentPhrase))

	return Model{
		base:  viewBuilder.NewBaseModel(),
		input: input,
		ctx:   context.Background(),
		mode:  viewModeMenu,
	}
}

func (m Model) Name() string { return "Fetch Inventory" }

func (m Model) PathSegments() []string {
	if !m.base.Chosen || m.base.Choice < 0 || m.base.Choice >= len(options) {
		return nil
	}
	return []string{options[m.base.Choice]}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.base.Spinner.Tick, checkConsentCmd(false))
}

func (m *Model) resetToMenu() {
	m.base.ResetToMenu()
	m.resetLocalMenuState()
	m.ctx = context.Background()
}

func (m *Model) resetLocalMenuState() {
	m.mode = viewModeMenu
	m.inputErr = ""
	m.input.Reset()
	m.input.Blur()
}

func (m *Model) setConsentStatus(granted bool, err error) {
	m.consentStatusErr = ""
	if err != nil {
		m.consentStatus = consentStatusUnavailable
		m.consentStatusErr = err.Error()
		return
	}
	if granted {
		m.consentStatus = consentStatusAllowed
		return
	}
	m.consentStatus = consentStatusDenied
}

func riskWarningLine() string {
	return viewBuilder.ErrorStyle.Render("Warning: scanning game memory could lead to a possible ban.")
}

func (m Model) statusLine() string {
	switch m.consentStatus {
	case consentStatusAllowed:
		return viewBuilder.SuccessStyle.Render("Permission status: allowed")
	case consentStatusDenied:
		return viewBuilder.WarningStyle.Render("Permission status: denied")
	case consentStatusUnavailable:
		return viewBuilder.WarningStyle.Render("Permission status unavailable: " + m.consentStatusErr)
	default:
		return "Checking permission status..."
	}
}

func (m Model) menuView() string {
	var sb strings.Builder
	sb.WriteString(m.statusLine())
	if m.consentStatus == consentStatusAllowed {
		sb.WriteString("\n\n")
		sb.WriteString(riskWarningLine())
	}
	sb.WriteString("\n\n")
	sb.WriteString(viewBuilder.ChoicesView(m.base.Choice, options))
	return sb.String()
}

func (m Model) promptView() string {
	var sb strings.Builder
	sb.WriteString(riskWarningLine())
	sb.WriteString("\n\nType the exact phrase to continue.\n\n")
	sb.WriteString(viewBuilder.WarningStyle.Render(memoryScanConsentPhrase))
	sb.WriteString("\n\n")
	sb.WriteString(m.input.View())
	if m.inputErr != "" {
		sb.WriteString("\n\n")
		sb.WriteString(viewBuilder.ErrorStyle.Render(m.inputErr))
	}
	sb.WriteString("\n\nPress Enter to continue, or q to cancel.")
	return sb.String()
}

func (m Model) Update(msg tea.Msg) (viewBuilder.SubView, tea.Cmd) {
	wasChosen := m.base.Chosen

	switch msg := msg.(type) {
	case viewBuilder.AvailableSizeMsg:
		m.input.SetWidth(max(24, min(msg.Width-4, len(memoryScanConsentPhrase))))
		return m, nil

	case consentCheckMsg:
		if msg.Start && (m.ctx.Err() != nil || m.base.Cancel == nil || m.base.Quitting) {
			m.resetToMenu()
			return m, refreshConsentCmd()
		}
		m.setConsentStatus(msg.Granted, msg.Err)
		if msg.Err != nil {
			if !msg.Start {
				return m, nil
			}
			m.base.HandleMsg(viewBuilder.ErrorMsg{Text: msg.Err.Error()}, len(options), nil)
			return m, nil
		}
		if !msg.Start {
			return m, nil
		}
		if msg.Granted {
			m.mode = viewModeProgress
			return m, fetchInventory(m.ctx)
		}
		m.mode = viewModePrompt
		m.inputErr = ""
		m.input.Reset()
		m.base.Steps = nil
		return m, m.input.Focus()

	case viewBuilder.CanceledMsg:
		m.resetToMenu()
		return m, refreshConsentCmd()

	case tea.KeyPressMsg:
		if m.mode == viewModePrompt {
			switch {
			case key.Matches(msg, m.base.Keys.Quit):
				if m.base.Cancel != nil {
					m.base.Cancel()
				}
				m.resetToMenu()
				return m, nil
			case key.Matches(msg, m.base.Keys.Enter):
				if strings.TrimSpace(m.input.Value()) != memoryScanConsentPhrase {
					m.inputErr = "Type the exact phrase to continue."
					return m, nil
				}
				m.mode = viewModeProgress
				m.inputErr = ""
				m.setConsentStatus(true, nil)
				m.input.Blur()
				return m, saveConsentAndFetchCmd(m.ctx)
			}

			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			if m.inputErr != "" {
				m.inputErr = ""
			}
			return m, cmd
		}
	}

	cmd := m.base.HandleMsg(msg, len(options), func(choice int, ctx context.Context) tea.Cmd {
		m.ctx = ctx
		m.mode = viewModeProgress
		switch choice {
		case scanChoice:
			return checkConsentCmd(true)
		case revokeChoice:
			return revokeConsentCmd(ctx)
		default:
			return nil
		}
	})
	if wasChosen && !m.base.Chosen {
		m.resetLocalMenuState()
		if cmd == nil {
			return m, refreshConsentCmd()
		}
		return m, tea.Batch(cmd, refreshConsentCmd())
	}
	return m, cmd
}

func (m Model) View() string {
	if !m.base.Chosen {
		return m.menuView()
	}
	switch m.mode {
	case viewModeMenu:
		return m.menuView()
	case viewModePrompt:
		return m.promptView()
	default:
		return viewBuilder.ProgressView(m.base.Steps, m.base.Spinner.View(), m.base.Done, m.base.Quitting)
	}
}

func consentPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".warframe-helper", memoryScanConsentFile), nil
}

func hasSavedConsent() (bool, error) {
	path, err := consentPath()
	if err != nil {
		return false, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	var record consentRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return false, nil
	}
	return record.Version == 1 && record.Phrase == memoryScanConsentPhrase, nil
}

func saveConsent() error {
	path, err := consentPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	record := consentRecord{
		Version:    1,
		Phrase:     memoryScanConsentPhrase,
		AcceptedAt: time.Now().UTC(),
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func revokeConsent() error {
	path, err := consentPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func checkConsentCmd(start bool) tea.Cmd {
	return func() tea.Msg {
		granted, err := hasSavedConsent()
		return consentCheckMsg{Granted: granted, Err: err, Start: start}
	}
}

func refreshConsentCmd() tea.Cmd {
	return checkConsentCmd(false)
}

func saveConsentAndFetchCmd(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		if ctx.Err() != nil {
			return viewBuilder.CanceledMsg{}
		}
		if err := saveConsent(); err != nil {
			return viewBuilder.ErrorMsg{Text: err.Error()}
		}
		if ctx.Err() != nil {
			return viewBuilder.CanceledMsg{}
		}
		return fetchInventory(ctx)()
	}
}

func revokeConsentCmd(ctx context.Context) tea.Cmd {
	failed := false
	return tea.Sequence(
		viewBuilder.StepCmd(ctx, "Revoking saved memory scan permission", &failed, revokeConsent),
		func() tea.Msg {
			if failed {
				return nil
			}
			if ctx.Err() != nil {
				return viewBuilder.CanceledMsg{}
			}
			return viewBuilder.DoneMsg{}
		},
	)
}

func fetchInventory(ctx context.Context) tea.Cmd {
	var (
		failed bool
		pid    int
		auth   string
		stream []byte
	)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return func() tea.Msg { return viewBuilder.ErrorMsg{Text: err.Error()} }
	}
	path := filepath.Join(homeDir, ".warframe-helper", "inventory.json")

	return tea.Sequence(
		viewBuilder.StepCmd(ctx, "Searching Warframe PID", &failed, func() error {
			var err error
			pid, err = memoryScanner.FindProcess()
			return err
		}),
		viewBuilder.StepCmd(ctx, "Scanning memory for Auth string", &failed, func() error {
			var err error
			auth, err = memoryScanner.ScanAuthz(pid)
			return err
		}),
		viewBuilder.StepCmd(ctx, "Fetching inventory", &failed, func() error {
			var err error
			stream, err = memoryScanner.FetchInventory(ctx, auth)
			return err
		}),
		viewBuilder.StepCmd(ctx, "Saving inventory", &failed, func() error {
			return memoryScanner.SaveInventory(stream, path)
		}),
		func() tea.Msg {
			if failed {
				return nil
			}
			if ctx.Err() != nil {
				return viewBuilder.CanceledMsg{}
			}
			return viewBuilder.DoneMsg{}
		},
	)
}
