package relicreward

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/gjrud/warframe-helper/utils/pricechecker"
	"github.com/gjrud/warframe-helper/utils/relicdetect"
	"github.com/gjrud/warframe-helper/utils/viewBuilder"
)

var options = []string{"Continuous Scan"}

var resultCols = []table.Column{
	{Title: "Name", Width: 38},
	{Title: "Avg (p)", Width: 9},
	{Title: "Vol", Width: 7},
}

type scanInitMsg struct {
	tmpl         relicdetect.Template
	prepared     relicdetect.PreparedItems
	capturer     relicdetect.Capturer
	wordListPath string
}

type scanTickMsg struct{}

type ocrDoneMsg struct {
	names []string
}

type fetchRewardMsg struct {
	toFetch   []pricechecker.GameObject
	rawNames  []string // parallel to toFetch; non-empty = unmatched (skip fetch, use as display name)
	approx    []bool   // parallel to toFetch; true = fuzzy match (display in orange)
	index     int
	items     []pricechecker.OutputItem
	itemFlags []bool
}

type cooldownDoneMsg struct{}

type Model struct {
	base         viewBuilder.BaseModel
	table        table.Model
	ctx          context.Context
	template     relicdetect.Template
	prepared     relicdetect.PreparedItems
	capturer     relicdetect.Capturer
	wordListPath string
}

func (m Model) Name() string { return "Relic Reward Checker" }

func (m Model) PathSegments() []string {
	if !m.base.Chosen || m.base.Choice < 0 || m.base.Choice >= len(options) {
		return nil
	}
	return []string{options[m.base.Choice]}
}

func (m Model) Init() tea.Cmd { return m.base.Spinner.Tick }

func (m *Model) resetToMenu() {
	m.base.ResetToMenu()
	m.table.SetRows(nil)
	m.capturer = nil
	m.prepared = relicdetect.PreparedItems{}
}

func (m Model) Update(msg tea.Msg) (viewBuilder.SubView, tea.Cmd) {
	switch msg := msg.(type) {
	case viewBuilder.AvailableSizeMsg:
		m.table.SetHeight(max(1, msg.Height))

	case scanInitMsg:
		m.template = msg.tmpl
		m.prepared = msg.prepared
		m.capturer = msg.capturer
		m.wordListPath = msg.wordListPath
		return m, scanCmd(m.ctx, m.capturer, m.template, m.wordListPath)

	case scanTickMsg:
		return m, scanCmd(m.ctx, m.capturer, m.template, m.wordListPath)

	case ocrDoneMsg:
		matches := relicdetect.MatchNames(msg.names, m.prepared)
		toFetch := make([]pricechecker.GameObject, len(matches))
		rawNames := make([]string, len(matches))
		approx := make([]bool, len(matches))
		for i, r := range matches {
			if r.Matched {
				toFetch[i] = r.Item
				approx[i] = r.Approximate
			} else {
				rawNames[i] = r.InputName
			}
		}
		return m, fetchNextReward(m.ctx, fetchRewardMsg{
			toFetch:   toFetch,
			rawNames:  rawNames,
			approx:    approx,
			items:     make([]pricechecker.OutputItem, 0, len(toFetch)),
			itemFlags: make([]bool, 0, len(toFetch)),
		})

	case fetchRewardMsg:
		return m, fetchNextReward(m.ctx, msg)

	case rewardReadyMsg:
		m.table.SetRows(msg.rows)
		m.table.GotoTop()
		return m, cooldownCmd(m.ctx)

	case cooldownDoneMsg:
		m.table.SetRows(nil)
		return m, scanCmd(m.ctx, m.capturer, m.template, m.wordListPath)

	case viewBuilder.CanceledMsg:
		m.resetToMenu()
		return m, nil

	case tea.KeyPressMsg:
		if key.Matches(msg, m.base.Keys.Quit) && m.base.Chosen {
			if m.base.Cancel != nil {
				m.base.Cancel()
			}
			m.resetToMenu()
			return m, nil
		}
		if len(m.table.Rows()) > 0 {
			var cmd tea.Cmd
			m.table, cmd = m.table.Update(msg)
			return m, cmd
		}
	}

	cmd := m.base.HandleMsg(msg, len(options), func(choice int, ctx context.Context) tea.Cmd {
		m.ctx = ctx
		return runInit(ctx)
	})
	return m, cmd
}

func (m Model) View() string {
	if !m.base.Chosen {
		return viewBuilder.ChoicesView(m.base.Choice, options)
	}
	if m.capturer == nil {
		return viewBuilder.ProgressView(m.base.Steps, m.base.Spinner.View(), false, m.base.Quitting)
	}
	if rows := m.table.Rows(); len(rows) > 0 {
		return m.table.View()
	}
	return m.base.Spinner.View() + " Scanning..."
}

func NewModel() viewBuilder.SubView {
	t := table.New(
		table.WithColumns(resultCols),
		table.WithWidth(viewBuilder.ColsWidth(resultCols)),
	)
	t.KeyMap.LineUp = viewBuilder.Defaultkeys.Up
	t.KeyMap.LineDown = viewBuilder.Defaultkeys.Down
	s := table.DefaultStyles()
	s.Selected = lipgloss.NewStyle()
	t.SetStyles(s)

	return Model{
		base:  viewBuilder.NewBaseModel(),
		table: t,
		ctx:   context.Background(),
	}
}

func runInit(ctx context.Context) tea.Cmd {
	var (
		failed       bool
		tmpl         relicdetect.Template
		dbItems      []pricechecker.GameObject
		capturer     relicdetect.Capturer
		wordListPath string
	)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return func() tea.Msg { return viewBuilder.ErrorMsg{Text: err.Error()} }
	}
	dbPath := filepath.Join(homeDir, ".warframe-helper", "warframe.db")

	templatePath := filepath.Join(homeDir, ".warframe-helper", "reward_template.png")

	return tea.Sequence(
		viewBuilder.StepCmd(ctx, "Loading template", &failed, func() error {
			if err := relicdetect.EnsureTemplate(ctx, templatePath); err != nil {
				return err
			}
			var err error
			tmpl, err = relicdetect.LoadTemplate(templatePath)
			return err
		}),
		viewBuilder.StepCmd(ctx, "Loading database items", &failed, func() error {
			var err error
			dbItems, err = pricechecker.QueryTradableItems(dbPath)
			if err != nil {
				return err
			}
			wlPath := filepath.Join(homeDir, ".warframe-helper", "wf-words.txt")
			if werr := relicdetect.WriteWordList(dbItems, wlPath); werr == nil {
				wordListPath = wlPath
			}
			return nil
		}),
		viewBuilder.StepCmd(ctx, "Probing screen capture", &failed, func() error {
			var err error
			capturer, err = relicdetect.ProbeCapture()
			return err
		}),
		func() tea.Msg {
			if failed || ctx.Err() != nil {
				return nil
			}
			return scanInitMsg{tmpl: tmpl, prepared: relicdetect.PrepareItems(dbItems), capturer: capturer, wordListPath: wordListPath}
		},
	)
}

func scanCmd(ctx context.Context, capturer relicdetect.Capturer, tmpl relicdetect.Template, wordListPath string) tea.Cmd {
	return func() tea.Msg {
		if !viewBuilder.Sleep(ctx, 5*time.Second) {
			return viewBuilder.CanceledMsg{}
		}
		imgs, err := capturer.CaptureAll()
		if err != nil {
			return viewBuilder.ErrorMsg{Text: err.Error()}
		}
		for _, img := range imgs {
			match := relicdetect.FindRewardScreen(img, tmpl, relicdetect.RewardThreshold)
			if match == nil {
				continue
			}
			names, err := relicdetect.ExtractItemNames(match, wordListPath)
			if err != nil {
				return viewBuilder.ErrorMsg{Text: err.Error()}
			}
			if len(names) > 0 {
				return ocrDoneMsg{names: names}
			}
		}
		return scanTickMsg{}
	}
}

func fetchNextReward(ctx context.Context, msg fetchRewardMsg) tea.Cmd {
	return func() tea.Msg {
		next := fetchRewardMsg{
			toFetch:   msg.toFetch,
			rawNames:  msg.rawNames,
			approx:    msg.approx,
			index:     msg.index + 1,
			items:     msg.items,
			itemFlags: msg.itemFlags,
		}
		if rawName := msg.rawNames[msg.index]; rawName != "" {
			next.items = append(next.items, pricechecker.OutputItem{Name: rawName})
			next.itemFlags = append(next.itemFlags, true)
		} else {
			if !viewBuilder.Sleep(ctx, 350*time.Millisecond) {
				return viewBuilder.CanceledMsg{}
			}
			item, err := pricechecker.FetchItemPrice(ctx, msg.toFetch[msg.index])
			if err == nil {
				next.items = append(next.items, item)
				next.itemFlags = append(next.itemFlags, msg.approx[msg.index])
			} else {
				if ctx.Err() != nil {
					return viewBuilder.CanceledMsg{}
				}
				next.items = append(next.items, pricechecker.OutputItem{
					Name:        msg.toFetch[msg.index].Name,
					GameRef:     msg.toFetch[msg.index].GameRef,
					FetchFailed: true,
				})
				next.itemFlags = append(next.itemFlags, msg.approx[msg.index])
			}
		}
		if next.index >= len(next.toFetch) {
			return buildRewardRows(next.items, next.itemFlags)
		}
		return next
	}
}

type rewardReadyMsg struct{ rows []table.Row }

type rewardItem struct {
	item      pricechecker.OutputItem
	unmatched bool
}

func buildRewardRows(items []pricechecker.OutputItem, flags []bool) rewardReadyMsg {
	paired := make([]rewardItem, len(items))
	for i := range items {
		paired[i] = rewardItem{item: items[i], unmatched: flags[i]}
	}
	slices.SortFunc(paired, func(a, b rewardItem) int {
		if a.item.FetchFailed != b.item.FetchFailed {
			if a.item.FetchFailed {
				return 1
			}
			return -1
		}
		scoreA := a.item.AvgPrice * float64(a.item.Count)
		scoreB := b.item.AvgPrice * float64(b.item.Count)
		if scoreA > scoreB {
			return -1
		}
		if scoreA < scoreB {
			return 1
		}
		return 0
	})
	rows := make([]table.Row, len(paired))
	for i, p := range paired {
		var priceStr, countStr string
		if p.item.FetchFailed {
			priceStr = "ERR"
			countStr = "ERR"
		} else if p.item.GameRef == "" {
			priceStr = "N/A"
			countStr = "-"
		} else {
			priceStr = fmt.Sprintf("%.1f", p.item.AvgPrice)
			countStr = fmt.Sprintf("%d", p.item.Count)
		}
		row := table.Row{p.item.Name, priceStr, countStr}
		if p.item.FetchFailed {
			for j, cell := range row {
				row[j] = viewBuilder.ErrorStyle.Render(cell)
			}
		} else if p.unmatched && p.item.GameRef != "" {
			for j, cell := range row {
				row[j] = viewBuilder.WarningStyle.Render(cell)
			}
		}
		rows[i] = row
	}
	return rewardReadyMsg{rows: rows}
}

func cooldownCmd(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		if !viewBuilder.Sleep(ctx, 30*time.Second) {
			return viewBuilder.CanceledMsg{}
		}
		return cooldownDoneMsg{}
	}
}
