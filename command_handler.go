package main

import (
	"strings"
	"sync"
	"time"

	"jurobot/commands"
	"jurobot/pkg/client"
	"jurobot/pkg/roles"
)

type CommandHandler struct {
	client   *client.Client
	commands map[string]commands.Command
	roles    *roles.Store
	mu       sync.RWMutex

	lastExecuted map[string]time.Time
	cooldown     time.Duration
}

func NewCommandHandler(c *client.Client, roleStore *roles.Store) *CommandHandler {
	return &CommandHandler{
		client:       c,
		commands:     make(map[string]commands.Command),
		roles:        roleStore,
		lastExecuted: make(map[string]time.Time),
		cooldown:     5 * time.Second,
	}
}

func (h *CommandHandler) Register(cmd commands.Command) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.commands[cmd.Trigger()] = cmd
}

func (h *CommandHandler) AllCommands() []commands.Command {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var cmds []commands.Command
	for _, cmd := range h.commands {
		cmds = append(cmds, cmd)
	}
	return cmds
}

func (h *CommandHandler) Handle(sender, message, source string, isWhisper bool) {
	msg := strings.TrimSpace(message)

	// Ban check — banned users get no response at all
	if h.roles != nil && h.roles.IsBanned(sender) {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for trigger, cmd := range h.commands {
		plain := strings.TrimPrefix(trigger, "*")
		if !matchTrigger(msg, trigger) && !matchTrigger(msg, plain) && !matchTrigger(msg, "juro:"+plain) {
			continue
		}

		// Discord users get member perms only — names can be spoofed
		if source == "discord" && h.roles != nil && !h.roles.RoleCanUseCommand("member", plain) {
			return
		}

		// MC-only check
		if cmd.MCOnly() && source != "mc" {
			return
		}

		// Role-based permission check (only for in-game MC, not Discord)
		if source == "mc" && h.roles != nil && !h.roles.CanUseCommand(sender, plain) {
			return
		}

		now := time.Now()
		if last, ok := h.lastExecuted[trigger]; ok && now.Sub(last) < h.cooldown {
			return
		}
		h.lastExecuted[trigger] = now

		ctx := &commands.Context{
			Client:    h.client,
			Sender:    sender,
			Message:   msg,
			Source:    source,
			IsWhisper: isWhisper,
		}
		cmd.Execute(ctx)
		return
	}
}

// matchTrigger checks if the message starts with the trigger as the first word.
func matchTrigger(msg, trigger string) bool {
	if trigger == "" {
		return false
	}
	first := strings.Fields(msg)
	if len(first) == 0 {
		return false
	}
	word := strings.Trim(first[0], ".,!?;:")
	return word == trigger
}
