package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
)

// readHorizontalInput returns the intended horizontal velocity
// based on player input (left/right or A/D).
func readHorizontalInput(moveSpeed float64) float64 {
	if ebiten.IsKeyPressed(ebiten.KeyLeft) || ebiten.IsKeyPressed(ebiten.KeyA) {
		return -moveSpeed
	}
	if ebiten.IsKeyPressed(ebiten.KeyRight) || ebiten.IsKeyPressed(ebiten.KeyD) {
		return moveSpeed
	}
	return 0
}

// handleJumpInput checks for jump key press and updates velocity and state.
// Returns true if a jump occurred.
func (p *Player) handleJumpInput(jumpSnd *audio.Player) bool {
	if p.OnGround && (ebiten.IsKeyPressed(ebiten.KeySpace) || ebiten.IsKeyPressed(ebiten.KeyUp)) {
		if p.framesSinceJump <= doubleJumpWindow && p.framesSinceJump > 5 {
			p.VelY = doubleJumpVel
		} else {
			p.VelY = normalJumpVel
		}
		p.framesSinceJump = 0
		p.OnGround = false
		jumpSnd.Rewind()
		jumpSnd.Play()
		return true
	}
	return false
}

// IsBesideInteractableObject checks whether the player is adjacent to something interactable.

func (p *Player) IsBesideInteractableObject() bool {

	ts := float64(TileSize)

	ty := int((p.Y + p.Height/2) / ts)
	var tx int
	if p.facingRight {
		tx = int((p.X + p.Width + 1) / ts)
	} else {
		tx = int((p.X - 1) / ts)
	}

	if ty < 0 || ty >= len(Levels[CurrentLevel].Tiles) ||
		tx < 0 || tx >= len(Levels[CurrentLevel].Tiles[0]) {
		return false
	}

	tileID := Levels[CurrentLevel].Tiles[ty][tx]
	return isInteractableObject(tileID)

}

// ShouldTriggerInteraction checks if 'E' was just pressed near an interactable object.

func (p *Player) ShouldTriggerInteraction() bool {
	pressed := ebiten.IsKeyPressed(ebiten.KeyE)
	nowUse := pressed && !p.prevUse
	p.prevUse = pressed
	p.interacting = pressed && p.IsBesideInteractableObject()
	return nowUse
}
