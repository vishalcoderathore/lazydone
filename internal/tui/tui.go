// Package tui is the interactive kanban board: rendering and key handling.
package tui

import (
	"errors"
	"slices"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/vishalcoderathore/lazydone/internal/board"
	"github.com/vishalcoderathore/lazydone/internal/capture"
)

// model holds all of the app's state.
type model struct {
	store      Store                 // where the tasks are saved
	version    string                // shown under the startup logo
	tasks      board.Board           // the tasks in each column
	active     int                   // index of the focused column
	cursors    [board.NumColumns]int // index of the selected task among each column's visible tasks
	adding     bool                  // true while the text input is open
	editing    string                // ID of the task being edited, or empty when adding
	input      textinput.Model       // the text input
	added      bool                  // true after adding a task, until the next key press
	err        error                 // last save error, shown on screen
	status     string                // message about the last action, until the next key
	scroll     int                   // first line of the details panel that is showing
	help       bool                  // true while the help popup is open
	helpScroll int                   // first line of the help popup that is showing
	pending    string                // first key of a two-key command such as dd
	history    []board.Change        // recent changes, for undo
	unsaved    []action              // changes shown but not yet saved, because saving failed
	seen       time.Time             // when the data file last changed, as last checked
	watching   bool                  // whether to check the data file for outside changes

	today     bool                           // true in the Today view
	tag       string                         // only tasks with this tag are shown, unless empty
	archive   bool                           // true while showing archived tasks instead of the board
	sortBy    board.Sort                     // the order each column is shown in
	query     string                         // the last search, for n and N
	searching bool                           // true while the search popup is open
	found     int                            // the chosen row of the search popup's matches
	picking   bool                           // true while the tag picker is open
	pick      int                            // the chosen row of the tag picker; 0 is All tags
	width     int                            // terminal width
	height    int                            // terminal height
	now       func() time.Time               // the clock, replaceable in tests
	editor    func(id, notes string) tea.Cmd // opens a task's notes in the user's editor, replaceable in tests

	animate     bool       // whether effects are switched on
	ticking     bool       // true while animation frames are scheduled
	splash      bool       // true until the startup logo is dismissed
	splashFrame int        // how far the logo has been revealed
	particles   []particle // confetti currently on screen
	bar         float64    // share of the progress bar currently filled
}

// Store holds the board, and may be changed by other processes too. The TUI
// never touches files itself.
type Store interface {
	Load() (board.Board, error)
	// Update applies a change to the latest board and saves it, as one step
	// that other processes can't interleave with.
	Update(func(board.Board) board.Board) (board.Board, error)
	// ModTime is when the board last changed.
	ModTime() (time.Time, error)
}

// action is one change to the board, made by task ID, so that it can be
// applied to whatever the latest board holds. A task that has gone is skipped.
type action func(board.Board) board.Board

// Run opens the board full screen, with its effects switched on and the data
// file watched for outside changes, until the user quits. The version is shown
// under the startup logo.
func Run(store Store, tasks board.Board, version string) error {
	m := newModel(store, tasks).withAnimations()
	m.version = version
	m.watching = true
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func newModel(store Store, tasks board.Board) model {
	input := textinput.New()
	input.Prompt = ""
	input.TextStyle = textStyle
	input.Cursor.SetMode(cursor.CursorStatic) // a blink would redraw when idle
	input.Focus()
	return model{store: store, tasks: tasks, now: time.Now, input: input, editor: editNotes}
}

func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.ticking {
		cmds = append(cmds, tick())
	}
	if m.watching {
		cmds = append(cmds, watch())
	}
	return tea.Batch(cmds...)
}

// watchInterval is how often the data file is checked for changes made by
// other processes. shortcut: polling a file's modification time once a second
// is cheap and needs no dependency, at the cost of up to a second's delay.
const watchInterval = time.Second

// fileCheck asks the model to check the data file for outside changes.
type fileCheck struct{}

func watch() tea.Cmd {
	return tea.Tick(watchInterval, func(time.Time) tea.Msg { return fileCheck{} })
}

// Update is called by Bubble Tea on every event, such as a key press.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.Width = m.inputWidth()
		// A bigger screen has less to scroll through.
		m.scroll = min(m.scroll, max(len(m.detailsLines())-detailsRows, 0))
		m.helpScroll = min(m.helpScroll, max(len(helpBindings())-m.helpRows(), 0))
	case animTick:
		return m.stepAnimation()
	case notesEdited:
		return m.saveNotes(msg), nil
	case fileCheck:
		m = m.refresh()
		if m.watching {
			return m, watch()
		}
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		if m.splash {
			return m.updateSplash(msg)
		}
		if m.help {
			return m.updateHelp(msg)
		}
		if m.picking {
			return m.updatePicker(msg)
		}
		if m.searching {
			return m.updateSearch(msg)
		}
		if m.adding {
			updated, input := m.updateAdding(msg)
			next, animation := updated.startAnimation()
			return next, tea.Batch(input, animation)
		}
		return m.updateBoard(msg)
	default:
		if m.adding || m.searching { // such as the clipboard text that ctrl+v asked for
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

// updateBoard handles keys while browsing the board.
func (m model) updateBoard(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.added = false
	m.status = ""

	// A two-key command runs if the same key comes twice. Any other key cancels
	// it and then does its usual job, so a stray press never eats the next key.
	pressed := k.String()
	if !key.Matches(k, keys.ScrollDown, keys.ScrollUp) {
		m.scroll = 0
	}
	repeated := m.pending == pressed
	m.pending = ""

	if m.editsArchived(k) {
		m.status = "Archived tasks can't be changed. Press A to restore one, or z to go back"
		return m, nil
	}

	switch {
	case key.Matches(k, keys.Quit):
		if m.err != nil && !repeated {
			m.pending = pressed
			m.status = "Changes are not saved. Press q again to quit, or r to retry"
			return m, nil
		}
		return m, tea.Quit
	case key.Matches(k, keys.Help):
		m.help = true
	case key.Matches(k, keys.Retry):
		if m.err != nil {
			if m = m.save(); m.err == nil {
				m.status = "Saved"
			}
		}
	case key.Matches(k, keys.PreviousColumn):
		m.active = (m.active + board.NumColumns - 1) % board.NumColumns
	case key.Matches(k, keys.NextColumn):
		m.active = (m.active + 1) % board.NumColumns
	case key.Matches(k, keys.JumpToColumn):
		m.active = int(pressed[0] - '1')
	case key.Matches(k, keys.PreviousTask):
		m = m.moveCursor(-1)
	case key.Matches(k, keys.NextTask):
		m = m.moveCursor(1)
	case key.Matches(k, keys.FirstTask):
		if repeated {
			m.cursors[m.active] = 0
		} else {
			m.pending = pressed
		}
	case key.Matches(k, keys.LastTask):
		m.cursors[m.active] = max(len(m.visible(m.active))-1, 0)
	case key.Matches(k, keys.HalfPageDown):
		m = m.moveCursor(m.columnRows() / 2)
	case key.Matches(k, keys.HalfPageUp):
		m = m.moveCursor(-m.columnRows() / 2)
	case key.Matches(k, keys.ScrollDown):
		m.scroll = min(m.scroll+1, max(len(m.detailsLines())-detailsRows, 0))
	case key.Matches(k, keys.ScrollUp):
		m.scroll = max(m.scroll-1, 0)
	case key.Matches(k, keys.Add):
		m.adding = true
	case key.Matches(k, keys.Edit):
		if selected, ok := m.selected(); ok {
			// Remember the task, not its place, because a refresh may move it.
			m.adding, m.editing = true, selected.ID
			m.input.SetValue(capture.Format(selected.Text, selected.Tags, selected.Due))
			m.input.CursorEnd()
		}
	case key.Matches(k, keys.EditNotes):
		if selected, ok := m.selected(); ok {
			return m, m.editor(selected.ID, selected.Notes)
		}
	case key.Matches(k, keys.Delete):
		if repeated {
			m = m.deleteSelected()
		} else if _, ok := m.selected(); ok {
			m.pending = pressed
			m.status = "Press d again to delete"
		}
	case key.Matches(k, keys.Undo):
		m = m.undo()
	case key.Matches(k, keys.Done):
		if m.active == board.Completed {
			m = m.moveSelected(board.ToDo)
		} else {
			m = m.moveSelected(board.Completed)
		}
	case key.Matches(k, keys.Priority):
		m = m.cyclePriority()
	case key.Matches(k, keys.MoveDown):
		m = m.reorderSelected(1)
	case key.Matches(k, keys.MoveUp):
		m = m.reorderSelected(-1)
	case key.Matches(k, keys.NextStage):
		m = m.moveSelected((m.active + 1) % board.NumColumns) // Completed wraps to To Do
	case key.Matches(k, keys.MoveLeft):
		m = m.moveSelected(m.active - 1)
	case key.Matches(k, keys.MoveRight):
		m = m.moveSelected(m.active + 1)
	case key.Matches(k, keys.Today):
		m = m.toggleToday()
	case key.Matches(k, keys.Filter):
		m = m.openPicker()
	case key.Matches(k, keys.Sort):
		m = m.cycleSort()
	case key.Matches(k, keys.Archive):
		m = m.archiveSelected()
	case key.Matches(k, keys.Archived):
		m = m.toggleArchive()
	case key.Matches(k, keys.Search):
		m = m.openSearch()
	case key.Matches(k, keys.NextMatch):
		m = m.jumpToMatch(1, false)
	case key.Matches(k, keys.PreviousMatch):
		m = m.jumpToMatch(-1, false)
	}
	return m.startAnimation()
}

// updateHelp handles keys while the help popup is open. Other keys are ignored,
// so that nothing happens to a task behind the popup.
func (m model) updateHelp(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(k, keys.Help), key.Matches(k, keys.Quit), k.Type == tea.KeyEsc:
		m.help, m.helpScroll = false, 0
	case key.Matches(k, keys.NextTask):
		m.helpScroll = min(m.helpScroll+1, max(len(helpBindings())-m.helpRows(), 0))
	case key.Matches(k, keys.PreviousTask):
		m.helpScroll = max(m.helpScroll-1, 0)
	}
	return m, nil
}

// moveCursor moves the selection by a number of tasks, stopping at the ends of
// the column.
func (m model) moveCursor(by int) model {
	last := max(len(m.visible(m.active))-1, 0)
	m.cursors[m.active] = max(0, min(m.cursors[m.active]+by, last))
	return m
}

// columnRows is how many tasks a column shows at once.
func (m model) columnRows() int {
	return m.height - detailsHeight - footerHeight - 2
}

// updateAdding handles keys while the text input is open. Enter reads tags and
// a due date out of the text. If they can't be read, the input stays open with
// the reason, so that nothing typed is lost.
func (m model) updateAdding(key tea.KeyMsg) (model, tea.Cmd) {
	var cmd tea.Cmd
	m.status = ""
	switch key.Type {
	case tea.KeyEnter:
		fields, err := capture.Parse(m.input.Value(), m.now())
		if errors.Is(err, capture.ErrNoText) && board.CleanText(m.input.Value()) == "" {
			err = nil // blank input adds nothing, and an edit keeps the task as it was
		}
		if err != nil {
			m.status = err.Error()
			return m, nil
		}
		if m.editing != "" {
			m = m.editTask(m.editing, fields)
		} else {
			m = m.addTask(fields)
		}
		m = m.closeInput()
	case tea.KeyEsc:
		m = m.closeInput()
	default:
		m.input, cmd = m.input.Update(key)
	}
	return m, cmd
}

func (m model) closeInput() model {
	m.input.Reset()
	m.adding, m.editing = false, ""
	return m
}

// change makes an action on one task, if it applies to the board as shown, and
// records how to undo it. The action is then saved to the latest board, which
// may also hold changes from other processes.
func (m model) change(id string, op func(board.Board) (board.Board, bool)) (model, bool) {
	if id == "" {
		return m, false
	}
	tasks, ok := op(m.tasks)
	if !ok {
		return m, false
	}
	m = m.remember(m.tasks.ChangeOf(id))
	m.tasks = tasks
	m = m.persist(func(latest board.Board) board.Board {
		next, _ := op(latest)
		return next
	})
	// A sorted column may have moved the task, so follow it there. A task that
	// was removed or is now hidden leaves the cursor where it was.
	m, _ = m.selectTask(id)
	return m, true
}

// editTask changes the text of a task and saves. Blank text keeps the task as
// it was, because dd is the way to delete one. If another process removed the
// task while it was being edited, a status says so, rather than the typed text
// vanishing without a word.
func (m model) editTask(id string, fields capture.Fields) model {
	if _, _, ok := m.tasks.Find(id); !ok {
		m.status = "That task was removed elsewhere"
		return m
	}
	now := m.now()
	m, ok := m.change(id, func(b board.Board) (board.Board, bool) {
		return b.Edit(id, fields.Text, fields.Tags, fields.Due, now)
	})
	if ok {
		m.status = "Task updated"
	}
	return m
}

// addTask adds a task to To Do, selects it and saves. Blank text is ignored.
func (m model) addTask(fields capture.Fields) model {
	if fields.Text == "" {
		return m
	}
	item := board.NewTask(fields.Text, m.now())
	item.Tags, item.Due = fields.Tags, fields.Due
	m, _ = m.change(item.ID, func(b board.Board) (board.Board, bool) { return b.Add(item), true })
	m.status = "Task added"
	if m, shown := m.selectTask(item.ID); shown {
		m.added = true
		return m
	}
	m.status = "Task added. The current view hides it"
	return m
}

// deleteSelected removes the selected task and saves.
func (m model) deleteSelected() model {
	id := m.selectedID()
	m, ok := m.change(id, func(b board.Board) (board.Board, bool) { return b.Delete(id) })
	if ok {
		m.status = "Task deleted, press u to undo"
	}
	return m
}

// moveSelected moves the selected task to the end of the target column and
// follows it there. Moving past the first or last column does nothing.
func (m model) moveSelected(target int) model {
	id, now := m.selectedID(), m.now()
	finished := target == board.Completed && m.active != board.Completed
	m, ok := m.change(id, func(b board.Board) (board.Board, bool) { return b.Move(id, target, now) })
	if !ok {
		return m
	}
	m, _ = m.selectTask(id)
	if finished {
		m = m.celebrate()
	}
	m.status = "Moved to " + columns[target].title
	return m
}

// cyclePriority gives the selected task the next priority and saves.
func (m model) cyclePriority() model {
	id, now := m.selectedID(), m.now()
	m, ok := m.change(id, func(b board.Board) (board.Board, bool) { return b.CyclePriority(id, now) })
	if !ok {
		return m
	}
	m.status = "Priority cleared"
	if column, index, found := m.tasks.Find(id); found && m.tasks[column][index].Priority != "" {
		m.status = "Priority: " + m.tasks[column][index].Priority
	}
	return m
}

// reorderSelected swaps the selected task with the one shown above or below
// it, and saves. Moving past either end of the column does nothing. Sorting
// would hide the change, so it only works in manual order.
func (m model) reorderSelected(by int) model {
	if m.sortBy != board.ByManual {
		m.status = "Reordering works in manual order. Press s until the board shows it"
		return m
	}
	shown, target := m.visible(m.active), m.cursors[m.active]+by
	if target < 0 || target >= len(shown) {
		return m
	}
	id, other := m.selectedID(), shown[target].ID
	m, _ = m.change(id, func(b board.Board) (board.Board, bool) { return b.Swap(id, other) })
	m, _ = m.selectTask(id)
	return m
}

const historySize = 50

func (m model) remember(c board.Change) model {
	m.history = append(slices.Clone(m.history), c)
	if len(m.history) > historySize {
		m.history = m.history[1:]
	}
	return m
}

// undo reverses the most recent change and selects the task it affected.
func (m model) undo() model {
	if len(m.history) == 0 {
		m.status = "Nothing to undo"
		return m
	}
	last := m.history[len(m.history)-1]
	m.history = m.history[:len(m.history)-1]

	m.tasks = m.tasks.Revert(last)
	m = m.persist(func(latest board.Board) board.Board { return latest.Revert(last) })
	m, _ = m.selectTask(last.ID)
	m.status = "Change undone"
	return m
}

// persist saves an action that the board on screen already shows.
func (m model) persist(a action) model {
	m.unsaved = append(slices.Clone(m.unsaved), a)
	return m.save()
}

// save applies every unsaved action to the latest board and shows the result.
// If saving fails, the actions are kept to try again, and the board on screen
// still shows them.
func (m model) save() model {
	latest, err := m.store.Update(func(latest board.Board) board.Board { return replay(latest, m.unsaved) })
	if err != nil {
		m.err = err
		return m
	}
	m.err, m.unsaved = nil, nil
	return m.show(latest)
}

// refresh shows changes that other processes have made to the data file.
func (m model) refresh() model {
	changed, err := m.store.ModTime()
	if err != nil || changed.Equal(m.seen) {
		return m
	}
	latest, err := m.store.Load()
	if err != nil {
		return m // keep showing the last good board, and let the next save report the problem
	}
	m.seen = changed
	return m.show(replay(latest, m.unsaved))
}

// show replaces the board on screen, keeping the selected task selected if it
// is still shown in the same column, and every cursor on a task.
func (m model) show(tasks board.Board) model {
	selected := m.selectedID()
	m.tasks = tasks
	for column := range board.NumColumns {
		shown := m.visible(column)
		m.cursors[column] = max(0, min(m.cursors[column], len(shown)-1))
		if column == m.active {
			if index := slices.IndexFunc(shown, func(item board.Task) bool { return item.ID == selected }); index >= 0 {
				m.cursors[column] = index
			}
		}
	}
	return m
}

func replay(tasks board.Board, actions []action) board.Board {
	for _, a := range actions {
		tasks = a(tasks)
	}
	return tasks
}
