package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

// readWidth is the widest the popup that shows a whole task gets.
const readWidth = 72

// viewTask draws the selected task in full in a popup, notes and all, for
// when the details panel is too small to show it. Lines that don't fit scroll.
func (m model) viewTask() string {
	width, lines := m.readLines()
	rows := m.readRows(len(lines))
	// The task may have got shorter since it was scrolled, as after E.
	first := min(m.readScroll, max(len(lines)-rows, 0))
	title := "Task"
	if len(lines) > rows {
		title += fmt.Sprintf(" %d-%d of %d", first+1, first+rows, len(lines))
	}
	popup := panel(title, "j/k: scroll · E: edit notes · esc: close", styled(lines[first:min(first+rows, len(lines))]),
		width, rows+2, true, inputBorder)
	if !m.hasMessage() {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup)
	}
	// The popup covers the footer, so what E did, or a failed save, shows
	// under it.
	popup = lipgloss.Place(m.width, m.height-1, lipgloss.Center, lipgloss.Center, popup)
	return lipgloss.JoinVertical(lipgloss.Left, popup, m.viewFooter())
}

// hasMessage reports whether the footer has a status or an error to show,
// rather than key hints.
func (m model) hasMessage() bool {
	return m.err != nil || m.status != ""
}

// readLines is the width of the popup that shows a whole task, and the
// task's lines wrapped to fit inside it.
func (m model) readLines() (width int, lines []string) {
	width = min(m.width, readWidth)
	return width, m.taskLines(width - 2)
}

// readRows is how many of a task's lines the popup shows at once.
func (m model) readRows(lines int) int {
	height := m.height - 2
	if m.hasMessage() {
		height-- // the line under the popup
	}
	return max(1, min(lines, height))
}

// updateReading handles keys while a whole task is shown: j and k scroll, E
// edits the notes, r retries a failed save, and keys that close the popup
// close it. Other board keys do nothing, so they can't change a task that
// isn't in view. As on the board, a key clears the last status.
func (m model) updateReading(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.status = ""
	_, lines := m.readLines()
	last := max(len(lines)-m.readRows(len(lines)), 0)
	switch {
	case key.Matches(k, keys.Read, keys.Quit) || k.Type == tea.KeyEsc:
		m.reading = false
	case key.Matches(k, keys.NextTask):
		m.readScroll = min(m.readScroll+1, last)
	case key.Matches(k, keys.PreviousTask):
		m.readScroll = max(min(m.readScroll, last)-1, 0) // from where it's shown
	case key.Matches(k, keys.EditNotes, keys.Retry):
		// As on the board, which also stops E for an archived task. The
		// popup stays open, to show the new notes.
		return m.updateBoard(k)
	}
	return m, nil
}

// keepReading keeps the task shown in full selected after the board changed
// elsewhere, as a refresh may have moved it. If it's gone, or the view now
// hides it, the popup closes and says so, rather than show another task.
func (m model) keepReading() model {
	if !m.reading {
		return m
	}
	if next, shown := m.selectTask(m.readID); shown {
		return next
	}
	m.reading = false
	if _, _, ok := m.tasks.Find(m.readID); !ok {
		m.status = "That task was removed elsewhere"
	} else {
		m.status = "That task was changed elsewhere and is no longer shown"
	}
	return m
}
