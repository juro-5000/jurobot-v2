package commands

import (
	"strings"
	"sync"

	"jurobot/pkg/client/modules/chat"
)

type ChatCommand struct {
	Mode     *string
	ModeMu   *sync.RWMutex
	Lang     *string
	LangMu   *sync.RWMutex
}

func (c *ChatCommand) Trigger() string          { return "chat" }
func (c *ChatCommand) Description() string      { return "Sets chat mode (.chat normal | anti_translate)" }
func (c *ChatCommand) AllowedSenders() []string { return nil }

func (c *ChatCommand) Execute(ctx *Context) {
	if ctx.Sender != "[console]" && ctx.Sender != ctx.Client.Username {
		return
	}
	parts := strings.Fields(ctx.Message)
	if len(parts) < 2 {
		return
	}
	method := strings.ToLower(parts[len(parts)-1])
	ch := chat.From(ctx.Client)
	if ch == nil {
		return
	}
	if method != "normal" && method != "default" && method != "off" && method != "anti_translate" {
		ctx.Client.Logger.Printf("Unknown chat mode: %s (use normal or anti_translate)", method)
		return
	}
	c.ModeMu.Lock()
	c.LangMu.Lock()
	if method == "normal" || method == "default" || method == "off" {
		*c.Mode = "normal"
		*c.Lang = ""
		c.LangMu.Unlock()
		c.ModeMu.Unlock()
		ch.SendMessage("Chat mode set to normal")
		return
	}
	*c.Mode = "anti_translate"
	*c.Lang = ""
	c.LangMu.Unlock()
	c.ModeMu.Unlock()
	ch.SendMessage("Chat mode set to anti_translate")
}
