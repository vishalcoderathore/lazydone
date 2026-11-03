package tui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

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

// searchWidth is the widest the search popup gets, and searchRows the most
// matches it lists at once.
const (
	searchWidth = 72
	searchRows  = 10
)

// searchMatch is a visible task that matches the search, and where.
type searchMatch struct {
	column, index int // where the task is shown
	item          board.Task
	positions     []int // matched runes in searchText(item)
	score         int
}

// searchText is what a search looks through: a task's text, then its tags,
// then its notes. Rows in the search popup show its start, up to the notes.
func searchText(item board.Task) string {
	parts := append([]string{item.Text}, item.Tags...)
	if item.Notes != "" {
		parts = append(parts, item.Notes)
	}
	return strings.Join(parts, " ")
}

// searchMatches lists the visible tasks that match query, best first. Of two
// equal matches, the one that starts earlier wins, so a match in a task's text
// beats one in its notes. Otherwise they keep board order.
func (m model) searchMatches(query string) []searchMatch {
	var found []searchMatch
	for column := range board.NumColumns {
		for index, item := range m.visible(column) {
			if positions, score, ok := fuzzyMatch(searchText(item), query); ok {
				found = append(found, searchMatch{column, index, item, positions, score})
			}
		}
	}
	slices.SortStableFunc(found, func(a, b searchMatch) int {
		return cmp.Or(b.score-a.score, a.positions[0]-b.positions[0])
	})
	return found
}

// searchPopupWidth is how wide the search popup is on this screen.
func (m model) searchPopupWidth() int {
	return min(m.width, searchWidth)
}

// inputWidth is how much text the input shows at once: the search popup's
// width while searching, or the details panel's otherwise. Both leave room for
// the borders and the cursor at the end, and the popup for its "/ " prompt.
func (m model) inputWidth() int {
	if m.searching {
		return m.searchPopupWidth() - 5
	}
	return m.width - 3
}

// openSearch opens the search popup for a search across the visible tasks.
func (m model) openSearch() model {
	m.searching = true
	m.input.Reset()
	m.input.Width = m.inputWidth()
	return m
}

// closeSearch closes the search popup, giving the input back to the details
// panel.
func (m model) closeSearch() model {
	m.searching, m.found = false, 0
	m.input.Reset()
	m.input.Width = m.inputWidth()
	return m
}

// updateSearch handles keys while the search popup is open. Typing narrows
// the matches, up and down (or ctrl+p and ctrl+n) choose one, and enter jumps
// to it. n and N then go through the matches in board order.
func (m model) updateSearch(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	query := strings.TrimSpace(m.input.Value())
	switch k.Type {
	case tea.KeyEnter:
		matches, found := m.searchMatches(query), m.found
		m = m.closeSearch()
		m.query = query
		if query == "" {
			return m, nil
		}
		if len(matches) > 0 {
			chosen := matches[min(found, len(matches)-1)]
			m.active, m.cursors[chosen.column] = chosen.column, chosen.index
		}
		return m.jumpToMatch(1, true), nil // says which match this is, or that none matched
	case tea.KeyEsc:
		return m.closeSearch(), nil
	case tea.KeyUp, tea.KeyCtrlP:
		m.found = max(m.found-1, 0)
		return m, nil
	case tea.KeyDown, tea.KeyCtrlN:
		m.found = max(0, min(m.found+1, len(m.searchMatches(query))-1))
		return m, nil
	}
	before := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(k)
	if m.input.Value() != before {
		m.found = 0 // the best match of the new search
	}
	return m, cmd
}

// viewSearch draws the search popup in the middle of the screen, like the tag
// picker: the input, a count, and the matches so far, best first.
func (m model) viewSearch() string {
	query := strings.TrimSpace(m.input.Value())
	matches := m.searchMatches(query)
	count := ""
	switch {
	case query == "":
	case len(matches) == 0:
		count = "No match"
	case len(matches) == 1:
		count = "1 match"
	default:
		count = fmt.Sprintf("%d matches", len(matches))
	}
	lines := []string{selectedStyle.Render("/ ") + m.input.View(), faintStyle.Render("  " + count)}

	rows := max(0, min(len(matches), searchRows, m.height-2-len(lines)))
	chosen := min(m.found, len(matches)-1)
	first := max(0, chosen-rows+1)
	for i, match := range matches[first : first+rows] {
		lines = append(lines, searchRow(match, first+i == chosen))
	}
	popup := panel("Search text, tags and notes", "↑↓: choose · enter: jump · esc: cancel", lines,
		m.searchPopupWidth(), len(lines)+2, true, inputBorder)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup)
}

// searchRow draws one match in the search popup: its column's bullet, then
// its text and tags with the matched letters picked out. A match in the notes,
// which the row leaves out, is said instead.
func searchRow(match searchMatch, chosen bool) string {
	style := textStyle
	prefix := "  "
	if chosen {
		style, prefix = selectedStyle, "❯ "
	}
	hit := style.Foreground(colorPeach).Bold(true).Underline(true)

	shown := []rune(strings.Join(append([]string{match.item.Text}, match.item.Tags...), " "))
	matched := make([]bool, len(shown))
	inNotes := false
	for _, at := range match.positions {
		if at < len(shown) {
			matched[at] = true
		} else {
			inNotes = true
		}
	}

	var s strings.Builder
	s.WriteString(style.Render(prefix) + columns[match.column].style.Render(columns[match.column].bullet) + style.Render(" "))
	for start := 0; start < len(shown); {
		end := start
		for end < len(shown) && matched[end] == matched[start] {
			end++
		}
		if matched[start] {
			s.WriteString(hit.Render(string(shown[start:end])))
		} else {
			s.WriteString(style.Render(string(shown[start:end])))
		}
		start = end
	}
	if inNotes {
		s.WriteString(faintStyle.Render(" · in notes"))
	}
	return s.String()
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
	_, _, ok := fuzzyMatch(searchText(item), query)
	return ok
}
