package console

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
)

// Handler is called for every submitted line.
// cmd is the dot-command name (no dot), args is everything after it.
// For plain text (no dot prefix) cmd is "say" and args is the full line.
type Handler func(cmd, args string) string

// Console wraps a readline instance.
type Console struct {
	rl      *readline.Instance
	handler Handler
}

// New creates a Console backed by historyFile.
// Pass empty string to default to ~/.bothistory.
//
// To add a new command (step 1 of 2):
// Append readline.PcItem(".yourcommand") to completions below.
// Then add a case in handleCommand in main.go (step 2).
func New(historyFile string, handler Handler) (*Console, error) {
	if historyFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		historyFile = filepath.Join(home, ".bothistory")
	}

	// Define available commands for both autocompletion and ghost-text
	cmdNames := []string{".say", ".inv", ".echest", ".hotbar", ".armor", ".forcekeeplist", ".dropspawners", ".pos", ".health", ".help", ".forcepearl", ".list", ".setlang", ".chat"}
	completions := make([]readline.PrefixCompleterInterface, len(cmdNames))
	for i, name := range cmdNames {
		switch name {
		case ".forcepearl":
			completions[i] = readline.PcItem(name,
				readline.PcItem("Juro5000"),
				readline.PcItem("Ruby81925"),
				readline.PcItem("EagleVigg978"),
			)
		case ".chat":
			completions[i] = readline.PcItem(name,
				readline.PcItem("normal"),
				readline.PcItem("anti_translate"),
			)
		default:
			completions[i] = readline.PcItem(name)
		}
	}

	c := &Console{handler: handler}

	rl, err := readline.NewEx(&readline.Config{
		Prompt:                 "> ",
		HistoryFile:            historyFile,
		HistoryLimit:           500,
		HistorySearchFold:      true,
		DisableAutoSaveHistory: true,
		AutoComplete:           readline.NewPrefixCompleter(completions...),
		InterruptPrompt:        "^C",
		EOFPrompt:              "exit",
		Painter:                &ghostPainter{cmds: cmdNames},
		Listener:               &ghostListener{con: c, cmds: cmdNames},
	})
	if err != nil {
		return nil, fmt.Errorf("console: readline init: %w", err)
	}

	c.rl = rl
	return c, nil
}

type ghostPainter struct {
	cmds []string
}

func (p *ghostPainter) Paint(line []rune, pos int) []rune {
	lineStr := string(line)
	if lineStr == "" || !strings.HasPrefix(lineStr, ".") || pos < len(line) {
		return line
	}

	suggestion := getSuggestion(lineStr, p.cmds)
	if suggestion == "" {
		return line
	}

	// Append suggestion in gray ANSI
	// After printing the suggestion, we use \033[%dD to move the cursor back 
	// to the original position so it doesn't stay at the end of the gray text.
	suggestRunes := []rune(fmt.Sprintf("\033[90m%s\033[0m\033[%dD", suggestion, len(suggestion)))
	return append(line, suggestRunes...)
}

type ghostListener struct {
	con  *Console
	cmds []string
}

func (g *ghostListener) OnChange(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
	// Handle special keys (Right Arrow or End) to accept a suggestion
	if (key == readline.CharForward || key == readline.CharLineEnd) && pos == len(line) {
		lineStr := string(line)
		suggestion := getSuggestion(lineStr, g.cmds)
		if suggestion != "" {
			combined := lineStr + suggestion
			return []rune(combined), len(combined), true
		}
	}
	return nil, 0, false
}

func getSuggestion(line string, cmds []string) string {
	if line == "" || !strings.HasPrefix(line, ".") {
		return ""
	}
	for _, cmd := range cmds {
		if strings.HasPrefix(cmd, line) && cmd != line {
			return cmd[len(line):]
		}
	}
	return ""
}

// Run blocks until EOF or Ctrl+C on empty line. Call in a goroutine.
func (c *Console) Run() {
	var lastLine string
	for {
		line, err := c.rl.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				return
			}
			continue
		}
		if err == io.EOF {
			return
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Avoid saving consecutive duplicate entries to history
		if trimmed != lastLine {
			_ = c.rl.SaveHistory(line)
			lastLine = trimmed
		}

		c.dispatch(trimmed)
	}
}

func (c *Console) dispatch(line string) {
	if !strings.HasPrefix(line, ".") {
		// Plain text (no dot) -> treated as a chat message
		if result := c.handler("say", line); result != "" {
			c.Println(result)
		}
		return
	}

	parts := strings.SplitN(strings.TrimPrefix(line, "."), " ", 2)
	cmd := parts[0]
	args := ""
	if len(parts) == 2 {
		args = strings.TrimSpace(parts[1])
	}

	if result := c.handler(cmd, args); result != "" {
		c.Println(result)
	}
}

// Println prints above the prompt without clobbering the input line.
func (c *Console) Println(msg string) {
	fmt.Fprintf(c.rl.Stdout(), "\033[K%s\n", msg)
	c.rl.Refresh()
}

// Writer returns an io.Writer safe for logger output.
func (c *Console) Writer() io.Writer { return &lineWriter{c} }

// Close releases readline resources.
func (c *Console) Close() { _ = c.rl.Close() }

type lineWriter struct{ con *Console }

func (w *lineWriter) Write(p []byte) (int, error) {
	if msg := strings.TrimSuffix(string(p), "\n"); msg != "" {
		w.con.Println(msg)
	}
	return len(p), nil
}

