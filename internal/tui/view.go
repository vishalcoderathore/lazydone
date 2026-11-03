package tui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/vishalcoderathore/lazydone/internal/board"
)

const (
	minWidth      = 60
	minHeight     = 12
	detailsHeight = 5 // height of the bottom panel, including its border
	footerHeight  = 1
	detailsRows   = detailsHeight - 2 // lines of text that fit in the bottom panel
)

// column describes how a column of the board looks.
type column struct {
	title  string
	icon   string // shown in the panel title
	bullet string // shown in front of each task
	style  lipgloss.Style
	border gradient
}

// Catppuccin Mocha colors, matching the lazydocker theme.
const (
	colorText     = lipgloss.Color("#CDD6F4")
	colorBlue     = lipgloss.Color("#89B4FA")
	colorYellow   = lipgloss.Color("#F9E2AF")
	colorGreen    = lipgloss.Color("#A6E3A1")
	colorRed      = lipgloss.Color("#F38BA8")
	colorMauve    = lipgloss.Color("#CBA6F7")
	colorPeach    = lipgloss.Color("#FAB387")
	colorTeal     = lipgloss.Color("#94E2D5")
	colorPink     = lipgloss.Color("#F5C2E7")
	colorOverlay  = lipgloss.Color("#7F849C")
	colorSurface0 = lipgloss.Color("#313244")
)

var (
	textStyle     = lipgloss.NewStyle().Foreground(colorText)
	selectedStyle = lipgloss.NewStyle().Foreground(colorText).Background(colorSurface0).Bold(true)
	faintStyle    = lipgloss.NewStyle().Foreground(colorOverlay)
	hintStyle     = lipgloss.NewStyle().Foreground(colorBlue)
	statusStyle   = lipgloss.NewStyle().Foreground(colorTeal).Bold(true)
	errorStyle    = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
)

var columns = [board.NumColumns]column{
	board.ToDo:       {"To Do", "📋", "○", lipgloss.NewStyle().Foreground(colorBlue), newGradient(colorBlue, colorMauve)},
	board.InProgress: {"In Progress", "🚧", "◐", lipgloss.NewStyle().Foreground(colorYellow), newGradient(colorYellow, colorPeach)},
	board.Completed:  {"Completed", "✅", "✓", lipgloss.NewStyle().Foreground(colorGreen), newGradient(colorGreen, colorTeal)},
}

// inputBorder is the border of the panel for typing a new task.
var inputBorder = newGradient(colorPink, colorMauve)

// gradient is a color fade that runs diagonally across a box.
type gradient struct {
	from, to colorful.Color
}

func newGradient(from, to lipgloss.Color) gradient {
	return gradient{parseColor(from), parseColor(to)}
}

func parseColor(hex lipgloss.Color) colorful.Color {
	c, err := colorful.Hex(string(hex))
	if err != nil {
		panic(err) // only ever called with the constants above
	}
	return c
}

// dimmed returns a fainter copy of the gradient, for panels without focus.
func (g gradient) dimmed() gradient {
	background := parseColor(colorSurface0)
	return gradient{g.from.BlendLab(background, 0.55), g.to.BlendLab(background, 0.55)}
}

// titleStyle is the style of a panel title that belongs to this gradient.
func (g gradient) titleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(g.from.Hex())).Bold(true)
}

// paint colors text that starts at cell (x, y) of a width by height box. Each
// character must be one cell wide.
func (g gradient) paint(text string, x, y, width, height int) string {
	var s strings.Builder
	for _, r := range text {
		t := (float64(x)/float64(width-1) + float64(y)/float64(height-1)) / 2
		color := lipgloss.Color(g.from.BlendLab(g.to, t).Hex())
		s.WriteString(lipgloss.NewStyle().Foreground(color).Render(string(r)))
		x++
	}
	return s.String()
}

// View returns the text to draw on screen.
func (m model) View() string {
	if m.width == 0 {
		return "" // the terminal size is not known yet
	}
	if m.width < minWidth || m.height < minHeight {
		return fmt.Sprintf("Please make the terminal at least %dx%d.", minWidth, minHeight)
	}

	if m.splash {
		return m.viewSplash()
	}
	if m.help {
		return m.viewHelp()
	}

	view := lipgloss.JoinVertical(lipgloss.Left, m.viewColumns(), m.withProgress(m.viewDetails()), m.viewFooter())
	return overlayParticles(view, m.particles, m.width)
}

func (m model) viewColumns() string {
	height := m.height - detailsHeight - footerHeight
	width := m.width / board.NumColumns

	var panels []string
	for i := range board.NumColumns {
		w := width
		if i == board.NumColumns-1 {
			w = m.width - width*(board.NumColumns-1) // the last column fills any leftover space
		}
		panels = append(panels, m.viewColumn(i, w, height))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, panels...)
}

func (m model) viewColumn(index, width, height int) string {
	col := columns[index]
	tasks := m.tasks[index]
	cursor := m.cursors[index]
	active := !m.adding && index == m.active

	title := fmt.Sprintf("[%d] %s %s", index+1, col.icon, col.title)
	position := ""
	if len(tasks) > 0 {
		position = fmt.Sprintf("%d of %d", cursor+1, len(tasks))
	}

	// Scroll just enough to keep the selected task on screen.
	rows := height - 2
	first := max(0, cursor-rows+1)
	last := min(len(tasks), first+rows)

	var lines []string
	if len(tasks) == 0 {
		lines = append(lines, faintStyle.Render("  No tasks"))
	}
	for i := first; i < last; i++ {
		lines = append(lines, viewTask(col, priorityMarkers[tasks[i].Priority]+tasks[i].Text, i == cursor, active, width-2))
	}
	return panel(title, position, lines, width, height, active, col.border)
}

// priorityMarkers are shown in front of a task's text.
var priorityMarkers = map[string]string{"low": "! ", "medium": "!! ", "high": "!!! "}

func viewTask(col column, task string, selected, active bool, width int) string {
	marker := "  "
	if selected && active {
		marker = "❯ "
	}
	row := ansi.Truncate(marker+col.bullet+" "+task, width, "…")

	if selected && active {
		return selectedStyle.Width(width).Render(row)
	}
	if selected {
		return col.style.Bold(true).Render(row)
	}
	return col.style.Render(row)
}

// viewDetails draws the bottom panel. It shows the full text of the selected
// task, which may be cut off in its column, or the input for a new task.
func (m model) viewDetails() string {
	border := columns[m.active].border

	if m.adding {
		title := "[4] 📝 New Task"
		if m.editing != "" {
			title = "[4] 📝 Edit Task"
		}
		return panel(title, "", []string{m.input.View()}, m.width, detailsHeight, true, inputBorder)
	}

	// Right after adding, the new task would be selected and shown here, which
	// looks as if the input box was not cleared. Stay blank until the next key.
	if m.added {
		return panel("[4] 🔍 Details", "", nil, m.width, detailsHeight, false, border)
	}

	lines := m.detailsLines()
	title := "[4] 🔍 Details"
	if len(lines) == 0 {
		return panel(title, "", []string{faintStyle.Render("Nothing selected")}, m.width, detailsHeight, false, border)
	}
	last := min(m.scroll+detailsRows, len(lines))
	if len(lines) > detailsRows {
		title += fmt.Sprintf(" %d-%d of %d", m.scroll+1, last, len(lines))
	}
	return panel(title, "", styled(slices.Clone(lines[m.scroll:last])), m.width, detailsHeight, false, border)
}

// detailsLines is everything that the details panel has to show about the
// selected task, wrapped to fit. It's empty if no task is selected.
func (m model) detailsLines() []string {
	tasks := m.tasks[m.active]
	if len(tasks) == 0 {
		return nil
	}
	selected := tasks[m.cursors[m.active]]
	lines := strings.Split(ansi.Wrap(selected.Text, m.width-2, ""), "\n")
	if selected.Priority != "" {
		lines = append([]string{"Priority: " + selected.Priority}, lines...)
	}
	return lines
}

// styled gives each line the normal text color.
func styled(lines []string) []string {
	for i, line := range lines {
		lines[i] = textStyle.Render(line)
	}
	return lines
}

func (m model) viewFooter() string {
	warning := m.err != nil && m.pending == keys.Quit.Keys()[0]
	if m.err != nil && !warning {
		return errorStyle.Render(ansi.Truncate("Could not save (r: retry): "+m.err.Error(), m.width, "…"))
	}
	if m.status != "" {
		return statusStyle.Render(ansi.Truncate(m.status, m.width, "…"))
	}
	hints := "enter: save | esc: cancel"
	if !m.adding {
		var parts []string
		for _, binding := range footerBindings() {
			parts = append(parts, binding.Help().Key+": "+binding.Help().Desc)
		}
		hints = strings.Join(parts, " | ")
	}
	return hintStyle.Render(ansi.Truncate(hints, m.width, "…"))
}

const helpLabelWidth = 10

// helpRows is how many keybindings the help popup shows at once.
func (m model) helpRows() int {
	return min(len(helpBindings()), m.height-2)
}

// viewHelp draws the help popup, listing keybindings from the keymap.
func (m model) viewHelp() string {
	bindings := helpBindings()
	rows := m.helpRows()
	var lines []string
	for _, binding := range bindings[m.helpScroll : m.helpScroll+rows] {
		label := lipgloss.NewStyle().Width(helpLabelWidth).Render(binding.Help().Key)
		lines = append(lines, " "+hintStyle.Render(label)+textStyle.Render(binding.Help().Desc))
	}
	note := ""
	if rows < len(bindings) {
		note = fmt.Sprintf("%d-%d of %d", m.helpScroll+1, m.helpScroll+rows, len(bindings))
	}
	popup := panel("Keybindings", note, lines, min(m.width, 44), rows+2, true, inputBorder)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup)
}

// panel draws a box with rounded corners, a title in its top border and a
// note in its bottom border. Lines that don't fit are cut off. The border
// fades through the gradient, and is dimmed unless the panel is active.
func panel(title, note string, lines []string, width, height int, active bool, border gradient) string {
	titleStyle := border.titleStyle()
	if !active {
		border = border.dimmed()
	}
	inner := width - 2

	title = ansi.Truncate(title, inner-1, "…")
	note = ansi.Truncate(note, inner-1, "…")

	titleEnd := 2 + lipgloss.Width(title)
	var s strings.Builder
	s.WriteString(border.paint("╭─", 0, 0, width, height) + titleStyle.Render(title) +
		border.paint(strings.Repeat("─", inner-1-lipgloss.Width(title))+"╮", titleEnd, 0, width, height) + "\n")

	for i := range height - 2 {
		line := ""
		if i < len(lines) {
			line = ansi.Truncate(lines[i], inner, "…")
		}
		s.WriteString(border.paint("│", 0, i+1, width, height) + line + strings.Repeat(" ", inner-lipgloss.Width(line)) +
			border.paint("│", width-1, i+1, width, height) + "\n")
	}

	s.WriteString(border.paint("╰"+strings.Repeat("─", inner-1-lipgloss.Width(note))+note+"─╯", 0, height-1, width, height))
	return s.String()
}
