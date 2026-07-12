package commands

import (
	"strings"

	"jurobot/pkg/roles"
)

type HelpCommand struct {
	CommandsFunc func() []Command
	Roles        *roles.Store
}

func (c *HelpCommand) Trigger() string     { return "*help" }
func (c *HelpCommand) Description() string { return "Shows all available commands." }
func (c *HelpCommand) MCOnly() bool        { return false }

func (c *HelpCommand) Execute(ctx *Context) {
	cmds := c.CommandsFunc()
	if len(cmds) == 0 {
		ctx.Reply("No commands registered.")
		return
	}

	var names []string
	for _, cmd := range cmds {
		if cmd.MCOnly() && ctx.Source != "mc" {
			continue
		}
		// Role-based permission check
		if c.Roles != nil {
			plain := strings.TrimPrefix(cmd.Trigger(), "*")
			if ctx.Source == "discord" {
				// Discord always gets member perms
				if !c.Roles.RoleCanUseCommand("member", plain) {
					continue
				}
			} else if ctx.Source == "mc" {
				if !c.Roles.CanUseCommand(ctx.Sender, plain) {
					continue
				}
			}
		}
		names = append(names, cmd.Trigger())
	}

	msg := strings.Join(names, " | ")
	if msg == "" {
		msg = "No commands available."
	}
	ctx.Reply(msg)
}
