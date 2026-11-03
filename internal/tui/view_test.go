package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vishalcoderathore/lazydone/internal/board"
	"github.com/vishalcoderathore/lazydone/internal/store"
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
	tasks := texts{
		board.ToDo:       {"plain", "日本語の長いタスクがここにあります、とても長いタスクです", "🚀 emoji task"},
		board.InProgress: {strings.Repeat("a very long task ", 20)},
	}

	for _, size := range [][2]int{{60, 12}, {61, 13}, {80, 24}, {121, 40}} {
		for _, adding := range []bool{false, true} {
			m := sized(testModel(t, tasks), size[0], size[1])
			m.adding = adding
			m.input.SetValue(strings.Repeat("typed text ", 30))

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
	m := sized(testModel(t, texts{}), 100, 24)

	for _, want := range []string{"[1] 📋 To Do", "[2] 🚧 In Progress", "[3] ✅ Completed", "No tasks"} {
		assertViewContains(t, m, want)
	}
}

func TestBottomPanelIsNumberFour(t *testing.T) {
	m := sized(testModel(t, texts{}), 100, 24)
	assertViewContains(t, m, "[4] 🔍 Details")

	m = press(t, m, "4")
	assertViewContains(t, m, "[4] 📝 New Task")
}

func TestViewMarksSelectedTaskInActiveColumnOnly(t *testing.T) {
	m := sized(testModel(t, texts{board.ToDo: {"one", "two"}, board.InProgress: {"three"}}), 100, 24)
	m = press(t, m, tea.KeyDown)

	assertViewContains(t, m, "❯ ○ two")
	assertViewContains(t, m, "  ○ one")
	assertViewContains(t, m, "  ◐ three")
}

func TestViewShowsPositionInColumn(t *testing.T) {
	m := sized(testModel(t, texts{board.ToDo: {"one", "two", "three"}}), 100, 24)
	m = press(t, m, tea.KeyDown)

	assertViewContains(t, m, "2 of 3")
}

func TestViewScrollsToKeepSelectionVisible(t *testing.T) {
	var tasks []string
	for i := range 50 {
		tasks = append(tasks, fmt.Sprintf("task-%02d", i))
	}
	m := sized(testModel(t, texts{board.ToDo: tasks}), 100, 15)

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
	m := sized(testModel(t, texts{board.ToDo: {task}}), 80, 24)

	lines := strings.Split(m.View(), "\n")
	column := strings.Join(lines[:len(lines)-6], "\n")
	if strings.Contains(column, "ending-word") {
		t.Fatalf("expected the task to be cut off in its column:\n%s", column)
	}
	assertViewContains(t, m, "ending-word")
}

func TestDetailsFollowsActiveColumn(t *testing.T) {
	m := sized(testModel(t, texts{board.ToDo: {"one"}}), 100, 24)
	assertViewOmits(t, m, "Nothing selected")

	m = press(t, m, "2")
	assertViewContains(t, m, "Nothing selected")
}

func TestViewShowsInputWhileAdding(t *testing.T) {
	m := sized(testModel(t, texts{}), 100, 24)
	m = press(t, m, "a", "buy milk")

	assertViewContains(t, m, "📝 New Task")
	assertViewContains(t, m, "buy milk")
	assertViewContains(t, m, "esc: cancel")
}

// bottomPanel returns the panel below the columns, without the footer.
func bottomPanel(m model) string {
	lines := strings.Split(m.View(), "\n")
	return strings.Join(lines[len(lines)-detailsHeight-footerHeight:len(lines)-footerHeight], "\n")
}

func TestBottomPanelIsBlankAfterAddingTask(t *testing.T) {
	m := sized(testModel(t, texts{}), 100, 24)
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
	m := sized(testModel(t, texts{board.ToDo: {"one"}}), 100, 24)
	m = press(t, m, "a", "two", tea.KeyEnter, tea.KeyUp)

	if !strings.Contains(bottomPanel(m), "one") {
		t.Errorf("expected details of the selected task:\n%s", bottomPanel(m))
	}
}

func TestBlankEnterKeepsDetails(t *testing.T) {
	m := sized(testModel(t, texts{board.ToDo: {"one"}}), 100, 24)
	m = press(t, m, "a", tea.KeyEnter)

	if !strings.Contains(bottomPanel(m), "one") {
		t.Errorf("nothing was added, so details should still show:\n%s", bottomPanel(m))
	}
}

func TestLongInputKeepsEndVisible(t *testing.T) {
	m := sized(testModel(t, texts{}), 60, 24)
	m = press(t, m, "a", strings.Repeat("word ", 100)+"final")

	assertViewContains(t, m, "final")
}

func TestSaveFailureIsShown(t *testing.T) {
	m := sized(testModel(t, texts{}), 100, 24)
	m.store = store.File{Path: t.TempDir()} // a directory can't be written to as a file

	m = press(t, m, "a", "one", tea.KeyEnter)

	if m.err == nil {
		t.Fatal("expected a save error, got nil")
	}
	assertViewContains(t, m, "Could not save")
}

func TestFooterFitsStandardTerminal(t *testing.T) {
	m := sized(testModel(t, texts{}), 80, 24)

	assertViewContains(t, m, "?: help")
	assertViewContains(t, m, "q: quit")
}

func TestViewAsksForBiggerTerminal(t *testing.T) {
	m := sized(testModel(t, texts{}), 40, 10)

	assertViewContains(t, m, "at least 60x12")
}

func TestViewIsEmptyUntilSizeIsKnown(t *testing.T) {
	if view := testModel(t, texts{}).View(); view != "" {
		t.Errorf("view = %q, want empty", view)
	}
}

func TestActionsShowAStatusUntilTheNextKey(t *testing.T) {
	cases := map[string]struct {
		keys []any
		want string
	}{
		"add":      {[]any{"a", "new", tea.KeyEnter}, "Task added"},
		"edit":     {[]any{"e", "!", tea.KeyEnter}, "Task updated"},
		"delete":   {[]any{"dd"}, "Task deleted, press u to undo"},
		"half dd":  {[]any{"d"}, "Press d again to delete"},
		"undo":     {[]any{"dd", "u"}, "Change undone"},
		"no undo":  {[]any{"u"}, "Nothing to undo"},
		"done":     {[]any{"x"}, "Moved to Completed"},
		"move":     {[]any{"L"}, "Moved to In Progress"},
		"priority": {[]any{"p"}, "Priority: low"},
		"clear":    {[]any{"p", "p", "p", "p"}, "Priority cleared"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			m := sized(testModel(t, texts{board.ToDo: {"one", "two"}}), 100, 24)

			m = press(t, m, c.keys...)
			assertViewContains(t, m, c.want)

			m = press(t, m, "h")
			assertViewOmits(t, m, c.want)
		})
	}
}

func TestSaveErrorIsNotHiddenByStatus(t *testing.T) {
	m := sized(testModel(t, texts{board.ToDo: {"one"}}), 100, 24)
	m.store = store.File{Path: t.TempDir()} // a directory can't be written to as a file

	m = press(t, m, "dd")

	assertViewContains(t, m, "Could not save")
}

// longTask wraps to many lines in a details panel 60 cells wide.
func longTask() string {
	var words []string
	for i := range 80 {
		words = append(words, fmt.Sprintf("word%d", i))
	}
	return strings.Join(words, " ")
}

func TestDetailsScrollThroughLongTask(t *testing.T) {
	m := sized(testModel(t, texts{board.ToDo: {longTask()}}), 60, 24)
	assertViewOmits(t, m, "word79")
	assertViewContains(t, m, "Details 1-3 of")

	for range 20 {
		m = press(t, m, tea.KeyCtrlE)
	}
	assertDetailsContain(t, m, "word79")

	for range 20 {
		m = press(t, m, tea.KeyCtrlY)
	}
	assertDetailsContain(t, m, "word0 ")
	assertViewContains(t, m, "Details 1-3 of")
}

// assertDetailsContain looks only at the details panel, because the column
// above it also shows the start of the task.
func assertDetailsContain(t *testing.T, m model, want string) {
	t.Helper()
	lines := strings.Split(m.View(), "\n")
	details := strings.Join(lines[len(lines)-detailsHeight-footerHeight:], "\n")
	if !strings.Contains(details, want) {
		t.Errorf("details are missing %q:\n%s", want, details)
	}
}

func TestDetailsDoNotScrollPastTheEnd(t *testing.T) {
	m := sized(testModel(t, texts{board.ToDo: {longTask()}}), 60, 24)
	for range 50 {
		m = press(t, m, tea.KeyCtrlE)
	}
	m = press(t, m, tea.KeyCtrlY)

	assertDetailsContain(t, m, "word7") // one line back from the end, not 30
	assertViewFitsScreen(t, m)
}

func assertViewFitsScreen(t *testing.T, m model) {
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

func TestShortTaskHasNoScrollPosition(t *testing.T) {
	m := sized(testModel(t, texts{board.ToDo: {"short"}}), 60, 24)

	assertViewOmits(t, m, "Details 1")
}

func TestScrollDoesNothingOnShortTask(t *testing.T) {
	m := sized(testModel(t, texts{board.ToDo: {"short"}}), 60, 24)
	m = press(t, m, tea.KeyCtrlE)

	assertDetailsContain(t, m, "short")
}

func TestSelectingAnotherTaskResetsDetailsScroll(t *testing.T) {
	m := sized(testModel(t, texts{board.ToDo: {longTask(), longTask()}}), 60, 24)
	m = press(t, m, tea.KeyCtrlE, tea.KeyCtrlE, "j")

	assertDetailsContain(t, m, "word0 ")
}

func TestDetailsIncludePriorityInScrolling(t *testing.T) {
	m := sized(testModel(t, texts{board.ToDo: {longTask()}}), 60, 24)
	m = press(t, m, "p")
	assertDetailsContain(t, m, "Priority: low")

	m = press(t, m, tea.KeyCtrlE)
	if details := strings.Join(strings.Split(m.View(), "\n")[18:], "\n"); strings.Contains(details, "Priority: low") {
		t.Errorf("the priority line should scroll away:\n%s", details)
	}
}

func TestHelpPopupListsEveryKeybinding(t *testing.T) {
	m := press(t, sized(testModel(t, texts{board.ToDo: {"one"}}), 100, 40), "?")

	for _, want := range []string{
		"h / ←", "previous column", "l / →", "next column", "1 2 3", "jump to column",
		"j / ↓", "next task", "k / ↑", "previous task", "gg", "first task", "G", "last task",
		"Ctrl-d", "half page down", "Ctrl-u", "half page up",
		"a / 4", "add", "e", "edit", "dd", "delete", "x", "done", "p", "priority", "space", "next stage",
		"H", "column on the left", "L", "column on the right", "J", "reorder down", "K", "reorder up",
		"Ctrl-e", "scroll details down", "Ctrl-y", "scroll details up", "u", "undo",
		"r", "retry save", "?", "help", "q", "quit",
	} {
		assertViewContains(t, m, want)
	}
	assertViewFitsScreen(t, m)
}

func TestHelpPopupClosesWithoutActing(t *testing.T) {
	for name, key := range map[string]any{"question mark": "?", "esc": tea.KeyEsc, "q": "q"} {
		t.Run(name, func(t *testing.T) {
			m := press(t, sized(testModel(t, texts{board.ToDo: {"one"}}), 100, 40), "?")
			updated, cmd := m.Update(toKey(key))
			if cmd != nil {
				t.Error("closing help should not quit")
			}
			assertViewOmits(t, updated.(model), "previous column")
			assertViewContains(t, updated.(model), "To Do")
		})
	}
}

func toKey(key any) tea.KeyMsg {
	switch k := key.(type) {
	case tea.KeyType:
		return tea.KeyMsg{Type: k}
	case string:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
	panic("unsupported key")
}

func TestKeysDoNothingBehindHelpPopup(t *testing.T) {
	m := press(t, sized(testModel(t, texts{board.ToDo: {"one"}}), 100, 40), "?", "dd", "x", "p")

	assertBoardsEqual(t, m.tasks, texts{board.ToDo: {"one"}})
	assertViewContains(t, m, "previous column")
}

func TestHelpPopupScrollsOnSmallTerminal(t *testing.T) {
	m := press(t, sized(testModel(t, texts{}), minWidth, minHeight), "?")
	assertViewFitsScreen(t, m)
	assertViewOmits(t, m, "quit")

	for range 40 {
		m = press(t, m, "j")
	}
	assertViewContains(t, m, "quit")
	assertViewFitsScreen(t, m)

	for range 40 {
		m = press(t, m, "k")
	}
	assertViewContains(t, m, "previous column")
}

func TestGrowingTerminalKeepsHelpPopupOnScreen(t *testing.T) {
	m := press(t, sized(testModel(t, texts{}), 80, 24), "?", "jjj")

	m = sized(m, 80, 40)

	assertViewContains(t, m, "previous column")
	assertViewContains(t, m, "quit")
	assertViewFitsScreen(t, m)
}

func TestWideningTerminalKeepsDetailsOnScreen(t *testing.T) {
	m := sized(testModel(t, texts{board.ToDo: {longTask()}}), 60, 24)
	for range 20 {
		m = press(t, m, tea.KeyCtrlE)
	}

	m = sized(m, 300, 24)

	assertDetailsContain(t, m, "word79")
	assertViewFitsScreen(t, m)
}

func TestFooterHintsFitStandardTerminal(t *testing.T) {
	m := sized(testModel(t, texts{}), 80, 24)

	assertViewOmits(t, m, "…")
}

func TestFailedSaveCanBeRetried(t *testing.T) {
	good := filepath.Join(t.TempDir(), "tasks.json")
	m := sized(modelAt(t, good, texts{board.ToDo: {"one"}}.Board()), 100, 24)
	m.store = store.File{Path: t.TempDir()} // a directory can't be written to as a file
	m = press(t, m, "dd")
	assertViewContains(t, m, "r: retry")

	m.store = store.File{Path: good}
	m = press(t, m, "r")

	assertViewOmits(t, m, "Could not save")
	assertViewContains(t, m, "Saved")
	assertSaved(t, m, texts{})
}

func TestRetryThatFailsKeepsTheError(t *testing.T) {
	m := sized(testModel(t, texts{board.ToDo: {"one"}}), 100, 24)
	m.store = store.File{Path: t.TempDir()}
	m = press(t, m, "dd", "r")

	assertViewContains(t, m, "Could not save")
}

func TestRetryDoesNothingWithoutAnError(t *testing.T) {
	m := sized(testModel(t, texts{board.ToDo: {"one"}}), 100, 24)
	before, err := m.store.ModTime()
	if err != nil {
		t.Fatal(err)
	}
	m = press(t, m, "r")

	assertViewOmits(t, m, "Saved")
	if after, err := m.store.ModTime(); err != nil || !after.Equal(before) {
		t.Errorf("nothing changed, so nothing should be written: modified %v, was %v, err %v", after, before, err)
	}
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestQuitWarnsAboutUnsavedChangesFirst(t *testing.T) {
	m := sized(testModel(t, texts{board.ToDo: {"one"}}), 100, 24)
	m.store = store.File{Path: t.TempDir()}
	m = press(t, m, "dd")

	updated, cmd := m.Update(toKey("q"))
	if isQuit(cmd) {
		t.Fatal("the first q should warn, not quit")
	}
	assertViewContains(t, updated.(model), "not saved")

	if _, cmd = updated.Update(toKey("q")); !isQuit(cmd) {
		t.Error("the second q should quit")
	}
}

func TestOtherKeyCancelsQuitWarning(t *testing.T) {
	m := sized(testModel(t, texts{board.ToDo: {"one"}}), 100, 24)
	m.store = store.File{Path: t.TempDir()}
	m = press(t, m, "dd", "q", "h")

	if _, cmd := m.Update(toKey("q")); isQuit(cmd) {
		t.Error("the warning should start over")
	}
}

func TestQuitsAtOnceAfterSuccessfulRetry(t *testing.T) {
	good := filepath.Join(t.TempDir(), "tasks.json")
	m := sized(modelAt(t, good, texts{board.ToDo: {"one"}}.Board()), 100, 24)
	m.store = store.File{Path: t.TempDir()}
	m = press(t, m, "dd")
	m.store = store.File{Path: good}
	m = press(t, m, "r")

	if _, cmd := m.Update(toKey("q")); !isQuit(cmd) {
		t.Error("with everything saved, q should quit")
	}
}
