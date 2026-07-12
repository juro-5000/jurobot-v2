package commands

import (
	"fmt"
	"strings"

	"jurobot/pkg/client/modules/playerlist"
)

type ListCommand struct{}

func (c *ListCommand) Trigger() string     { return "*list" }
func (c *ListCommand) Description() string { return "Lists online players." }
func (c *ListCommand) MCOnly() bool        { return false }

func (c *ListCommand) Execute(ctx *Context) {
	plist := playerlist.From(ctx.Client)
	if plist == nil {
		ctx.Reply("playerlist not available yet")
		return
	}

	players := plist.GetAllPlayers()
	if len(players) == 0 {
		ctx.Reply("no players online")
		return
	}

	var entries []string
	for _, p := range players {
		ping := p.Ping
		if ping < 0 {
			ping = 0
		}
		entries = append(entries, fmt.Sprintf("%s (%dms)", p.Name, ping))
	}

	msg := fmt.Sprintf("Online (%d): %s", len(players), strings.Join(entries, ", "))
	if len(msg) > 256 {
		msg = msg[:253] + "..."
	}
	ctx.Reply(msg)
}
