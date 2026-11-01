package main

import (
	"slices"

	tea "github.com/charmbracelet/bubbletea"
)

// model holds all of the app's state.
type model struct {
	path    string          // file the tasks are saved to
	tasks   board           // the tasks in each column
	active  int             // index of the focused column
	cursors [numColumns]int // index of the selected task in each column
	adding  bool            // true while typing a new task
	input   string          // text typed so far for the new task
	added   bool            // true after adding a task, until the next key press
	err     error           // last save error, shown on screen
	width   int             // terminal width
	height  int             // terminal height
}

func newModel(path string, tasks board) model {
	return model{path: path, tasks: tasks}
}

func (m model) Init() tea.Cmd {
	return nil
}

// Update is called by Bubble Tea on every event, such as a key press.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		if m.adding {
			return m.updateAdding(msg), nil
		}
		return m.updateBoard(msg)
	}
	return m, nil
}

// updateBoard handles keys while browsing the board.
func (m model) updateBoard(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.added = false

	switch key.String() {
	case "q":
		return m, tea.Quit
	case "left", "h", "shift+tab":
		m.active = (m.active + numColumns - 1) % numColumns
	case "right", "l", "tab":
		m.active = (m.active + 1) % numColumns
	case "1":
		m.active = toDo
	case "2":
		m.active = inProgress
	case "3":
		m.active = completed
	case "up", "k":
		if m.cursors[m.active] > 0 {
			m.cursors[m.active]--
		}
	case "down", "j":
		if m.cursors[m.active] < len(m.tasks[m.active])-1 {
			m.cursors[m.active]++
		}
	case "a", "4":
		m.adding = true
	case "d":
		m = m.deleteSelected()
	case " ":
		m = m.moveSelected((m.active + 1) % numColumns) // Completed wraps to To Do
	case "shift+left", "H":
		m = m.moveSelected(m.active - 1)
	case "shift+right", "L":
		m = m.moveSelected(m.active + 1)
	}
	return m, nil
}

// updateAdding handles keys while typing a new task.
func (m model) updateAdding(key tea.KeyMsg) model {
	switch key.Type {
	case tea.KeyEnter:
		m = m.addTask(m.input)
		m.input = ""
		m.adding = false
	case tea.KeyEsc:
		m.input = ""
		m.adding = false
	case tea.KeyBackspace:
		runes := []rune(m.input)
		if len(runes) > 0 {
			m.input = string(runes[:len(runes)-1])
		}
	case tea.KeyRunes, tea.KeySpace:
		m.input += controlsToSpaces(string(key.Runes))
	}
	return m
}

// addTask adds a task to To Do, selects it and saves. Blank text is ignored.
func (m model) addTask(text string) model {
	text = cleanText(text)
	if text == "" {
		return m
	}
	m.tasks[toDo] = append(m.tasks[toDo], text)
	m.active = toDo
	m.cursors[toDo] = len(m.tasks[toDo]) - 1
	m.added = true
	return m.save()
}

// deleteSelected removes the selected task and saves.
func (m model) deleteSelected() model {
	if len(m.tasks[m.active]) == 0 {
		return m
	}
	m = m.removeSelected()
	return m.save()
}

// moveSelected moves the selected task to the end of the target column and
// follows it there. Moving past the first or last column does nothing.
func (m model) moveSelected(target int) model {
	if target < 0 || target >= numColumns || len(m.tasks[m.active]) == 0 {
		return m
	}
	task := m.tasks[m.active][m.cursors[m.active]]
	m = m.removeSelected()
	m.tasks[target] = append(m.tasks[target], task)
	m.active = target
	m.cursors[target] = len(m.tasks[target]) - 1
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

func (m model) save() model {
	m.err = saveBoard(m.path, m.tasks)
	return m
}
