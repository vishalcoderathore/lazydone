package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/lucasb-eyer/go-colorful"
)

const (
	minWidth      = 60
	minHeight     = 12
	detailsHeight = 5 // height of the bottom panel, including its border
	footerHeight  = 1
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
	errorStyle    = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
)

var columns = [numColumns]column{
	toDo:       {"To Do", "📋", "○", lipgloss.NewStyle().Foreground(colorBlue), newGradient(colorBlue, colorMauve)},
	inProgress: {"In Progress", "🚧", "◐", lipgloss.NewStyle().Foreground(colorYellow), newGradient(colorYellow, colorPeach)},
	completed:  {"Completed", "✅", "✓", lipgloss.NewStyle().Foreground(colorGreen), newGradient(colorGreen, colorTeal)},
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

	view := lipgloss.JoinVertical(lipgloss.Left, m.viewColumns(), m.withProgress(m.viewDetails()), m.viewFooter())
	return overlayParticles(view, m.particles, m.width)
}

func (m model) viewColumns() string {
	height := m.height - detailsHeight - footerHeight
	width := m.width / numColumns

	var panels []string
	for i := range numColumns {
		w := width
		if i == numColumns-1 {
			w = m.width - width*(numColumns-1) // the last column fills any leftover space
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
		lines = append(lines, viewTask(col, tasks[i], i == cursor, active, width-2))
	}
	return panel(title, position, lines, width, height, active, col.border)
}

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
	width := m.width - 2
	rows := detailsHeight - 2
	border := columns[m.active].border

	if m.adding {
		lines := strings.Split(ansi.Wrap(m.input+"█", width, ""), "\n")
		lines = lines[max(0, len(lines)-rows):] // keep the end of the text in view
		return panel("[4] 📝 New Task", "", styled(lines), m.width, detailsHeight, true, inputBorder)
	}

	// Right after adding, the new task would be selected and shown here, which
	// looks as if the input box was not cleared. Stay blank until the next key.
	if m.added {
		return panel("[4] 🔍 Details", "", nil, m.width, detailsHeight, false, border)
	}

	var lines []string
	if tasks := m.tasks[m.active]; len(tasks) > 0 {
		lines = strings.Split(ansi.Wrap(tasks[m.cursors[m.active]], width, ""), "\n")
		lines = styled(lines[:min(len(lines), rows)])
	} else {
		lines = []string{faintStyle.Render("Nothing selected")}
	}
	return panel("[4] 🔍 Details", "", lines, m.width, detailsHeight, false, border)
}

// styled gives each line the normal text color.
func styled(lines []string) []string {
	for i, line := range lines {
		lines[i] = textStyle.Render(line)
	}
	return lines
}

func (m model) viewFooter() string {
	if m.err != nil {
		return errorStyle.Render(ansi.Truncate("Could not save: "+m.err.Error(), m.width, "…"))
	}
	hints := "←/→: column | ↑/↓: task | a: add | d: delete | space: next | H/L: move | q: quit"
	if m.adding {
		hints = "enter: save | esc: cancel"
	}
	return hintStyle.Render(ansi.Truncate(hints, m.width, "…"))
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
