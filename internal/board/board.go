// Package board holds the tasks of the kanban board and every rule for changing
// them. It knows nothing about the terminal or the file format.
//
// A Board is a value. Every change returns a new Board and leaves the old one as
// it was, so that a caller can keep an old Board without it changing underneath.
package board

import (
	"crypto/rand"
	"slices"
	"strings"
	"time"
	"unicode"
)

// The columns of the board, from left to right. A task's status is the column
// that holds it.
const (
	ToDo = iota
	InProgress
	Completed
	NumColumns
)

// Task is one item on the board. Its place in its column is its manual order.
type Task struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	Priority  string    `json:"priority,omitempty"`
	Due       time.Time `json:"due,omitzero"`
	Tags      []string  `json:"tags,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	Archived  bool      `json:"archived,omitempty"`
	Created   time.Time `json:"created,omitzero"`
	Updated   time.Time `json:"updated,omitzero"`
	Completed time.Time `json:"completed,omitzero"`
}

// NewTask returns a task created at the given time, with a new ID.
func NewTask(text string, now time.Time) Task {
	return Task{ID: rand.Text(), Text: text, Created: now, Updated: now}
}

// Board holds the tasks of each column.
type Board [NumColumns][]Task

// Statuses name the columns, in the data file and on the command line.
var Statuses = [NumColumns]string{"todo", "doing", "done"}

// Priorities are the priority levels, in the order that CyclePriority steps
// through them.
var Priorities = []string{"", "low", "medium", "high"}

// CleanText trims a task and collapses its whitespace, so that a task always
// fits on one line on screen.
func CleanText(text string) string {
	return strings.Join(strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}), " ")
}

// Total is the number of tasks in every column.
func (b Board) Total() int {
	total := 0
	for _, list := range b {
		total += len(list)
	}
	return total
}

// Find returns the column and index of the task with the given ID.
func (b Board) Find(id string) (column, index int, ok bool) {
	for column, list := range b {
		if index := slices.IndexFunc(list, func(item Task) bool { return item.ID == id }); index >= 0 {
			return column, index, true
		}
	}
	return 0, 0, false
}

// Add puts a task at the end of To Do.
func (b Board) Add(item Task) Board {
	b[ToDo] = append(slices.Clip(b[ToDo]), item)
	return b
}

// update changes the task with the given ID in place, returning false if no
// task has that ID.
func (b Board) update(id string, change func(*Task)) (Board, bool) {
	column, index, ok := b.Find(id)
	if !ok {
		return b, false
	}
	b[column] = slices.Clone(b[column])
	change(&b[column][index])
	return b, true
}

// Edit changes the text of a task. Blank or unchanged text keeps the task as it
// was and returns false, because Delete is the way to remove a task.
func (b Board) Edit(id, text string, now time.Time) (Board, bool) {
	text = CleanText(text)
	if column, index, ok := b.Find(id); !ok || text == "" || text == b[column][index].Text {
		return b, false
	}
	return b.update(id, func(item *Task) { item.Text, item.Updated = text, now })
}

// Delete removes a task.
func (b Board) Delete(id string) (Board, bool) {
	column, index, ok := b.Find(id)
	if !ok {
		return b, false
	}
	b[column] = slices.Delete(slices.Clone(b[column]), index, index+1)
	return b, true
}

// Move puts a task at the end of the target column. A task that reaches
// Completed from another column gets a completed time, and a task that leaves
// Completed loses it. Moving to a column that doesn't exist returns false.
func (b Board) Move(id string, target int, now time.Time) (Board, bool) {
	column, index, ok := b.Find(id)
	if !ok || target < 0 || target >= NumColumns {
		return b, false
	}
	item := b[column][index]
	item.Updated = now
	if target != Completed {
		item.Completed = time.Time{}
	} else if column != Completed {
		item.Completed = now
	}
	b, _ = b.Delete(id)
	b[target] = append(slices.Clip(b[target]), item)
	return b, true
}

// CyclePriority gives a task the next priority, wrapping from high to none.
func (b Board) CyclePriority(id string, now time.Time) (Board, bool) {
	return b.update(id, func(item *Task) {
		item.Priority = Priorities[(slices.Index(Priorities, item.Priority)+1)%len(Priorities)]
		item.Updated = now
	})
}

// Reorder moves a task up (negative) or down (positive) within its column,
// which changes its manual order. Moving past either end returns false.
func (b Board) Reorder(id string, by int) (Board, bool) {
	column, index, ok := b.Find(id)
	target := index + by
	if !ok || target < 0 || target >= len(b[column]) {
		return b, false
	}
	b[column] = slices.Clone(b[column])
	b[column][index], b[column][target] = b[column][target], b[column][index]
	return b, true
}

// Change records how to undo one change to the board. Keeping just the
// affected task, rather than a copy of the whole board, means that undo can't
// erase anything else that has changed since, such as a task added by another
// process.
type Change struct {
	ID     string // the task that changed
	Before *Task  // the task as it was, or nil if the change created it
	Column int    // where the task was
	Index  int
}

// ChangeOf records the task with the given ID before a change to it. A task
// that doesn't exist yet is about to be created, so undoing removes it.
func (b Board) ChangeOf(id string) Change {
	column, index, ok := b.Find(id)
	if !ok {
		return Change{ID: id}
	}
	item := b[column][index]
	return Change{ID: id, Before: &item, Column: column, Index: index}
}

// Revert undoes a change. It removes the task the change affected and, if the
// task existed before, puts it back as it was, as near its old place as the
// column now allows.
func (b Board) Revert(c Change) Board {
	b, _ = b.Delete(c.ID)
	if c.Before == nil {
		return b
	}
	index := min(c.Index, len(b[c.Column]))
	b[c.Column] = slices.Insert(slices.Clone(b[c.Column]), index, *c.Before)
	return b
}
