package tui

import (
	"github.com/vishalcoderathore/lazydone/internal/board"
)

// notesEdited is sent when the editor opened by E closes, with the notes it
// left for the task, or why it couldn't be used.
type notesEdited struct {
	id    string
	notes string
	err   error
}

// saveNotes saves the notes the editor left for a task. If the editor failed,
// nothing is saved, because the notes may be half written.
func (m model) saveNotes(msg notesEdited) model {
	if msg.err != nil {
		m.status = "Could not edit notes: " + msg.err.Error()
		return m
	}
	// The board isn't checked for outside changes while the editor is open, so
	// check now, before deciding whether the task still exists.
	m = m.refresh()
	if _, _, ok := m.tasks.Find(msg.id); !ok {
		m.status = "That task was removed elsewhere"
		return m
	}
	now := m.now()
	m, ok := m.change(msg.id, func(b board.Board) (board.Board, bool) {
		return b.EditNotes(msg.id, msg.notes, now)
	})
	if ok {
		m.status = "Notes updated, press u to undo"
	}
	return m
}
