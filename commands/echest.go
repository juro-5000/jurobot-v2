package commands

import (
	"fmt"
	"strings"
	"time"

	"jurobot/pkg/client/modules/inventory"
	"jurobot/pkg/client/modules/self"
	"jurobot/pkg/client/modules/world"
	"github.com/go-mclib/data/pkg/data/blocks"
	"github.com/go-mclib/data/pkg/data/items"
)

type EchestCommand struct{}

func (c *EchestCommand) Trigger() string     { return ".echest" }
func (c *EchestCommand) Description() string { return "Opens the nearest ender chest and lists its contents." }

func (c *EchestCommand) Execute(ctx *Context) {
	client := ctx.Client

	s := self.From(client)
	w := world.From(client)
	inv := inventory.From(client)

	if s == nil || w == nil || inv == nil {
		client.Logger.Printf("Echest command: required modules not found")
		return
	}

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

	if !found {
		client.Logger.Printf("Echest command: no ender chest found in loaded chunks")
		return
	}

	// Interact with the Ender Chest
	client.Logger.Printf("Echest command: opening ender chest at %d, %d, %d", bestX, bestY, bestZ)
	client.InteractBlock(bestX, bestY, bestZ, world.FaceTop, world.HandMain, 0.5, 0.5, 0.5)
	client.SwingArm(world.HandMain)

	// Wait for the container to open and sync
	go func() {
		timeout := time.After(5 * time.Second)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-timeout:
				client.Logger.Printf("Echest command: timed out waiting for ender chest to open")
				return
			case <-ticker.C:
				if inv.ContainerOpen() {
					// Give it a tiny bit more time to receive slots
					time.Sleep(200 * time.Millisecond)

					slots := inv.ContainerSlots()
					var sb strings.Builder
					sb.WriteString("\033[93m[Ender Chest Contents]\033[0m\n")

					itemFound := false
					for i, item := range slots {
						if item != nil && !item.IsEmpty() {
							itemFound = true
							itemName := items.ItemName(item.ID)
							sb.WriteString(fmt.Sprintf("  Slot %d: %s x%d", i, itemName, item.Count))
							
							// Add durability if applicable
							if item.Components != nil && item.Components.MaxDamage > 0 {
								dur := item.Components.MaxDamage - item.Components.Damage
								sb.WriteString(fmt.Sprintf(" (durability: %d/%d)", dur, item.Components.MaxDamage))
							}
							sb.WriteString("\n")
						}
					}

					if !itemFound {
						sb.WriteString("  (empty)\n")
					}

					// Print to console via Logger (which is redirected to console in main.go)
					// Use Println directly to bypass timestamping if needed, but Logger.Print is safer for the redirector
					client.Logger.Print(sb.String())

					// Close the container
					inv.CloseContainer()
					return
				}
			}
		}
	}()
}
