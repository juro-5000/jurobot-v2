package configui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"jurobot/pkg/config"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type fieldType int

const (
	fieldSection fieldType = iota
	fieldString
	fieldInt
	fieldBool
	fieldFloat
)

type field struct {
	label string
	typ   fieldType
	value interface{} // *string, *int, *bool
	input textinput.Model
}

type Model struct {
	fields   []field
	focused  int
	cfg      *config.Config
	width    int
	height   int
	saved    bool
	quitting bool
	errMsg   string
}

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

var logo = []string{
	"     ██╗██╗   ██╗██████╗  ██████╗ ██████╗  ██████╗ ████████╗",
	"     ██║██║   ██║██╔══██╗██╔═══██╗██╔══██╗██╔═══██╗╚══██╔══╝",
	"     ██║██║   ██║██████╔╝██║   ██║██████╔╝██║   ██║   ██║   ",
	"██   ██║██║   ██║██╔══██╗██║   ██║██╔══██╗██║   ██║   ██║   ",
	"╚█████╔╝╚██████╔╝██║  ██║╚██████╔╝██████╔╝╚██████╔╝   ██║   ",
	" ╚════╝  ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚═════╝  ╚═════╝   ╚═╝   ",
}

func makeField(label string, typ fieldType, value interface{}) field {
	if typ == fieldSection {
		return field{label: label, typ: fieldSection}
	}

	ti := textinput.New()
	ti.CharLimit = 128
	ti.Width = 40

	switch typ {
	case fieldString:
		ti.SetValue(*(value.(*string)))
	case fieldInt:
		ti.SetValue(fmt.Sprintf("%d", *value.(*int)))
	case fieldBool:
		ti.SetValue(fmt.Sprintf("%t", *value.(*bool)))
	case fieldFloat:
		ti.SetValue(fmt.Sprintf("%v", *value.(*float64)))
	}

	return field{
		label: label,
		typ:   typ,
		value: value,
		input: ti,
	}
}

func New(cfg *config.Config) Model {
	fields := []field{
		makeField("SERVER", fieldSection, nil),
		makeField("Server Address", fieldString, &cfg.Server.Address),
		makeField("Server Port", fieldInt, &cfg.Server.Port),

		makeField("DEBUG", fieldSection, nil),
		makeField("Debug Enabled", fieldBool, &cfg.Debug.Enabled),
		makeField("Show Chat Packets", fieldBool, &cfg.Debug.ShowChatPackets),
		makeField("Show Raw Hex", fieldBool, &cfg.Debug.ShowRawHex),
		makeField("Show JSON", fieldBool, &cfg.Debug.ShowJSON),

		makeField("TRANSLATION", fieldSection, nil),
		makeField("Translation Enabled", fieldBool, &cfg.Translation.Enabled),
		makeField("Target Language", fieldString, &cfg.Translation.TargetLang),
		makeField("Verbose", fieldBool, &cfg.Translation.Verbose),

		makeField("SORTER", fieldSection, nil),
		makeField("Auto Ender Chest", fieldBool, &cfg.Sorter.AutoEchest),

		makeField("COMBAT", fieldSection, nil),
		makeField("Auto Eat", fieldBool, &cfg.Combat.AutoEat),
		makeField("Auto Totem", fieldBool, &cfg.Combat.AutoTotem),
		makeField("Auto Armor", fieldBool, &cfg.Combat.AutoArmor),

		makeField("KEEPALIVE", fieldSection, nil),
		makeField("Keepalive Enabled", fieldBool, &cfg.Keepalive.Enabled),
		makeField("Health Threshold", fieldFloat, &cfg.Keepalive.HealthThreshold),
		makeField("Void Y Threshold", fieldFloat, &cfg.Keepalive.VoidYThreshold),

		makeField("ACCOUNT", fieldSection, nil),
		makeField("Username", fieldString, &cfg.Account.Username),
		makeField("Owner Username", fieldString, &cfg.Account.OwnerUsername),
		makeField("Refresh Token", fieldString, &cfg.Account.RefreshToken),
	}

	focused := 0
	for i, f := range fields {
		if f.typ != fieldSection {
			focused = i
			break
		}
	}

	return Model{
		fields:  fields,
		focused: focused,
		cfg:     cfg,
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = v.Width
		m.height = v.Height
		return m, nil

	case tea.KeyMsg:
		switch v.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			if v.Type == tea.KeyEsc {
				m.quitting = true
				return m, tea.Quit
			}
			m.quitting = true
			return m, tea.Quit

		case tea.KeyCtrlS, tea.KeyF10:
			m.saved = true
			return m, tea.Quit

		case tea.KeyTab, tea.KeyDown:
			if m.fields[m.focused].typ == fieldBool {
				m.fields[m.focused].input.Blur()
			}
			for i := 0; i < len(m.fields); i++ {
				m.focused = (m.focused + 1) % len(m.fields)
				if m.fields[m.focused].typ != fieldSection {
					break
				}
			}
			if m.fields[m.focused].typ == fieldString || m.fields[m.focused].typ == fieldInt || m.fields[m.focused].typ == fieldFloat {
				m.fields[m.focused].input.Focus()
			}
			return m, nil

		case tea.KeyUp:
			if m.fields[m.focused].typ == fieldBool {
				m.fields[m.focused].input.Blur()
			}
			for i := 0; i < len(m.fields); i++ {
				m.focused = (m.focused - 1 + len(m.fields)) % len(m.fields)
				if m.fields[m.focused].typ != fieldSection {
					break
				}
			}
			if m.fields[m.focused].typ == fieldString || m.fields[m.focused].typ == fieldInt || m.fields[m.focused].typ == fieldFloat {
				m.fields[m.focused].input.Focus()
			}
			return m, nil

		case tea.KeyEnter:
			f := &m.fields[m.focused]
			switch f.typ {
			case fieldBool:
				val := f.value.(*bool)
				*val = !*val
				f.input.SetValue(fmt.Sprintf("%t", *val))
			case fieldSection:
				// skip
			case fieldString, fieldInt, fieldFloat:
			}
			return m, nil

		case tea.KeySpace:
			f := &m.fields[m.focused]
			if f.typ == fieldBool {
				val := f.value.(*bool)
				*val = !*val
				f.input.SetValue(fmt.Sprintf("%t", *val))
			}
			return m, nil
		}
	}

	if m.focused >= 0 && m.focused < len(m.fields) {
		f := &m.fields[m.focused]
		if f.typ == fieldString || f.typ == fieldInt || f.typ == fieldFloat {
			if f.value == nil {
				return m, nil
			}
			var cmd tea.Cmd
			f.input, cmd = f.input.Update(msg)
			switch f.typ {
			case fieldInt:
				v := strings.TrimSpace(f.input.Value())
				if v == "" {
					v = "0"
				}
				if n, err := strconv.Atoi(v); err == nil {
					*f.value.(*int) = n
				} else {
					m.errMsg = fmt.Sprintf("Invalid number: %s", v)
				}
			case fieldFloat:
				v := strings.TrimSpace(f.input.Value())
				if v == "" {
					v = "0"
				}
				if n, err := strconv.ParseFloat(v, 64); err == nil {
					*f.value.(*float64) = n
				} else {
					m.errMsg = fmt.Sprintf("Invalid number: %s", v)
				}
			default:
				*f.value.(*string) = f.input.Value()
			}
			return m, cmd
		}
	}

	return m, nil
}

func sectionHeader(name string) string {
	const totalWidth = 60
	left := (totalWidth - len(name) - 2) / 2
	if left < 1 {
		left = 1
	}
	right := totalWidth - left*2 - len(name)
	if right < 0 {
		right = 0
	}
	return strings.Repeat("─", left) + " " + name + " " + strings.Repeat("─", right)
}

func renderBool(val bool) string {
	if val {
		return "\033[92mtrue\033[0m"
	}
	return "\033[91mfalse\033[0m"
}

func (m Model) View() string {
	var sb strings.Builder

	if m.width == 0 || m.height == 0 {
		m.width = 80
		m.height = 40
	}

	waveOffset := float64(0)

	// Logo
	for _, line := range logo {
		sb.WriteString(colorLine(line, waveOffset))
		sb.WriteString("\n")
	}
	sb.WriteString(colorLine("— config editor —", waveOffset+80))
	sb.WriteString("\n")
	sb.WriteString(colorLine(strings.Repeat("─", 64), waveOffset+160))
	sb.WriteString("\n\n")

	// Fields
	for i, f := range m.fields {
		if f.typ == fieldSection {
			sb.WriteString(colorLine(sectionHeader(f.label), waveOffset+40))
			sb.WriteString("\n\n")
			continue
		}

		// Field row
		prefix := "  "
		if m.focused == i {
			prefix = "\033[38;2;255;200;100m› \033[0m"
		}

		label := f.label
		padding := 25 - len(label)
		if padding < 1 {
			padding = 1
		}

		sb.WriteString(prefix)
		if m.focused == i {
			sb.WriteString("\033[38;2;255;200;100m")
		}
		sb.WriteString(label)
		if m.focused == i {
			sb.WriteString("\033[0m")
		}
		sb.WriteString(strings.Repeat(" ", padding))

		switch f.typ {
		case fieldBool:
			val := *f.value.(*bool)
			sb.WriteString(renderBool(val))
		case fieldString, fieldInt, fieldFloat:
			if m.focused == i {
				sb.WriteString(f.input.View())
			} else {
				val := f.input.Value()
				sb.WriteString("\033[38;2;150;200;255m" + val + "\033[0m")
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(colorLine(strings.Repeat("─", 64), waveOffset+200))
	sb.WriteString("\n")
	sb.WriteString(colorLine("  Ctrl+S: Save  |  Esc: Cancel  |  ↑/↓: Navigate  |  Enter/Space: Toggle", waveOffset+120))
	sb.WriteString("\n")

	if m.errMsg != "" {
		sb.WriteString("\n  \033[91m" + m.errMsg + "\033[0m\n")
	}

	return sb.String()
}

func (m Model) Saved() bool {
	return m.saved
}

func (m Model) Config() config.Config {
	return *m.cfg
}

func Run(cfg *config.Config) (bool, error) {
	m := New(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return false, err
	}
	model := final.(Model)
	return model.Saved(), nil
}
