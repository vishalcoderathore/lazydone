package tui

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/vishalcoderathore/lazydone/internal/board"
)

// The tests here run on Sunday 1 November 2026 at 09:00 UTC.
func day(d int) time.Time { return time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC) }

// boardModel opens a board with tasks given in full, at clockOne, 100x30.
func boardModel(t *testing.T, tasks board.Board) model {
	t.Helper()
	m := modelAt(t, filepath.Join(t.TempDir(), "tasks.json"), tasks)
	return sized(at(m, clockOne), 100, 30)
}

// shown lists the text of each column's visible tasks.
func shown(m model) texts {
	var got texts
	for column := range board.NumColumns {
		for _, item := range m.visible(column) {
			got[column] = append(got[column], item.Text)
		}
	}
	return got
}

func assertShown(t *testing.T, m model, want texts) {
	t.Helper()
	got := shown(m)
	for column := range board.NumColumns {
		if !slices.Equal(got[column], want[column]) {
			t.Fatalf("shown = %q, want %q", got, want)
		}
	}
}

func selectedText(m model) string {
	item, _ := m.selected()
	return item.Text
}

func TestAddReadsTagsAndDueDate(t *testing.T) {
	m := press(t, at(testModel(t, texts{}), clockOne), "a", "pay rent +home @bank due:tomorrow", tea.KeyEnter)

	got := reload(t, m)[board.ToDo][0]
	if got.Text != "pay rent" || !slices.Equal(got.Tags, []string{"+home", "@bank"}) || !got.Due.Equal(day(2)) {
		t.Errorf("got %+v", got)
	}
}

func TestAddWithBadDueDateKeepsTheInput(t *testing.T) {
	m := press(t, sized(testModel(t, texts{}), 100, 24), "a", "pay due:someday", tea.KeyEnter)

	if !m.adding || m.input.Value() != "pay due:someday" {
		t.Errorf("adding = %v, input = %q, want the input kept to fix", m.adding, m.input.Value())
	}
	assertViewContains(t, m, `"someday" isn't one I know`)
	assertSaved(t, m, texts{})

	m = press(t, m, tea.KeyBackspace, tea.KeyBackspace, tea.KeyBackspace, tea.KeyBackspace, tea.KeyBackspace, tea.KeyBackspace, tea.KeyBackspace, "fri", tea.KeyEnter)
	assertSaved(t, m, texts{board.ToDo: {"pay"}})
}

func TestAddOfOnlyTagsKeepsTheInput(t *testing.T) {
	m := press(t, sized(testModel(t, texts{}), 100, 24), "a", "+work", tea.KeyEnter)

	if !m.adding {
		t.Error("the input should stay open")
	}
	assertViewContains(t, m, "needs some text")
}

func TestEditShowsAndChangesTagsAndDueDate(t *testing.T) {
	m := boardModel(t, board.Board{board.ToDo: {{ID: "a", Text: "pay", Tags: []string{"+home"}, Due: day(5)}}})

	m = press(t, m, "e")
	if want := "pay +home due:05-11-2026"; m.input.Value() != want {
		t.Fatalf("input = %q, want %q", m.input.Value(), want)
	}
	m.input.SetValue("pay rent @bank")
	m = press(t, m, tea.KeyEnter)

	got := reload(t, m)[board.ToDo][0]
	if got.Text != "pay rent" || !slices.Equal(got.Tags, []string{"@bank"}) || !got.Due.IsZero() {
		t.Errorf("got %+v, want new text and tag, and no due date", got)
	}
}

func TestTasksShowTagsAndDueDates(t *testing.T) {
	m := boardModel(t, board.Board{
		board.ToDo: {
			{ID: "a", Text: "pay", Tags: []string{"+home"}, Due: day(1)},
			{ID: "b", Text: "plan", Due: day(20)},
			{ID: "c", Text: "renew", Due: time.Date(2027, 1, 2, 0, 0, 0, 0, time.UTC)},
		},
		board.Completed: {{ID: "d", Text: "filed", Due: day(1)}},
	})

	assertViewContains(t, m, "pay · due today +home")
	assertViewContains(t, m, "plan · due 20-11")
	assertViewContains(t, m, "renew · due 02-01-2027")
	assertViewContains(t, m, "✓ filed")
	assertViewOmits(t, m, "filed ·")
}

func TestOverdueTasksAreRed(t *testing.T) {
	useColors(t)
	m := boardModel(t, board.Board{board.ToDo: {{ID: "a", Text: "on time"}, {ID: "b", Text: "late", Due: day(1).AddDate(0, 0, -1)}}})

	red := overdueStyle.Render("x")
	red = red[:strings.Index(red, "x")]
	for _, line := range strings.Split(m.View(), "\n") {
		if strings.Contains(line, "late") && !strings.Contains(line, red) {
			t.Errorf("the overdue task should be red: %q", line)
		}
		if strings.Contains(line, "on time") && strings.Contains(line, red) {
			t.Errorf("a task on time should not be red: %q", line)
		}
	}
}

func TestDetailsShowDueDateTagsAndNotes(t *testing.T) {
	m := boardModel(t, board.Board{board.ToDo: {{ID: "a", Text: "pay", Tags: []string{"+home"}, Due: day(1).AddDate(0, 0, -1), Notes: "ref 123\nsecond line"}}})

	lines := strings.Join(m.detailsLines(), "\n")
	for _, want := range []string{"Due: 31-10-2026 (overdue)", "Tags: +home", "pay", "Notes:", "ref 123", "second line"} {
		if !strings.Contains(lines, want) {
			t.Errorf("details should contain %q, got %q", want, lines)
		}
	}
}

func todayTasks() board.Board {
	return board.Board{
		board.ToDo: {
			{ID: "a", Text: "late", Due: day(1).AddDate(0, 0, -3)},
			{ID: "b", Text: "later", Due: day(4)},
			{ID: "c", Text: "whenever"},
		},
		board.InProgress: {{ID: "d", Text: "now", Due: day(1)}},
		board.Completed: {
			{ID: "e", Text: "done today", Completed: clockOne},
			{ID: "f", Text: "done before", Completed: day(1).Add(-time.Minute), Due: day(1)},
		},
	}
}

func TestTodayViewShowsWhatIsDueAndDoneToday(t *testing.T) {
	m := press(t, boardModel(t, todayTasks()), "t")

	assertShown(t, m, texts{board.ToDo: {"late"}, board.InProgress: {"now"}, board.Completed: {"done today"}})
	assertViewContains(t, m, "Details · Today")

	m = press(t, m, "t")
	assertShown(t, m, texts{board.ToDo: {"late", "later", "whenever"}, board.InProgress: {"now"}, board.Completed: {"done today", "done before"}})
}

func TestFinishingATaskKeepsItInTheTodayView(t *testing.T) {
	m := press(t, boardModel(t, todayTasks()), "t", "x")

	assertShown(t, m, texts{board.InProgress: {"now"}, board.Completed: {"done today", "late"}})
	if m.active != board.Completed || selectedText(m) != "late" {
		t.Errorf("selected %q in column %d, want the finished task", selectedText(m), m.active)
	}
}

func TestAddedTaskHiddenByTheViewSaysSo(t *testing.T) {
	m := press(t, boardModel(t, todayTasks()), "t", "a", "someday", tea.KeyEnter)

	assertViewContains(t, m, "The current view hides it")
	assertBoardsEqual(t, reload(t, m), texts{
		board.ToDo: {"late", "later", "whenever", "someday"}, board.InProgress: {"now"}, board.Completed: {"done today", "done before"},
	})
}

func taggedTasks() board.Board {
	return board.Board{
		board.ToDo: {
			{ID: "a", Text: "a", Tags: []string{"+work"}},
			{ID: "b", Text: "b", Tags: []string{"@home"}},
			{ID: "c", Text: "c", Tags: []string{"+work", "@home"}},
		},
		board.Completed: {{ID: "d", Text: "d", Tags: []string{"+work"}}},
	}
}

func TestFilterByTag(t *testing.T) {
	m := press(t, boardModel(t, taggedTasks()), "f")
	if !m.picking {
		t.Fatal("f should open the tag picker")
	}
	assertViewContains(t, m, "All tags")
	assertViewContains(t, m, "+work")

	m = press(t, m, "jj", tea.KeyEnter) // All tags, +work, @home
	assertShown(t, m, texts{board.ToDo: {"b", "c"}})
	assertViewContains(t, m, "Details · @home")

	m = press(t, m, "f")
	if m.pick != 2 {
		t.Errorf("the picker should start on the current tag, got row %d", m.pick)
	}
	m = press(t, m, "kk", tea.KeyEnter)
	assertShown(t, m, texts{board.ToDo: {"a", "b", "c"}, board.Completed: {"d"}})
}

func TestPickerClosesWithoutChanging(t *testing.T) {
	m := press(t, boardModel(t, taggedTasks()), "f", "j", tea.KeyEsc)

	if m.picking || m.tag != "" {
		t.Errorf("picking = %v, tag = %q, want closed with no filter", m.picking, m.tag)
	}
	m = press(t, m, "f", "x", "dd", tea.KeyEsc)
	assertSaved(t, m, texts{board.ToDo: {"a", "b", "c"}, board.Completed: {"d"}})
}

func TestPickerDrawsOnATinyScreen(t *testing.T) {
	for _, height := range []int{0, 1, 2, 3} {
		m := press(t, sized(boardModel(t, taggedTasks()), 40, height), "f", "j")
		_ = m.View() // must not panic
	}
}

func TestFilterWithNoTagsSaysSo(t *testing.T) {
	m := press(t, boardModel(t, texts{board.ToDo: {"a"}}.Board()), "f")

	if m.picking {
		t.Error("there is nothing to pick")
	}
	assertViewContains(t, m, "No tags yet")
}

func TestReorderInAFilteredViewSwapsWithTheTaskShown(t *testing.T) {
	m := press(t, boardModel(t, taggedTasks()), "f", "j", tea.KeyEnter, "j", "K") // +work: a, c

	assertShown(t, m, texts{board.ToDo: {"c", "a"}, board.Completed: {"d"}})
	assertBoardsEqual(t, reload(t, m), texts{board.ToDo: {"c", "b", "a"}, board.Completed: {"d"}})
	if selectedText(m) != "c" {
		t.Errorf("selected %q, want the moved task", selectedText(m))
	}
}

func sortTasks() board.Board {
	return board.Board{board.ToDo: {
		{ID: "a", Text: "a", Priority: "low", Due: day(9), Created: day(1)},
		{ID: "b", Text: "b", Created: day(3)},
		{ID: "c", Text: "c", Priority: "high", Due: day(5), Created: day(2)},
	}}
}

func TestSortCyclesAndKeepsManualOrder(t *testing.T) {
	m := boardModel(t, sortTasks())
	for _, step := range []struct {
		order string
		want  []string
	}{
		{"priority", []string{"c", "a", "b"}},
		{"due date", []string{"c", "a", "b"}},
		{"creation date", []string{"b", "c", "a"}},
		{"manual order", []string{"a", "b", "c"}},
	} {
		m = press(t, m, "s")
		assertShown(t, m, texts{board.ToDo: step.want})
		assertViewContains(t, m, "Sorted by "+step.order)
	}
	assertBoardsEqual(t, reload(t, m), texts{board.ToDo: {"a", "b", "c"}})
}

func TestSortKeepsTheSelectedTask(t *testing.T) {
	m := press(t, boardModel(t, sortTasks()), "jj", "s") // select c, then sort by priority

	if selectedText(m) != "c" {
		t.Errorf("selected %q, want c", selectedText(m))
	}
}

func TestReorderIsOffWhileSorted(t *testing.T) {
	m := press(t, boardModel(t, sortTasks()), "s", "J")

	assertViewContains(t, m, "Reordering works in manual order")
	assertBoardsEqual(t, reload(t, m), texts{board.ToDo: {"a", "b", "c"}})
}

func archiveTasks() board.Board {
	return board.Board{
		board.ToDo:      {{ID: "a", Text: "open"}},
		board.Completed: {{ID: "b", Text: "finished"}, {ID: "c", Text: "old", Archived: true}},
	}
}

func TestArchiveHidesACompletedTask(t *testing.T) {
	m := press(t, boardModel(t, archiveTasks()), "3", "A")

	assertShown(t, m, texts{board.ToDo: {"open"}})
	if got := reload(t, m)[board.Completed][0]; !got.Archived || got.Text != "finished" {
		t.Errorf("got %+v, want it saved as archived", got)
	}
	assertViewContains(t, m, "Task archived")
	assertViewContains(t, m, "0/1 done")

	m = press(t, m, "u")
	assertShown(t, m, texts{board.ToDo: {"open"}, board.Completed: {"finished"}})
}

func TestOnlyCompletedTasksCanBeArchived(t *testing.T) {
	m := press(t, boardModel(t, archiveTasks()), "A")

	assertViewContains(t, m, "Only completed tasks can be archived")
	if reload(t, m)[board.ToDo][0].Archived {
		t.Error("the open task should not be archived")
	}
}

func TestArchiveViewShowsAndRestoresArchivedTasks(t *testing.T) {
	m := press(t, boardModel(t, archiveTasks()), "z")

	assertShown(t, m, texts{board.Completed: {"old"}})
	assertViewContains(t, m, "Details · Archived")
	if m.active != board.Completed {
		t.Error("the archive view should focus Completed")
	}

	m = press(t, m, "A")
	assertShown(t, m, texts{})
	assertViewContains(t, m, "Task restored")

	m = press(t, m, "z")
	assertShown(t, m, texts{board.ToDo: {"open"}, board.Completed: {"finished", "old"}})
}

func TestArchivedTasksCantBeChanged(t *testing.T) {
	m := press(t, boardModel(t, archiveTasks()), "z")
	for _, k := range []string{"x", "p", " ", "H", "e", "a"} {
		m = press(t, m, k)
		assertViewContains(t, m, "Archived tasks can't be changed")
	}
	assertBoardsEqual(t, reload(t, m), texts{board.ToDo: {"open"}, board.Completed: {"finished", "old"}})
	if got := reload(t, m)[board.Completed][1]; !got.Archived || got.Priority != "" {
		t.Errorf("got %+v, want it unchanged", got)
	}
}

func searchTasks() board.Board {
	return board.Board{
		board.ToDo:       {{ID: "a", Text: "buy milk"}, {ID: "b", Text: "call bank", Tags: []string{"+money"}}},
		board.InProgress: {{ID: "c", Text: "write report", Notes: "mention milk prices"}},
		board.Completed:  {{ID: "d", Text: "milkshake"}},
	}
}

func TestSearchJumpsBetweenMatchesAcrossColumns(t *testing.T) {
	// All three match "mlk" equally tightly, and milkshake's match starts first.
	m := press(t, boardModel(t, searchTasks()), "j", "/", "mlk", tea.KeyEnter)

	if m.active != board.Completed || selectedText(m) != "milkshake" {
		t.Errorf("selected %q, want the best match", selectedText(m))
	}
	assertViewContains(t, m, `Match 3 of 3 for "mlk"`)

	m = press(t, m, "n")
	if selectedText(m) != "buy milk" {
		t.Errorf("n should wrap around to the first match on the board, selected %q", selectedText(m))
	}
	m = press(t, m, "n")
	if m.active != board.InProgress || selectedText(m) != "write report" {
		t.Errorf("n selected %q, want the next match on the board, found in notes", selectedText(m))
	}
	m = press(t, m, "N")
	if selectedText(m) != "buy milk" {
		t.Errorf("N should go back, selected %q", selectedText(m))
	}
}

func rankedTasks() board.Board {
	return board.Board{
		board.ToDo:       {{ID: "a", Text: "summarily kept"}},
		board.InProgress: {{ID: "b", Text: "milk"}},
		board.Completed:  {{ID: "c", Text: "write report", Notes: "about milk"}},
	}
}

func TestSearchListsTheBestMatchesFirst(t *testing.T) {
	m := press(t, boardModel(t, rankedTasks()), "/", "milk")

	lines := strings.Split(ansi.Strip(m.View()), "\n")
	rowOf := func(column int, text string) int { // rows start with their column's bullet
		for i, line := range lines {
			if strings.Contains(line, columns[column].bullet+" "+text) {
				return i
			}
		}
		return -1
	}
	// write report matches as tightly, but in its notes, so after milk.
	// summarily kept matches spread out, so last.
	best, second, third := rowOf(board.InProgress, "milk"), rowOf(board.Completed, "write report"), rowOf(board.ToDo, "summarily kept")
	if best < 0 || second <= best || third <= second {
		t.Fatalf("want milk, then write report, then summarily kept:\n%s", strings.Join(lines, "\n"))
	}
	assertViewContains(t, m, "in notes") // write report matched only in its notes

	m = press(t, m, tea.KeyEnter)
	if selectedText(m) != "milk" {
		t.Errorf("enter selected %q, want the best match", selectedText(m))
	}
}

func TestSearchChoosesAMatchWithArrowsOrCtrlNP(t *testing.T) {
	m := press(t, boardModel(t, rankedTasks()), "/", "milk", tea.KeyDown, tea.KeyEnter)
	if selectedText(m) != "write report" {
		t.Errorf("down then enter selected %q, want the second match", selectedText(m))
	}

	m = press(t, m, "/", "milk", tea.KeyCtrlN, tea.KeyCtrlN, tea.KeyCtrlN, tea.KeyCtrlP, tea.KeyEnter)
	if selectedText(m) != "write report" {
		t.Errorf("selected %q; ctrl+n should stop at the last match, then ctrl+p go up one", selectedText(m))
	}

	m = press(t, m, "/", "mi", tea.KeyDown, "l", tea.KeyEnter) // typing goes back to the best match
	if selectedText(m) != "milk" {
		t.Errorf("selected %q, want the best match for %q", selectedText(m), "mil")
	}
}

func TestSearchOpensAPopupThatCountsMatches(t *testing.T) {
	m := press(t, boardModel(t, searchTasks()), "/")
	assertViewContains(t, m, "Search text, tags and notes")
	assertViewOmits(t, m, "buy milk") // the popup takes the screen, like the tag picker

	m = press(t, m, "mlk")
	assertViewContains(t, m, "3 matches")
	m = press(t, m, tea.KeyBackspace, tea.KeyBackspace, tea.KeyBackspace, "money")
	assertViewContains(t, m, "1 match")
	m = press(t, m, "z")
	assertViewContains(t, m, "No match")

	m = press(t, m, tea.KeyEsc)
	assertViewContains(t, m, "buy milk")
}

func TestSearchPopupFitsATinyScreen(t *testing.T) {
	m := press(t, boardModel(t, searchTasks()), "/", "a very long search that is wider than the popup")
	for _, width := range []int{minWidth, 60, 200} {
		m = sized(m, width, minHeight)
		for _, line := range strings.Split(m.View(), "\n") {
			if w := lipgloss.Width(line); w > width {
				t.Fatalf("at width %d a line is %d wide: %q", width, w, line)
			}
		}
	}
}

func TestSearchFindsTagsAndTheSelectedTask(t *testing.T) {
	m := press(t, boardModel(t, searchTasks()), "j", "/", "MONEY", tea.KeyEnter)

	if selectedText(m) != "call bank" {
		t.Errorf("selected %q, want the task tagged +money", selectedText(m))
	}
}

func TestSearchWithoutAMatchSaysSo(t *testing.T) {
	m := press(t, boardModel(t, searchTasks()), "/", "zebra", tea.KeyEnter)

	assertViewContains(t, m, `No match for "zebra"`)
	if selectedText(m) != "buy milk" {
		t.Error("the selection should not move")
	}
}

func TestSearchInputDoesNotActOnTasks(t *testing.T) {
	m := press(t, boardModel(t, searchTasks()), "/", "dd x", tea.KeyEsc)

	if m.searching || m.query != "" {
		t.Errorf("searching = %v, query = %q, want the search cancelled", m.searching, m.query)
	}
	assertBoardsEqual(t, reload(t, m), texts{board.ToDo: {"buy milk", "call bank"}, board.InProgress: {"write report"}, board.Completed: {"milkshake"}})
}

func TestNextMatchBeforeSearchingSaysHow(t *testing.T) {
	m := press(t, boardModel(t, searchTasks()), "n")

	assertViewContains(t, m, "Press / to search")
}

func TestSearchFindsArchivedTasksInTheArchiveView(t *testing.T) {
	m := press(t, boardModel(t, archiveTasks()), "/", "old", tea.KeyEnter)
	assertViewContains(t, m, `No match for "old"`)

	m = press(t, m, "z", "n")
	if selectedText(m) != "old" {
		t.Errorf("selected %q, want the archived task", selectedText(m))
	}
}

func TestPickerSurvivesTagsRemovedElsewhere(t *testing.T) {
	m := press(t, boardModel(t, taggedTasks()), "f", "jj") // on @home, the last row
	changeElsewhere(t, m, func(b board.Board) board.Board {
		for column := range b {
			for i := range b[column] {
				b[column][i].Tags = nil
			}
		}
		return b
	})
	m = send(m, fileCheck{})

	assertViewContains(t, m, "All tags") // drawing must not crash
	m = press(t, m, tea.KeyEnter)
	if m.picking || m.tag != "" {
		t.Errorf("picking = %v, tag = %q, want the filter off", m.picking, m.tag)
	}
}

func TestChangesWhileSortedKeepTheSameTaskSelected(t *testing.T) {
	m := boardModel(t, board.Board{board.ToDo: {
		{ID: "a", Text: "a", Priority: "low"},
		{ID: "b", Text: "b", Priority: "medium"},
		{ID: "c", Text: "c"},
	}})
	m = press(t, m, "s") // by priority: b, a, c, with a still selected

	m = press(t, m, "p", "p") // a moves up past b, and must stay selected

	if selectedText(m) != "a" {
		t.Errorf("selected %q, want a", selectedText(m))
	}
	got := reload(t, m)[board.ToDo]
	if got[0].Priority != "high" || got[1].Priority != "medium" {
		t.Errorf("a = %q, b = %q, want only a changed, to high", got[0].Priority, got[1].Priority)
	}
}

func TestDueDatesKeepTheirDayInAnotherTimeZone(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("no time zone data:", err)
	}
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("no time zone data:", err)
	}
	friday := time.Date(2026, 11, 6, 0, 0, 0, 0, berlin) // due:fri, typed in Berlin
	m := boardModel(t, board.Board{board.ToDo: {{ID: "a", Text: "pay", Due: friday}}})
	m = at(m, time.Date(2026, 11, 5, 20, 0, 0, 0, newYork)) // Thursday evening in New York

	assertViewContains(t, m, "pay · due 06-11")
	assertViewOmits(t, m, "due today")
	if m = press(t, m, "e"); m.input.Value() != "pay due:06-11-2026" {
		t.Errorf("edit shows %q, want the same day as the board", m.input.Value())
	}
}
