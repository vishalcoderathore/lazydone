package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// testModel returns a model that saves to a temporary file.
func testModel(t *testing.T, tasks texts) model {
	t.Helper()
	return newModel(filepath.Join(t.TempDir(), "tasks.json"), tasks.board())
}

// press sends key presses to the model. Named keys are given as tea.KeyType
// values, and strings are typed one character at a time.
func press(t *testing.T, m model, keys ...any) model {
	t.Helper()
	for _, key := range keys {
		switch k := key.(type) {
		case tea.KeyType:
			m = send(m, tea.KeyMsg{Type: k})
		case string:
			for _, r := range k {
				if r == ' ' {
					m = send(m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{r}})
				} else {
					m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				}
			}
		default:
			t.Fatalf("unsupported key %v", key)
		}
	}
	return m
}

func send(m model, msg tea.Msg) model {
	updated, _ := m.Update(msg)
	return updated.(model)
}

// assertSaved checks both the board in memory and the one in the file.
func assertSaved(t *testing.T, m model, want texts) {
	t.Helper()
	assertBoardsEqual(t, m.tasks, want)
	saved, err := loadBoard(m.path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	assertBoardsEqual(t, saved, want)
}

func TestAddPutsTaskInToDo(t *testing.T) {
	m := press(t, testModel(t, texts{}), "a", "buy milk", tea.KeyEnter)

	assertSaved(t, m, texts{toDo: {"buy milk"}})
	if m.adding || m.input.Value() != "" {
		t.Errorf("expected input to be closed and empty, got adding=%v input=%q", m.adding, m.input.Value())
	}
}

func TestFourKeyAddsTaskLikeA(t *testing.T) {
	m := press(t, testModel(t, texts{}), "4")
	if !m.adding {
		t.Fatal("4 should open the new task input")
	}

	m = press(t, m, "buy milk", tea.KeyEnter)
	assertSaved(t, m, texts{toDo: {"buy milk"}})
}

func TestFourIsTypedAsTextWhileAdding(t *testing.T) {
	m := press(t, testModel(t, texts{}), "4", "buy 4 eggs", tea.KeyEnter)

	assertBoardsEqual(t, m.tasks, texts{toDo: {"buy 4 eggs"}})
}

func TestAddFromAnotherColumnFocusesNewTask(t *testing.T) {
	m := testModel(t, texts{toDo: {"one"}})
	m = press(t, m, tea.KeyRight, tea.KeyRight, "a", "two", tea.KeyEnter)

	if m.active != toDo || m.cursors[toDo] != 1 {
		t.Errorf("active = %d, cursor = %d, want column %d, cursor 1", m.active, m.cursors[toDo], toDo)
	}
}

func TestAddIgnoresBlankText(t *testing.T) {
	m := press(t, testModel(t, texts{}), "a", "   ", tea.KeyEnter)

	assertBoardsEqual(t, m.tasks, texts{})
}

func TestAddCollapsesPastedNewlines(t *testing.T) {
	m := press(t, testModel(t, texts{}), "a")
	m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("  one\ntwo  ")})
	m = press(t, m, tea.KeyEnter)

	assertSaved(t, m, texts{toDo: {"one two"}})
}

func TestEscCancelsAdd(t *testing.T) {
	m := press(t, testModel(t, texts{}), "a", "oops", tea.KeyEsc)

	assertBoardsEqual(t, m.tasks, texts{})
	if m.adding || m.input.Value() != "" {
		t.Errorf("expected input to be closed and empty, got adding=%v input=%q", m.adding, m.input.Value())
	}
}

func TestBackspaceRemovesWholeCharacter(t *testing.T) {
	m := press(t, testModel(t, texts{}), "a", "ab日", tea.KeyBackspace)
	if m.input.Value() != "ab" {
		t.Errorf("input = %q, want %q", m.input.Value(), "ab")
	}
}

func TestBackspaceOnEmptyInputDoesNothing(t *testing.T) {
	m := press(t, testModel(t, texts{}), "a", tea.KeyBackspace)
	if m.input.Value() != "" {
		t.Errorf("input = %q, want empty", m.input.Value())
	}
}

func TestCommandKeysAreTypedAsTextWhileAdding(t *testing.T) {
	m := press(t, testModel(t, texts{}), "a", "dqLH1", tea.KeyEnter)

	assertBoardsEqual(t, m.tasks, texts{toDo: {"dqLH1"}})
}

func TestDeleteOnlyAffectsActiveColumn(t *testing.T) {
	m := testModel(t, texts{toDo: {"one"}, inProgress: {"two", "three"}})
	m = press(t, m, "2", tea.KeyDown, "dd")

	assertSaved(t, m, texts{toDo: {"one"}, inProgress: {"two"}})
	if m.cursors[inProgress] != 0 {
		t.Errorf("cursor = %d, want 0", m.cursors[inProgress])
	}
}

func TestDeleteOnEmptyColumnDoesNothing(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"one"}}), "2", "dd")

	assertBoardsEqual(t, m.tasks, texts{toDo: {"one"}})
}

func TestMoveTaskAcrossBoard(t *testing.T) {
	m := testModel(t, texts{toDo: {"one", "two"}, inProgress: {"three"}})

	m = press(t, m, "L")
	assertSaved(t, m, texts{toDo: {"two"}, inProgress: {"three", "one"}})
	if m.active != inProgress || m.cursors[inProgress] != 1 {
		t.Errorf("selection did not follow the task: active=%d cursor=%d", m.active, m.cursors[inProgress])
	}

	m = press(t, m, "L")
	assertSaved(t, m, texts{toDo: {"two"}, inProgress: {"three"}, completed: {"one"}})

	m = press(t, m, "H")
	assertSaved(t, m, texts{toDo: {"two"}, inProgress: {"three", "one"}})
}

func TestMoveWithShiftArrows(t *testing.T) {
	m := testModel(t, texts{toDo: {"one"}})

	m = press(t, m, tea.KeyShiftRight)
	assertSaved(t, m, texts{inProgress: {"one"}})

	m = press(t, m, tea.KeyShiftLeft)
	assertSaved(t, m, texts{toDo: {"one"}})
}

func TestSpaceCyclesTaskThroughEveryStage(t *testing.T) {
	m := testModel(t, texts{toDo: {"one", "two"}})

	m = press(t, m, " ")
	assertSaved(t, m, texts{toDo: {"two"}, inProgress: {"one"}})

	m = press(t, m, " ")
	assertSaved(t, m, texts{toDo: {"two"}, completed: {"one"}})

	m = press(t, m, " ")
	assertSaved(t, m, texts{toDo: {"two", "one"}})
	if m.active != toDo || m.cursors[toDo] != 1 {
		t.Errorf("selection did not follow the task: active=%d cursor=%d", m.active, m.cursors[toDo])
	}
}

func TestSpaceOnEmptyColumnDoesNothing(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"one"}}), "2", " ")

	assertBoardsEqual(t, m.tasks, texts{toDo: {"one"}})
	if m.active != inProgress {
		t.Errorf("active = %d, want %d", m.active, inProgress)
	}
}

func TestSpaceIsTypedAsTextWhileAdding(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"one"}}), "a", "two words", tea.KeyEnter)

	assertBoardsEqual(t, m.tasks, texts{toDo: {"one", "two words"}})
}

func TestMovePastEdgesDoesNothing(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"one"}, completed: {"two"}}), "H", "3", "L")

	assertBoardsEqual(t, m.tasks, texts{toDo: {"one"}, completed: {"two"}})
}

func TestMoveFromEmptyColumnDoesNothing(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"one"}}), "2", "L")

	assertBoardsEqual(t, m.tasks, texts{toDo: {"one"}})
	if m.active != inProgress {
		t.Errorf("active = %d, want %d", m.active, inProgress)
	}
}

func TestMoveLastTaskKeepsSourceCursorValid(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"one", "two"}}), tea.KeyDown, "L")

	if m.cursors[toDo] != 0 {
		t.Errorf("cursor = %d, want 0", m.cursors[toDo])
	}
}

func TestColumnNavigationWrapsAround(t *testing.T) {
	m := testModel(t, texts{})

	m = press(t, m, tea.KeyLeft)
	if m.active != completed {
		t.Errorf("active = %d after left from first column, want %d", m.active, completed)
	}

	m = press(t, m, tea.KeyTab)
	if m.active != toDo {
		t.Errorf("active = %d after tab from last column, want %d", m.active, toDo)
	}
}

func TestEachColumnRemembersItsCursor(t *testing.T) {
	m := testModel(t, texts{toDo: {"a", "b"}, inProgress: {"c"}})
	m = press(t, m, tea.KeyDown, tea.KeyRight, tea.KeyLeft)

	if m.cursors[toDo] != 1 {
		t.Errorf("cursor = %d, want 1", m.cursors[toDo])
	}
}

func TestCursorStaysInsideColumn(t *testing.T) {
	m := testModel(t, texts{toDo: {"one", "two"}})

	m = press(t, m, tea.KeyUp)
	if m.cursors[toDo] != 0 {
		t.Errorf("cursor = %d after moving above the top, want 0", m.cursors[toDo])
	}

	m = press(t, m, tea.KeyDown, tea.KeyDown, tea.KeyDown)
	if m.cursors[toDo] != 1 {
		t.Errorf("cursor = %d after moving past the bottom, want 1", m.cursors[toDo])
	}
}

func TestQuitKeys(t *testing.T) {
	m := testModel(t, texts{})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Error("q should quit")
	}

	m.adding = true
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("ctrl+c should quit while adding")
	}
}

var (
	clockOne = time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	clockTwo = time.Date(2026, 11, 2, 17, 30, 0, 0, time.UTC)
)

func at(m model, now time.Time) model {
	m.now = func() time.Time { return now }
	return m
}

// reload reads back what the model saved, as the next run of the app would.
func reload(t *testing.T, m model) board {
	t.Helper()
	saved, err := loadBoard(m.path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	return saved
}

func TestAddedTaskGetsIDAndTimestamps(t *testing.T) {
	m := press(t, at(testModel(t, texts{}), clockOne), "a", "one", tea.KeyEnter)

	got := reload(t, m)[toDo][0]
	if got.ID == "" {
		t.Error("task should have an ID")
	}
	if !got.Created.Equal(clockOne) || !got.Updated.Equal(clockOne) {
		t.Errorf("created = %v, updated = %v, want %v", got.Created, got.Updated, clockOne)
	}
	if !got.Completed.IsZero() {
		t.Errorf("a new task should not be completed, got %v", got.Completed)
	}
}

func TestAddedTasksHaveDifferentIDs(t *testing.T) {
	m := press(t, testModel(t, texts{}), "a", "one", tea.KeyEnter, "a", "two", tea.KeyEnter)

	tasks := reload(t, m)[toDo]
	if tasks[0].ID == tasks[1].ID {
		t.Errorf("both tasks have ID %q", tasks[0].ID)
	}
}

func TestMovingTaskKeepsItsFieldsAndRecordsTheChange(t *testing.T) {
	due := time.Date(2026, 12, 25, 0, 0, 0, 0, time.UTC)
	original := task{
		ID: "keep", Text: "one", Priority: "high", Due: due, Tags: []string{"+work"},
		Notes: "note", Created: clockOne, Updated: clockOne,
	}
	m := at(newModel(filepath.Join(t.TempDir(), "tasks.json"), board{toDo: {original}}), clockTwo)

	m = press(t, m, "L")

	want := original
	want.Updated = clockTwo
	if got := reload(t, m)[inProgress][0]; !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestCompletingTaskRecordsWhen(t *testing.T) {
	m := at(testModel(t, texts{inProgress: {"one"}}), clockTwo)

	m = press(t, m, "2", "L")

	if got := reload(t, m)[completed][0].Completed; !got.Equal(clockTwo) {
		t.Errorf("completed = %v, want %v", got, clockTwo)
	}
}

func TestReopeningTaskClearsCompleted(t *testing.T) {
	m := at(testModel(t, texts{inProgress: {"one"}}), clockTwo)

	m = press(t, m, "2", "L", "H")

	if got := reload(t, m)[inProgress][0].Completed; !got.IsZero() {
		t.Errorf("a reopened task should not be completed, got %v", got)
	}
}

func TestBoardSurvivesRestart(t *testing.T) {
	m := sized(testModel(t, texts{}), 100, 24)
	m = press(t, m, "a", "one", tea.KeyEnter, "a", "two", tea.KeyEnter, "L")

	restarted := sized(newModel(m.path, reload(t, m)), 100, 24)

	assertBoardsEqual(t, restarted.tasks, texts{toDo: {"one"}, inProgress: {"two"}})
	assertViewContains(t, restarted, "one")
	assertViewContains(t, restarted, "two")
}

func TestOneDDoesNotDelete(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"one"}}), "d")

	assertBoardsEqual(t, m.tasks, texts{toDo: {"one"}})
}

func TestDDDeletesAndSaves(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"one", "two"}}), "dd")

	assertSaved(t, m, texts{toDo: {"two"}})
}

func TestOtherKeyCancelsDeleteAndStillWorks(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"one", "two"}}), "d", tea.KeyDown, "d")

	assertBoardsEqual(t, m.tasks, texts{toDo: {"one", "two"}})
	if m.cursors[toDo] != 1 {
		t.Errorf("cursor = %d, the other key should still move it to 1", m.cursors[toDo])
	}
}

func TestDInAnotherColumnStartsOver(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"one"}, inProgress: {"two"}}), "d", "l", "d")

	assertBoardsEqual(t, m.tasks, texts{toDo: {"one"}, inProgress: {"two"}})
}

func TestUndoBringsDeletedTaskBackInPlace(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"one", "two", "three"}}), "j", "dd", "u")

	assertSaved(t, m, texts{toDo: {"one", "two", "three"}})
	if m.active != toDo || m.cursors[toDo] != 1 {
		t.Errorf("active = %d, cursor = %d, the restored task should be selected", m.active, m.cursors[toDo])
	}
}

func TestUndoRestoresEveryFieldOfDeletedTask(t *testing.T) {
	original := task{ID: "keep", Text: "one", Priority: "high", Tags: []string{"+work"}, Notes: "note", Created: clockOne, Updated: clockOne}
	m := newModel(filepath.Join(t.TempDir(), "tasks.json"), board{toDo: {original}})

	m = press(t, m, "dd", "u")

	if got := reload(t, m)[toDo][0]; !reflect.DeepEqual(got, original) {
		t.Errorf("got %+v, want %+v", got, original)
	}
}

func TestUndoRemovesAddedTask(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"one"}}), "a", "two", tea.KeyEnter, "u")

	assertSaved(t, m, texts{toDo: {"one"}})
}

func TestUndoReturnsMovedTaskToItsPlace(t *testing.T) {
	m := at(testModel(t, texts{inProgress: {"one", "two"}}), clockOne)
	m = press(t, m, "2", "L")
	if got := reload(t, m)[completed][0].Completed; got.IsZero() {
		t.Fatal("the task should have been completed")
	}

	m = press(t, m, "u")

	assertSaved(t, m, texts{inProgress: {"one", "two"}})
	if got := reload(t, m)[inProgress][0].Completed; !got.IsZero() {
		t.Errorf("undo should clear the completed time, got %v", got)
	}
	if m.active != inProgress {
		t.Errorf("active = %d, should follow the task back", m.active)
	}
}

func TestUndoWorksBackThroughHistory(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"one", "two"}}), "dd", "dd", "u", "u")

	assertSaved(t, m, texts{toDo: {"one", "two"}})
}

func TestUndoWithEmptyHistoryDoesNothing(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"one"}}), "u")

	assertBoardsEqual(t, m.tasks, texts{toDo: {"one"}})
}

func TestUndoHistoryHoldsFiftyActions(t *testing.T) {
	m := testModel(t, texts{})
	for range 55 {
		m = press(t, m, "a", "x", tea.KeyEnter)
	}
	for range 60 {
		m = press(t, m, "u")
	}

	if got := len(m.tasks[toDo]); got != 5 {
		t.Errorf("%d tasks left, want the 5 too old to undo", got)
	}
}

func TestUndoFindsTaskWithoutIDFromAHandEditedFile(t *testing.T) {
	m := newModel(filepath.Join(t.TempDir(), "tasks.json"), board{toDo: {{Text: "one"}, {Text: "two"}}})

	m = press(t, m, "L", "u")

	assertSaved(t, m, texts{toDo: {"one", "two"}})
}

func manyTasks(count int) texts {
	var ts texts
	for i := range count {
		ts[toDo] = append(ts[toDo], fmt.Sprintf("task %d", i))
	}
	return ts
}

func TestGJumpsToLastTaskAndGGToFirst(t *testing.T) {
	m := press(t, testModel(t, manyTasks(5)), "G")
	if m.cursors[toDo] != 4 {
		t.Errorf("after G, cursor = %d, want 4", m.cursors[toDo])
	}

	m = press(t, m, "gg")
	if m.cursors[toDo] != 0 {
		t.Errorf("after gg, cursor = %d, want 0", m.cursors[toDo])
	}
}

func TestOneGDoesNotJump(t *testing.T) {
	m := press(t, testModel(t, manyTasks(5)), "G", "g")

	if m.cursors[toDo] != 4 {
		t.Errorf("cursor = %d, want 4", m.cursors[toDo])
	}
}

func TestJumpsOnEmptyColumnDoNothing(t *testing.T) {
	m := press(t, testModel(t, texts{}), "G", "gg", tea.KeyCtrlD, tea.KeyCtrlU)

	if m.cursors[toDo] != 0 {
		t.Errorf("cursor = %d, want 0", m.cursors[toDo])
	}
}

func TestCtrlDAndCtrlUMoveHalfAPage(t *testing.T) {
	// At a height of 24, a column shows 16 tasks.
	m := sized(testModel(t, manyTasks(40)), 100, 24)

	m = press(t, m, tea.KeyCtrlD)
	if m.cursors[toDo] != 8 {
		t.Errorf("after ctrl+d, cursor = %d, want 8", m.cursors[toDo])
	}

	m = press(t, m, tea.KeyCtrlD, tea.KeyCtrlU, tea.KeyCtrlU)
	if m.cursors[toDo] != 0 {
		t.Errorf("after moving back, cursor = %d, want 0", m.cursors[toDo])
	}
}

func TestHalfPageMovesStopAtTheEnds(t *testing.T) {
	m := sized(testModel(t, manyTasks(5)), 100, 24)

	m = press(t, m, tea.KeyCtrlD)
	if m.cursors[toDo] != 4 {
		t.Errorf("after ctrl+d, cursor = %d, want 4", m.cursors[toDo])
	}

	m = press(t, m, tea.KeyCtrlU)
	if m.cursors[toDo] != 0 {
		t.Errorf("after ctrl+u, cursor = %d, want 0", m.cursors[toDo])
	}
}

func TestXCompletesTaskFromAnyOpenColumn(t *testing.T) {
	for name, keys := range map[string]string{"to do": "x", "in progress": "2x"} {
		t.Run(name, func(t *testing.T) {
			m := at(testModel(t, texts{toDo: {"a"}, inProgress: {"b"}}), clockTwo)
			m = press(t, m, keys)

			if m.active != completed || len(m.tasks[completed]) != 1 {
				t.Fatalf("active = %d, completed = %v", m.active, m.tasks[completed])
			}
			if got := reload(t, m)[completed][0].Completed; !got.Equal(clockTwo) {
				t.Errorf("completed time = %v, want %v", got, clockTwo)
			}
		})
	}
}

func TestXReopensCompletedTaskInToDo(t *testing.T) {
	m := press(t, testModel(t, texts{completed: {"a"}}), "3", "x")

	assertSaved(t, m, texts{toDo: {"a"}})
	if m.active != toDo {
		t.Errorf("active = %d, should follow the task", m.active)
	}
}

func TestXOnEmptyColumnDoesNothing(t *testing.T) {
	m := press(t, testModel(t, texts{inProgress: {"a"}}), "x")

	assertBoardsEqual(t, m.tasks, texts{inProgress: {"a"}})
}

func TestUndoReversesX(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"a"}}), "x", "u")

	assertSaved(t, m, texts{toDo: {"a"}})
}

func TestPCyclesPriorityAndSaves(t *testing.T) {
	m := at(testModel(t, texts{toDo: {"a"}}), clockTwo)

	for _, want := range []string{"low", "medium", "high", ""} {
		m = press(t, m, "p")
		got := reload(t, m)[toDo][0]
		if got.Priority != want {
			t.Errorf("priority = %q, want %q", got.Priority, want)
		}
		if !got.Updated.Equal(clockTwo) {
			t.Errorf("updated = %v, want %v", got.Updated, clockTwo)
		}
	}
}

func TestPOnEmptyColumnDoesNothing(t *testing.T) {
	press(t, testModel(t, texts{}), "p")
}

func TestUndoReversesPriority(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"a"}}), "p", "p", "u")

	if got := reload(t, m)[toDo][0].Priority; got != "low" {
		t.Errorf("priority = %q, want low", got)
	}
}

func TestBoardAndDetailsShowPriority(t *testing.T) {
	m := sized(testModel(t, texts{toDo: {"a"}}), 100, 24)
	assertViewOmits(t, m, "Priority")

	for _, c := range []struct{ marker, name string }{{"! a", "low"}, {"!! a", "medium"}, {"!!! a", "high"}} {
		m = press(t, m, "p")
		assertViewContains(t, m, c.marker)
		assertViewContains(t, m, "Priority: "+c.name)
	}
}

func TestShiftJMovesTaskDownAndKeepsItSelected(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"a", "b", "c"}}), "J")

	assertSaved(t, m, texts{toDo: {"b", "a", "c"}})
	if m.cursors[toDo] != 1 {
		t.Errorf("cursor = %d, want 1", m.cursors[toDo])
	}
}

func TestShiftKMovesTaskUpAndKeepsItSelected(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"a", "b", "c"}}), "G", "K")

	assertSaved(t, m, texts{toDo: {"a", "c", "b"}})
	if m.cursors[toDo] != 1 {
		t.Errorf("cursor = %d, want 1", m.cursors[toDo])
	}
}

func TestReorderingStopsAtTheEnds(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"a", "b"}}), "K", "G", "J")

	assertBoardsEqual(t, m.tasks, texts{toDo: {"a", "b"}})
}

func TestReorderThatDoesNothingIsNotUndone(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"a", "b"}}), "dd", "K", "u")

	assertBoardsEqual(t, m.tasks, texts{toDo: {"a", "b"}})
}

func TestReorderingEmptyColumnDoesNothing(t *testing.T) {
	press(t, testModel(t, texts{}), "J", "K")
}

func TestUndoReversesReorder(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"a", "b", "c"}}), "J", "u")

	assertSaved(t, m, texts{toDo: {"a", "b", "c"}})
	if m.cursors[toDo] != 0 {
		t.Errorf("cursor = %d, want 0", m.cursors[toDo])
	}
}

func TestInputSupportsCursorMovement(t *testing.T) {
	m := press(t, testModel(t, texts{}), "a", "bd", tea.KeyLeft, "c", tea.KeyHome, "a", tea.KeyEnd, "e", tea.KeyEnter)

	assertSaved(t, m, texts{toDo: {"abcde"}})
}

func TestInputDeletesWholeWord(t *testing.T) {
	m := press(t, testModel(t, texts{}), "a", "keep drop", tea.KeyCtrlW, tea.KeyEnter)

	assertSaved(t, m, texts{toDo: {"keep"}})
}

// The clipboard library chooses its command when the program starts, and the
// message carrying the text is private to Bubbles. So this test runs itself again
// with a fake xclip first on the PATH, and follows a real paste from the key press
// to the saved task.
func TestCtrlVPastesClipboardText(t *testing.T) {
	const childEnv = "LAZYDONE_PASTE_TEST_CHILD"
	if os.Getenv(childEnv) == "" {
		dir := t.TempDir()
		script := "#!/bin/sh\nprintf 'pasted text'\n"
		if err := os.WriteFile(filepath.Join(dir, "xclip"), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		binary, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		child := exec.Command(binary, "-test.run=^TestCtrlVPastesClipboardText$")
		child.Env = append(os.Environ(), childEnv+"=1", "PATH="+dir, "WAYLAND_DISPLAY=")
		if output, err := child.CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, output)
		}
		return
	}

	m := press(t, testModel(t, texts{}), "a")
	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyCtrlV})
	if cmd == nil {
		t.Fatal("ctrl+v should return the command that reads the clipboard")
	}
	m = send(m, cmd())

	m = press(t, m, "!", tea.KeyEnter)
	assertSaved(t, m, texts{toDo: {"pasted text!"}})
}

func TestNewInputStartsEmptyAfterCancel(t *testing.T) {
	m := press(t, testModel(t, texts{}), "a", "gone", tea.KeyEsc, "a", "new", tea.KeyEnter)

	assertSaved(t, m, texts{toDo: {"new"}})
}

func TestEditChangesTextAndSaves(t *testing.T) {
	m := at(testModel(t, texts{toDo: {"one", "two"}}), clockTwo)

	m = press(t, m, "j", "e", "!", tea.KeyEnter)

	assertSaved(t, m, texts{toDo: {"one", "two!"}})
	if got := reload(t, m)[toDo][1].Updated; !got.Equal(clockTwo) {
		t.Errorf("updated = %v, want %v", got, clockTwo)
	}
	if m.adding || m.editing {
		t.Error("the input should close after saving")
	}
}

func TestEditKeepsTaskSelectedAndItsOtherFields(t *testing.T) {
	original := task{ID: "keep", Text: "one", Priority: "high", Created: clockOne, Updated: clockOne}
	m := newModel(filepath.Join(t.TempDir(), "tasks.json"), board{inProgress: {{ID: "x", Text: "first"}, original}})

	m = at(press(t, m, "2", "j"), clockTwo)
	m = press(t, m, "e", "2", tea.KeyEnter)

	got := reload(t, m)[inProgress][1]
	want := original
	want.Text, want.Updated = "one2", clockTwo
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if m.active != inProgress || m.cursors[inProgress] != 1 {
		t.Errorf("active = %d, cursor = %d, selection should not move", m.active, m.cursors[inProgress])
	}
}

func TestEscCancelsEdit(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"one"}}), "e", "changed", tea.KeyEsc)

	assertBoardsEqual(t, m.tasks, texts{toDo: {"one"}})
	if m.adding || m.editing {
		t.Error("the input should close")
	}
}

func TestBlankEditKeepsTask(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"one"}}), "e", tea.KeyCtrlU, tea.KeyEnter)

	assertBoardsEqual(t, m.tasks, texts{toDo: {"one"}})
}

func TestEditCleansWhitespace(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"one"}}), "e", "   two  ", tea.KeyEnter)

	assertSaved(t, m, texts{toDo: {"one two"}})
}

func TestEditOnEmptyColumnDoesNothing(t *testing.T) {
	m := press(t, testModel(t, texts{}), "e")

	if m.adding || m.editing {
		t.Error("there is nothing to edit")
	}
}

func TestUndoReversesEdit(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"one"}}), "e", "!", tea.KeyEnter, "u")

	assertSaved(t, m, texts{toDo: {"one"}})
}

func TestEditThatChangesNothingIsNotUndone(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"one", "two"}}), "dd", "e", tea.KeyEnter, "u")

	assertBoardsEqual(t, m.tasks, texts{toDo: {"one", "two"}})
}

func TestAddAfterEditStartsEmpty(t *testing.T) {
	m := press(t, testModel(t, texts{toDo: {"one"}}), "e", tea.KeyEsc, "a", "new", tea.KeyEnter)

	assertSaved(t, m, texts{toDo: {"one", "new"}})
}

func TestViewShowsEditPanelWithCurrentText(t *testing.T) {
	m := press(t, sized(testModel(t, texts{toDo: {"one"}}), 100, 24), "e")

	assertViewContains(t, m, "Edit Task")
	assertViewContains(t, m, "│one")
}
