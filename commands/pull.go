package commands

import (
	"jurobot/pkg/client/modules/self"
	"jurobot/pkg/client/modules/world"
	"github.com/go-mclib/data/pkg/data/blocks"
)

type PullCommand struct{}

func (c *PullCommand) Trigger() string          { return "*pull" }
func (c *PullCommand) Description() string      { return "Presses your button (MC only)." }
func (c *PullCommand) MCOnly() bool             { return true }

func (c *PullCommand) Execute(ctx *Context) {
	client := ctx.Client
	sender := ctx.Sender

	s := self.From(client)
	w := world.From(client)
	if s == nil || w == nil {
		client.Logger.Printf("Pull command: self or world module not found")
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
	if sender == "Juro5000" {
		targetBlockID = blocks.BlockID("minecraft:spruce_button")
	} else if sender == "Ruby81925" {
		targetBlockID = blocks.BlockID("minecraft:stone_button")
	} else if sender == "TidierLeek3903" {
		targetBlockID = blocks.BlockID("minecraft:bamboo_button")
	} else {
		client.Logger.Printf("Pull command: unauthorized sender %s", sender)
		return
	}

	client.Logger.Printf("Pull command from %s (target block ID: %d)", sender, targetBlockID)

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
		client.Logger.Printf("Pull command from %s: pressing button at %d, %d, %d", sender, bestX, bestY, bestZ)
		for face := int8(0); face <= 5; face++ {
			client.InteractBlock(bestX, bestY, bestZ, face, 0, 0.5, 0.5, 0.5)
		}
		client.SwingArm(0)
	} else {
		client.Logger.Printf("Pull command from %s: no button found in range", sender)
		ctx.Reply("no button found")
	}
}
