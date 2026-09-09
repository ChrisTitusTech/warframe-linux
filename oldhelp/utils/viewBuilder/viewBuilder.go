package viewBuilder

import (
	"context"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
)

type BackMsg struct{}

type StepMsg struct{ Text string }
type ErrorMsg struct{ Text string }
type WarnMsg struct{ Text string }
type AlertMsg struct{ Text string }
type CanceledMsg struct{}
type DoneMsg struct{}
type AvailableSizeMsg struct {
	Height int
	Width  int
}

type SubView interface {
	Update(tea.Msg) (SubView, tea.Cmd)
	View() string
	Init() tea.Cmd
	Name() string
}

type PathView interface {
	PathSegments() []string
}

var SpinnerType = spinner.Dot

func ColsWidth(cols []table.Column) int {
	w := 0
	for _, c := range cols {
		w += c.Width + 2
	}
	return w
}

func Sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func NewSpinner() spinner.Model {
	s := spinner.New()
	s.Style = HighlightStyle
	s.Spinner = SpinnerType
	return s
}

func ChoicesView(choice int, options []string) string {
	var sb strings.Builder
	for i, option := range options {
		sb.WriteString(checkbox(option, choice == i) + "\n")
	}
	return sb.String()
}

func checkbox(label string, checked bool) string {
	if checked {
		return HighlightStyle.Render("[x] " + label)
	}
	return "[ ] " + label
}

func StepCmd(ctx context.Context, text string, failed *bool, work func() error) tea.Cmd {
	return tea.Sequence(
		func() tea.Msg {
			if *failed {
				return nil
			}
			if ctx.Err() != nil {
				return CanceledMsg{}
			}
			return StepMsg{Text: text}
		},
		func() tea.Msg {
			if *failed {
				return nil
			}
			if ctx.Err() != nil {
				return CanceledMsg{}
			}
			if err := work(); err != nil {
				*failed = true
				return ErrorMsg{Text: err.Error()}
			}
			return nil
		},
	)
}

func ProgressView(steps []string, spinnerStr string, done bool, quitting bool) string {
	var sb strings.Builder
	for i, s := range steps {
		isLast := i == len(steps)-1
		isError := strings.HasPrefix(s, "error:")
		isWarn := strings.HasPrefix(s, "warn:")
		isAlert := strings.HasPrefix(s, "alert:")
		if isError || isAlert || (quitting && isLast) {
			s = strings.TrimPrefix(s, "alert: ")
			sb.WriteString(ErrorStyle.Render("✗ "+strings.TrimPrefix(s, "error: ")) + "\n")
		} else if isWarn {
			sb.WriteString(WarningStyle.Render("⚠ "+strings.TrimPrefix(s, "warn: ")) + "\n")
		} else if !isLast || done {
			sb.WriteString(SuccessStyle.Render("✓ "+s) + "\n")
		} else {
			sb.WriteString(spinnerStr + s + "\n")
		}
	}
	if quitting {
		sb.WriteString(spinnerStr + "Quitting...\n")
	}
	return sb.String()
}

type BaseModel struct {
	Keys     KeyMap
	Spinner  spinner.Model
	Choice   int
	Chosen   bool
	Done     bool
	Quitting bool
	Steps    []string
	Cancel   context.CancelFunc
}

func NewBaseModel() BaseModel {
	return BaseModel{
		Keys:    Defaultkeys,
		Spinner: NewSpinner(),
	}
}

func (b *BaseModel) ResetToMenu() {
	b.Chosen = false
	b.Steps = nil
	b.Done = false
	b.Quitting = false
	b.Cancel = nil
}

func (b *BaseModel) HandleMsg(msg tea.Msg, numOptions int, onEnter func(int, context.Context) tea.Cmd) tea.Cmd {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		b.Spinner, cmd = b.Spinner.Update(msg)
		return cmd

	case StepMsg:
		b.Steps = append(b.Steps, msg.Text)
		return nil

	case WarnMsg:
		b.Steps = append(b.Steps, "warn: "+msg.Text)
		return nil

	case AlertMsg:
		b.Steps = append(b.Steps, "alert: "+msg.Text)
		return nil

	case ErrorMsg:
		if len(b.Steps) > 0 {
			b.Steps[len(b.Steps)-1] = "error: " + b.Steps[len(b.Steps)-1] + ": " + msg.Text
		} else {
			b.Steps = append(b.Steps, "error: "+msg.Text)
		}
		if b.Cancel != nil {
			b.Cancel()
			b.Cancel = nil
		}
		return nil

	case DoneMsg:
		b.Steps = append(b.Steps, "Done")
		b.Done = true
		b.Cancel = nil
		return nil

	case CanceledMsg:
		b.ResetToMenu()
		return nil

	case tea.KeyPressMsg:
		if key.Matches(msg, b.Keys.Quit) {
			if b.Cancel != nil {
				b.Quitting = true
				b.Cancel()
				return nil
			}
			if b.Chosen {
				b.ResetToMenu()
				return nil
			}
			return func() tea.Msg { return BackMsg{} }
		}
		if !b.Chosen {
			switch {
			case key.Matches(msg, b.Keys.Down):
				if b.Choice++; b.Choice >= numOptions {
					b.Choice = numOptions - 1
				}
				return nil
			case key.Matches(msg, b.Keys.Up):
				if b.Choice--; b.Choice < 0 {
					b.Choice = 0
				}
				return nil
			case key.Matches(msg, b.Keys.Enter):
				ctx, cancel := context.WithCancel(context.Background())
				b.Cancel = cancel
				b.Chosen = true
				b.Done = false
				b.Steps = nil
				return onEnter(b.Choice, ctx)
			}
		}
	}
	return nil
}

type simpleView struct {
	base    BaseModel
	name    string
	options []string
	run     func(int, context.Context) tea.Cmd
}

func NewSimpleView(name string, options []string, run func(int, context.Context) tea.Cmd) SubView {
	return simpleView{
		base:    NewBaseModel(),
		name:    name,
		options: options,
		run:     run,
	}
}

func (s simpleView) Name() string { return s.name }

func (s simpleView) PathSegments() []string {
	if !s.base.Chosen || s.base.Choice < 0 || s.base.Choice >= len(s.options) {
		return nil
	}
	return []string{s.options[s.base.Choice]}
}

func (s simpleView) Init() tea.Cmd { return s.base.Spinner.Tick }

func (s simpleView) Update(msg tea.Msg) (SubView, tea.Cmd) {
	cmd := s.base.HandleMsg(msg, len(s.options), s.run)
	return s, cmd
}

func (s simpleView) View() string {
	if !s.base.Chosen {
		return ChoicesView(s.base.Choice, s.options)
	}
	return ProgressView(s.base.Steps, s.base.Spinner.View(), s.base.Done, s.base.Quitting)
}
