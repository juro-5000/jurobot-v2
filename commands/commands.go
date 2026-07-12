package commands

import (
	"jurobot/pkg/client"
	"jurobot/pkg/client/modules/chat"
)

// Context provides information about the command execution environment.
type Context struct {
	Client    *client.Client
	Sender    string
	Message   string
	Source    string // "mc", "discord", or "console"
	IsWhisper bool
}

// Reply sends a response. If the command was whispered, it whispers back.
func (ctx *Context) Reply(msg string) {
	ch := chat.From(ctx.Client)
	if ch == nil {
		return
	}
	if ctx.IsWhisper {
		ch.SendWhisper(ctx.Sender, msg)
	} else {
		ch.SendMessage(msg)
	}
}

// Command is the interface that all bot commands must implement.
type Command interface {
	Execute(ctx *Context)
	Trigger() string
	Description() string
	MCOnly() bool
}
