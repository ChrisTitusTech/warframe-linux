package fetchdata

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gjrud/warframe-helper/utils/database"
	"github.com/gjrud/warframe-helper/utils/viewBuilder"
)

type statusState int

const (
	statusChecking statusState = iota
	statusUpToDate
	statusStale
	statusUnknown
	statusMissing
)

type localStatusMsg struct {
	Exists bool
	Status database.SyncStatus
	Err    error
}

type upstreamStatusMsg struct {
	SHA string
	Err error
}

type syncCompleteMsg struct {
	At  time.Time
	SHA string
}

type Model struct {
	base        viewBuilder.BaseModel
	status      statusState
	storedSHA   string
	latestSHA   string
	lastSync    time.Time
	hasLastSync bool
}

func syndicateResultMsg(label string, before int, after int) tea.Msg {
	text := fmt.Sprintf("%s syndicate rows: %d -> %d (%+d)", label, before, after, after-before)
	if after == 0 {
		return viewBuilder.AlertMsg{Text: text}
	}
	if after < before {
		return viewBuilder.WarnMsg{Text: text}
	}
	return viewBuilder.StepMsg{Text: text}
}

func syndicateStepCmd(ctx context.Context, db *sql.DB, label string, itemType string, stepText string, failed *bool, work func() error) tea.Cmd {
	var before int
	return tea.Sequence(
		func() tea.Msg {
			if *failed {
				return nil
			}
			if ctx.Err() != nil {
				return viewBuilder.CanceledMsg{}
			}
			count, err := database.CountSyndicateItems(ctx, db, itemType)
			if err != nil {
				*failed = true
				return viewBuilder.ErrorMsg{Text: err.Error()}
			}
			before = count
			return nil
		},
		viewBuilder.StepCmd(ctx, stepText, failed, work),
		func() tea.Msg {
			if *failed {
				return nil
			}
			if ctx.Err() != nil {
				return viewBuilder.CanceledMsg{}
			}
			after, err := database.CountSyndicateItems(ctx, db, itemType)
			if err != nil {
				*failed = true
				return viewBuilder.ErrorMsg{Text: err.Error()}
			}
			return syndicateResultMsg(label, before, after)
		},
	)
}

func dbPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".warframe-helper", "warframe.db"), nil
}

func loadStatusCmd() tea.Cmd {
	return func() tea.Msg {
		path, err := dbPath()
		if err != nil {
			return localStatusMsg{Err: err}
		}
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return localStatusMsg{}
			}
			return localStatusMsg{Err: err}
		}

		db, err := database.OpenReadOnlyDB(path)
		if err != nil {
			return localStatusMsg{Exists: true, Err: err}
		}
		defer db.Close()

		status, err := database.ReadSyncStatus(db)
		return localStatusMsg{Exists: true, Status: status, Err: err}
	}
}

func checkUpstreamCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		sha, err := database.LatestWFCDCommitSHA(ctx)
		return upstreamStatusMsg{SHA: sha, Err: err}
	}
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func (m Model) Name() string {
	return "Database Operations"
}

func (m Model) PathSegments() []string {
	if !m.base.Chosen {
		return nil
	}
	return []string{"Update"}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.base.Spinner.Tick, loadStatusCmd())
}

func (m Model) statusLine() string {
	switch m.status {
	case statusUpToDate:
		return viewBuilder.SuccessStyle.Render("Database up to date: " + shortSHA(m.storedSHA))
	case statusStale:
		return viewBuilder.WarningStyle.Render(fmt.Sprintf("Database stale: %s -> %s", shortSHA(m.storedSHA), shortSHA(m.latestSHA)))
	case statusUnknown:
		return viewBuilder.WarningStyle.Render("Database status unknown")
	case statusMissing:
		return viewBuilder.ErrorStyle.Render("Database not present")
	default:
		return "Checking upstream status..."
	}
}

func (m Model) lastSyncLine() string {
	if !m.hasLastSync {
		return "Last sync: unknown"
	}
	return "Last sync: " + m.lastSync.UTC().Format("2006-01-02 15:04 UTC")
}

func (m Model) View() string {
	if !m.base.Chosen {
		return strings.Join([]string{
			m.statusLine(),
			m.lastSyncLine(),
			"",
			viewBuilder.ChoicesView(m.base.Choice, []string{"Update"}),
		}, "\n")
	}
	return viewBuilder.ProgressView(m.base.Steps, m.base.Spinner.View(), m.base.Done, m.base.Quitting)
}

func (m Model) Update(msg tea.Msg) (viewBuilder.SubView, tea.Cmd) {
	switch msg := msg.(type) {
	case localStatusMsg:
		m.latestSHA = ""
		m.storedSHA = ""
		m.hasLastSync = false
		m.lastSync = time.Time{}

		if !msg.Exists {
			m.status = statusMissing
			return m, nil
		}
		if msg.Err != nil || !msg.Status.Exists || msg.Status.WFCDCommitSHA == "" {
			m.status = statusUnknown
			return m, nil
		}

		m.storedSHA = msg.Status.WFCDCommitSHA
		m.hasLastSync = msg.Status.HasLastCompletedAt
		m.lastSync = msg.Status.LastCompletedAt
		m.status = statusChecking
		return m, checkUpstreamCmd()

	case upstreamStatusMsg:
		if msg.Err != nil || msg.SHA == "" || m.storedSHA == "" {
			m.status = statusUnknown
			m.latestSHA = ""
			return m, nil
		}

		m.latestSHA = msg.SHA
		if msg.SHA == m.storedSHA {
			m.status = statusUpToDate
		} else {
			m.status = statusStale
		}
		return m, nil

	case syncCompleteMsg:
		m.storedSHA = msg.SHA
		m.latestSHA = msg.SHA
		m.lastSync = msg.At.UTC()
		m.hasLastSync = true
		m.status = statusUpToDate
		m.base.Steps = append(m.base.Steps, "Done")
		m.base.Done = true
		m.base.Cancel = nil
		return m, nil
	}

	cmd := m.base.HandleMsg(msg, 1, func(_ int, ctx context.Context) tea.Cmd { return runSync(ctx) })
	return m, cmd
}

func NewModel() viewBuilder.SubView {
	return Model{
		base:   viewBuilder.NewBaseModel(),
		status: statusChecking,
	}
}

func runSync(ctx context.Context) tea.Cmd {
	path, err := dbPath()
	if err != nil {
		return func() tea.Msg { return viewBuilder.ErrorMsg{Text: err.Error()} }
	}
	db, err := database.OpenDB(path)
	if err != nil {
		return func() tea.Msg { return viewBuilder.ErrorMsg{Text: err.Error()} }
	}
	if err := database.EnsureSyndicates(db); err != nil {
		_ = db.Close()
		return func() tea.Msg { return viewBuilder.ErrorMsg{Text: err.Error()} }
	}
	if err := database.EnsureOfficialDrops(db); err != nil {
		_ = db.Close()
		return func() tea.Msg { return viewBuilder.ErrorMsg{Text: err.Error()} }
	}

	failed := false
	wfcdSHA := ""
	completedAt := time.Time{}
	done := func() tea.Msg {
		db.Close()
		if failed {
			return nil
		}
		if ctx.Err() != nil {
			return viewBuilder.CanceledMsg{}
		}
		return syncCompleteMsg{At: completedAt, SHA: wfcdSHA}
	}

	return tea.Sequence(
		viewBuilder.StepCmd(ctx, "Resolving WFCD snapshot", &failed, func() error {
			var err error
			wfcdSHA, err = database.LatestWFCDCommitSHA(ctx)
			return err
		}),
		viewBuilder.StepCmd(ctx, "Fetching item catalog from Warframe Market", &failed, func() error {
			return database.SyncFromWFM(ctx, db, "")
		}),
		syndicateStepCmd(ctx, db, "Mods", "mods", "Syncing syndicate data for mods", &failed, func() error {
			return database.EnrichMods(ctx, db, wfcdSHA)
		}),
		syndicateStepCmd(ctx, db, "Arcanes", "arcanes", "Syncing syndicate data for arcanes", &failed, func() error {
			return database.EnrichArcanes(ctx, db, wfcdSHA)
		}),
		syndicateStepCmd(ctx, db, "Weapons", "weapons", "Syncing syndicate and ducat data for weapons", &failed, func() error {
			return database.EnrichWeapons(ctx, db, wfcdSHA)
		}),
		viewBuilder.StepCmd(ctx, "Syncing ducat data for warframes, archwing, companions", &failed, func() error {
			return database.EnrichDucatCategories(ctx, db, wfcdSHA)
		}),
		viewBuilder.StepCmd(ctx, "Syncing official drops for nightmare and vault mods", &failed, func() error {
			return database.SyncOfficialModDrops(ctx, db)
		}),
		viewBuilder.StepCmd(ctx, "Saving sync status", &failed, func() error {
			completedAt = time.Now().UTC()
			return database.WriteSyncStatus(ctx, db, database.SyncStatus{
				HasLastCompletedAt: true,
				LastCompletedAt:    completedAt,
				WFCDCommitSHA:      wfcdSHA,
			})
		}),
		done,
	)
}
