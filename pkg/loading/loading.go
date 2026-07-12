// Package loading provides a Bubbletea-based terminal loading screen that
// advances through real startup events rather than fake timers.
package loading

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ─── Event type ───────────────────────────────────────────────────────────────

// StepID identifies which loading step has just been reached.
type StepID int

const (
	StepTranslations StepID = iota // langua.json loaded
	StepAccount                    // account auto-selected
	StepTunnel                     // cloudflared tunnel started
	StepAuthenticating             // MS auth / refresh token exchange started
	StepEncrypting                 // server sent encryption request
	StepSecuring                   // compression threshold received
	StepConnecting                 // S2CLoginFinished — login successful
	StepEnteringWorld              // configuration → play state
	StepReady                      // spawned · ready
)

// ─── ANSI helpers ─────────────────────────────────────────────────────────────

func hueToRGB(hue float64) (int, int, int) {
	hue = math.Mod(hue, 360)
	if hue < 0 {
		hue += 360
	}
	h := hue / 60.0
	x := 1.0 - math.Abs(math.Mod(h, 2)-1)
	var r, g, b float64
	switch {
	case h < 1:
		r, g, b = 1, x, 0
	case h < 2:
		r, g, b = x, 1, 0
	case h < 3:
		r, g, b = 0, 1, x
	case h < 4:
		r, g, b = 0, x, 1
	case h < 5:
		r, g, b = x, 0, 1
	default:
		r, g, b = 1, 0, x
	}
	return int(r * 255), int(g * 255), int(b * 255)
}

func colorLine(line string, offset float64) string {
	var sb strings.Builder
	runes := []rune(line)
	n := len(runes)
	if n == 0 {
		return ""
	}
	for i, ch := range runes {
		hue := offset + float64(i)/float64(n)*200.0
		r, g, b := hueToRGB(hue)
		fmt.Fprintf(&sb, "\033[38;2;%d;%d;%dm%c\033[0m", r, g, b, ch)
	}
	return sb.String()
}

// ─── ASCII logo ───────────────────────────────────────────────────────────────

var logo = []string{
	"     ██╗██╗   ██╗██████╗  ██████╗ ██████╗  ██████╗ ████████╗",
	"     ██║██║   ██║██╔══██╗██╔═══██╗██╔══██╗██╔═══██╗╚══██╔══╝",
	"     ██║██║   ██║██████╔╝██║   ██║██████╔╝██║   ██║   ██║   ",
	"██   ██║██║   ██║██╔══██╗██║   ██║██╔══██╗██║   ██║   ██║   ",
	"╚█████╔╝╚██████╔╝██║  ██║╚██████╔╝██████╔╝╚██████╔╝   ██║   ",
	" ╚════╝  ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚═════╝  ╚═════╝   ╚═╝   ",
}

// ─── Step metadata ────────────────────────────────────────────────────────────

type stepMeta struct {
	label  string
	detail string
}

var stepsMeta = []stepMeta{
	{label: "LOADING TRANSLATIONS", detail: "loaded 6722 translations · langua.json"},
	{label: "SELECTING ACCOUNT", detail: "auto-selected: JuroBot5000 (prism launcher)"},
	{label: "STARTING TUNNEL", detail: "starting cloudflared tunnel..."},
	{label: "AUTHENTICATING", detail: "attempting login with refresh token..."},
	{label: "ENCRYPTING", detail: "received encryption request"},
	{label: "SECURING", detail: "encryption enabled · compression: 256"},
	{label: "CONNECTING", detail: "login successful"},
	{label: "ENTERING WORLD", detail: "login → configuration → play state"},
	{label: "READY", detail: "spawned · ready"},
}

// ─── Chaos tip ────────────────────────────────────────────────────────────────

var chaosPool = []rune{'$', '%', '&', '@', '!', '?', '~', '*', '+', '='}

const tipLen = 8

func tipChar(distFromTip int) rune {
	if distFromTip >= 6 {
		return '#'
	}
	return chaosPool[rand.Intn(len(chaosPool))]
}

// ─── Bubbletea messages ───────────────────────────────────────────────────────

type tickMsg time.Time
type stepAdvanceMsg StepID
type allDoneMsg struct{}

// ─── Model ────────────────────────────────────────────────────────────────────

const (
	barWidth = 58
	tickMs   = 32 // Slowed down from 16
	fillRate = 1  // Slowed down from 2
)

type Model struct {
	stepIdx    int
	fillPct    int
	waveOffset float64
	detail     string
	events     <-chan StepID
	doneCh     chan<- struct{}
	width      int
	height     int
}

func newModel(events <-chan StepID, doneCh chan<- struct{}) Model {
	return Model{
		events: events,
		doneCh: doneCh,
		detail: stepsMeta[0].detail,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tick(), m.waitForEvent())
}

func tick() tea.Cmd {
	return tea.Tick(time.Duration(tickMs)*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) waitForEvent() tea.Cmd {
	return func() tea.Msg {
		id, ok := <-m.events
		if !ok {
			return nil
		}
		return stepAdvanceMsg(id)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = v.Width
		m.height = v.Height
		return m, nil

	case tea.KeyMsg:
		if v.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

	case stepAdvanceMsg:
		nextIdx := int(v)
		if nextIdx > m.stepIdx {
			m.stepIdx = nextIdx
			if m.stepIdx < len(stepsMeta) {
				m.detail = stepsMeta[m.stepIdx].detail
			}
		}

		if m.stepIdx >= len(stepsMeta)-1 {
			// Stay at READY step until fillPct hits 100
		}

		return m, m.waitForEvent()

	case tickMsg:
		m.waveOffset += 1.5 // Slowed down from 4.0

		targetPct := (m.stepIdx + 1) * 100 / len(stepsMeta)
		if m.fillPct < targetPct {
			m.fillPct += fillRate
			if m.fillPct > targetPct {
				m.fillPct = targetPct
			}
		}

		if m.fillPct == 100 && m.stepIdx >= len(stepsMeta)-1 {
			// We reached the end. Wait a few ticks so the user sees 100%.
			return m, tea.Tick(time.Millisecond*300, func(t time.Time) tea.Msg {
				return allDoneMsg{}
			})
		}

		return m, tick()

	case allDoneMsg:
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	var sb strings.Builder

	// The content width is approx 64 characters.
	contentWidth := 64
	
	// Prepare lines
	var lines []string

	for _, line := range logo {
		lines = append(lines, colorLine(line, m.waveOffset))
	}

	lines = append(lines, colorLine("— minecraft automation client —", m.waveOffset+80))
	lines = append(lines, "")
	lines = append(lines, colorLine(strings.Repeat("─", 64), m.waveOffset+160))
	lines = append(lines, "")

	label := "INITIALIZING"
	if m.stepIdx < len(stepsMeta) {
		label = stepsMeta[m.stepIdx].label
	}
	pct := fmt.Sprintf("%d%%", m.fillPct)
	gap := 64 - len(label) - len(pct)
	if gap < 1 {
		gap = 1
	}
	lines = append(lines, colorLine(label+strings.Repeat(" ", gap)+pct, m.waveOffset+40))
	lines = append(lines, "")

	lines = append(lines, colorLine("╔"+strings.Repeat("═", barWidth+2)+"╗", m.waveOffset+60))
	
	filled := (m.fillPct * barWidth) / 100
	empty := barWidth - filled
	var inner strings.Builder
	inner.WriteString("║ ")
	for i := 0; i < filled; i++ {
		dist := filled - 1 - i
		if dist < tipLen {
			inner.WriteRune(tipChar(dist))
		} else {
			inner.WriteRune('#')
		}
	}
	inner.WriteString(strings.Repeat(" ", empty))
	inner.WriteString(" ║")
	lines = append(lines, colorLine(inner.String(), m.waveOffset+60))
	lines = append(lines, colorLine("╚"+strings.Repeat("═", barWidth+2)+"╝", m.waveOffset+60))
	lines = append(lines, "")

	detail := m.detail
	if detail == "" && m.stepIdx < len(stepsMeta) {
		detail = stepsMeta[m.stepIdx].detail
	}
	lines = append(lines, colorLine("> "+detail, m.waveOffset+120))

	// Center vertically
	topPadding := (m.height - len(lines)) / 2
	if topPadding < 0 {
		topPadding = 0
	}
	sb.WriteString(strings.Repeat("\n", topPadding))

	// Center horizontally and join
	leftPadding := (m.width - contentWidth) / 2
	if leftPadding < 0 {
		leftPadding = 0
	}
	pad := strings.Repeat(" ", leftPadding)

	for _, line := range lines {
		sb.WriteString(pad)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// Fill the rest of the height to prevent ghosting on shrink
	bottomPadding := m.height - topPadding - len(lines)
	if bottomPadding > 0 {
		sb.WriteString(strings.Repeat("\n", bottomPadding))
	}

	return sb.String()
}

// ─── Public API ───────────────────────────────────────────────────────────────

// Screen manages the lifecycle of the loading animation.
type Screen struct {
	events chan StepID
	done   chan struct{}
	prog   *tea.Program
}

// New creates and starts the loading screen in the background.
// If no TTY is available (e.g. piped or non-interactive), the loading
// screen is skipped entirely and Advance/Wait/Close become no-ops.
func New() *Screen {
	events := make(chan StepID, 16)
	doneCh := make(chan struct{})

	// Check whether a TTY is available; Bubbletea requires one.
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		// No TTY — close the file (which we couldn't open) and return a
		// lightweight no-op screen that immediately signals "done".
		s := &Screen{events: events, done: doneCh, prog: nil}
		close(doneCh)
		return s
	}
	tty.Close()

	m := newModel(events, nil) // model doesn't need doneCh anymore
	p := tea.NewProgram(m, tea.WithAltScreen())

	s := &Screen{events: events, done: doneCh, prog: p}

	go func() {
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running loading screen: %v\n", err)
		}
		// NOW close it: only after p.Run() has returned and terminal is restored
		close(doneCh)
	}()

	return s
}

// Advance signals that the given step has just been reached.
// Safe to call from any goroutine.
func (s *Screen) Advance(id StepID) {
	select {
	case s.events <- id:
	default:
	}
}

// Wait blocks until the loading screen has finished (READY bar complete).
func (s *Screen) Wait() {
	<-s.done
}

// Close stops the loading screen and cleans up.
func (s *Screen) Close() {
	if s.prog != nil {
		s.prog.Quit()
	}
}

