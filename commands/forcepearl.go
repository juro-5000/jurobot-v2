package commands

import (
	"strings"
	"jurobot/pkg/client/modules/self"
	"jurobot/pkg/client/modules/world"
	"github.com/go-mclib/data/pkg/data/blocks"
)

type ForcePearlCommand struct{}

func (c *ForcePearlCommand) Trigger() string     { return ".forcepearl" }
func (c *ForcePearlCommand) Description() string { return "Force pulls a specific user's pearl. Usage: .forcepearl <username>" }

func (c *ForcePearlCommand) Execute(ctx *Context) {
	client := ctx.Client
	msg := ctx.Message

	// Parse username from message: ".forcepearl <username>"
	parts := strings.Fields(msg)
	if len(parts) < 2 {
		return
	}
	targetUser := parts[1]

	s := self.From(client)
	w := world.From(client)
	if s == nil || w == nil {
		client.Logger.Printf("ForcePearl command: self or world module not found")
		return
	}

	buttonIDs := []int32{
		blocks.BlockID("minecraft:spruce_button"),
		blocks.BlockID("minecraft:stone_button"),
		blocks.BlockID("minecraft:oak_button"),
		blocks.BlockID("minecraft:birch_button"),
		blocks.BlockID("minecraft:jungle_button"),
		blocks.BlockID("minecraft:acacia_button"),
		blocks.BlockID("minecraft:dark_oak_button"),
		blocks.BlockID("minecraft:mangrove_button"),
		blocks.BlockID("minecraft:cherry_button"),
		blocks.BlockID("minecraft:bamboo_button"),
		blocks.BlockID("minecraft:crimson_button"),
		blocks.BlockID("minecraft:warped_button"),
		blocks.BlockID("minecraft:polished_blackstone_button"),
	}

	var targetBlockID int32
	switch strings.ToLower(targetUser) {
	case "juro5000":
		targetBlockID = blocks.BlockID("minecraft:spruce_button")
	case "ruby81925":
		targetBlockID = blocks.BlockID("minecraft:stone_button")
	case "tidierleek3903":
		targetBlockID = blocks.BlockID("minecraft:bamboo_button")
	default:
		client.Logger.Printf("ForcePearl command: unknown target user %s", targetUser)
		return
	}

	client.Logger.Printf("ForcePearl command for %s (target block ID: %d)", targetUser, targetBlockID)

	px, py, pz := s.Position()
	var bestX, bestY, bestZ int
	bestDist := -1.0
	interactionRange := 5.0
	rangeSq := interactionRange * interactionRange

	w.FindBlocks(buttonIDs, func(x, y, z int, stateID int32) bool {
		blockID, _ := blocks.StateProperties(int(stateID))
		if blockID != targetBlockID {
			return true
		}

		dx := float64(x) - px
		dy := float64(y) - py
		dz := float64(z) - pz
		distSq := dx*dx + dy*dy + dz*dz
		if distSq > rangeSq {
			return true // too far
		}
		if bestDist < 0 || distSq < bestDist {
			bestDist = distSq
			bestX, bestY, bestZ = x, y, z
		}
		return true
	})

	if bestDist >= 0 {
		client.Logger.Printf("ForcePearl command: pressing button for %s at %d, %d, %d", targetUser, bestX, bestY, bestZ)
		for face := int8(0); face <= 5; face++ {
			client.InteractBlock(bestX, bestY, bestZ, face, 0, 0.5, 0.5, 0.5)
		}
		client.SwingArm(0)
	} else {
		client.Logger.Printf("ForcePearl command: no button found for %s in range", targetUser)
	}
}
