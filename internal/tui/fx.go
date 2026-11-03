package tui

import (
	"fmt"
	"maps"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/vishalcoderathore/lazydone/internal/board"
)

const (
	frameTime   = time.Second / 30
	sweepFrames = 15 // length of the logo's reveal

	barWidth       = 12    // cells in the progress bar
	barEase        = 0.2   // share of the remaining distance covered each frame
	barCloseEnough = 0.002 // distance at which the bar snaps to its target

	confettiCount = 70
	confettiLife  = 32 // frames that the longest-lived particle lasts
	gravity       = 0.09
	drag          = 0.97
)

const splashHint = "press space or enter"

var logo = []string{
	"██╗      █████╗ ███████╗██╗   ██╗██████╗  ██████╗ ███╗   ██╗███████╗",
	"██║     ██╔══██╗╚══███╔╝╚██╗ ██╔╝██╔══██╗██╔═══██╗████╗  ██║██╔════╝",
	"██║     ███████║  ███╔╝  ╚████╔╝ ██║  ██║██║   ██║██╔██╗ ██║█████╗  ",
	"██║     ██╔══██║ ███╔╝    ╚██╔╝  ██║  ██║██║   ██║██║╚██╗██║██╔══╝  ",
	"███████╗██║  ██║███████╗   ██║   ██████╔╝╚██████╔╝██║ ╚████║███████╗",
	"╚══════╝╚═╝  ╚═╝╚══════╝   ╚═╝   ╚═════╝  ╚═════╝ ╚═╝  ╚═══╝╚══════╝",
}

var (
	barEdges       = []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉"} // eighths of a cell
	logoGradient   = newGradient(colorPink, colorBlue)
	barGradient    = newGradient(colorPink, colorTeal)
	confettiGlyphs = []string{"●", "■", "▲", "◆", "✦", "*", "+"}
	confettiColors = []lipgloss.Color{colorRed, colorPeach, colorYellow, colorGreen, colorTeal, colorBlue, colorMauve, colorPink}
)

// animTick asks the model to draw its next animation frame.
type animTick struct{}

func tick() tea.Cmd {
	return tea.Tick(frameTime, func(time.Time) tea.Msg { return animTick{} })
}

// withAnimations switches on the splash and confetti. They stay off by default
// so that a model behaves the same on every run unless asked otherwise.
func (m model) withAnimations() model {
	m.animate = true
	m.splash = true
	m.ticking = true
	return m
}

// sweeping reports whether the logo is still being revealed. The finished logo
// doesn't change, so it needs no frames while it waits for a key.
func (m model) sweeping() bool {
	return m.splash && m.splashFrame < sweepFrames
}

func (m model) animating() bool {
	return m.sweeping() || len(m.particles) > 0 || m.barMoving()
}

// startAnimation schedules frames if something needs animating and frames
// aren't already scheduled. Keeping a single chain of ticks, which ends when
// nothing is moving, means an idle app does no work.
func (m model) startAnimation() (tea.Model, tea.Cmd) {
	if m.ticking || !m.animating() {
		return m, nil
	}
	m.ticking = true
	return m, tick()
}

func (m model) stepAnimation() (tea.Model, tea.Cmd) {
	if m.sweeping() {
		m.splashFrame++
	}
	m.particles = stepParticles(m.particles, m.width, m.height)
	if m.barMoving() {
		target := m.doneFraction()
		m.bar += (target - m.bar) * barEase
		if math.Abs(target-m.bar) < barCloseEnough {
			m.bar = target
		}
	}
	if !m.animating() {
		m.ticking = false
		return m, nil
	}
	return m, tick()
}

// celebrate bursts confetti from the selected task in the Completed column.
func (m model) celebrate() model {
	if !m.animate || m.width == 0 {
		return m
	}
	columnWidth := m.width - (m.width/board.NumColumns)*(board.NumColumns-1)
	x := m.width - columnWidth/2
	y := 1 + min(m.cursors[board.Completed], max(m.columnRows()-1, 0))
	m.particles = append(slices.Clone(m.particles), newBurst(float64(x), float64(y))...)
	return m
}

// particle is one piece of confetti, positioned in terminal cells.
type particle struct {
	x, y   float64
	vx, vy float64
	life   int    // frames left
	glyph  string // already styled, one cell wide
}

func newBurst(x, y float64) []particle {
	burst := make([]particle, confettiCount)
	for i := range burst {
		color := confettiColors[rand.IntN(len(confettiColors))]
		glyph := confettiGlyphs[rand.IntN(len(confettiGlyphs))]
		burst[i] = particle{
			x:     x,
			y:     y,
			vx:    (rand.Float64() - 0.5) * 3.6, // cells are tall, so spread wider than high
			vy:    -0.1 - rand.Float64()*0.7,
			life:  confettiLife/2 + rand.IntN(confettiLife/2+1),
			glyph: lipgloss.NewStyle().Foreground(color).Bold(true).Render(glyph),
		}
	}
	return burst
}

// stepParticles returns the particles one frame later, without those that have
// expired or left the sides or bottom of the screen. A particle above the
// screen is kept, since gravity may bring it back.
func stepParticles(particles []particle, width, height int) []particle {
	var next []particle
	for _, p := range particles {
		p.x += p.vx
		p.y += p.vy
		p.vx *= drag
		p.vy += gravity
		p.life--
		if p.life > 0 && p.x >= 0 && p.x < float64(width) && p.y < float64(height) {
			next = append(next, p)
		}
	}
	return next
}

// overlayParticles draws particles on top of a finished view. A row is left
// alone for this frame if a particle would split a wide character, because
// that would push the panel borders out of line.
func overlayParticles(view string, particles []particle, width int) string {
	if len(particles) == 0 {
		return view
	}
	rows := map[int]map[int]string{}
	for _, p := range particles {
		x, y := int(math.Floor(p.x)), int(math.Floor(p.y))
		if rows[y] == nil {
			rows[y] = map[int]string{}
		}
		rows[y][x] = p.glyph
	}

	lines := strings.Split(view, "\n")
	for y, glyphs := range rows {
		if y < 0 || y >= len(lines) {
			continue
		}
		if line := overlayLine(lines[y], glyphs, width); lipgloss.Width(line) == width {
			lines[y] = line
		}
	}
	return strings.Join(lines, "\n")
}

func overlayLine(line string, glyphs map[int]string, width int) string {
	line += strings.Repeat(" ", max(0, width-lipgloss.Width(line))) // the footer may be short

	var s strings.Builder
	next := 0
	for _, x := range slices.Sorted(maps.Keys(glyphs)) {
		if x < 0 || x >= width {
			continue
		}
		s.WriteString(ansi.Cut(line, next, x) + glyphs[x])
		next = x + 1
	}
	s.WriteString(ansi.Cut(line, next, width))
	return s.String()
}

// doneFraction is the share of all tasks that are completed.
func (m model) doneFraction() float64 {
	total := m.tasks.Total()
	if total == 0 {
		return 0
	}
	return float64(len(m.tasks[board.Completed])) / float64(total)
}

// barMoving reports whether the progress bar is still catching up with the
// board. The bar waits behind the logo, so that it fills as the board opens.
func (m model) barMoving() bool {
	return m.animate && !m.splash && m.bar != m.doneFraction()
}

// viewProgress draws the progress bar and count, or nothing on an empty board.
func (m model) viewProgress() string {
	total := m.tasks.Total()
	if total == 0 {
		return ""
	}

	shown := m.bar
	if !m.animate {
		shown = m.doneFraction()
	}
	fill := shown * barWidth
	full := int(fill)
	partial := int((fill - float64(full)) * float64(len(barEdges)))

	var bar strings.Builder
	used := 0
	for ; used < full; used++ {
		bar.WriteString(barGradient.paint("█", used, used, barWidth, barWidth))
	}
	if partial > 0 && used < barWidth {
		bar.WriteString(barGradient.paint(barEdges[partial], used, used, barWidth, barWidth))
		used++
	}
	bar.WriteString(faintStyle.Render(strings.Repeat("░", barWidth-used)))

	count := fmt.Sprintf("%d/%d done", len(m.tasks[board.Completed]), total)
	return " " + bar.String() + " " + lipgloss.NewStyle().Foreground(colorTeal).Bold(true).Render(count) + " "
}

// withProgress puts the progress bar at the right end of a panel's bottom
// border.
func (m model) withProgress(panel string) string {
	progress := m.viewProgress()
	if progress == "" {
		return panel
	}
	lines := strings.Split(panel, "\n")
	last := lines[len(lines)-1]
	x := m.width - 2 - lipgloss.Width(progress)
	lines[len(lines)-1] = ansi.Cut(last, 0, x) + progress + ansi.Cut(last, x+lipgloss.Width(progress), m.width)
	return strings.Join(lines, "\n")
}

// updateSplash handles keys while the logo is showing. Only space and enter
// open the board, and q quits. Other keys are ignored, so that nothing happens
// to a task by accident.
func (m model) updateSplash(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case " ", "enter":
		m.splash = false
		return m.startAnimation() // the progress bar fills as the board opens
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

// viewSplash draws the logo, sweeping in from the left, in the middle of the
// screen.
func (m model) viewSplash() string {
	content := logoGradient.paint("lazydone", 0, 0, len("lazydone"), 2)
	if width := lipgloss.Width(logo[0]); m.width >= width {
		revealed := min(width, m.splashFrame*width/sweepFrames)
		lines := make([]string, len(logo))
		for y, row := range logo {
			var s strings.Builder
			for x, r := range []rune(row)[:revealed] {
				s.WriteString(logoGradient.paint(string(r), x, y, width, len(logo)))
			}
			lines[y] = s.String() + strings.Repeat(" ", width-revealed)
		}
		content = strings.Join(lines, "\n")
	}
	content = lipgloss.JoinVertical(lipgloss.Center, content, "", faintStyle.Render(m.version), "", hintStyle.Render(splashHint))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
