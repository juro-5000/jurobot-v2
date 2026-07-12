package commands

import (
	"fmt"
	"strings"

	"jurobot/pkg/client/modules/inventory"

	"github.com/go-mclib/data/pkg/data/items"
)

type InvCommand struct{}

func (c *InvCommand) Trigger() string     { return "*inv" }
func (c *InvCommand) Description() string { return "Shows your current inventory." }
func (c *InvCommand) MCOnly() bool        { return false }

func (c *InvCommand) Execute(ctx *Context) {
	inv := inventory.From(ctx.Client)
	if inv == nil {
		ctx.Reply("inventory not available yet")
		return
	}

	var parts []string

	armorSlots := []struct {
		name string
		slot int
	}{
		{"Helmet", inventory.SlotArmorHead},
		{"Chestplate", inventory.SlotArmorChest},
		{"Leggings", inventory.SlotArmorLegs},
		{"Boots", inventory.SlotArmorFeet},
	}
	var armor []string
	for _, a := range armorSlots {
		item := inv.GetSlot(a.slot)
		if item != nil && !item.IsEmpty() {
			armor = append(armor, fmt.Sprintf("%s: %s", a.name, itemName(item)))
		}
	}
	if len(armor) > 0 {
		parts = append(parts, "[Armor] "+strings.Join(armor, " | "))
	}

	var hotbar []string
	for i := 0; i < 9; i++ {
		item := inv.GetSlot(inventory.SlotHotbarStart + i)
		if item != nil && !item.IsEmpty() {
			hotbar = append(hotbar, fmt.Sprintf("%d: %s", i+1, itemName(item)))
		}
	}
	if len(hotbar) > 0 {
		parts = append(parts, "[Hotbar] "+strings.Join(hotbar, " | "))
	}

	offhand := inv.GetSlot(inventory.SlotOffhand)
	if offhand != nil && !offhand.IsEmpty() {
		parts = append(parts, "[Offhand] "+itemName(offhand))
	}

	if len(parts) == 0 {
		parts = append(parts, "(empty)")
	}

	msg := strings.Join(parts, " ")
	if len(msg) > 256 {
		msg = msg[:253] + "..."
	}
	ctx.Reply(msg)
}

func itemName(item *items.ItemStack) string {
	if item == nil || item.IsEmpty() {
		return "-"
	}
	name := items.ItemName(item.ID)
	name = strings.TrimPrefix(name, "minecraft:")
	if len(name) > 20 {
		name = name[:18] + ".."
	}
	if item.Count > 1 {
		return fmt.Sprintf("%s(x%d)", name, item.Count)
	}
	return name
}
