package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/gjrud/warframe-helper/utils/viewBuilder"
	"github.com/gjrud/warframe-helper/views/fetchdata"
	"github.com/gjrud/warframe-helper/views/memoryscan"
	"github.com/gjrud/warframe-helper/views/pricecheck"
	"github.com/gjrud/warframe-helper/views/relicreward"
)

const (
	terminalOverhead = 4
	borderWidth      = 2
	titleSeparator   = " ⋗ "
)

var (
	contentStyle = lipgloss.NewStyle().Padding(1, 1, 0, 1)
)

type model struct {
	views     []viewBuilder.SubView
	keys      viewBuilder.KeyMap
	help      help.Model
	quitting  bool
	choice    int
	chosen    bool
	height    int
	width     int
	available int
}

func (m model) helpView() string {
	return "\n" + m.help.View(m.keys)
}

func (m model) computeAvailable() int {
	return m.height - strings.Count(m.helpView(), "\n") - terminalOverhead
}

func (m model) Init() tea.Cmd {
	cmds := make([]tea.Cmd, len(m.views))
	for i, v := range m.views {
		cmds[i] = v.Init()
	}
	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case viewBuilder.BackMsg:
		m.chosen = false
		return m, nil

	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		m.available = m.computeAvailable()
		if m.chosen {
			return m, func() tea.Msg {
				return viewBuilder.AvailableSizeMsg{Height: m.available, Width: m.width - borderWidth}
			}
		}
		return m, nil

	case tea.KeyPressMsg:
		if key.Matches(msg, m.keys.Help) {
			m.help.ShowAll = !m.help.ShowAll
			m.available = m.computeAvailable()
			if m.chosen {
				return m, func() tea.Msg {
					return viewBuilder.AvailableSizeMsg{Height: m.available, Width: m.width - borderWidth}
				}
			}
			return m, nil
		} else if !m.chosen {
			switch {
			case key.Matches(msg, m.keys.Down):
				if m.choice++; m.choice >= len(m.views) {
					m.choice = len(m.views) - 1
				}
			case key.Matches(msg, m.keys.Up):
				if m.choice--; m.choice < 0 {
					m.choice = 0
				}
			case key.Matches(msg, m.keys.Enter):
				m.chosen = true
				return m, tea.Batch(
					m.views[m.choice].Init(),
					func() tea.Msg {
						return viewBuilder.AvailableSizeMsg{Height: m.available, Width: m.width - borderWidth}
					},
				)
			case key.Matches(msg, m.keys.Quit):
				m.quitting = true
				return m, tea.Quit
			}
		}
	}

	if m.chosen {
		m.views[m.choice], cmd = m.views[m.choice].Update(msg)
	}

	return m, cmd
}

func injectBorderTitle(rendered, title string) string {
	newline := strings.IndexByte(rendered, '\n')
	if newline < 0 {
		return rendered
	}
	topLine := rendered[:newline]
	topWidth := lipgloss.Width(topLine)
	maxTitleWidth := topWidth - 5
	if maxTitleWidth <= 0 {
		return rendered
	}
	titleWidth := min(lipgloss.Width(title), maxTitleWidth)
	title = ansi.Cut(title, 0, titleWidth)
	skip := titleWidth + 4
	return ansi.Cut(topLine, 0, 2) + " " + title + " " + ansi.Cut(topLine, skip, topWidth) + rendered[newline:]
}

func (m model) titleSegments() []string {
	segments := []string{"Warframe Helper"}
	if !m.chosen || m.choice < 0 || m.choice >= len(m.views) {
		return segments
	}

	active := m.views[m.choice]
	segments = append(segments, active.Name())
	if pathView, ok := active.(viewBuilder.PathView); ok {
		segments = append(segments, pathView.PathSegments()...)
	}
	return segments
}

func (m model) title() string {
	return strings.Join(m.titleSegments(), titleSeparator)
}

func (m model) View() tea.View {
	helpView := m.helpView()

	content := ""
	if m.quitting {
		content = "Bye!"
	} else if !m.chosen {
		viewNames := make([]string, len(m.views))
		for i, v := range m.views {
			viewNames[i] = v.Name()
		}
		content = viewBuilder.ChoicesView(m.choice, viewNames)
	} else {
		content = m.views[m.choice].View()
	}

	if m.available > 0 {
		if contentLines := strings.Split(content, "\n"); len(contentLines) > m.available {
			content = strings.Join(contentLines[:m.available], "\n")
		}
	}
	pad := max(0, m.available-strings.Count(content, "\n"))

	paddedContent := contentStyle.Width(m.width - borderWidth).Render(content + strings.Repeat("\n", pad))
	full := injectBorderTitle(viewBuilder.BorderStyle.Width(m.width).Render(paddedContent+helpView), viewBuilder.TitleStyle.Render(m.title()))
	v := tea.NewView(full + "\n")
	v.AltScreen = true
	return v
}

func newModel() model {
	views := []viewBuilder.SubView{
		fetchdata.NewModel(),
		memoryscan.NewModel(),
		pricecheck.NewModel(),
		relicreward.NewModel(),
	}
	return model{
		views:    views,
		keys:     viewBuilder.Defaultkeys,
		help:     help.New(),
		quitting: false,
		choice:   0,
		chosen:   false,
	}
}

func main() {
	if len(os.Getenv("DEBUG")) > 0 {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			fmt.Println("fatal:", err)
			os.Exit(1)
		}
		defer f.Close()
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error resolving home directory:", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(filepath.Join(homeDir, ".warframe-helper"), 0755); err != nil {
		fmt.Println("Error creating app data directory:", err)
		os.Exit(1)
	}

	if _, err := tea.NewProgram(newModel()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
