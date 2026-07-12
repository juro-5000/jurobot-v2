package plugins

import (
	"jurobot/pkg/client"
)

// Plugin is the base interface for all bot plugins.
type Plugin interface {
	Name() string
	Init(c *client.Client)
}

// DatabasePlugin placeholder for future implementation
type DatabasePlugin struct{}

func (p *DatabasePlugin) Name() string     { return "database" }
func (p *DatabasePlugin) Init(c *client.Client) {
	// TODO: initialize database connection
}

// DiscordPlugin placeholder for future implementation
type DiscordPlugin struct{}

func (p *DiscordPlugin) Name() string     { return "discord" }
func (p *DiscordPlugin) Init(c *client.Client) {
	// TODO: initialize discord bot
}
