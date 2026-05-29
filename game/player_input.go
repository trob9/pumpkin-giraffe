// Reads and interprets user input for movement (A/D or ←/→) and jumping (Space/Up)
// including single vs. double-jump timing and detection of interact-key “edge” presses.
package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
)

// readHorizontalInput reads the left/right movement keys (arrow keys or A/D)
// and returns the desired horizontal speed: negative for left, positive for right,
// zero if no movement key is held.
func readHorizontalInput(moveSpeed float64) float64 {
	// Sprint multiplies speed; arrow keys / right-shift are always-on alternates.
	mult := 1.0
	if Down(ActSprint) || ebiten.IsKeyPressed(ebiten.KeyShiftRight) {
		mult = 2.5
	}
	if Down(ActLeft) || ebiten.IsKeyPressed(ebiten.KeyLeft) {
		return -moveSpeed * mult
	}
	if Down(ActRight) || ebiten.IsKeyPressed(ebiten.KeyRight) {
		return +moveSpeed * mult
	}
	return 0 // no horizontal input
}

// handleJumpInput checks if the player is on solid ground and has just pressed
// the jump key (Space or Up arrow). It applies either a normal jump or a stronger
// double jump based on how many frames have elapsed since the last jump.
// It resets jump timers, plays the jump sound, and returns true when a jump occurs.
func (p *Player) handleJumpInput(jumpSnd *audio.Player) bool {
	// Only allow jumping when standing on something
	if p.OnGround && (Down(ActJump) || ebiten.IsKeyPressed(ebiten.KeyUp)) {
		// If within the double-jump window but not too soon (to prevent immediate re-jump)
		if p.framesSinceJump > 5 && p.framesSinceJump <= doubleJumpWindow {
			p.VelY = doubleJumpVel // stronger second jump
		} else {
			p.VelY = normalJumpVel // standard first jump
		}
		// Reset jump tracking and mark as airborne
		p.framesSinceJump = 0
		p.OnGround = false
		p.ridingPlatform = nil // leaving the platform when we jump off it

		// Play jump sound from the start
		jumpSnd.Rewind()
		jumpSnd.Play()
		return true
	}
	return false // no jump this frame
}

// IsBesideInteractableObject returns true if the player is adjacent to a tile
// that supports interaction (e.g., a barrel). It calculates the tile just left
// or right of the player's mid-height, then checks bounds and interactable status.
func (p *Player) IsBesideInteractableObject() bool {
	ts := float64(TileSize)
	// Determine the vertical tile row at the player's mid-height
	ty := int((p.Y + p.Height/2) / ts)

	// Determine the horizontal tile column just outside the player's bounding box
	var tx int
	if p.facingRight {
		tx = int((p.X + p.Width + 1) / ts)
	} else {
		tx = int((p.X - 1) / ts)
	}

	// If that tile coordinate is off the map, there can be no object
	if ty < 0 || ty >= len(Levels[CurrentLevel].Tiles) ||
		tx < 0 || tx >= len(Levels[CurrentLevel].Tiles[0]) {
		return false
	}

	// Look up the tile ID and return whether it's flagged as interactable
	tileID := Levels[CurrentLevel].Tiles[ty][tx]
	return isInteractableObject(tileID)
}
