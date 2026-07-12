package plugins

import (
	"hash/crc32"
	"time"

	"jurobot/pkg/client"
	"jurobot/pkg/client/modules/inventory"
	"jurobot/pkg/client/modules/self"
	"jurobot/pkg/client/modules/world"
	"github.com/go-mclib/data/pkg/data/blocks"
	"github.com/go-mclib/data/pkg/data/items"
	"github.com/go-mclib/data/pkg/packets"
	ns "github.com/go-mclib/protocol/java_protocol/net_structures"
)

var crc32cTable = crc32.MakeTable(crc32.Castagnoli)

type SorterPlugin struct {
	AutoEchestEnabled bool
	CheckDisabled     func() bool // if non-nil and returns true, skips auto-sort
}

func (p *SorterPlugin) Name() string { return "sorter" }

func (p *SorterPlugin) Init(c *client.Client) {
	ticker := time.NewTicker(120 * time.Second)
	go func() {
		for range ticker.C {
			p.RunSortingRoutine(c)
		}
	}()
}

func (p *SorterPlugin) RunSortingRoutine(c *client.Client) {
	if p.CheckDisabled != nil && p.CheckDisabled() {
		return
	}
	s := self.From(c)
	inv := inventory.From(c)
	w := world.From(c)
	if s == nil || inv == nil || w == nil {
		return
	}

	// Phase 0: Stow Spawners in Echest
	if p.AutoEchestEnabled {
		p.stowSpawners(c, s, inv, w)
	}

	// Phase 1: Valuables (North: -180 degrees)
	s.SetRotation(-180, 0)
	time.Sleep(500 * time.Millisecond)
	p.dropItems(c, inv, true)

	// Phase 2: Junk (East: -90 degrees)
	s.SetRotation(-90, 0)
	time.Sleep(500 * time.Millisecond)
	p.dropItems(c, inv, false)
}

func isEchestStowItem(name string) bool {
	switch name {
	case
		"minecraft:spawner",
		"minecraft:reinforced_deepslate",
		"minecraft:diamond_block",
		"minecraft:netherite_block",
		"minecraft:enchanted_golden_apple",
		"minecraft:honey_block",
		"minecraft:netherite_upgrade_smithing_template",
		"minecraft:silence_armor_trim_smithing_template",
		"minecraft:flow_armor_trim_smithing_template",
		"minecraft:dragon_egg",
		"minecraft:vindicator_spawn_egg",
		"minecraft:witch_spawn_egg":
		return true
	}
	return false
}

func (p *SorterPlugin) stowSpawners(c *client.Client, s *self.Module, inv *inventory.Module, w *world.Module) {
	// 1. Check if we have any echest-stowable items
	var spawnerSlots []int
	for i := inventory.SlotMainStart; i < inventory.SlotHotbarEnd; i++ {
		item := inv.GetSlot(i)
		if item == nil || item.IsEmpty() {
			continue
		}
		name := items.ItemName(item.ID)
		if isEchestStowItem(name) {
			spawnerSlots = append(spawnerSlots, i)
		}
	}

	if len(spawnerSlots) == 0 {
		return
	}

	// 2. Find nearest Ender Chest
	echestID := blocks.BlockID("minecraft:ender_chest")
	px, py, pz := s.Position()
	var bestX, bestY, bestZ int
	bestDistSq := -1.0
	found := false

	w.FindBlocks([]int32{echestID}, func(x, y, z int, stateID int32) bool {
		dx := float64(x) - px
		dy := float64(y) - py
		dz := float64(z) - pz
		distSq := dx*dx + dy*dy + dz*dz
		if bestDistSq < 0 || distSq < bestDistSq {
			bestDistSq = distSq
			bestX, bestY, bestZ = x, y, z
			found = true
		}
		return true
	})

	if !found || bestDistSq > 25.0 { // limit to 5 blocks
		return
	}

	// 3. Open Echest
	c.InteractBlock(bestX, bestY, bestZ, world.FaceTop, world.HandMain, 0.5, 0.5, 0.5)
	c.SwingArm(world.HandMain)

	// 4. Wait for it to open and stow
	timeout := time.After(3 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return
		case <-ticker.C:
			if inv.ContainerOpen() {
				time.Sleep(500 * time.Millisecond) // initial sync wait

				containerSlotCount := inv.ContainerSlotCount()
				for _, slot := range spawnerSlots {
					// viewIndex in container is containerSlots + (slot - SlotMainStart)
					viewIndex := containerSlotCount + (slot - inventory.SlotMainStart)
					inv.ContainerShiftClick(viewIndex)
					time.Sleep(200 * time.Millisecond)
				}

				time.Sleep(1500 * time.Millisecond) // "a bit longer" to satisfy anti-cheat
				inv.CloseContainer()
				return
			}
		}
	}
}

func (p *SorterPlugin) dropItems(c *client.Client, inv *inventory.Module, dropValuables bool) {
	for i := inventory.SlotMainStart; i < inventory.SlotHotbarEnd; i++ {
		item := inv.GetSlot(i)
		if item == nil || item.IsEmpty() {
			continue
		}

		name := items.ItemName(item.ID)
		if isEchestStowItem(name) {
			continue
		}

		isValuable := IsKeeplistItem(name)
		if (dropValuables && isValuable) || (!dropValuables && !isValuable) {
			p.dropSlot(c, inv, i)
			time.Sleep(150 * time.Millisecond)
		}
	}
}

func (p *SorterPlugin) DropSpawnersFromEchest(c *client.Client, s *self.Module, inv *inventory.Module, w *world.Module) {
	// Find nearest Ender Chest
	echestID := blocks.BlockID("minecraft:ender_chest")
	px, py, pz := s.Position()
	var bestX, bestY, bestZ int
	bestDistSq := -1.0
	found := false

	w.FindBlocks([]int32{echestID}, func(x, y, z int, stateID int32) bool {
		dx := float64(x) - px
		dy := float64(y) - py
		dz := float64(z) - pz
		distSq := dx*dx + dy*dy + dz*dz
		if bestDistSq < 0 || distSq < bestDistSq {
			bestDistSq = distSq
			bestX, bestY, bestZ = x, y, z
			found = true
		}
		return true
	})

	if !found || bestDistSq > 25.0 {
		return
	}

	// Open Echest
	c.InteractBlock(bestX, bestY, bestZ, world.FaceTop, world.HandMain, 0.5, 0.5, 0.5)
	c.SwingArm(world.HandMain)

	// Wait for it to open
	timeout := time.After(3 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var spawnerSlots []int

	for {
		select {
		case <-timeout:
			return
		case <-ticker.C:
			if inv.ContainerOpen() {
				time.Sleep(500 * time.Millisecond)

				containerSlotCount := inv.ContainerSlotCount()
				// Find spawners in echest
				for i := 0; i < containerSlotCount; i++ {
					item := inv.ContainerSlot(i)
					if item == nil || item.IsEmpty() {
						continue
					}
					if items.ItemName(item.ID) == "minecraft:spawner" {
						spawnerSlots = append(spawnerSlots, i)
					}
				}

				if len(spawnerSlots) == 0 {
					inv.CloseContainer()
					return
				}

				// Shift-click spawners from echest into player inventory
				for _, slot := range spawnerSlots {
					inv.ContainerShiftClick(slot)
					time.Sleep(200 * time.Millisecond)
				}

				time.Sleep(1500 * time.Millisecond)
				inv.CloseContainer()

				// Rotate to valuable drop direction (North)
				s.SetRotation(-180, 0)
				time.Sleep(500 * time.Millisecond)

				// Drop spawners from inventory at the good spot
				for i := inventory.SlotMainStart; i < inventory.SlotHotbarEnd; i++ {
					item := inv.GetSlot(i)
					if item == nil || item.IsEmpty() {
						continue
					}
					if items.ItemName(item.ID) == "minecraft:spawner" {
						p.dropSlot(c, inv, i)
						time.Sleep(150 * time.Millisecond)
					}
				}
				return
			}
		}
	}
}

func (p *SorterPlugin) dropSlot(c *client.Client, inv *inventory.Module, slot int) {
	slots, stateID, cursor := inv.RawSlots()
	if slot < 0 || slot >= len(slots) {
		return
	}

	c.WritePacket(&packets.C2SContainerClick{
		WindowId: 0,
		StateId:  ns.VarInt(stateID),
		Slot:     ns.Int16(slot),
		Button:   1, // Drop whole stack
		Mode:     4, // DROP
		ChangedSlots: []packets.ChangedSlot{
			{SlotNum: ns.Int16(slot), Item: ns.EmptyHashedSlot()},
		},
		CarriedItem: toHashed(cursor),
	})
}

func toHashed(s ns.Slot) ns.HashedSlot {
	if s.IsEmpty() {
		return ns.EmptyHashedSlot()
	}
	hs := ns.HashedSlot{
		Present: true,
		ItemID:  s.ItemID,
		Count:   s.Count,
	}
	for _, comp := range s.Components.Add {
		hs.Components.Add = append(hs.Components.Add, ns.HashedComponent{
			ID:   comp.ID,
			Hash: ns.Int32(crc32.Checksum(comp.Data, crc32cTable)),
		})
	}
	hs.Components.Remove = s.Components.Remove
	return hs
}

var keeplist = map[string]bool{
	"minecraft:elytra":                             true,
	"minecraft:netherite_sword":                   true,
	"minecraft:netherite_axe":                     true,
	"minecraft:netherite_pickaxe":                 true,
	"minecraft:netherite_helmet":                  true,
	"minecraft:netherite_chestplate":              true,
	"minecraft:netherite_leggings":                true,
	"minecraft:netherite_boots":                   true,
	"minecraft:netherite_ingot":                   true,
	"minecraft:netherite_block":                   true,
	"minecraft:diamond":                           true,
	"minecraft:diamond_block":                     true,
	"minecraft:enchanted_golden_apple":            true,
	"minecraft:netherite_upgrade_smithing_template": true,
	"minecraft:bedrock":                           true,
	"minecraft:barrier":                           true,
	"minecraft:mace":                              true,
	"minecraft:netherite_spear":                   true,
	"minecraft:trident":                           true,
	"minecraft:reinforced_deepslate":              true,
	"minecraft:honey_block":                       true,
	"minecraft:silence_armor_trim_smithing_template": true,
	"minecraft:flow_armor_trim_smithing_template":    true,
	"minecraft:dragon_egg":                        true,
}

func IsKeeplistItem(name string) bool {
	return keeplist[name]
}
