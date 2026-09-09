package viewBuilder

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func collectCmdMsgs(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()

	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}

	v := reflect.ValueOf(msg)
	if v.IsValid() && v.Kind() == reflect.Slice {
		msgs := make([]tea.Msg, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			sub, ok := v.Index(i).Interface().(tea.Cmd)
			if !ok {
				t.Fatalf("slice message item %d = %T, want tea.Cmd", i, v.Index(i).Interface())
			}
			msgs = append(msgs, collectCmdMsgs(t, sub)...)
		}
		return msgs
	}

	return []tea.Msg{msg}
}

func TestChoicesView(t *testing.T) {
	got := ChoicesView(1, []string{"Alpha", "Beta", "Gamma"})
	want := "[ ] Alpha\n" + HighlightStyle.Render("[x] Beta") + "\n[ ] Gamma\n"
	if got != want {
		t.Fatalf("ChoicesView() = %q, want %q", got, want)
	}
}

func TestSimpleViewPathSegments(t *testing.T) {
	s := NewSimpleView("Test", []string{"Alpha", "Beta"}, nil).(simpleView)
	if got := s.PathSegments(); got != nil {
		t.Fatalf("PathSegments() = %#v, want nil before enter", got)
	}

	s.base.Chosen = true
	s.base.Choice = 1
	if got, want := s.PathSegments(), []string{"Beta"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("PathSegments() = %#v, want %#v", got, want)
	}
}

func TestProgressView(t *testing.T) {
	t.Run("in progress", func(t *testing.T) {
		got := ProgressView([]string{"prep", "warn: careful", "alert: missing", "working"}, "> ", false, false)
		want := SuccessStyle.Render("✓ prep") + "\n" +
			WarningStyle.Render("⚠ careful") + "\n" +
			ErrorStyle.Render("✗ missing") + "\n" +
			"> working\n"
		if got != want {
			t.Fatalf("ProgressView() = %q, want %q", got, want)
		}
	})

	t.Run("explicit error", func(t *testing.T) {
		got := ProgressView([]string{"error: boom"}, "> ", false, false)
		want := ErrorStyle.Render("✗ boom") + "\n"
		if got != want {
			t.Fatalf("ProgressView() = %q, want %q", got, want)
		}
	})

	t.Run("completed final step", func(t *testing.T) {
		got := ProgressView([]string{"prep", "finish"}, "> ", true, false)
		want := SuccessStyle.Render("✓ prep") + "\n" +
			SuccessStyle.Render("✓ finish") + "\n"
		if got != want {
			t.Fatalf("ProgressView() = %q, want %q", got, want)
		}
	})

	t.Run("done and quitting", func(t *testing.T) {
		got := ProgressView([]string{"prep", "last"}, "> ", true, true)
		want := SuccessStyle.Render("✓ prep") + "\n" +
			ErrorStyle.Render("✗ last") + "\n" +
			"> Quitting...\n"
		if got != want {
			t.Fatalf("ProgressView() = %q, want %q", got, want)
		}
	})
}

func TestBaseModelHandleMsgNavigationBounds(t *testing.T) {
	t.Run("menu clamps within bounds", func(t *testing.T) {
		b := NewBaseModel()

		b.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown}, 2, nil)
		b.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown}, 2, nil)
		if b.Choice != 1 {
			t.Fatalf("down bound choice = %d, want 1", b.Choice)
		}

		b.HandleMsg(tea.KeyPressMsg{Code: tea.KeyUp}, 2, nil)
		b.HandleMsg(tea.KeyPressMsg{Code: tea.KeyUp}, 2, nil)
		if b.Choice != 0 {
			t.Fatalf("up bound choice = %d, want 0", b.Choice)
		}
	})

	t.Run("chosen state ignores navigation", func(t *testing.T) {
		b := NewBaseModel()
		b.Chosen = true

		b.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown}, 2, nil)
		if b.Choice != 0 {
			t.Fatalf("chosen state changed choice = %d, want 0", b.Choice)
		}
	})
}

func TestBaseModelHandleMsgEnterBehavior(t *testing.T) {
	b := NewBaseModel()
	b.Choice = 1
	b.Done = true
	b.Steps = []string{"stale"}

	called := false
	var gotChoice int
	var gotCtx context.Context
	cmd := b.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter}, 3, func(choice int, ctx context.Context) tea.Cmd {
		called = true
		gotChoice = choice
		gotCtx = ctx
		return func() tea.Msg { return StepMsg{Text: "started"} }
	})

	if !called {
		t.Fatal("enter did not call onEnter")
	}
	if gotChoice != 1 {
		t.Fatalf("onEnter choice = %d, want 1", gotChoice)
	}
	if gotCtx == nil {
		t.Fatal("onEnter context is nil")
	}
	if b.Cancel == nil {
		t.Fatal("enter did not store cancel func")
	}
	if !b.Chosen {
		t.Fatal("enter did not mark model chosen")
	}
	if b.Done {
		t.Fatal("enter did not clear done flag")
	}
	if b.Steps != nil {
		t.Fatalf("enter steps = %#v, want nil", b.Steps)
	}
	if cmd == nil {
		t.Fatal("enter returned nil cmd")
	}
	if msg := cmd(); !reflect.DeepEqual(msg, StepMsg{Text: "started"}) {
		t.Fatalf("cmd() = %#v, want StepMsg", msg)
	}

	b.Cancel()
	select {
	case <-gotCtx.Done():
	default:
		t.Fatal("stored cancel did not cancel enter context")
	}
}

func TestBaseModelHandleMsgQuitBehavior(t *testing.T) {
	t.Run("cancel in progress", func(t *testing.T) {
		b := NewBaseModel()
		canceled := false
		b.Cancel = func() { canceled = true }

		cmd := b.HandleMsg(tea.KeyPressMsg{Text: "q"}, 1, nil)
		if cmd != nil {
			t.Fatal("quit during cancelable run returned cmd")
		}
		if !b.Quitting {
			t.Fatal("quit during cancelable run did not set quitting")
		}
		if !canceled {
			t.Fatal("quit during cancelable run did not call cancel")
		}
	})

	t.Run("chosen resets to menu", func(t *testing.T) {
		b := NewBaseModel()
		b.Chosen = true
		b.Done = true
		b.Quitting = true
		b.Steps = []string{"step"}

		cmd := b.HandleMsg(tea.KeyPressMsg{Text: "q"}, 1, nil)
		if cmd != nil {
			t.Fatal("quit from chosen state returned cmd")
		}
		if b.Chosen || b.Done || b.Quitting || b.Steps != nil || b.Cancel != nil {
			t.Fatalf("quit from chosen state did not reset menu: %+v", b)
		}
	})

	t.Run("menu returns back message", func(t *testing.T) {
		b := NewBaseModel()

		cmd := b.HandleMsg(tea.KeyPressMsg{Text: "q"}, 1, nil)
		if cmd == nil {
			t.Fatal("quit from menu returned nil cmd")
		}
		if msg := cmd(); !reflect.DeepEqual(msg, BackMsg{}) {
			t.Fatalf("cmd() = %#v, want BackMsg{}", msg)
		}
	})
}

func TestBaseModelHandleMsgStatusMessages(t *testing.T) {
	t.Run("step warn and alert append", func(t *testing.T) {
		b := NewBaseModel()
		b.HandleMsg(StepMsg{Text: "step"}, 1, nil)
		b.HandleMsg(WarnMsg{Text: "warn"}, 1, nil)
		b.HandleMsg(AlertMsg{Text: "alert"}, 1, nil)

		want := []string{"step", "warn: warn", "alert: alert"}
		if !reflect.DeepEqual(b.Steps, want) {
			t.Fatalf("steps = %#v, want %#v", b.Steps, want)
		}
	})

	t.Run("error rewrites last step and clears cancel", func(t *testing.T) {
		b := NewBaseModel()
		b.Steps = []string{"sync"}
		canceled := false
		b.Cancel = func() { canceled = true }

		b.HandleMsg(ErrorMsg{Text: "boom"}, 1, nil)

		want := []string{"error: sync: boom"}
		if !reflect.DeepEqual(b.Steps, want) {
			t.Fatalf("steps = %#v, want %#v", b.Steps, want)
		}
		if !canceled {
			t.Fatal("error did not cancel in-flight work")
		}
		if b.Cancel != nil {
			t.Fatal("error did not clear cancel")
		}
	})

	t.Run("error without prior step appends", func(t *testing.T) {
		b := NewBaseModel()
		b.HandleMsg(ErrorMsg{Text: "boom"}, 1, nil)
		want := []string{"error: boom"}
		if !reflect.DeepEqual(b.Steps, want) {
			t.Fatalf("steps = %#v, want %#v", b.Steps, want)
		}
	})

	t.Run("done appends and marks complete", func(t *testing.T) {
		b := NewBaseModel()
		b.Cancel = func() {}

		b.HandleMsg(DoneMsg{}, 1, nil)

		want := []string{"Done"}
		if !reflect.DeepEqual(b.Steps, want) {
			t.Fatalf("steps = %#v, want %#v", b.Steps, want)
		}
		if !b.Done {
			t.Fatal("done did not set Done")
		}
		if b.Cancel != nil {
			t.Fatal("done did not clear cancel")
		}
	})

	t.Run("canceled resets menu", func(t *testing.T) {
		b := NewBaseModel()
		b.Chosen = true
		b.Done = true
		b.Quitting = true
		b.Steps = []string{"step"}
		b.Cancel = func() {}

		b.HandleMsg(CanceledMsg{}, 1, nil)

		if b.Chosen || b.Done || b.Quitting || b.Steps != nil || b.Cancel != nil {
			t.Fatalf("canceled did not reset menu: %+v", b)
		}
	})
}

func TestSleep(t *testing.T) {
	t.Run("timer fires", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		if !Sleep(ctx, 5*time.Millisecond) {
			t.Fatal("Sleep() = false, want true")
		}
	})

	t.Run("canceled context stops sleep", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		if Sleep(ctx, time.Second) {
			t.Fatal("Sleep() = true, want false")
		}
	})
}

func TestStepCmd(t *testing.T) {
	t.Run("pre-failed short circuits", func(t *testing.T) {
		failed := true
		called := false

		msgs := collectCmdMsgs(t, StepCmd(context.Background(), "sync", &failed, func() error {
			called = true
			return nil
		}))

		if len(msgs) != 0 {
			t.Fatalf("msgs = %#v, want none", msgs)
		}
		if called {
			t.Fatal("work ran despite pre-failed state")
		}
	})

	t.Run("canceled context returns canceled message", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		failed := false
		called := false

		msgs := collectCmdMsgs(t, StepCmd(ctx, "sync", &failed, func() error {
			called = true
			return nil
		}))

		want := []tea.Msg{CanceledMsg{}, CanceledMsg{}}
		if !reflect.DeepEqual(msgs, want) {
			t.Fatalf("msgs = %#v, want %#v", msgs, want)
		}
		if called {
			t.Fatal("work ran despite canceled context")
		}
		if failed {
			t.Fatal("canceled context unexpectedly flipped failed")
		}
	})

	t.Run("work failure returns step then error and flips failed", func(t *testing.T) {
		failed := false

		msgs := collectCmdMsgs(t, StepCmd(context.Background(), "sync", &failed, func() error {
			return errors.New("boom")
		}))

		want := []tea.Msg{StepMsg{Text: "sync"}, ErrorMsg{Text: "boom"}}
		if !reflect.DeepEqual(msgs, want) {
			t.Fatalf("msgs = %#v, want %#v", msgs, want)
		}
		if !failed {
			t.Fatal("work failure did not flip failed")
		}
	})
}
