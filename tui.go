package main

import (
	"crypto/rand"
	"slices"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// model holds all of the app's state.
type model struct {
	path       string           // file the tasks are saved to
	tasks      board            // the tasks in each column
	active     int              // index of the focused column
	cursors    [numColumns]int  // index of the selected task in each column
	adding     bool             // true while the text input is open
	editing    bool             // true if the input is editing the selected task
	input      textinput.Model  // the text input
	added      bool             // true after adding a task, until the next key press
	err        error            // last save error, shown on screen
	status     string           // message about the last action, until the next key
	scroll     int              // first line of the details panel that is showing
	help       bool             // true while the help popup is open
	helpScroll int              // first line of the help popup that is showing
	pending    string           // first key of a two-key command such as dd
	history    []change         // recent changes, for undo
	width      int              // terminal width
	height     int              // terminal height
	now        func() time.Time // the clock, replaceable in tests

	animate     bool       // whether effects are switched on
	ticking     bool       // true while animation frames are scheduled
	splash      bool       // true until the startup logo is dismissed
	splashFrame int        // how far the logo has been revealed
	particles   []particle // confetti currently on screen
	bar         float64    // share of the progress bar currently filled
}

func newModel(path string, tasks board) model {
	// Undo finds tasks by ID, so tasks from a hand-edited file need one too.
	for column := range tasks {
		for i := range tasks[column] {
			if tasks[column][i].ID == "" {
				tasks[column][i].ID = rand.Text()
			}
		}
	}
	input := textinput.New()
	input.Prompt = ""
	input.TextStyle = textStyle
	input.Cursor.SetMode(cursor.CursorStatic) // a blink would redraw when idle
	input.Focus()
	return model{path: path, tasks: tasks, now: time.Now, input: input}
}

func (m model) Init() tea.Cmd {
	if m.ticking {
		return tick()
	}
	return nil
}

// Update is called by Bubble Tea on every event, such as a key press.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.Width = msg.Width - 3 // the panel's borders, and the cursor at the end
		// A bigger screen has less to scroll through.
		m.scroll = min(m.scroll, max(len(m.detailsLines())-detailsRows, 0))
		m.helpScroll = min(m.helpScroll, max(len(helpBindings())-m.helpRows(), 0))
	case animTick:
		return m.stepAnimation()
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
		if m.adding {
			updated, input := m.updateAdding(msg)
			next, animation := updated.startAnimation()
			return next, tea.Batch(input, animation)
		}
		return m.updateBoard(msg)
	default:
		if m.adding { // such as the clipboard text that ctrl+v asked for
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
		m.active = (m.active + numColumns - 1) % numColumns
	case key.Matches(k, keys.NextColumn):
		m.active = (m.active + 1) % numColumns
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
		m.cursors[m.active] = max(len(m.tasks[m.active])-1, 0)
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
		if tasks := m.tasks[m.active]; len(tasks) > 0 {
			m.adding, m.editing = true, true
			m.input.SetValue(tasks[m.cursors[m.active]].Text)
			m.input.CursorEnd()
		}
	case key.Matches(k, keys.Delete):
		if repeated {
			m = m.deleteSelected()
		} else if len(m.tasks[m.active]) > 0 {
			m.pending = pressed
			m.status = "Press d again to delete"
		}
	case key.Matches(k, keys.Undo):
		m = m.undo()
	case key.Matches(k, keys.Done):
		if m.active == completed {
			m = m.moveSelected(toDo)
		} else {
			m = m.moveSelected(completed)
		}
	case key.Matches(k, keys.Priority):
		m = m.cyclePriority()
	case key.Matches(k, keys.MoveDown):
		m = m.reorderSelected(1)
	case key.Matches(k, keys.MoveUp):
		m = m.reorderSelected(-1)
	case key.Matches(k, keys.NextStage):
		m = m.moveSelected((m.active + 1) % numColumns) // Completed wraps to To Do
	case key.Matches(k, keys.MoveLeft):
		m = m.moveSelected(m.active - 1)
	case key.Matches(k, keys.MoveRight):
		m = m.moveSelected(m.active + 1)
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
	last := max(len(m.tasks[m.active])-1, 0)
	m.cursors[m.active] = max(0, min(m.cursors[m.active]+by, last))
	return m
}

// columnRows is how many tasks a column shows at once.
func (m model) columnRows() int {
	return m.height - detailsHeight - footerHeight - 2
}

// updateAdding handles keys while the text input is open.
func (m model) updateAdding(key tea.KeyMsg) (model, tea.Cmd) {
	var cmd tea.Cmd
	switch key.Type {
	case tea.KeyEnter:
		if m.editing {
			m = m.editSelected(m.input.Value())
		} else {
			m = m.addTask(m.input.Value())
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
	m.adding, m.editing = false, false
	return m
}

// editSelected changes the text of the selected task and saves. Blank text
// keeps the task as it was, because dd is the way to delete one.
func (m model) editSelected(text string) model {
	text = cleanText(text)
	column := slices.Clone(m.tasks[m.active])
	item := &column[m.cursors[m.active]]
	if text == "" || text == item.Text {
		return m
	}
	m = m.rememberSelected()
	item.Text = text
	item.Updated = m.now()
	m.tasks[m.active] = column
	m.status = "Task updated"
	return m.save()
}

// addTask adds a task to To Do, selects it and saves. Blank text is ignored.
func (m model) addTask(text string) model {
	text = cleanText(text)
	if text == "" {
		return m
	}
	item := newTask(text, m.now())
	m = m.remember(change{id: item.ID})
	m.tasks[toDo] = append(m.tasks[toDo], item)
	m.active = toDo
	m.cursors[toDo] = len(m.tasks[toDo]) - 1
	m.added = true
	m.status = "Task added"
	return m.save()
}

// deleteSelected removes the selected task and saves.
func (m model) deleteSelected() model {
	if len(m.tasks[m.active]) == 0 {
		return m
	}
	m = m.rememberSelected()
	m = m.removeSelected()
	m.status = "Task deleted, press u to undo"
	return m.save()
}

// moveSelected moves the selected task to the end of the target column and
// follows it there. Moving past the first or last column does nothing.
func (m model) moveSelected(target int) model {
	if target < 0 || target >= numColumns || len(m.tasks[m.active]) == 0 {
		return m
	}
	m = m.rememberSelected()
	item := m.tasks[m.active][m.cursors[m.active]]
	now := m.now()
	item.Updated = now
	if target != completed {
		item.Completed = time.Time{}
	}
	finished := target == completed && m.active != completed
	if finished {
		item.Completed = now
	}
	m = m.removeSelected()
	m.tasks[target] = append(m.tasks[target], item)
	m.active = target
	m.cursors[target] = len(m.tasks[target]) - 1
	if finished {
		m = m.celebrate()
	}
	m.status = "Moved to " + columns[target].title
	return m.save()
}

// removeSelected takes the selected task out of its column and keeps the
// cursor on a valid task.
func (m model) removeSelected() model {
	column, cursor := m.active, m.cursors[m.active]
	m.tasks[column] = slices.Delete(m.tasks[column], cursor, cursor+1)
	if cursor > 0 && cursor == len(m.tasks[column]) {
		m.cursors[column]--
	}
	return m
}

// priorities are the priority levels in the order that p steps through them.
var priorities = []string{"", "low", "medium", "high"}

// cyclePriority gives the selected task the next priority and saves.
func (m model) cyclePriority() model {
	if len(m.tasks[m.active]) == 0 {
		return m
	}
	m = m.rememberSelected()
	column := slices.Clone(m.tasks[m.active])
	item := &column[m.cursors[m.active]]
	item.Priority = priorities[(slices.Index(priorities, item.Priority)+1)%len(priorities)]
	item.Updated = m.now()
	m.tasks[m.active] = column
	m.status = "Priority cleared"
	if item.Priority != "" {
		m.status = "Priority: " + item.Priority
	}
	return m.save()
}

// reorderSelected moves the selected task up or down within its column and
// saves. Moving past either end of the column does nothing.
func (m model) reorderSelected(by int) model {
	cursor, target := m.cursors[m.active], m.cursors[m.active]+by
	if target < 0 || target >= len(m.tasks[m.active]) {
		return m
	}
	m = m.rememberSelected()
	column := slices.Clone(m.tasks[m.active])
	column[cursor], column[target] = column[target], column[cursor]
	m.tasks[m.active] = column
	m.cursors[m.active] = target
	return m.save()
}

// change records how to undo one action. Keeping just the affected task, rather
// than a copy of the whole board, means that undo can't erase anything else that
// has changed since.
type change struct {
	id     string // the task that changed
	before *task  // the task as it was, or nil if the action created it
	column int    // where the task was
	index  int
}

const historySize = 50

func (m model) remember(c change) model {
	m.history = append(slices.Clone(m.history), c)
	if len(m.history) > historySize {
		m.history = m.history[1:]
	}
	return m
}

// rememberSelected records the selected task before an action changes it.
func (m model) rememberSelected() model {
	item := m.tasks[m.active][m.cursors[m.active]]
	return m.remember(change{id: item.ID, before: &item, column: m.active, index: m.cursors[m.active]})
}

// undo reverses the most recent change and selects the task it affected.
func (m model) undo() model {
	if len(m.history) == 0 {
		m.status = "Nothing to undo"
		return m
	}
	last := m.history[len(m.history)-1]
	m.history = m.history[:len(m.history)-1]

	m = m.removeByID(last.id)
	if last.before != nil {
		index := min(last.index, len(m.tasks[last.column]))
		m.tasks[last.column] = slices.Insert(m.tasks[last.column], index, *last.before)
		m.active, m.cursors[last.column] = last.column, index
	}
	m.status = "Change undone"
	return m.save()
}

// removeByID takes a task out of whichever column holds it, keeping that
// column's cursor on a valid task.
func (m model) removeByID(id string) model {
	for column, list := range m.tasks {
		index := slices.IndexFunc(list, func(item task) bool { return item.ID == id })
		if index < 0 {
			continue
		}
		m.tasks[column] = slices.Delete(slices.Clone(list), index, index+1)
		if last := len(m.tasks[column]) - 1; m.cursors[column] > last {
			m.cursors[column] = max(last, 0)
		}
	}
	return m
}

func (m model) save() model {
	m.err = saveBoard(m.path, m.tasks)
	return m
}
