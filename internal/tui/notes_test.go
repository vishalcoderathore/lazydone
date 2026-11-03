package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vishalcoderathore/lazydone/internal/board"
)

// These tests cover E, which edits a task's notes in the user's editor. The
// editor itself is covered in editor_test.go; here the model is sent what the
// editor left behind when it closed.

func notesTasks() board.Board {
	return board.Board{board.ToDo: {{ID: "a", Text: "pay rent"}, {ID: "b", Text: "call bank", Notes: "ask about fees"}}}
}

func TestEditedNotesAreSaved(t *testing.T) {
	m := send(boardModel(t, notesTasks()), notesEdited{id: "a", notes: "landlord is Sam"})

	if got := reload(t, m)[board.ToDo][0].Notes; got != "landlord is Sam" {
		t.Errorf("saved notes = %q, want the edited notes", got)
	}
	assertDetailsContain(t, m, "landlord is Sam")
}

func TestEditedNotesCanBeUndone(t *testing.T) {
	m := send(boardModel(t, notesTasks()), notesEdited{id: "b", notes: "fees waived"})
	assertViewContains(t, m, "Notes updated")

	m = press(t, m, "u")
	if got := reload(t, m)[board.ToDo][1].Notes; got != "ask about fees" {
		t.Errorf("after undo, saved notes = %q, want the old notes back", got)
	}
}

func TestClosingTheEditorWithoutChangesChangesNothing(t *testing.T) {
	// Editors end a file with a newline, which isn't part of the notes.
	m := send(boardModel(t, notesTasks()), notesEdited{id: "b", notes: "ask about fees\n"})

	assertViewOmits(t, m, "Notes updated")
	got := reload(t, m)[board.ToDo][1]
	if got.Notes != "ask about fees" || !got.Updated.IsZero() {
		t.Errorf("saved %q, updated %v; want the task untouched", got.Notes, got.Updated)
	}
}

func TestEditedNotesLoseTrailingBlankLines(t *testing.T) {
	m := send(boardModel(t, notesTasks()), notesEdited{id: "a", notes: "first\n\nsecond  \n\n\n"})

	if got := reload(t, m)[board.ToDo][0].Notes; got != "first\n\nsecond" {
		t.Errorf("saved notes = %q, want blank lines kept inside but not at the end", got)
	}
}

func TestAFailedEditorSaysWhyAndSavesNothing(t *testing.T) {
	m := send(boardModel(t, notesTasks()), notesEdited{id: "b", notes: "half typed", err: errors.New("exit status 1")})

	assertViewContains(t, m, "Could not edit notes: exit status 1")
	if got := reload(t, m)[board.ToDo][1].Notes; got != "ask about fees" {
		t.Errorf("saved notes = %q, want them left alone", got)
	}
}

func TestNotesForATaskRemovedElsewhereSaySo(t *testing.T) {
	// Another process deleted the task while its notes were open in the editor.
	m := send(boardModel(t, notesTasks()), notesEdited{id: "gone", notes: "lost words"})

	assertViewContains(t, m, "That task was removed elsewhere")
}

func TestNotesForATaskDeletedWhileTheEditorWasOpenSaySo(t *testing.T) {
	// The board still shows the task, because it isn't checked for outside
	// changes while the editor is open.
	m := boardModel(t, notesTasks())
	deleteElsewhere(t, m, "call bank")

	m = send(m, notesEdited{id: "b", notes: "fees waived"})
	assertViewContains(t, m, "That task was removed elsewhere")

	m = press(t, m, "u")
	if _, _, ok := reload(t, m).Find("b"); ok {
		t.Error("after u, the deleted task is back, want it to stay deleted")
	}
}

// withEditor stands in for the user's editor, which needs a real terminal. It
// records the notes it was opened on, and closes having written written.
func withEditor(m model, written string, opened *string) model {
	m.editor = func(id, notes string) tea.Cmd {
		*opened = notes
		return func() tea.Msg { return notesEdited{id: id, notes: written} }
	}
	return m
}

func TestEOpensTheSelectedTasksNotesInTheEditor(t *testing.T) {
	var opened string
	m := press(t, withEditor(boardModel(t, notesTasks()), "fees waived", &opened), "j")

	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	if cmd == nil {
		t.Fatal("E should open the editor")
	}
	m = send(m, cmd())

	if opened != "ask about fees" {
		t.Errorf("the editor opened on %q, want the selected task's notes", opened)
	}
	if got := reload(t, m)[board.ToDo][1].Notes; got != "fees waived" {
		t.Errorf("saved notes = %q, want what the editor left", got)
	}
}

func TestEDoesNotOpenTheEditorForAnArchivedTask(t *testing.T) {
	var opened string
	m := press(t, withEditor(boardModel(t, archiveTasks()), "new", &opened), "z")

	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	if cmd != nil {
		t.Error("E should not open the editor in the archive view")
	}
	assertViewContains(t, m, "Archived tasks can't be changed")
}

func TestEWithNothingSelectedDoesNothing(t *testing.T) {
	var opened string
	m := withEditor(testModel(t, texts{}), "new", &opened)

	if _, cmd := update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}}); cmd != nil {
		t.Error("E with no task selected should not open the editor")
	}
}
