package commands

import (
	"fmt"
	"strings"

	"jurobot/pkg/roles"
)

var adminCanGive = []string{"trusted"}

type RoleCommand struct {
	Roles *roles.Store
}

func (c *RoleCommand) Trigger() string     { return "*role" }
func (c *RoleCommand) Description() string { return "Manage roles (*role add <user> <role> | *role remove <user> <role> | *role list)" }
func (c *RoleCommand) MCOnly() bool        { return false }

func (c *RoleCommand) Execute(ctx *Context) {
	if c.Roles == nil {
		return
	}

	parts := strings.Fields(ctx.Message)
	if len(parts) < 2 {
		ctx.Reply("*role add <user> <role> | *role remove <user> <role> | *role list")
		return
	}

	sub := strings.ToLower(parts[1])
	switch sub {
	case "add", "give":
		if len(parts) < 4 {
			ctx.Reply("Usage: *role add <user> <role>")
			return
		}
		target := parts[2]
		newRole := parts[3]

		if c.Roles.IsOwner(ctx.Sender) {
			// owner can give anything
		} else if c.Roles.HasRole(ctx.Sender, "admin") {
			allowed := false
			for _, r := range adminCanGive {
				if r == newRole {
					allowed = true
					break
				}
			}
			if !allowed {
				ctx.Reply("Admins can only give the trusted role.")
				return
			}
		} else {
			ctx.Reply("Only owner/admin can manage roles.")
			return
		}

		if err := c.Roles.AddRole(target, newRole); err != nil {
			ctx.Reply(fmt.Sprintf("Error: %v", err))
			return
		}
		ctx.Reply(fmt.Sprintf("Added role %s to %s", newRole, target))

	case "remove", "revoke":
		if len(parts) < 4 {
			ctx.Reply("Usage: *role remove <user> <role>")
			return
		}
		target := parts[2]
		removeRole := parts[3]

		if c.Roles.IsOwner(ctx.Sender) {
			// owner can remove anything
		} else if c.Roles.HasRole(ctx.Sender, "admin") {
			allowed := false
			for _, r := range adminCanGive {
				if r == removeRole {
					allowed = true
					break
				}
			}
			if !allowed {
				ctx.Reply("Admins can only remove the trusted role.")
				return
			}
		} else {
			ctx.Reply("Only owner/admin can manage roles.")
			return
		}

		if err := c.Roles.RemoveRole(target, removeRole); err != nil {
			ctx.Reply(fmt.Sprintf("Error: %v", err))
			return
		}
		ctx.Reply(fmt.Sprintf("Removed role %s from %s", removeRole, target))

	case "list":
		rolesList := c.Roles.ListRoles()
		users := c.Roles.ListUsers()

		if len(parts) >= 3 {
			target := parts[2]
			userRoles := c.Roles.GetUserRoles(target)
			if len(userRoles) == 0 {
				ctx.Reply(fmt.Sprintf("%s has no roles", target))
			} else {
				ctx.Reply(fmt.Sprintf("%s: %s", target, strings.Join(userRoles, ", ")))
			}
			return
		}

		var lines []string
		for _, r := range rolesList {
			var members []string
			for user, userRoles := range users {
				for _, ur := range userRoles {
					if ur == r {
						members = append(members, user)
						break
					}
				}
			}
			if len(members) > 0 {
				lines = append(lines, fmt.Sprintf("%s: %s", r, strings.Join(members, ", ")))
			}
		}
		if len(lines) == 0 {
			ctx.Reply("No roles assigned.")
		} else {
			ctx.Reply(strings.Join(lines, " | "))
		}

	default:
		ctx.Reply("*role add <user> <role> | *role remove <user> <role> | *role list")
	}
}
