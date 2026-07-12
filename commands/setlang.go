package commands

import (
	"fmt"
	"strings"
	"sync"

	"jurobot/pkg/client/modules/chat"
)

type SetLangCommand struct {
	Lang       *string
	LangMu     *sync.RWMutex
	ChatMode   *string
	ChatModeMu *sync.RWMutex
}

func (c *SetLangCommand) Trigger() string          { return "setlang" }
func (c *SetLangCommand) Description() string      { return "Sets auto-translate language (.setlang <lang> | off)" }
func (c *SetLangCommand) AllowedSenders() []string { return nil }

func (c *SetLangCommand) Execute(ctx *Context) {
	if ctx.Sender != "[console]" && ctx.Sender != ctx.Client.Username {
		return
	}
	parts := strings.Fields(ctx.Message)
	if len(parts) < 2 {
		return
	}
	lang := strings.ToLower(parts[len(parts)-1])
	ch := chat.From(ctx.Client)
	if ch == nil {
		return
	}
	c.LangMu.Lock()
	c.ChatModeMu.Lock()
	if lang == "off" || lang == "default" {
		*c.Lang = ""
		*c.ChatMode = "normal"
		c.ChatModeMu.Unlock()
		c.LangMu.Unlock()
		ch.SendMessage("Auto-translate disabled")
		return
	}
	*c.Lang = lang
	*c.ChatMode = "normal"
	c.ChatModeMu.Unlock()
	c.LangMu.Unlock()
	ch.SendMessage(fmt.Sprintf("Auto-translate set to %s", lang))
}
