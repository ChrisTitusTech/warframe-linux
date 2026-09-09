package main

import (
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/gjrud/warframe-helper/utils/viewBuilder"
)

type testMsg struct{ text string }

type stubView struct {
	name    string
	initMsg tea.Msg
	view    string
	path    []string
	updates []tea.Msg
}

func (s *stubView) Update(msg tea.Msg) (viewBuilder.SubView, tea.Cmd) {
	s.updates = append(s.updates, msg)
	return s, nil
}

func (s *stubView) View() string {
	if s.view != "" {
		return s.view
	}
	return s.name
}

func (s *stubView) Init() tea.Cmd {
	return func() tea.Msg { return s.initMsg }
}

func (s *stubView) Name() string { return s.name }

func (s *stubView) PathSegments() []string { return s.path }

func TestComputeAvailable(t *testing.T) {
	m := model{
		keys:   viewBuilder.Defaultkeys,
		help:   help.New(),
		height: 20,
	}

	gotShort := m.computeAvailable()
	wantShort := m.height - strings.Count(m.helpView(), "\n") - terminalOverhead
	if gotShort != wantShort {
		t.Fatalf("computeAvailable() with short help = %d, want %d", gotShort, wantShort)
	}

	m.help.ShowAll = true
	gotFull := m.computeAvailable()
	wantFull := m.height - strings.Count(m.helpView(), "\n") - terminalOverhead
	if gotFull != wantFull {
		t.Fatalf("computeAvailable() with full help = %d, want %d", gotFull, wantFull)
	}
	if gotFull >= gotShort {
		t.Fatalf("computeAvailable() with full help = %d, want less than short help %d", gotFull, gotShort)
	}
}

func TestInjectBorderTitle(t *testing.T) {
	t.Run("injects title into first line", func(t *testing.T) {
		rendered := "+--------+\n| body |\n+--------+"
		got := injectBorderTitle(rendered, "WF")
		want := "+- WF ---+\n| body |\n+--------+"
		if got != want {
			t.Fatalf("injectBorderTitle() = %q, want %q", got, want)
		}
	})

	t.Run("leaves too narrow border unchanged", func(t *testing.T) {
		rendered := "+--+\n|x|\n+--+"
		if got := injectBorderTitle(rendered, "Warframe"); got != rendered {
			t.Fatalf("injectBorderTitle() = %q, want original %q", got, rendered)
		}
	})

	t.Run("truncates long title to fit border", func(t *testing.T) {
		rendered := "+--------+\n| body |\n+--------+"
		got := injectBorderTitle(rendered, "Warframe")
		want := "+- Warfr +\n| body |\n+--------+"
		if got != want {
			t.Fatalf("injectBorderTitle() = %q, want %q", got, want)
		}
	})
}

func TestTitle(t *testing.T) {
	t.Run("root menu title", func(t *testing.T) {
		m := model{}
		if got, want := m.title(), "Warframe Helper"; got != want {
			t.Fatalf("title() = %q, want %q", got, want)
		}
	})

	t.Run("selected top level title", func(t *testing.T) {
		m := model{
			views:  []viewBuilder.SubView{&stubView{name: "Price Check"}},
			choice: 0,
			chosen: true,
		}
		if got, want := m.title(), "Warframe Helper ⋗ Price Check"; got != want {
			t.Fatalf("title() = %q, want %q", got, want)
		}
	})

	t.Run("selected nested title", func(t *testing.T) {
		m := model{
			views:  []viewBuilder.SubView{&stubView{name: "Price Check", path: []string{"Nightmare Mods"}}},
			choice: 0,
			chosen: true,
		}
		if got, want := m.title(), "Warframe Helper ⋗ Price Check ⋗ Nightmare Mods"; got != want {
			t.Fatalf("title() = %q, want %q", got, want)
		}
	})
}

func TestUpdate(t *testing.T) {
	t.Run("menu navigation and enter", func(t *testing.T) {
		first := &stubView{name: "First", initMsg: viewBuilder.StepMsg{Text: "first"}}
		second := &stubView{name: "Second", initMsg: viewBuilder.StepMsg{Text: "second"}}
		m := model{
			views:     []viewBuilder.SubView{first, second},
			keys:      viewBuilder.Defaultkeys,
			help:      help.New(),
			width:     40,
			available: 11,
		}

		updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		got := updated.(model)
		if cmd != nil {
			t.Fatal("down navigation returned unexpected cmd")
		}
		if got.choice != 1 {
			t.Fatalf("choice after down = %d, want 1", got.choice)
		}

		updated, _ = got.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		got = updated.(model)
		if got.choice != 1 {
			t.Fatalf("choice after lower clamp = %d, want 1", got.choice)
		}

		updated, _ = got.Update(tea.KeyPressMsg{Code: tea.KeyUp})
		got = updated.(model)
		if got.choice != 0 {
			t.Fatalf("choice after up = %d, want 0", got.choice)
		}

		updated, _ = got.Update(tea.KeyPressMsg{Code: tea.KeyUp})
		got = updated.(model)
		if got.choice != 0 {
			t.Fatalf("choice after upper clamp = %d, want 0", got.choice)
		}

		updated, _ = got.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		got = updated.(model)
		updated, cmd = got.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		chosen := updated.(model)
		if !chosen.chosen {
			t.Fatal("enter did not mark model chosen")
		}
		if chosen.choice != 1 {
			t.Fatalf("chosen choice = %d, want 1", chosen.choice)
		}
		if cmd == nil {
			t.Fatal("enter returned nil cmd")
		}

		batch, ok := cmd().(tea.BatchMsg)
		if !ok {
			t.Fatalf("enter cmd() returned %T, want tea.BatchMsg", cmd())
		}
		if len(batch) != 2 {
			t.Fatalf("enter batch len = %d, want 2", len(batch))
		}

		msgs := []tea.Msg{batch[0](), batch[1]()}
		if !containsMsg(msgs, viewBuilder.StepMsg{Text: "second"}) {
			t.Fatalf("enter batch messages = %#v, want init message", msgs)
		}
		if !containsMsg(msgs, viewBuilder.AvailableSizeMsg{Height: chosen.available, Width: chosen.width - borderWidth}) {
			t.Fatalf("enter batch messages = %#v, want size message", msgs)
		}
	})

	t.Run("help toggle", func(t *testing.T) {
		t.Run("menu toggles help without cmd", func(t *testing.T) {
			m := model{
				keys:   viewBuilder.Defaultkeys,
				help:   help.New(),
				height: 20,
				width:  40,
			}
			m.available = m.computeAvailable()
			before := m.available

			updated, cmd := m.Update(tea.KeyPressMsg{Text: "?"})
			got := updated.(model)

			if !got.help.ShowAll {
				t.Fatal("help toggle did not enable full help")
			}
			if cmd != nil {
				t.Fatal("help toggle in menu returned unexpected cmd")
			}
			if got.available != got.computeAvailable() {
				t.Fatalf("available after help toggle = %d, want %d", got.available, got.computeAvailable())
			}
			if got.available >= before {
				t.Fatalf("full help available = %d, want less than %d", got.available, before)
			}
		})

		t.Run("chosen view returns resized message", func(t *testing.T) {
			m := model{
				views:  []viewBuilder.SubView{&stubView{name: "Only"}},
				keys:   viewBuilder.Defaultkeys,
				help:   help.New(),
				chosen: true,
				height: 20,
				width:  40,
			}
			m.available = m.computeAvailable()

			updated, cmd := m.Update(tea.KeyPressMsg{Text: "?"})
			got := updated.(model)

			if !got.help.ShowAll {
				t.Fatal("help toggle did not enable full help")
			}
			if cmd == nil {
				t.Fatal("help toggle in chosen view returned nil cmd")
			}
			if msg := cmd(); !reflect.DeepEqual(msg, viewBuilder.AvailableSizeMsg{Height: got.available, Width: got.width - borderWidth}) {
				t.Fatalf("help toggle cmd() = %#v", msg)
			}
		})
	})

	t.Run("window size", func(t *testing.T) {
		t.Run("menu updates size without cmd", func(t *testing.T) {
			m := model{
				keys: viewBuilder.Defaultkeys,
				help: help.New(),
			}

			updated, cmd := m.Update(tea.WindowSizeMsg{Height: 24, Width: 80})
			got := updated.(model)

			if cmd != nil {
				t.Fatal("window size in menu returned unexpected cmd")
			}
			if got.height != 24 || got.width != 80 {
				t.Fatalf("size = %dx%d, want 24x80", got.height, got.width)
			}
			if got.available != got.computeAvailable() {
				t.Fatalf("available = %d, want %d", got.available, got.computeAvailable())
			}
		})

		t.Run("chosen view returns resized message", func(t *testing.T) {
			m := model{
				views:  []viewBuilder.SubView{&stubView{name: "Only"}},
				keys:   viewBuilder.Defaultkeys,
				help:   help.New(),
				chosen: true,
			}

			updated, cmd := m.Update(tea.WindowSizeMsg{Height: 30, Width: 100})
			got := updated.(model)

			if got.height != 30 || got.width != 100 {
				t.Fatalf("size = %dx%d, want 30x100", got.height, got.width)
			}
			if cmd == nil {
				t.Fatal("window size in chosen view returned nil cmd")
			}
			if msg := cmd(); !reflect.DeepEqual(msg, viewBuilder.AvailableSizeMsg{Height: got.available, Width: got.width - borderWidth}) {
				t.Fatalf("window size cmd() = %#v", msg)
			}
		})
	})

	t.Run("back clears chosen state", func(t *testing.T) {
		m := model{
			views:  []viewBuilder.SubView{&stubView{name: "Only"}},
			keys:   viewBuilder.Defaultkeys,
			help:   help.New(),
			chosen: true,
			choice: 0,
		}

		updated, cmd := m.Update(viewBuilder.BackMsg{})
		got := updated.(model)

		if cmd != nil {
			t.Fatal("BackMsg returned unexpected cmd")
		}
		if got.chosen {
			t.Fatal("BackMsg did not clear chosen state")
		}
	})

	t.Run("quit from menu", func(t *testing.T) {
		m := model{keys: viewBuilder.Defaultkeys, help: help.New()}

		updated, cmd := m.Update(tea.KeyPressMsg{Text: "q"})
		got := updated.(model)

		if !got.quitting {
			t.Fatal("quit from menu did not set quitting")
		}
		if cmd == nil {
			t.Fatal("quit from menu returned nil cmd")
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("quit cmd() returned %T, want tea.QuitMsg", cmd())
		}
	})

	t.Run("chosen state forwards messages and ignores menu keys", func(t *testing.T) {
		t.Run("forwards arbitrary messages", func(t *testing.T) {
			selected := &stubView{name: "Selected"}
			m := model{
				views:  []viewBuilder.SubView{selected},
				keys:   viewBuilder.Defaultkeys,
				help:   help.New(),
				chosen: true,
			}

			msg := testMsg{text: "forward me"}
			updated, cmd := m.Update(msg)
			got := updated.(model)

			if cmd != nil {
				t.Fatal("forwarded message returned unexpected cmd")
			}
			if !got.chosen {
				t.Fatal("forwarded message cleared chosen state")
			}
			if len(selected.updates) != 1 || !reflect.DeepEqual(selected.updates[0], msg) {
				t.Fatalf("selected updates = %#v, want %#v", selected.updates, []tea.Msg{msg})
			}
		})

		t.Run("menu keys stay inside selected view", func(t *testing.T) {
			selected := &stubView{name: "Selected"}
			m := model{
				views:  []viewBuilder.SubView{&stubView{name: "First"}, selected},
				keys:   viewBuilder.Defaultkeys,
				help:   help.New(),
				chosen: true,
				choice: 1,
			}

			msgs := []tea.KeyPressMsg{{Code: tea.KeyUp}, {Code: tea.KeyDown}, {Code: tea.KeyEnter}}
			for _, msg := range msgs {
				updated, cmd := m.Update(msg)
				m = updated.(model)
				if cmd != nil {
					t.Fatalf("chosen key %v returned unexpected cmd", msg.Code)
				}
				if m.choice != 1 {
					t.Fatalf("chosen key %v changed choice to %d, want 1", msg.Code, m.choice)
				}
			}

			if !reflect.DeepEqual(selected.updates, []tea.Msg{msgs[0], msgs[1], msgs[2]}) {
				t.Fatalf("selected updates = %#v, want %#v", selected.updates, []tea.Msg{msgs[0], msgs[1], msgs[2]})
			}
		})
	})
}

func TestView(t *testing.T) {
	t.Run("quitting state", func(t *testing.T) {
		m := model{
			keys:      viewBuilder.Defaultkeys,
			help:      help.New(),
			quitting:  true,
			width:     40,
			available: 4,
		}

		v := m.View()
		content := ansi.Strip(v.Content)

		if !v.AltScreen {
			t.Fatal("view should use alt screen")
		}
		if !strings.Contains(content, "Bye!") {
			t.Fatalf("view content = %q, want Bye!", content)
		}
	})

	t.Run("main menu rendering", func(t *testing.T) {
		m := model{
			views:     []viewBuilder.SubView{&stubView{name: "Alpha"}, &stubView{name: "Beta"}},
			keys:      viewBuilder.Defaultkeys,
			help:      help.New(),
			choice:    1,
			width:     40,
			available: 4,
		}

		v := m.View()
		content := ansi.Strip(v.Content)
		choices := strings.Split(strings.TrimSuffix(ansi.Strip(viewBuilder.ChoicesView(1, []string{"Alpha", "Beta"})), "\n"), "\n")

		for _, line := range choices {
			if !strings.Contains(content, line) {
				t.Fatalf("view content missing choice line %q in %q", line, content)
			}
		}
	})

	t.Run("selected subview rendering", func(t *testing.T) {
		m := model{
			views:     []viewBuilder.SubView{&stubView{name: "Alpha"}, &stubView{name: "Price Check", view: "subview body", path: []string{"Nightmare Mods"}}},
			keys:      viewBuilder.Defaultkeys,
			help:      help.New(),
			choice:    1,
			chosen:    true,
			width:     80,
			available: 4,
		}

		content := ansi.Strip(m.View().Content)

		if !strings.Contains(content, "subview body") {
			t.Fatalf("view content = %q, want subview body", content)
		}
		if strings.Contains(content, "[ ] Alpha") {
			t.Fatalf("view content still shows menu: %q", content)
		}
		firstLine := strings.SplitN(content, "\n", 2)[0]
		if !strings.Contains(firstLine, "Warframe Helper ⋗ Price Check ⋗ Nightmare Mods") {
			t.Fatalf("first line = %q, want nested breadcrumb", firstLine)
		}
	})

	t.Run("content truncation", func(t *testing.T) {
		m := model{
			views:     []viewBuilder.SubView{&stubView{name: "Alpha", view: "line1\nline2\nline3"}},
			keys:      viewBuilder.Defaultkeys,
			help:      help.New(),
			chosen:    true,
			width:     40,
			available: 2,
		}

		content := ansi.Strip(m.View().Content)

		if !strings.Contains(content, "line1") || !strings.Contains(content, "line2") {
			t.Fatalf("truncated content = %q, want first two lines", content)
		}
		if strings.Contains(content, "line3") {
			t.Fatalf("truncated content still contains line3: %q", content)
		}
	})

	t.Run("border title integration", func(t *testing.T) {
		m := model{
			views:     []viewBuilder.SubView{&stubView{name: "Alpha"}},
			keys:      viewBuilder.Defaultkeys,
			help:      help.New(),
			width:     50,
			available: 4,
		}

		content := ansi.Strip(m.View().Content)
		firstLine := strings.SplitN(content, "\n", 2)[0]

		if !strings.Contains(firstLine, "Warframe Helper") {
			t.Fatalf("first line = %q, want injected title", firstLine)
		}
	})
}

func containsMsg(msgs []tea.Msg, want tea.Msg) bool {
	for _, msg := range msgs {
		if reflect.DeepEqual(msg, want) {
			return true
		}
	}
	return false
}
