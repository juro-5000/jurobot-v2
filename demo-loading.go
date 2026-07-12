//go:build ignore

package main

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
)

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

func tipChar(distFromTip int) rune {
	chaosPool := []rune{'$', '%', '&', '@', '!', '?', '~', '*', '+', '='}
	if distFromTip >= 6 {
		return '#'
	}
	return chaosPool[rand.Intn(len(chaosPool))]
}

var logo = []string{
	"     ██╗██╗   ██╗██████╗  ██████╗ ██████╗  ██████╗ ████████╗",
	"     ██║██║   ██║██╔══██╗██╔═══██╗██╔══██╗██╔═══██╗╚══██╔══╝",
	"     ██║██║   ██║██████╔╝██║   ██║██████╔╝██║   ██║   ██║   ",
	"██   ██║██║   ██║██╔══██╗██║   ██║██╔══██╗██║   ██║   ██║   ",
	"╚█████╔╝╚██████╔╝██║  ██║╚██████╔╝██████╔╝╚██████╔╝   ██║   ",
	" ╚════╝  ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚═════╝  ╚═════╝   ╚═╝   ",
}

var steps = []struct {
	label  string
	detail string
}{
	{"LOADING TRANSLATIONS", "loaded 6722 translations · langua.json"},
	{"SELECTING ACCOUNT", "auto-selected: JuroBot5000 (prism launcher)"},
	{"AUTHENTICATING", "attempting login with refresh token..."},
	{"ENCRYPTING", "received encryption request"},
	{"SECURING", "encryption enabled · compression: 256"},
	{"CONNECTING", "login successful"},
	{"ENTERING WORLD", "login → configuration → play state"},
	{"READY", "spawned · ready"},
}

func main() {
	barWidth := 58
	tickMs := 32 * time.Millisecond
	fillRate := 1

	waveOffset := 0.0
	stepIdx := 0
	fillPct := 0

	for {
		fmt.Print("\033[H\033[J")

		for _, line := range logo {
			fmt.Println(colorLine(line, waveOffset))
		}
		fmt.Println(colorLine("— minecraft automation client —", waveOffset+80))
		fmt.Println()
		fmt.Println(colorLine(strings.Repeat("─", 64), waveOffset+160))
		fmt.Println()

		label := steps[stepIdx].label
		pctStr := fmt.Sprintf("%d%%", fillPct)
		gap := 64 - len(label) - len(pctStr)
		if gap < 1 {
			gap = 1
		}
		fmt.Println(colorLine(label+strings.Repeat(" ", gap)+pctStr, waveOffset+40))
		fmt.Println()

		fmt.Println(colorLine("╔"+strings.Repeat("═", barWidth+2)+"╗", waveOffset+60))

		filled := (fillPct * barWidth) / 100
		empty := barWidth - filled
		var inner strings.Builder
		inner.WriteString("║ ")
		for i := 0; i < filled; i++ {
			dist := filled - 1 - i
			if dist < 8 {
				inner.WriteRune(tipChar(dist))
			} else {
				inner.WriteRune('#')
			}
		}
		inner.WriteString(strings.Repeat(" ", empty))
		inner.WriteString(" ║")
		fmt.Println(colorLine(inner.String(), waveOffset+60))
		fmt.Println(colorLine("╚"+strings.Repeat("═", barWidth+2)+"╝", waveOffset+60))
		fmt.Println()

		fmt.Println(colorLine("> "+steps[stepIdx].detail, waveOffset+120))

		waveOffset += 1.5
		time.Sleep(tickMs)

		targetPct := (stepIdx + 1) * 100 / len(steps)
		if fillPct < targetPct {
			fillPct += fillRate
			if fillPct > targetPct {
				fillPct = targetPct
			}
		}

		if fillPct >= 100 && stepIdx >= len(steps)-1 {
			time.Sleep(500 * time.Millisecond)
			fmt.Print("\033[H\033[J")
			fmt.Println("spawned; ready")
			return
		}

		if fillPct >= targetPct && stepIdx < len(steps)-1 {
			stepIdx++
		}
	}
}
