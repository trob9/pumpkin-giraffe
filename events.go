package main

import "github.com/Trob999/PumpkinGiraffe/game"

// PumpkinSystem handles everything related to pumpkins in the game world:
// spawning them when conditions are met, applying gravity each frame,
// detecting when the player catches one, and removing pumpkins that fall out of bounds.
type PumpkinSystem struct{}

// NewPumpkinSystem constructs a fresh PumpkinSystem with no internal state.
func NewPumpkinSystem() *PumpkinSystem {
	return &PumpkinSystem{}
}

// Update steps the pumpkin simulation forward. It should be called once per frame
// from Game.Update. Responsibilities include:
//
//  1) Dropping the very first pumpkin once all enemies are defeated.
//  2) Applying gravity to each active pumpkin.
//  3) Checking for player–pumpkin collisions (caught pumpkins).
//  4) Removing pumpkins that have fallen below the level (missed pumpkins).
func (ps *PumpkinSystem) Update(g *Game) {
	// Grab the current level layout, which contains enemy and tile data.
	lvl := game.Levels[game.CurrentLevel]

	// --- 1) Initial drop logic ---
	// Count how many enemies are still alive.
	alive := 0
	for _, en := range lvl.Enemies {
		if en.Alive {
			alive++
		}
	}
	// If no enemies remain and we haven't dropped the first pumpkin yet:
	if alive == 0 && !g.initialPumpkinDropped {
		// Spawn at a fixed position (x=390, y=0).
		g.spawnPumpkinAt(390, 0)
		g.initialPumpkinDropped = true
		g.pumpkinSpawned = true
	}

	// --- 2) Per-pumpkin physics & collision ---
	// Loop by index so we can remove items safely during iteration.
	for i := 0; i < len(g.pumpkins); i++ {
		pk := g.pumpkins[i]

		// Apply gravity: accelerate downward, then clamp to max fall speed.
		pk.VelY += pumpkinGravity
		if pk.VelY > pumpkinMaxFall {
			pk.VelY = pumpkinMaxFall
		}
		pk.Y += pk.VelY

		// --- 3) Caught check ---
		// If the player's bounding box overlaps the pumpkin's box:
		if g.player.Rect().Overlaps(pk.Rect()) {
			// Increment the player's pumpkin count and play sound.
			g.player.Pumpkins++
			g.pumpkinSnd.Rewind()
			g.pumpkinSnd.Play()

			// Remove this pumpkin from the slice.
			g.pumpkins = append(g.pumpkins[:i], g.pumpkins[i+1:]...)
			i-- // step back one index to continue correctly

			// Reset spawn/miss flags so the next pumpkin can drop.
			g.pumpkinSpawned = false
			g.pumpkinMissed = false
			continue
		}

		// --- 4) Missed check ---
		// If the pumpkin falls below the bottom of the level grid:
		bottom := float64(len(lvl.Tiles)) * float64(game.TileSize)
		if pk.Y > bottom {
			// Remove it from the slice.
			g.pumpkins = append(g.pumpkins[:i], g.pumpkins[i+1:]...)
			i--

			// Mark that the pumpkin was missed and allow a new one to spawn.
			g.pumpkinMissed = true
			g.pumpkinSpawned = false
			continue
		}
	}
}
