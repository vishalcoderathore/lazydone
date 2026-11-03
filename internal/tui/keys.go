package tui

import "github.com/charmbracelet/bubbles/key"

// keys is every keybinding of the board. The key handler, the footer and the
// help popup all read from here, so that they can't disagree. A two-key command
// such as dd is bound to its single key, and its help shows both presses.
var keys = struct {
	PreviousColumn, NextColumn, JumpToColumn      key.Binding
	NextTask, PreviousTask, FirstTask, LastTask   key.Binding
	HalfPageDown, HalfPageUp                      key.Binding
	Add, Edit, Delete, Done, Priority, NextStage  key.Binding
	MoveLeft, MoveRight, MoveDown, MoveUp         key.Binding
	ScrollDown, ScrollUp, Undo, Retry, Help, Quit key.Binding
	Today, Filter, Sort, Archive, Archived        key.Binding
	Search, NextMatch, PreviousMatch              key.Binding
}{
	PreviousColumn: bind("h / ←", "previous column", "left", "h", "shift+tab"),
	NextColumn:     bind("l / →", "next column", "right", "l", "tab"),
	JumpToColumn:   bind("1 2 3", "jump to column", "1", "2", "3"),
	NextTask:       bind("j / ↓", "next task", "down", "j"),
	PreviousTask:   bind("k / ↑", "previous task", "up", "k"),
	FirstTask:      bind("gg", "first task", "g"),
	LastTask:       bind("G", "last task", "G"),
	HalfPageDown:   bind("Ctrl-d", "half page down", "ctrl+d"),
	HalfPageUp:     bind("Ctrl-u", "half page up", "ctrl+u"),
	Add:            bind("a / 4", "add", "a", "4"),
	Edit:           bind("e", "edit", "e"),
	Delete:         bind("dd", "delete", "d"),
	Done:           bind("x", "done", "x"),
	Priority:       bind("p", "priority", "p"),
	NextStage:      bind("space", "next stage", " "),
	MoveLeft:       bind("H", "move to column on the left", "H", "shift+left"),
	MoveRight:      bind("L", "move to column on the right", "L", "shift+right"),
	MoveDown:       bind("J", "reorder down", "J"),
	MoveUp:         bind("K", "reorder up", "K"),
	ScrollDown:     bind("Ctrl-e", "scroll details down", "ctrl+e"),
	ScrollUp:       bind("Ctrl-y", "scroll details up", "ctrl+y"),
	Undo:           bind("u", "undo", "u"),
	Today:          bind("t", "Today view on / off", "t"),
	Filter:         bind("f", "filter by tag", "f"),
	Sort:           bind("s", "next sort order", "s"),
	Archive:        bind("A", "archive / restore task", "A"),
	Archived:       bind("z", "archived tasks on / off", "z"),
	Search:         bind("/", "fuzzy search", "/"),
	NextMatch:      bind("n", "next match", "n"),
	PreviousMatch:  bind("N", "previous match", "N"),
	Retry:          bind("r", "retry save", "r"),
	Help:           bind("?", "help", "?"),
	Quit:           bind("q", "quit", "q"),
}

func bind(label, description string, pressed ...string) key.Binding {
	return key.NewBinding(key.WithKeys(pressed...), key.WithHelp(label, description))
}

// helpBindings are shown in the help popup, in this order.
func helpBindings() []key.Binding {
	k := keys
	return []key.Binding{
		k.PreviousColumn, k.NextColumn, k.JumpToColumn,
		k.NextTask, k.PreviousTask, k.FirstTask, k.LastTask, k.HalfPageDown, k.HalfPageUp,
		k.Add, k.Edit, k.Delete, k.Done, k.Priority, k.NextStage,
		k.MoveLeft, k.MoveRight, k.MoveDown, k.MoveUp,
		k.Today, k.Filter, k.Sort, k.Archive, k.Archived, k.Search, k.NextMatch, k.PreviousMatch,
		k.ScrollDown, k.ScrollUp, k.Undo, k.Retry, k.Help, k.Quit,
	}
}

// footerBindings are shown in the footer, and must fit in 80 cells.
func footerBindings() []key.Binding {
	k := keys
	return []key.Binding{bind("hjkl", "move"), bind("a", "add"), bind("dd", "delete"), k.Done, k.Undo, k.Help, k.Quit}
}
