package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func animated(t *testing.T, tasks board) model {
	t.Helper()
	return sized(testModel(t, tasks).withAnimations(), 80, 24)
}

// finishSplash lets the logo finish its reveal, then opens the board.
func finishSplash(t *testing.T, m model) model {
	t.Helper()
	m, cmd := update(m, animTick{})
	for cmd != nil {
		m, cmd = update(m, animTick{})
	}
	return press(t, m, tea.KeyEnter)
}

func update(m model, msg tea.Msg) (model, tea.Cmd) {
	updated, cmd := m.Update(msg)
	return updated.(model), cmd
}

func assertFillsScreen(t *testing.T, m model) {
	t.Helper()
	lines := strings.Split(m.View(), "\n")
	if len(lines) != m.height {
		t.Errorf("got %d lines, want %d", len(lines), m.height)
	}
	for i, line := range lines[:len(lines)-1] {
		if got := lipgloss.Width(line); got != m.width {
			t.Errorf("line %d is %d wide, want %d: %q", i, got, m.width, line)
		}
	}
}

func TestEffectsAreOffByDefault(t *testing.T) {
	m := sized(testModel(t, board{inProgress: {"task"}}), 80, 24)
	if m.Init() != nil {
		t.Error("a plain model should not schedule frames")
	}
	assertViewContains(t, m, "To Do")

	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	if cmd != nil || len(m.particles) != 0 {
		t.Error("a plain model should not start confetti")
	}
}

func TestSplashStaysUntilDismissed(t *testing.T) {
	m := animated(t, board{toDo: {"keep me"}})
	if m.Init() == nil {
		t.Error("the reveal should schedule frames")
	}
	assertViewOmits(t, m, "To Do")

	var cmd tea.Cmd
	for range sweepFrames {
		m, cmd = update(m, animTick{})
		assertFillsScreen(t, m)
	}
	if cmd != nil {
		t.Error("the finished logo should not schedule more frames")
	}
	assertViewContains(t, m, logo[0])
	assertViewContains(t, m, version)
	assertViewContains(t, m, splashHint)

	m = send(m, animTick{}) // the logo must outlast any number of frames
	assertViewContains(t, m, logo[0])
}

func TestSpaceAndEnterDismissSplashWithoutActing(t *testing.T) {
	for name, key := range map[string]any{"space": " ", "enter": tea.KeyEnter} {
		t.Run(name, func(t *testing.T) {
			m := animated(t, board{toDo: {"keep me"}})
			m = press(t, m, key)

			assertViewOmits(t, m, logo[0])
			assertViewContains(t, m, "To Do")
			assertBoardsEqual(t, m.tasks, board{toDo: {"keep me"}})
		})
	}
}

func TestOtherKeysDoNotDismissSplash(t *testing.T) {
	m := finishReveal(t, animated(t, board{toDo: {"keep me"}}))
	m = press(t, m, "dLa", tea.KeyEsc, tea.KeyTab)

	assertViewContains(t, m, logo[0])
	assertBoardsEqual(t, m.tasks, board{toDo: {"keep me"}})
	if m.adding {
		t.Error("keys should not act behind the logo")
	}
}

func TestDismissingDuringRevealStopsFrames(t *testing.T) {
	m := press(t, animated(t, board{}), tea.KeyEnter)
	m, cmd := update(m, animTick{})
	if cmd != nil || m.ticking {
		t.Error("frames should stop once the logo is dismissed")
	}
}

func TestQQuitsDuringSplash(t *testing.T) {
	_, cmd := update(animated(t, board{}), tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("q should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q should quit")
	}
}

func finishReveal(t *testing.T, m model) model {
	t.Helper()
	for range sweepFrames {
		m = send(m, animTick{})
	}
	return m
}

func TestCtrlCQuitsDuringSplash(t *testing.T) {
	_, cmd := update(animated(t, board{}), tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("ctrl+c should quit")
	}
}

func TestSplashFitsNarrowTerminal(t *testing.T) {
	m := sized(testModel(t, board{}).withAnimations(), minWidth, minHeight)
	assertFillsScreen(t, m)
	assertViewContains(t, m, "lazydone")
}

func TestCompletingATaskStartsConfetti(t *testing.T) {
	for name, keys := range map[string]string{"space": " ", "shift move": "L"} {
		t.Run(name, func(t *testing.T) {
			m := finishSplash(t, animated(t, board{inProgress: {"task"}}))
			m = press(t, m, "2")

			var key tea.KeyMsg
			if keys == " " {
				key = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
			} else {
				key = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keys)}
			}
			m, cmd := update(m, key)
			if len(m.particles) == 0 {
				t.Error("completing a task should start confetti")
			}
			if cmd == nil {
				t.Error("confetti should schedule frames")
			}
		})
	}
}

func TestOtherMovesDoNotStartConfetti(t *testing.T) {
	m := finishSplash(t, animated(t, board{toDo: {"a"}, completed: {"b"}}))

	m, _ = update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}}) // To Do to In Progress
	if len(m.particles) != 0 {
		t.Error("moving to In Progress should not start confetti")
	}

	m = press(t, m, "3")
	m, _ = update(m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}) // Completed wraps to To Do
	if len(m.particles) != 0 {
		t.Error("reopening a task should not start confetti")
	}
}

func TestConfettiEndsAndStopsFrames(t *testing.T) {
	m := finishSplash(t, animated(t, board{inProgress: {"task"}}))
	m = press(t, m, "2", "L")

	var cmd tea.Cmd
	frames := 0
	for {
		m, cmd = update(m, animTick{})
		frames++
		if cmd == nil {
			break
		}
		if frames > confettiLife {
			t.Fatal("confetti should have ended")
		}
	}
	if len(m.particles) != 0 {
		t.Error("no particles should remain")
	}
	if m.ticking {
		t.Error("an idle app should not be ticking")
	}
}

func TestSecondBurstDoesNotStartSecondTimer(t *testing.T) {
	m := finishSplash(t, animated(t, board{inProgress: {"a", "b"}}))
	m = press(t, m, "2")

	m, first := update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	m = press(t, m, "2")
	m, second := update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})

	if first == nil {
		t.Error("the first burst should schedule frames")
	}
	if second != nil {
		t.Error("the second burst should reuse the running timer")
	}
	if len(m.particles) < confettiCount*2 {
		t.Errorf("got %d particles, want both bursts", len(m.particles))
	}
}

func TestConfettiKeepsLayoutOnEveryFrame(t *testing.T) {
	useColors(t)
	tasks := board{
		toDo:       {"日本語の長いタスクがここにあります、とても長いタスクです", "🚀 emoji task"},
		inProgress: {"finish this 🎉 日本語", "second"},
		completed:  {"done 🎉"},
	}
	for _, size := range [][2]int{{minWidth, minHeight}, {80, 24}, {121, 40}} {
		m := sized(testModel(t, tasks).withAnimations(), size[0], size[1])
		m = finishSplash(t, m)
		m = press(t, m, "2", "L")
		if len(m.particles) == 0 {
			t.Fatalf("%v: expected confetti", size)
		}
		for m.animating() {
			assertFillsScreen(t, m)
			m = send(m, animTick{})
		}
	}
}

func TestConfettiAppearsOnScreen(t *testing.T) {
	m := finishSplash(t, animated(t, board{inProgress: {"task"}}))
	plain := m.View()
	m = press(t, m, "2", "L")
	m = send(m, animTick{})

	if view := m.View(); view == plain {
		t.Error("confetti should change the view")
	}
	found := false
	for _, glyph := range confettiGlyphs {
		if strings.Contains(m.View(), glyph) {
			found = true
		}
	}
	if !found {
		t.Error("view should contain confetti")
	}
}

func TestBordersUseADifferentGradientPerColumn(t *testing.T) {
	useColors(t)
	m := sized(testModel(t, board{}), 80, 24)
	view := m.View()

	for name, g := range map[string]gradient{"To Do": columns[toDo].border, "In Progress": columns[inProgress].border.dimmed(), "Completed": columns[completed].border.dimmed()} {
		first := g.paint("╭", 0, 0, 2, 2)
		if !strings.Contains(view, first) {
			t.Errorf("%s border should start with its own color: %q", name, first)
		}
	}
	assertFillsScreen(t, m)
}

func TestGradientFadesAlongABorder(t *testing.T) {
	useColors(t)
	g := columns[toDo].border
	if g.paint("─", 0, 0, 10, 10) == g.paint("─", 9, 9, 10, 10) {
		t.Error("the two ends of a gradient should differ")
	}
}

// useColors turns colors on, which are normally off when not in a terminal.
func useColors(t *testing.T) {
	t.Helper()
	original := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(original) })
}

func TestProgressShowsCountAtBottomRight(t *testing.T) {
	m := sized(testModel(t, board{toDo: {"a", "b"}, completed: {"c"}}), 80, 24)

	lines := strings.Split(m.View(), "\n")
	border := lines[len(lines)-2]
	if !strings.HasSuffix(border, " 1/3 done ─╯") {
		t.Errorf("progress should end the details border: %q", border)
	}
}

func TestProgressHiddenOnEmptyBoard(t *testing.T) {
	assertViewOmits(t, sized(testModel(t, board{}), 80, 24), " done ")
}

func TestProgressCountFollowsBoard(t *testing.T) {
	m := sized(testModel(t, board{inProgress: {"a", "b"}}), 80, 24)
	assertViewContains(t, m, "0/2 done")

	m = press(t, m, "2", "L")
	assertViewContains(t, m, "1/2 done")

	m = press(t, m, "a", "new", tea.KeyEnter)
	assertViewContains(t, m, "1/3 done")
}

func TestProgressBarFillsAsBoardOpens(t *testing.T) {
	m := finishReveal(t, animated(t, board{completed: {"a"}}))
	if m.bar != 0 {
		t.Errorf("the bar should wait behind the logo, got %v", m.bar)
	}

	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("opening the board should start the bar")
	}
	assertBarReaches(t, m, 1)
}

func TestProgressBarEasesBothWays(t *testing.T) {
	useColors(t)
	m := finishSplash(t, animated(t, board{inProgress: {"a"}, completed: {"b"}}))
	m = assertBarReaches(t, m, 0.5)

	m = press(t, m, "2", "L")
	m = assertBarReaches(t, m, 1) // rises when a task is completed
	for m.ticking {
		m = send(m, animTick{}) // let the confetti finish
	}

	m, cmd := update(press(t, m, "a", "new"), tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("adding a task should start the bar, once it has stopped")
	}
	m = assertBarReaches(t, m, 2.0/3.0) // falls when a task is added
	if m.ticking {
		t.Error("frames should stop once the bar arrives")
	}
}

// assertBarReaches runs frames until the bar stops, checking that it moves
// steadily towards the target and never disturbs the layout.
func assertBarReaches(t *testing.T, m model, want float64) model {
	t.Helper()
	gap := func(value float64) float64 {
		if value > want {
			return value - want
		}
		return want - value
	}
	before := m.bar
	for frames := 0; m.barMoving(); frames++ {
		if frames > 60 {
			t.Fatal("the bar should have stopped moving")
		}
		m = send(m, animTick{})
		if gap(m.bar) > gap(before) {
			t.Errorf("bar moved away from %v: %v to %v", want, before, m.bar)
		}
		before = m.bar
		assertFillsScreen(t, m)
	}
	if m.bar != want {
		t.Errorf("bar = %v, want %v", m.bar, want)
	}
	return send(m, animTick{}) // deliver the last scheduled frame
}

func TestProgressFitsSmallestTerminal(t *testing.T) {
	useColors(t)
	m := sized(testModel(t, board{toDo: {"日本語の長いタスク"}, completed: {"🎉 done"}}), minWidth, minHeight)
	assertFillsScreen(t, m)
}
