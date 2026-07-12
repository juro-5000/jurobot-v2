package commands

import (
	"fmt"
	"strings"

	"jurobot/pkg/roles"
)

type UnbanCommand struct {
	Roles *roles.Store
}

func (c *UnbanCommand) Trigger() string     { return "*unban" }
func (c *UnbanCommand) Description() string { return "Unban a player (*unban <user>)" }
func (c *UnbanCommand) MCOnly() bool        { return false }

func (c *UnbanCommand) Execute(ctx *Context) {
	if c.Roles == nil || !c.Roles.IsOwner(ctx.Sender) {
		return
	}

	parts := strings.Fields(ctx.Message)
	if len(parts) < 2 {
		ctx.Reply("Usage: *unban <user>")
		return
	}

	target := parts[1]
	if err := c.Roles.Unban(target); err != nil {
		ctx.Reply(fmt.Sprintf("Error: %v", err))
		return
	}
	ctx.Reply(fmt.Sprintf("Unbanned %s", target))
}
