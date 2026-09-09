package pricecheck

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/gjrud/warframe-helper/utils/pricechecker"
	"github.com/gjrud/warframe-helper/utils/viewBuilder"
)

type ResultsMsg struct {
	Mode  resultMode
	Items []pricechecker.OutputItem
}

type resultMode int

const (
	pricecheckMode resultMode = iota
	syndicateMode
	ducatMode
	nightmareMode
	vaultMode
)

type fetchItemsMsg struct {
	mode    resultMode
	items   []pricechecker.GameObject
	index   int
	results []pricechecker.OutputItem
}

var options = []string{"Inventory", "Ducats", "Syndicates", "Nightmare Mods", "Corrupted Mods"}

const inventoryRequiredSuffix = " (requires inventory.json)"

var disabledChoiceStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))

var resultCols = []table.Column{
	{Title: "Name", Width: 38},
	{Title: "Type", Width: 12},
	{Title: "Avg (p)", Width: 9},
	{Title: "Vol", Width: 7},
}

var syndicateResultCols = []table.Column{
	{Title: "Name", Width: 25},
	{Title: "Type", Width: 12},
	{Title: "Avg (p)", Width: 9},
	{Title: "Vol", Width: 7},
	{Title: "Syndicates", Width: 30},
}

var ducatResultCols = []table.Column{
	{Title: "Name", Width: 38},
	{Title: "Ducats", Width: 8},
	{Title: "Avg (p)", Width: 9},
}

var nightmareResultCols = []table.Column{
	{Title: "Name", Width: 28},
	{Title: "Avg (p)", Width: 9},
	{Title: "Vol", Width: 7},
	{Title: "Rotation", Width: 12},
	{Title: "Chance (%)", Width: 12},
}

var vaultResultCols = []table.Column{
	{Title: "Name", Width: 38},
	{Title: "Avg (p)", Width: 9},
	{Title: "Vol", Width: 7},
}

type Model struct {
	base     viewBuilder.BaseModel
	table    table.Model
	progress progress.Model
	ctx      context.Context
	fetching bool
}

func (m Model) Name() string {
	return "Price Check"
}

func (m Model) PathSegments() []string {
	if !m.base.Chosen || m.base.Choice < 0 || m.base.Choice >= len(options) {
		return nil
	}
	return []string{options[m.base.Choice]}
}

func (m *Model) resetToMenu() {
	m.base.ResetToMenu()
	m.table.SetRows(nil)
	m.table.SetColumns(resultCols)
	m.table.SetWidth(viewBuilder.ColsWidth(resultCols))
	m.fetching = false
}

func (m Model) Init() tea.Cmd {
	return m.base.Spinner.Tick
}

func sortResults(mode resultMode, items []pricechecker.OutputItem) {
	if mode == ducatMode {
		pricechecker.SortDucatResults(items)
		return
	}
	pricechecker.SortResults(items)
}

func inventoryFilePath(homeDir string) string {
	return filepath.Join(homeDir, ".warframe-helper", "inventory.json")
}

func inventoryFileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return !info.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func inventoryAvailable() (bool, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false, err
	}
	return inventoryFileExists(inventoryFilePath(homeDir))
}

func normalizeChoice(choice int, hasInventory bool) int {
	if choice < 0 {
		choice = 0
	}
	if choice >= len(options) {
		choice = len(options) - 1
	}
	if hasInventory || choice >= 2 {
		return choice
	}
	return 2
}

func moveChoice(choice int, delta int, hasInventory bool) int {
	choice = normalizeChoice(choice, hasInventory)
	choice += delta
	if choice < 0 {
		return 0
	}
	if choice >= len(options) {
		return len(options) - 1
	}
	if !hasInventory && choice < 2 {
		return 2
	}
	return choice
}

func choicesView(choice int, hasInventory bool) string {
	choice = normalizeChoice(choice, hasInventory)
	var sb strings.Builder
	for i := range options {
		disabled := !hasInventory && i < 2
		label := options[i]
		if disabled {
			label += inventoryRequiredSuffix
		}
		line := "[ ] " + label
		if i == choice {
			line = "[x] " + label
		}
		switch {
		case disabled:
			line = disabledChoiceStyle.Render(line)
		case i == choice:
			line = viewBuilder.HighlightStyle.Render(line)
		}
		sb.WriteString(line + "\n")
	}
	return sb.String()
}

func failedItem(item pricechecker.GameObject) pricechecker.OutputItem {
	return pricechecker.OutputItem{
		Name:        item.Name,
		GameRef:     item.GameRef,
		Category:    item.Category,
		Syndicates:  item.Syndicates,
		Ducats:      item.Ducats,
		Rotation:    item.Rotation,
		Chance:      item.Chance,
		FetchFailed: true,
	}
}

func startFetch(mode resultMode, items []pricechecker.GameObject) tea.Msg {
	if len(items) == 0 {
		return ResultsMsg{Mode: mode}
	}
	return fetchItemsMsg{
		mode:    mode,
		items:   items,
		results: make([]pricechecker.OutputItem, 0, len(items)),
	}
}

func renderRows(mode resultMode, items []pricechecker.OutputItem) ([]table.Column, []table.Row) {
	var cols []table.Column
	switch mode {
	case ducatMode:
		cols = ducatResultCols
	case nightmareMode:
		cols = nightmareResultCols
	case syndicateMode:
		cols = syndicateResultCols
	case vaultMode:
		cols = vaultResultCols
	default:
		cols = resultCols
	}

	rows := make([]table.Row, len(items))
	for i, item := range items {
		avg := fmt.Sprintf("%.1f", item.AvgPrice)
		count := fmt.Sprintf("%d", item.Count)

		var row table.Row
		switch mode {
		case ducatMode:
			row = table.Row{item.Name, fmt.Sprintf("%d", item.Ducats), avg}
		case nightmareMode:
			row = table.Row{item.Name, avg, count, item.Rotation, fmt.Sprintf("%.2f", item.Chance)}
		case syndicateMode:
			row = table.Row{item.Name, item.Category, avg, count, item.Syndicates}
		case vaultMode:
			row = table.Row{item.Name, avg, count}
		default:
			row = table.Row{item.Name, item.Category, avg, count}
		}

		if item.FetchFailed {
			for j := range row {
				if row[j] == "0" || row[j] == "0.0" {
					row[j] = "ERR"
				}
				row[j] = viewBuilder.ErrorStyle.Render(row[j])
			}
		}
		rows[i] = row
	}
	return cols, rows
}

func fetchNextItem(ctx context.Context, msg fetchItemsMsg) tea.Cmd {
	return func() tea.Msg {
		if !viewBuilder.Sleep(ctx, 350*time.Millisecond) {
			return viewBuilder.CanceledMsg{}
		}
		item, err := pricechecker.FetchItemPrice(ctx, msg.items[msg.index])
		next := fetchItemsMsg{
			mode:    msg.mode,
			items:   msg.items,
			index:   msg.index + 1,
			results: msg.results,
		}
		if err != nil {
			if ctx.Err() != nil {
				return viewBuilder.CanceledMsg{}
			}
			next.results = append(next.results, failedItem(msg.items[msg.index]))
		} else {
			next.results = append(next.results, item)
		}
		if next.index >= len(next.items) {
			sortResults(next.mode, next.results)
			return ResultsMsg{Mode: next.mode, Items: next.results}
		}
		return next
	}
}

func (m Model) Update(msg tea.Msg) (viewBuilder.SubView, tea.Cmd) {
	switch msg := msg.(type) {
	case progress.FrameMsg:
		var cmd tea.Cmd
		m.progress, cmd = m.progress.Update(msg)
		return m, cmd

	case viewBuilder.AvailableSizeMsg:
		m.table.SetHeight(max(1, msg.Height))
		if msg.Width > 0 {
			m.progress.SetWidth(max(1, msg.Width-3))
		}

	case fetchItemsMsg:
		m.fetching = true
		var pct float64
		if len(msg.items) > 0 {
			pct = float64(msg.index) / float64(len(msg.items))
		}
		animCmd := m.progress.SetPercent(pct)
		return m, tea.Batch(fetchNextItem(m.ctx, msg), animCmd)

	case ResultsMsg:
		cols, rows := renderRows(msg.Mode, msg.Items)
		m.table.SetColumns(cols)
		m.table.SetWidth(viewBuilder.ColsWidth(cols))
		m.table.SetRows(rows)
		m.table.GotoTop()
		m.base.Steps = append(m.base.Steps, "Done")
		m.base.Done = true
		m.base.Cancel = nil
		m.fetching = false
		return m, m.progress.SetPercent(1.0)

	case viewBuilder.CanceledMsg:
		m.resetToMenu()
		return m, nil

	case tea.KeyPressMsg:
		if key.Matches(msg, m.base.Keys.Quit) && m.base.Cancel == nil && m.base.Chosen {
			m.resetToMenu()
			return m, nil
		}
		if !m.base.Chosen {
			if hasInventory, err := inventoryAvailable(); err == nil {
				switch {
				case key.Matches(msg, m.base.Keys.Down):
					m.base.Choice = moveChoice(m.base.Choice, 1, hasInventory)
					return m, nil
				case key.Matches(msg, m.base.Keys.Up):
					m.base.Choice = moveChoice(m.base.Choice, -1, hasInventory)
					return m, nil
				case key.Matches(msg, m.base.Keys.Enter):
					m.base.Choice = normalizeChoice(m.base.Choice, hasInventory)
					ctx, cancel := context.WithCancel(context.Background())
					m.base.Cancel = cancel
					m.base.Chosen = true
					m.base.Done = false
					m.base.Steps = nil
					m.ctx = ctx
					return m, runOperation(m.base.Choice, ctx)
				}
			}
		}
		if m.base.Done {
			var cmd tea.Cmd
			m.table, cmd = m.table.Update(msg)
			return m, cmd
		}
	}

	cmd := m.base.HandleMsg(msg, len(options), func(choice int, ctx context.Context) tea.Cmd {
		m.ctx = ctx
		return runOperation(choice, ctx)
	})
	return m, cmd
}

func (m Model) View() string {
	if !m.base.Chosen {
		hasInventory, err := inventoryAvailable()
		if err != nil {
			hasInventory = true
		}
		return choicesView(m.base.Choice, hasInventory)
	}
	if m.base.Done {
		return m.table.View()
	}
	s := viewBuilder.ProgressView(m.base.Steps, m.base.Spinner.View(), m.base.Done, m.base.Quitting)
	if m.fetching {
		s += "\n" + m.progress.View() + "\n"
	}
	return s
}

func NewModel() viewBuilder.SubView {
	t := table.New(
		table.WithColumns(resultCols),
		table.WithWidth(viewBuilder.ColsWidth(resultCols)),
		table.WithFocused(true),
	)
	styles := table.DefaultStyles()
	styles.Selected = styles.Selected.Foreground(viewBuilder.HighlightColor)
	t.SetStyles(styles)
	t.KeyMap.LineUp = viewBuilder.Defaultkeys.Up
	t.KeyMap.LineDown = viewBuilder.Defaultkeys.Down

	p := progress.New(
		progress.WithColors(viewBuilder.ProgressBlend()...),
	)

	base := viewBuilder.NewBaseModel()

	return Model{
		base:     base,
		table:    t,
		progress: p,
		ctx:      context.Background(),
	}
}

func runOperation(choice int, ctx context.Context) tea.Cmd {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return func() tea.Msg { return viewBuilder.ErrorMsg{Text: err.Error()} }
	}
	if choice < 2 {
		path := inventoryFilePath(homeDir)
		hasInventory, err := inventoryFileExists(path)
		if err != nil {
			return func() tea.Msg { return viewBuilder.ErrorMsg{Text: err.Error()} }
		}
		if !hasInventory {
			return func() tea.Msg { return viewBuilder.ErrorMsg{Text: fmt.Sprintf("%s is required for this choice", path)} }
		}
	}

	var (
		mode      resultMode
		loadStep  string
		query     func(string) ([]pricechecker.GameObject, error)
		readOwned bool
		failed    bool
		owned     []string
		items     []pricechecker.GameObject
	)

	switch choice {
	case 0:
		mode = pricecheckMode
		query = pricechecker.QueryTradableItems
		readOwned = true
	case 1:
		mode = ducatMode
		query = pricechecker.QueryDucatItems
		readOwned = true
	case 2:
		mode = syndicateMode
		loadStep = "Loading syndicate items"
		query = pricechecker.QuerySyndicateTradableItems
	case 3:
		mode = nightmareMode
		loadStep = "Loading nightmare mods"
		query = pricechecker.QueryNightmareItems
	default:
		mode = vaultMode
		loadStep = "Loading vault mods"
		query = pricechecker.QueryVaultItems
	}

	dbPath := filepath.Join(homeDir, ".warframe-helper", "warframe.db")
	if readOwned {
		return tea.Sequence(
			viewBuilder.StepCmd(ctx, "Reading inventory file", &failed, func() error {
				var err error
				owned, err = pricechecker.ReadInventory()
				return err
			}),
			viewBuilder.StepCmd(ctx, "Loading database items", &failed, func() error {
				var err error
				items, err = query(dbPath)
				return err
			}),
			func() tea.Msg {
				if failed || ctx.Err() != nil {
					if ctx.Err() != nil {
						return viewBuilder.CanceledMsg{}
					}
					return nil
				}
				return viewBuilder.StepMsg{Text: "Fetching market prices"}
			},
			func() tea.Msg {
				if failed || ctx.Err() != nil {
					if ctx.Err() != nil {
						return viewBuilder.CanceledMsg{}
					}
					return nil
				}
				return startFetch(mode, pricechecker.FilterOwned(owned, items))
			},
		)
	}

	return tea.Sequence(
		viewBuilder.StepCmd(ctx, loadStep, &failed, func() error {
			var err error
			items, err = query(dbPath)
			return err
		}),
		func() tea.Msg {
			if failed || ctx.Err() != nil {
				if ctx.Err() != nil {
					return viewBuilder.CanceledMsg{}
				}
				return nil
			}
			return viewBuilder.StepMsg{Text: "Fetching market prices"}
		},
		func() tea.Msg {
			if failed || ctx.Err() != nil {
				if ctx.Err() != nil {
					return viewBuilder.CanceledMsg{}
				}
				return nil
			}
			return startFetch(mode, items)
		},
	)
}
