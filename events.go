package main

import "PumpkinGiraffe/game"

// PumpkinSystem encapsulates all pumpkin‐spawn & physics logic.
type PumpkinSystem struct{}

// NewPumpkinSystem returns a ready‐to‐use PumpkinSystem.
func NewPumpkinSystem() *PumpkinSystem {
	return &PumpkinSystem{}
}

// Update advances gravity, catches, and misses for all pumpkins.
// We’ll wire this into Game.Update next.

func (ps *PumpkinSystem) Update(g *Game) {
	lvl := game.Levels[game.CurrentLevel]
	// 0) Initial pumpkin drop when last enemy dies
	alive := 0
	for _, en := range lvl.Enemies {
		if en.Alive {
			alive++
		}
	}
	if alive == 0 && !g.initialPumpkinDropped {
		g.spawnPumpkinAt(390, 0)
		g.initialPumpkinDropped = true
		g.pumpkinSpawned = true
	}

	for i := 0; i < len(g.pumpkins); i++ {
		pk := g.pumpkins[i]
		// gravity
		pk.VelY += pumpkinGravity
		if pk.VelY > pumpkinMaxFall {
			pk.VelY = pumpkinMaxFall
		}
		pk.Y += pk.VelY

		// caught?
		if g.player.Rect().Overlaps(pk.Rect()) {
			g.player.Pumpkins++
			g.pumpkinSnd.Rewind()
			g.pumpkinSnd.Play()
			g.pumpkins = append(g.pumpkins[:i], g.pumpkins[i+1:]...)
			i--
			g.pumpkinSpawned = false
			g.pumpkinMissed = false
			continue
		}

		// missed?
		bottom := float64(len(lvl.Tiles)) * float64(game.TileSize)
		if pk.Y > bottom {
			g.pumpkins = append(g.pumpkins[:i], g.pumpkins[i+1:]...)
			i--
			g.pumpkinMissed = true
			g.pumpkinSpawned = false
			continue
		}
	}
}
