package commands

import (
	"fmt"
	"strings"

	"jurobot/pkg/roles"
)

type BanCommand struct {
	Roles *roles.Store
}

func (c *BanCommand) Trigger() string     { return "*ban" }
func (c *BanCommand) Description() string { return "Ban a player (*ban <user>)" }
func (c *BanCommand) MCOnly() bool        { return false }

func (c *BanCommand) Execute(ctx *Context) {
	if c.Roles == nil || !c.Roles.IsOwner(ctx.Sender) {
		return
	}

	parts := strings.Fields(ctx.Message)
	if len(parts) < 2 {
		ctx.Reply("Usage: *ban <user>")
		return
	}

	target := parts[1]
	if c.Roles.IsOwner(target) {
		ctx.Reply("Can't ban the owner.")
		return
	}
	if err := c.Roles.Ban(target); err != nil {
		ctx.Reply(fmt.Sprintf("Error: %v", err))
		return
	}
	ctx.Reply(fmt.Sprintf("Banned %s", target))
}
