package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vishalcoderathore/lazydone/internal/board"
	"github.com/vishalcoderathore/lazydone/internal/store"
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

func longNotesTasks() board.Board {
	return board.Board{board.ToDo: {{ID: "a", Text: "call landlord", Tags: []string{"+home"},
		Notes: "landlord is Sam\nref 123\nflat 4B, buzzer broken\nask about the boiler\ndeposit due on the 1st"}}}
}

func TestEnterShowsTheWholeTask(t *testing.T) {
	m := press(t, boardModel(t, longNotesTasks()), tea.KeyEnter)

	for _, want := range []string{"call landlord", "Tags: +home", "landlord is Sam", "ref 123", "flat 4B, buzzer broken", "ask about the boiler", "deposit due on the 1st"} {
		assertViewContains(t, m, want)
	}
	assertViewContains(t, m, "j/k: scroll · E: edit notes · esc: close")
}

func TestTheWholeTaskClosesWithoutActingOnIt(t *testing.T) {
	for _, closer := range []any{tea.KeyEsc, "q", tea.KeyEnter} {
		m := press(t, boardModel(t, longNotesTasks()), tea.KeyEnter, "x", "dd", closer)

		assertViewOmits(t, m, "ref 123") // back to the board, whose details panel is too small for it
		assertViewContains(t, m, "To Do")
		assertSaved(t, m, texts{board.ToDo: {"call landlord"}})
	}
}

func TestLongNotesScrollWithJAndK(t *testing.T) {
	var notes []string
	for i := 1; i <= 20; i++ {
		notes = append(notes, fmt.Sprintf("note line %02d", i))
	}
	tasks := board.Board{board.ToDo: {{ID: "a", Text: "long", Notes: strings.Join(notes, "\n")}}}
	m := press(t, sized(boardModel(t, tasks), 80, minHeight), tea.KeyEnter)
	assertViewContains(t, m, "note line 01")
	assertViewOmits(t, m, "note line 20")

	m = press(t, m, strings.Repeat("j", 30)) // more than enough, and it stops at the end
	assertViewContains(t, m, "note line 20")
	assertViewOmits(t, m, "note line 01")

	m = press(t, m, strings.Repeat("k", 30))
	assertViewContains(t, m, "note line 01")
}

func TestEInTheWholeTaskEditsItsNotes(t *testing.T) {
	var opened string
	m := press(t, withEditor(boardModel(t, longNotesTasks()), "rent is paid", &opened), tea.KeyEnter)

	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	if cmd == nil {
		t.Fatal("E should open the editor")
	}
	m = send(m, cmd())

	if !strings.HasPrefix(opened, "landlord is Sam") {
		t.Errorf("the editor opened on %q, want the task's notes", opened)
	}
	assertViewContains(t, m, "rent is paid") // still showing the whole task, with its new notes
	assertViewOmits(t, m, "ref 123")
}

func TestTheWholeArchivedTaskCanBeReadButNotEdited(t *testing.T) {
	var opened string
	tasks := board.Board{board.Completed: {{ID: "a", Text: "old", Notes: "kept for the record", Archived: true}}}
	m := press(t, withEditor(boardModel(t, tasks), "changed", &opened), "z", tea.KeyEnter)
	assertViewContains(t, m, "kept for the record")

	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	if cmd != nil {
		t.Error("E should not open the editor for an archived task")
	}
	assertViewContains(t, m, "Archived tasks can't be changed")
	assertViewContains(t, m, "kept for the record") // still showing the task
}

func TestShorterNotesAfterScrollingStillShow(t *testing.T) {
	var opened string
	var notes []string
	for i := 1; i <= 20; i++ {
		notes = append(notes, fmt.Sprintf("note line %02d", i))
	}
	tasks := board.Board{board.ToDo: {{ID: "a", Text: "long", Notes: strings.Join(notes, "\n")}}}
	m := press(t, sized(withEditor(boardModel(t, tasks), "short now", &opened), 80, minHeight), tea.KeyEnter, strings.Repeat("j", 30))

	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	m = send(m, cmd())

	assertViewContains(t, m, "short now")
}

func TestTheWholeTaskClosesIfRemovedElsewhere(t *testing.T) {
	m := press(t, boardModel(t, notesTasks()), "j", tea.KeyEnter)
	deleteElsewhere(t, m, "call bank")
	m = send(m, fileCheck{})

	assertViewContains(t, m, "That task was removed elsewhere")
	assertViewContains(t, m, "To Do") // back on the board, not showing another task in its place
	if m.reading {
		t.Error("the popup should close")
	}
}

func TestTheDetailsPanelSaysWhenThereIsMoreToShow(t *testing.T) {
	assertViewContains(t, boardModel(t, longNotesTasks()), "enter: show all")
	assertViewOmits(t, boardModel(t, board.Board{board.ToDo: {{ID: "a", Text: "short"}}}), "enter: show all")
}

func TestTheWholeTaskClosesIfRemovedWhileItsNotesWereInTheEditor(t *testing.T) {
	var opened string
	m := press(t, withEditor(boardModel(t, notesTasks()), "fees waived", &opened), "j", tea.KeyEnter)

	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	deleteElsewhere(t, m, "call bank") // while the editor is open
	m = send(m, cmd())

	if m.reading {
		t.Error("the popup should close, rather than show another task")
	}
	assertViewContains(t, m, "That task was removed elsewhere")
}

func TestTheWholeTaskShowsWhatEDid(t *testing.T) {
	m := press(t, boardModel(t, notesTasks()), "j", tea.KeyEnter)

	m = send(m, notesEdited{id: "b", err: errors.New("exit status 1")})
	assertViewContains(t, m, "Could not edit notes: exit status 1")
	assertViewContains(t, m, "ask about fees") // still showing the task

	m = press(t, m, "j")
	assertViewOmits(t, m, "Could not edit notes")
}

func TestTheWholeTaskShowsAFailedSaveAndRetries(t *testing.T) {
	m := press(t, boardModel(t, notesTasks()), "j", tea.KeyEnter)
	good := m.store
	m.store = store.File{Path: t.TempDir()} // a directory can't be written to as a file

	m = send(m, notesEdited{id: "b", notes: "fees waived"})
	assertViewContains(t, m, "Could not save")

	m.store = good
	m = press(t, m, "r")
	if got := reload(t, m)[board.ToDo][1].Notes; got != "fees waived" {
		t.Errorf("after r, saved notes = %q, want the edited notes", got)
	}
	if !m.reading {
		t.Error("r should keep the popup open")
	}
}
