package main

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// sized gives the model a terminal size, as Bubble Tea does on startup.
func sized(m model, width, height int) model {
	return send(m, tea.WindowSizeMsg{Width: width, Height: height})
}

func assertViewContains(t *testing.T, m model, want string) {
	t.Helper()
	if view := m.View(); !strings.Contains(view, want) {
		t.Errorf("view is missing %q:\n%s", want, view)
	}
}

func assertViewOmits(t *testing.T, m model, unwanted string) {
	t.Helper()
	if view := m.View(); strings.Contains(view, unwanted) {
		t.Errorf("view should not contain %q:\n%s", unwanted, view)
	}
}

func TestViewFillsTerminalExactly(t *testing.T) {
	tasks := board{
		toDo:       {"plain", "日本語の長いタスクがここにあります、とても長いタスクです", "🚀 emoji task"},
		inProgress: {strings.Repeat("a very long task ", 20)},
	}

	for _, size := range [][2]int{{60, 12}, {61, 13}, {80, 24}, {121, 40}} {
		for _, adding := range []bool{false, true} {
			m := sized(testModel(t, tasks), size[0], size[1])
			m.adding = adding
			m.input = strings.Repeat("typed text ", 30)

			lines := strings.Split(m.View(), "\n")
			if len(lines) != size[1] {
				t.Errorf("%v: got %d lines, want %d", size, len(lines), size[1])
			}
			// The footer is shorter than the screen; everything else is a panel.
			for i, line := range lines[:len(lines)-1] {
				if got := lipgloss.Width(line); got != size[0] {
					t.Errorf("%v adding=%v: line %d is %d wide, want %d: %q", size, adding, i, got, size[0], line)
				}
			}
		}
	}
}

func TestViewShowsColumnsAndIcons(t *testing.T) {
	m := sized(testModel(t, board{}), 100, 24)

	for _, want := range []string{"[1] 📋 To Do", "[2] 🚧 In Progress", "[3] ✅ Completed", "No tasks"} {
		assertViewContains(t, m, want)
	}
}

func TestBottomPanelIsNumberFour(t *testing.T) {
	m := sized(testModel(t, board{}), 100, 24)
	assertViewContains(t, m, "[4] 🔍 Details")

	m = press(t, m, "4")
	assertViewContains(t, m, "[4] 📝 New Task")
}

func TestViewMarksSelectedTaskInActiveColumnOnly(t *testing.T) {
	m := sized(testModel(t, board{toDo: {"one", "two"}, inProgress: {"three"}}), 100, 24)
	m = press(t, m, tea.KeyDown)

	assertViewContains(t, m, "❯ ○ two")
	assertViewContains(t, m, "  ○ one")
	assertViewContains(t, m, "  ◐ three")
}

func TestViewShowsPositionInColumn(t *testing.T) {
	m := sized(testModel(t, board{toDo: {"one", "two", "three"}}), 100, 24)
	m = press(t, m, tea.KeyDown)

	assertViewContains(t, m, "2 of 3")
}

func TestViewScrollsToKeepSelectionVisible(t *testing.T) {
	var tasks []string
	for i := range 50 {
		tasks = append(tasks, fmt.Sprintf("task-%02d", i))
	}
	m := sized(testModel(t, board{toDo: tasks}), 100, 15)

	assertViewContains(t, m, "task-00")
	assertViewOmits(t, m, "task-49")

	for range 49 {
		m = press(t, m, tea.KeyDown)
	}
	assertViewContains(t, m, "❯ ○ task-49")
	assertViewContains(t, m, "50 of 50")
	assertViewOmits(t, m, "task-00")
}

func TestDetailsShowsFullTextOfCutOffTask(t *testing.T) {
	task := "this long task does not fit in a column but the ending-word is visible below"
	m := sized(testModel(t, board{toDo: {task}}), 80, 24)

	lines := strings.Split(m.View(), "\n")
	column := strings.Join(lines[:len(lines)-6], "\n")
	if strings.Contains(column, "ending-word") {
		t.Fatalf("expected the task to be cut off in its column:\n%s", column)
	}
	assertViewContains(t, m, "ending-word")
}

func TestDetailsFollowsActiveColumn(t *testing.T) {
	m := sized(testModel(t, board{toDo: {"one"}}), 100, 24)
	assertViewOmits(t, m, "Nothing selected")

	m = press(t, m, "2")
	assertViewContains(t, m, "Nothing selected")
}

func TestViewShowsInputWhileAdding(t *testing.T) {
	m := sized(testModel(t, board{}), 100, 24)
	m = press(t, m, "a", "buy milk")

	assertViewContains(t, m, "📝 New Task")
	assertViewContains(t, m, "buy milk█")
	assertViewContains(t, m, "esc: cancel")
}

// bottomPanel returns the panel below the columns, without the footer.
func bottomPanel(m model) string {
	lines := strings.Split(m.View(), "\n")
	return strings.Join(lines[len(lines)-detailsHeight-footerHeight:len(lines)-footerHeight], "\n")
}

func TestBottomPanelIsBlankAfterAddingTask(t *testing.T) {
	m := sized(testModel(t, board{}), 100, 24)
	m = press(t, m, "a", "buy milk")
	if !strings.Contains(bottomPanel(m), "buy milk") {
		t.Fatalf("expected the typed text in the input box:\n%s", bottomPanel(m))
	}

	m = press(t, m, tea.KeyEnter)

	if strings.Contains(bottomPanel(m), "buy milk") {
		t.Errorf("expected the bottom panel to be blank:\n%s", bottomPanel(m))
	}
	assertViewContains(t, m, "❯ ○ buy milk") // the task itself is on the board
}

func TestDetailsComeBackOnNextKeyAfterAdding(t *testing.T) {
	m := sized(testModel(t, board{toDo: {"one"}}), 100, 24)
	m = press(t, m, "a", "two", tea.KeyEnter, tea.KeyUp)

	if !strings.Contains(bottomPanel(m), "one") {
		t.Errorf("expected details of the selected task:\n%s", bottomPanel(m))
	}
}

func TestBlankEnterKeepsDetails(t *testing.T) {
	m := sized(testModel(t, board{toDo: {"one"}}), 100, 24)
	m = press(t, m, "a", tea.KeyEnter)

	if !strings.Contains(bottomPanel(m), "one") {
		t.Errorf("nothing was added, so details should still show:\n%s", bottomPanel(m))
	}
}

func TestLongInputKeepsEndVisible(t *testing.T) {
	m := sized(testModel(t, board{}), 60, 24)
	m = press(t, m, "a", strings.Repeat("word ", 100)+"final")

	assertViewContains(t, m, "final█")
}

func TestSaveFailureIsShown(t *testing.T) {
	m := sized(testModel(t, board{}), 100, 24)
	m.path = t.TempDir() // a directory can't be written to as a file

	m = press(t, m, "a", "one", tea.KeyEnter)

	if m.err == nil {
		t.Fatal("expected a save error, got nil")
	}
	assertViewContains(t, m, "Could not save")
}

func TestFooterFitsStandardTerminal(t *testing.T) {
	m := sized(testModel(t, board{}), 80, 24)

	assertViewContains(t, m, "space: next")
	assertViewContains(t, m, "q: quit")
}

func TestViewAsksForBiggerTerminal(t *testing.T) {
	m := sized(testModel(t, board{}), 40, 10)

	assertViewContains(t, m, "at least 60x12")
}

func TestViewIsEmptyUntilSizeIsKnown(t *testing.T) {
	if view := testModel(t, board{}).View(); view != "" {
		t.Errorf("view = %q, want empty", view)
	}
}
