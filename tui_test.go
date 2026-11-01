package main

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// testModel returns a model that saves to a temporary file.
func testModel(t *testing.T, tasks board) model {
	t.Helper()
	return newModel(filepath.Join(t.TempDir(), "todos.txt"), tasks)
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
func assertSaved(t *testing.T, m model, want board) {
	t.Helper()
	assertBoardsEqual(t, m.tasks, want)
	saved, err := loadBoard(m.path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	assertBoardsEqual(t, saved, want)
}

func TestAddPutsTaskInToDo(t *testing.T) {
	m := press(t, testModel(t, board{}), "a", "buy milk", tea.KeyEnter)

	assertSaved(t, m, board{toDo: {"buy milk"}})
	if m.adding || m.input != "" {
		t.Errorf("expected input to be closed and empty, got adding=%v input=%q", m.adding, m.input)
	}
}

func TestFourKeyAddsTaskLikeA(t *testing.T) {
	m := press(t, testModel(t, board{}), "4")
	if !m.adding {
		t.Fatal("4 should open the new task input")
	}

	m = press(t, m, "buy milk", tea.KeyEnter)
	assertSaved(t, m, board{toDo: {"buy milk"}})
}

func TestFourIsTypedAsTextWhileAdding(t *testing.T) {
	m := press(t, testModel(t, board{}), "4", "buy 4 eggs", tea.KeyEnter)

	assertBoardsEqual(t, m.tasks, board{toDo: {"buy 4 eggs"}})
}

func TestAddFromAnotherColumnFocusesNewTask(t *testing.T) {
	m := testModel(t, board{toDo: {"one"}})
	m = press(t, m, tea.KeyRight, tea.KeyRight, "a", "two", tea.KeyEnter)

	if m.active != toDo || m.cursors[toDo] != 1 {
		t.Errorf("active = %d, cursor = %d, want column %d, cursor 1", m.active, m.cursors[toDo], toDo)
	}
}

func TestAddIgnoresBlankText(t *testing.T) {
	m := press(t, testModel(t, board{}), "a", "   ", tea.KeyEnter)

	assertBoardsEqual(t, m.tasks, board{})
}

func TestAddCollapsesPastedNewlines(t *testing.T) {
	m := press(t, testModel(t, board{}), "a")
	m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("  one\ntwo  ")})
	m = press(t, m, tea.KeyEnter)

	assertSaved(t, m, board{toDo: {"one two"}})
}

func TestEscCancelsAdd(t *testing.T) {
	m := press(t, testModel(t, board{}), "a", "oops", tea.KeyEsc)

	assertBoardsEqual(t, m.tasks, board{})
	if m.adding || m.input != "" {
		t.Errorf("expected input to be closed and empty, got adding=%v input=%q", m.adding, m.input)
	}
}

func TestBackspaceRemovesWholeCharacter(t *testing.T) {
	m := press(t, testModel(t, board{}), "a", "ab日", tea.KeyBackspace)
	if m.input != "ab" {
		t.Errorf("input = %q, want %q", m.input, "ab")
	}
}

func TestBackspaceOnEmptyInputDoesNothing(t *testing.T) {
	m := press(t, testModel(t, board{}), "a", tea.KeyBackspace)
	if m.input != "" {
		t.Errorf("input = %q, want empty", m.input)
	}
}

func TestCommandKeysAreTypedAsTextWhileAdding(t *testing.T) {
	m := press(t, testModel(t, board{}), "a", "dqLH1", tea.KeyEnter)

	assertBoardsEqual(t, m.tasks, board{toDo: {"dqLH1"}})
}

func TestDeleteOnlyAffectsActiveColumn(t *testing.T) {
	m := testModel(t, board{toDo: {"one"}, inProgress: {"two", "three"}})
	m = press(t, m, "2", tea.KeyDown, "d")

	assertSaved(t, m, board{toDo: {"one"}, inProgress: {"two"}})
	if m.cursors[inProgress] != 0 {
		t.Errorf("cursor = %d, want 0", m.cursors[inProgress])
	}
}

func TestDeleteOnEmptyColumnDoesNothing(t *testing.T) {
	m := press(t, testModel(t, board{toDo: {"one"}}), "2", "d")

	assertBoardsEqual(t, m.tasks, board{toDo: {"one"}})
}

func TestMoveTaskAcrossBoard(t *testing.T) {
	m := testModel(t, board{toDo: {"one", "two"}, inProgress: {"three"}})

	m = press(t, m, "L")
	assertSaved(t, m, board{toDo: {"two"}, inProgress: {"three", "one"}})
	if m.active != inProgress || m.cursors[inProgress] != 1 {
		t.Errorf("selection did not follow the task: active=%d cursor=%d", m.active, m.cursors[inProgress])
	}

	m = press(t, m, "L")
	assertSaved(t, m, board{toDo: {"two"}, inProgress: {"three"}, completed: {"one"}})

	m = press(t, m, "H")
	assertSaved(t, m, board{toDo: {"two"}, inProgress: {"three", "one"}})
}

func TestMoveWithShiftArrows(t *testing.T) {
	m := testModel(t, board{toDo: {"one"}})

	m = press(t, m, tea.KeyShiftRight)
	assertSaved(t, m, board{inProgress: {"one"}})

	m = press(t, m, tea.KeyShiftLeft)
	assertSaved(t, m, board{toDo: {"one"}})
}

func TestSpaceCyclesTaskThroughEveryStage(t *testing.T) {
	m := testModel(t, board{toDo: {"one", "two"}})

	m = press(t, m, " ")
	assertSaved(t, m, board{toDo: {"two"}, inProgress: {"one"}})

	m = press(t, m, " ")
	assertSaved(t, m, board{toDo: {"two"}, completed: {"one"}})

	m = press(t, m, " ")
	assertSaved(t, m, board{toDo: {"two", "one"}})
	if m.active != toDo || m.cursors[toDo] != 1 {
		t.Errorf("selection did not follow the task: active=%d cursor=%d", m.active, m.cursors[toDo])
	}
}

func TestSpaceOnEmptyColumnDoesNothing(t *testing.T) {
	m := press(t, testModel(t, board{toDo: {"one"}}), "2", " ")

	assertBoardsEqual(t, m.tasks, board{toDo: {"one"}})
	if m.active != inProgress {
		t.Errorf("active = %d, want %d", m.active, inProgress)
	}
}

func TestSpaceIsTypedAsTextWhileAdding(t *testing.T) {
	m := press(t, testModel(t, board{toDo: {"one"}}), "a", "two words", tea.KeyEnter)

	assertBoardsEqual(t, m.tasks, board{toDo: {"one", "two words"}})
}

func TestMovePastEdgesDoesNothing(t *testing.T) {
	m := press(t, testModel(t, board{toDo: {"one"}, completed: {"two"}}), "H", "3", "L")

	assertBoardsEqual(t, m.tasks, board{toDo: {"one"}, completed: {"two"}})
}

func TestMoveFromEmptyColumnDoesNothing(t *testing.T) {
	m := press(t, testModel(t, board{toDo: {"one"}}), "2", "L")

	assertBoardsEqual(t, m.tasks, board{toDo: {"one"}})
	if m.active != inProgress {
		t.Errorf("active = %d, want %d", m.active, inProgress)
	}
}

func TestMoveLastTaskKeepsSourceCursorValid(t *testing.T) {
	m := press(t, testModel(t, board{toDo: {"one", "two"}}), tea.KeyDown, "L")

	if m.cursors[toDo] != 0 {
		t.Errorf("cursor = %d, want 0", m.cursors[toDo])
	}
}

func TestColumnNavigationWrapsAround(t *testing.T) {
	m := testModel(t, board{})

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
	m := testModel(t, board{toDo: {"a", "b"}, inProgress: {"c"}})
	m = press(t, m, tea.KeyDown, tea.KeyRight, tea.KeyLeft)

	if m.cursors[toDo] != 1 {
		t.Errorf("cursor = %d, want 1", m.cursors[toDo])
	}
}

func TestCursorStaysInsideColumn(t *testing.T) {
	m := testModel(t, board{toDo: {"one", "two"}})

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
	m := testModel(t, board{})

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
