package tui

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vishalcoderathore/lazydone/internal/board"
)

// This file holds the ways of looking at the board: the Today view, the tag
// filter, sorting, the archive view and search. None of them changes a task.
// Each column shows its visible tasks, and the column cursors point into those.

// visible is what a column shows: the tasks that the current view lets
// through, in the current sort order.
func (m model) visible(column int) []board.Task {
	now := m.now()
	var shown []board.Task
	for _, item := range m.tasks[column] {
		if item.Archived == m.archive &&
			(!m.today || board.InToday(column, item, now)) &&
			(m.tag == "" || slices.Contains(item.Tags, m.tag)) {
			shown = append(shown, item)
		}
	}
	return board.Sorted(shown, m.sortBy)
}

// selected is the selected task, if the active column shows any. The cursor
// may be past the end just after a change has removed a task.
func (m model) selected() (board.Task, bool) {
	if shown, cursor := m.visible(m.active), m.cursors[m.active]; cursor < len(shown) {
		return shown[cursor], true
	}
	return board.Task{}, false
}

// selectedID is the ID of the selected task, or empty if there is none.
func (m model) selectedID() string {
	item, _ := m.selected()
	return item.ID
}

// selectTask focuses the column showing a task and puts the cursor on it. It
// returns false if the current view hides the task.
func (m model) selectTask(id string) (model, bool) {
	for column := range board.NumColumns {
		if index := slices.IndexFunc(m.visible(column), func(item board.Task) bool { return item.ID == id }); index >= 0 {
			m.active, m.cursors[column] = column, index
			return m, true
		}
	}
	return m, false
}

// viewNote describes the active view, such as "Today · +work · by due date",
// or is empty for the plain board.
func (m model) viewNote() string {
	var parts []string
	if m.archive {
		parts = append(parts, "Archived")
	}
	if m.today {
		parts = append(parts, "Today")
	}
	if m.tag != "" {
		parts = append(parts, m.tag)
	}
	if m.sortBy != board.ByManual {
		parts = append(parts, "by "+m.sortBy.String())
	}
	return strings.Join(parts, " · ")
}

// reshow keeps the task that was selected before the view changed selected,
// if the new view still shows it, and every cursor on a task.
func (m model) reshow(selected string) model {
	m = m.show(m.tasks)
	m, _ = m.selectTask(selected)
	return m
}

func (m model) toggleToday() model {
	selected := m.selectedID()
	m.today = !m.today
	m.status = "Showing all tasks"
	if m.today {
		m.status = "Today: tasks due today or overdue, and tasks done today"
	}
	return m.reshow(selected)
}

func (m model) cycleSort() model {
	selected := m.selectedID()
	m.sortBy = m.sortBy.Next()
	m.status = "Sorted by " + m.sortBy.String()
	return m.reshow(selected)
}

// toggleArchive switches between the board and the archived tasks, which are
// all completed.
func (m model) toggleArchive() model {
	selected := m.selectedID()
	m.archive = !m.archive
	m.status = "Showing the board"
	if m.archive {
		m.active = board.Completed
		m.status = "Showing archived tasks. A restores one, z goes back"
	}
	return m.reshow(selected)
}

// archiveSelected archives the selected completed task, or restores it in the
// archive view.
func (m model) archiveSelected() model {
	id, now := m.selectedID(), m.now()
	if m.archive {
		m, ok := m.change(id, func(b board.Board) (board.Board, bool) { return b.Restore(id, now) })
		if ok {
			m.status = "Task restored to Completed"
		}
		return m
	}
	if id != "" && m.active != board.Completed {
		m.status = "Only completed tasks can be archived"
		return m
	}
	m, ok := m.change(id, func(b board.Board) (board.Board, bool) { return b.Archive(id, now) })
	if ok {
		m.status = "Task archived, press u to undo or z to see archived tasks"
	}
	return m
}

// editsArchived reports whether a key would change an archived task in a way
// that only makes sense on the board, which the archive view doesn't allow.
func (m model) editsArchived(k tea.KeyMsg) bool {
	return m.archive && key.Matches(k, keys.Add, keys.Edit, keys.Done, keys.Priority, keys.NextStage,
		keys.MoveLeft, keys.MoveRight, keys.MoveDown, keys.MoveUp)
}

// tagChoices are the tags the picker offers: those on any task in the board or
// archive, whichever is showing, ignoring other filters.
func (m model) tagChoices() []string {
	var tags []string
	for _, list := range m.tasks {
		for _, item := range list {
			if item.Archived == m.archive {
				tags = append(tags, item.Tags...)
			}
		}
	}
	slices.Sort(tags)
	return slices.Compact(tags)
}

// openPicker opens the tag picker, with the current filter chosen. The first
// choice, All tags, turns the filter off.
func (m model) openPicker() model {
	tags := m.tagChoices()
	if len(tags) == 0 {
		m.status = "No tags yet. Add one by typing +word or @word in a task"
		return m
	}
	m.picking, m.pick = true, slices.Index(tags, m.tag)+1
	return m
}

// updatePicker handles keys while the tag picker is open.
func (m model) updatePicker(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	tags := m.tagChoices()
	m.pick = min(m.pick, len(tags)) // a refresh may have taken tags away
	switch {
	case k.Type == tea.KeyEnter:
		selected := m.selectedID()
		m.picking, m.tag = false, ""
		m.status = "Showing all tags"
		if m.pick > 0 && m.pick <= len(tags) {
			m.tag = tags[m.pick-1]
			m.status = "Showing tasks tagged " + m.tag
		}
		return m.reshow(selected), nil
	case k.Type == tea.KeyEsc, key.Matches(k, keys.Quit), key.Matches(k, keys.Filter):
		m.picking = false
	case key.Matches(k, keys.NextTask):
		m.pick = min(m.pick+1, len(tags))
	case key.Matches(k, keys.PreviousTask):
		m.pick = max(m.pick-1, 0)
	}
	return m, nil
}

// viewPicker draws the tag picker in the middle of the screen.
func (m model) viewPicker() string {
	choices := append([]string{"All tags"}, m.tagChoices()...)
	pick := min(m.pick, len(choices)-1)           // a refresh may have taken tags away
	rows := max(1, min(len(choices), m.height-2)) // at least one row, even before the first resize
	first := max(0, pick-rows+1)
	var lines []string
	for i, choice := range choices[first : first+rows] {
		if first+i == pick {
			lines = append(lines, selectedStyle.Render("❯ "+choice))
		} else {
			lines = append(lines, textStyle.Render("  "+choice))
		}
	}
	popup := panel("Filter by tag", "enter: choose", lines, min(m.width, 36), rows+2, true, inputBorder)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup)
}

// openSearch opens the input for a search across the visible tasks.
func (m model) openSearch() model {
	m.searching = true
	m.input.Reset()
	return m
}

// updateSearch handles keys while the search input is open. Enter jumps to
// the first match from the selected task on.
func (m model) updateSearch(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.Type {
	case tea.KeyEnter:
		m.query = strings.TrimSpace(m.input.Value())
		m.searching = false
		m.input.Reset()
		if m.query == "" {
			return m, nil
		}
		return m.jumpToMatch(1, true), nil
	case tea.KeyEsc:
		m.searching = false
		m.input.Reset()
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(k)
	return m, cmd
}

// jumpToMatch selects the next (1) or previous (-1) visible task that matches
// the search, going through the columns in order and wrapping around. With
// fromHere, the selected task itself counts if it matches.
func (m model) jumpToMatch(direction int, fromHere bool) model {
	if m.query == "" {
		m.status = "Press / to search"
		return m
	}
	type place struct {
		column, index int
		match         bool
	}
	var places []place
	here, total := -1, 0
	for column := range board.NumColumns {
		for index, item := range m.visible(column) {
			if column == m.active && index == m.cursors[column] {
				here = len(places)
			}
			match := matchesSearch(item, m.query)
			places = append(places, place{column, index, match})
			total += boolInt(match)
		}
	}
	if total == 0 {
		m.status = fmt.Sprintf("No match for %q", m.query)
		return m
	}

	start := here // with nothing selected, start just outside the list
	if here < 0 {
		start = map[int]int{1: -1, -1: len(places)}[direction]
	} else if fromHere {
		start -= direction
	}
	for n := 1; n <= len(places); n++ {
		i := ((start+direction*n)%len(places) + len(places)) % len(places)
		if at := places[i]; at.match {
			number := 0
			for _, before := range places[:i+1] {
				number += boolInt(before.match)
			}
			m.active, m.cursors[at.column] = at.column, at.index
			m.status = fmt.Sprintf("Match %d of %d for %q, n / N for next / previous", number, total, m.query)
			return m
		}
	}
	return m
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// matchesSearch reports whether a task matches a search: the search's letters
// appear in order, though not necessarily together, in the task's text, tags
// or notes, ignoring case. So "mlk" finds "buy milk".
func matchesSearch(item board.Task, query string) bool {
	haystack := strings.ToLower(strings.Join(append([]string{item.Text, item.Notes}, item.Tags...), " "))
	for _, r := range strings.ToLower(query) {
		if r == ' ' {
			continue
		}
		i := strings.IndexRune(haystack, r)
		if i < 0 {
			return false
		}
		haystack = haystack[i+utf8.RuneLen(r):]
	}
	return true
}
