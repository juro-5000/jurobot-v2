package plugins

import (
	"time"

	"jurobot/pkg/client"
	"jurobot/pkg/client/modules/inventory"
	"jurobot/pkg/client/modules/physics"
	"jurobot/pkg/client/modules/self"
	"github.com/go-mclib/data/pkg/data/items"
)

type CombatPlugin struct {
	AutoEat       bool
	AutoTotem     bool
	AutoArmor     bool
	DisableEatArmor func() bool // if non-nil and returns true, skips auto-eat and auto-armor

	busyUntil time.Time
}

func (p *CombatPlugin) Name() string { return "combat_plugin" }

func (p *CombatPlugin) Init(c *client.Client) {
	phys := physics.From(c)
	if phys == nil {
		return
	}

	phys.OnTick(func() {
		if time.Now().Before(p.busyUntil) {
			return
		}
		p.run(c)
	})
}

func (p *CombatPlugin) run(c *client.Client) {
	s := self.From(c)
	inv := inventory.From(c)
	if s == nil || inv == nil {
		return
	}

	disabled := p.DisableEatArmor != nil && p.DisableEatArmor()

	// Priority: Totem > Armor > Eat
	if p.AutoTotem && p.checkTotem(c, inv) {
		p.busyUntil = time.Now().Add(500 * time.Millisecond)
		return
	}
	if !disabled && p.AutoArmor && p.checkArmor(c, inv) {
		p.busyUntil = time.Now().Add(500 * time.Millisecond)
		return
	}
	if !disabled && p.AutoEat && p.checkEat(c, s, inv) {
		p.busyUntil = time.Now().Add(1 * time.Second)
		return
	}
}

func (p *CombatPlugin) checkTotem(c *client.Client, inv *inventory.Module) bool {
	offhand := inv.GetOffhand()
	if offhand != nil && !offhand.IsEmpty() && items.ItemName(offhand.ID) == "minecraft:totem_of_undying" {
		return false
	}

	// find totem in inventory
	totemID := items.ItemID("minecraft:totem_of_undying")
	if totemID < 0 {
		return false
	}

	slot := inv.FindItem(totemID)
	if slot < 0 {
		return false
	}

	// Use inventory clicks to move to offhand (slot 45)
	go func() {
		// 1. Pick up the totem
		inv.ClickSlot(slot)
		time.Sleep(250 * time.Millisecond)

		// 2. Click the offhand slot (45) to place it
		inv.ClickSlot(45)
		time.Sleep(250 * time.Millisecond)

		// 3. If we are now holding something else on the cursor, put it back
		if !inv.CursorItem().IsEmpty() {
			inv.ClickSlot(slot)
		}
	}()

	return true
}

func (p *CombatPlugin) checkArmor(c *client.Client, inv *inventory.Module) bool {
	head, chest, legs, feet := inv.GetArmor()

	armorTypes := []string{"helmet", "chestplate", "leggings", "boots"}
	currentArmor := []*items.ItemStack{head, chest, legs, feet}

	for i, slot := range currentArmor {
		if slot == nil || slot.IsEmpty() {
			// find best armor for this slot
			bestSlot := p.findBestArmor(inv, armorTypes[i])
			if bestSlot >= 0 {
				// equip it with a delay
				go func(slotToEquip int) {
					time.Sleep(200 * time.Millisecond)
					inv.ShiftClickSlot(slotToEquip)
				}(bestSlot)
				return true
			}
		}
	}
	return false
}

func (p *CombatPlugin) findBestArmor(inv *inventory.Module, armorType string) int {
	// Special case: Prefer Elytra over chestplates
	if armorType == "chestplate" {
		elytraID := items.ItemID("minecraft:elytra")
		if elytraID >= 0 {
			for i := inventory.SlotMainStart; i < inventory.SlotHotbarEnd; i++ {
				item := inv.GetSlot(i)
				if item != nil && item.ID == elytraID {
					return i
				}
			}
		}
	}

	bestSlot := -1
	bestValue := -1

	// simple ranking: netherite > diamond > iron > chainmail > gold > leather
	ranks := map[string]int{
		"netherite": 6,
		"diamond":   5,
		"iron":      4,
		"chainmail": 3,
		"gold":      2,
		"leather":   1,
	}

	for i := inventory.SlotMainStart; i < inventory.SlotHotbarEnd; i++ {
		item := inv.GetSlot(i)
		if item == nil || item.IsEmpty() {
			continue
		}
		name := items.ItemName(item.ID)
		for material, rank := range ranks {
			if name == "minecraft:"+material+"_"+armorType {
				if rank > bestValue {
					bestValue = rank
					bestSlot = i
				}
				break
			}
		}
	}
	return bestSlot
}

var lastEatTime time.Time

func (p *CombatPlugin) checkEat(c *client.Client, s *self.Module, inv *inventory.Module) bool {
	if s.Food() >= 19 || time.Since(lastEatTime) < 10*time.Second {
		return false
	}

	// find food not in keeplist
	foodSlot := -1
	for i := inventory.SlotMainStart; i < inventory.SlotHotbarEnd; i++ {
		item := inv.GetSlot(i)
		if item == nil || item.IsEmpty() {
			continue
		}
		name := items.ItemName(item.ID)
		if !IsKeeplistItem(name) && isFoodItem(name) {
			foodSlot = i
			break
		}
	}

	if foodSlot >= 0 {
		lastEatTime = time.Now()
		go func() {
			item := inv.GetSlot(foodSlot)
			if item != nil {
				// The Eat function in self module is blocking and handles its own timing.
				s.Eat([]int32{item.ID})
			}
		}()
		return true
	}
	return false
}

func isFoodItem(name string) bool {
	foodItems := map[string]bool{
		"minecraft:cooked_beef":     true,
		"minecraft:cooked_porkchop": true,
		"minecraft:cooked_chicken":  true,
		"minecraft:cooked_mutton":   true,
		"minecraft:cooked_rabbit":   true,
		"minecraft:cooked_salmon":   true,
		"minecraft:cooked_cod":      true,
		"minecraft:golden_carrot":   true,
		"minecraft:apple":           true,
		"minecraft:bread":           true,
		"minecraft:carrot":          true,
		"minecraft:potato":          true,
		"minecraft:baked_potato":    true,
		"minecraft:melon_slice":     true,
		"minecraft:pumpkin_pie":     true,
	}
	return foodItems[name]
}
