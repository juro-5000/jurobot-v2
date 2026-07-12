package commands

import (
	"fmt"
	"sync"
	"time"

	"jurobot/pkg/roles"
)

type endUsage struct {
	count    int
	lastUsed time.Time
}

type EndCommand struct {
	Roles *roles.Store
	mu    sync.Mutex
	usage map[string]*endUsage
}

func (c *EndCommand) Trigger() string     { return "*end" }
func (c *EndCommand) Description() string { return "Reconnect the bot." }
func (c *EndCommand) MCOnly() bool        { return false }

type endLimit struct {
	maxPerDay int
	cooldown  time.Duration
}

func (c *EndCommand) getLimit(sender string) endLimit {
	if c.Roles.IsOwner(sender) {
		return endLimit{maxPerDay: -1, cooldown: 0}
	}
	if c.Roles.HasRole(sender, "admin") {
		return endLimit{maxPerDay: 100, cooldown: 5 * time.Minute}
	}
	if c.Roles.HasRole(sender, "trusted") {
		return endLimit{maxPerDay: 10, cooldown: 10 * time.Minute}
	}
	return endLimit{maxPerDay: 0, cooldown: 0}
}

func (c *EndCommand) Execute(ctx *Context) {
	limit := c.getLimit(ctx.Sender)
	if limit.maxPerDay == 0 {
		return
	}

	c.mu.Lock()
	if c.usage == nil {
		c.usage = make(map[string]*endUsage)
	}
	u, ok := c.usage[ctx.Sender]
	if !ok {
		u = &endUsage{}
		c.usage[ctx.Sender] = u
	}

	now := time.Now()

	if now.Sub(u.lastUsed) > 24*time.Hour {
		u.count = 0
	}

	if !u.lastUsed.IsZero() && now.Sub(u.lastUsed) < limit.cooldown {
		remaining := limit.cooldown - now.Sub(u.lastUsed)
		mins := int(remaining.Minutes())
		secs := int(remaining.Seconds()) % 60
		c.mu.Unlock()
		ctx.Reply(fmt.Sprintf("Wait %dm %ds", mins, secs))
		return
	}

	if limit.maxPerDay > 0 && u.count >= limit.maxPerDay {
		c.mu.Unlock()
		ctx.Reply("Daily limit reached.")
		return
	}

	u.count++
	u.lastUsed = now
	c.mu.Unlock()

	ctx.Reply("Reconnecting...")
	ctx.Client.Disconnect(false)
}
