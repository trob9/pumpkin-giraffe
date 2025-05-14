//Core player logic: world-space position, physics (gravity, collision with walls/floor), 
//movement, respawning, pumpkin pickup, animation state, and sound-effect playback for footsteps, jumps, landings, etc.
package game

import (
	// embed gives us access to files compiled into the binary (images, levels, sounds).
	"embed"
	// image is used for working with rectangles and image bounds.
	"image"
	// image/png registers the PNG decoder so we can load .png files.
	_ "image/png"

	// ebiten is the game library for drawing to the screen and handling input.
	"github.com/hajimehoshi/ebiten/v2"
	// audio provides playback support for sound effects and music.
	"github.com/hajimehoshi/ebiten/v2/audio"
)

const (
	// ScreenWidth and ScreenHeight define the fixed, logical size of our game world.
	// Ebiten will scale this to whatever window size you choose.
	ScreenWidth  = 1280
	ScreenHeight = 720

	// stepInterval controls how often a footstep sound can play (in frames).
	stepInterval = 20

	// doubleJumpWindow is the number of frames after a jump during which a second jump is allowed.
	doubleJumpWindow = 24

	// normalJumpVel is the upward speed applied when the player jumps.
	normalJumpVel = -6.5
	// doubleJumpVel is a stronger lift for the second jump (50% more than normal).
	doubleJumpVel = normalJumpVel * 1.5

	// idleFPS and walkFPS determine how many animation frames we show per second.
	idleFPS = 3
	walkFPS = 7

	// walkFrames is the number of frames in the walking sprite sheet.
	walkFrames = 4
	// idleFrames is the number of frames in the idle (standing) sprite sheet.
	idleFrames = 2
	// numFrames is a generic fallback frame count (not used directly here).
	numFrames = 4

	// spriteW and spriteH are the width and height (in pixels) of each character frame.
	spriteW = 16
	spriteH = 16
)

// Player holds everything needed to update, animate, and draw the main character.
type Player struct {
	// X, Y        — current world position (floating point for smooth movement)
	X, Y float64
	// VelX, VelY  — current horizontal and vertical speed
	VelX, VelY float64
	// Width, Height — collision size and drawing size for the sprite
	Width, Height float64

	// OnGround          — true if the player is currently standing on solid ground
	OnGround bool
	// stepTimer         — counts down to when the next footstep sound can play
	stepTimer int
	// framesSinceJump   — tracks frames since last jump to enforce double-jump timing
	framesSinceJump int

	// Pumpkins — how many pumpkins the player has collected
	Pumpkins int

	// Animation image slices for moving right and left:
	walkR, walkL []*ebiten.Image
	idleR, idleL []*ebiten.Image
	// Single-frame images for jumping and interacting:
	jumpR, jumpL         *ebiten.Image
	interactR, interactL *ebiten.Image

	// Animation state for walking:
	frameIndex, walkTimer, walkDelay int
	// Animation state for idling:
	idleIndex, idleTimer, idleDelay int
	// facingRight determines whether to draw right-facing or left-facing frames
	facingRight bool

	// Interaction handling:
	// interacting — true on the single frame when the player is performing an “use” action
	// onInteract  — callback function to display messages (e.g., “You found a secret!”)
	interacting bool
	onInteract  func(msg string)

	// prevUse — remembers the previous frame’s E-key state to detect when it’s first pressed
	prevUse bool
}

// spawnX and spawnY define the starting position when a new Player is created.
var (
	spawnX float64 = 64
	spawnY float64 = 300
)

// NewPlayer loads all the sprite sheets and returns a fresh Player at the spawn point.
// It also takes an onInteract callback so the player can show messages when using barrels.
func NewPlayer(assets embed.FS, onInteract func(string)) *Player {
	// Load walking animations (4 frames each direction).
	walkR := loadAnimationSheet(assets, "assets/sprites/player_walk_right.png", walkFrames)
	walkL := loadAnimationSheet(assets, "assets/sprites/player_walk_left.png", walkFrames)

	// Load idle animations (2 frames each direction).
	idleR := loadAnimationSheet(assets, "assets/sprites/player_idle_right.png", idleFrames)
	idleL := loadAnimationSheet(assets, "assets/sprites/player_idle_left.png", idleFrames)

	// Load single-frame poses for jumping and interacting.
	jr := loadPoseSheet(assets, "assets/sprites/player_jump_right.png")
	jl := loadPoseSheet(assets, "assets/sprites/player_jump_left.png")
	interactR := loadPoseSheet(assets, "assets/sprites/player_interact_right.png")
	interactL := loadPoseSheet(assets, "assets/sprites/player_interact_left.png")

	// Calculate how many frames to wait between animation updates:
	//   walkDelay = 60 frames ÷ walkFPS
	//   idleDelay = (60 frames ÷ idleFPS) × some extra factor for slower idle blink
	walkDelay := 60 / walkFPS
	idleDelay := (60 / idleFPS) * 15

	return &Player{
		// starting position and sprite size
		X:      spawnX,
		Y:      spawnY,
		Width:  spriteW,
		Height: spriteH,

		// timing for footsteps and double-jump window
		stepTimer:       stepInterval,
		framesSinceJump: doubleJumpWindow + 1,

		// assign loaded animations and poses
		walkR:     walkR,
		walkL:     walkL,
		idleR:     idleR,
		idleL:     idleL,
		jumpR:     jr,
		jumpL:     jl,
		interactR: interactR,
		interactL: interactL,

		// set animation delays and initialize timers
		walkDelay: walkDelay,
		walkTimer: walkDelay,
		idleDelay: idleDelay,
		idleTimer: idleDelay,

		// default direction facing right
		facingRight: true,

		// hook up the interaction callback
		onInteract: onInteract,

		// no interaction key pressed yet
		prevUse: false,
	}
}

// Respawn sends the player back to the starting point with zero velocity
// and resets jump state so they can jump again normally.
func (p *Player) Respawn() {
	// teleport to the spawn coordinates
	p.X, p.Y = spawnX, spawnY

	// stop any current motion
	p.VelX, p.VelY = 0, 0

	// mark as in the air until landing
	p.OnGround = false

	// allow a fresh double-jump (reset the jump cooldown counter)
	p.framesSinceJump = doubleJumpWindow + 1
}

// currentFloorID returns the tile index directly beneath the player's feet.
// This helps determine what type of ground the player is standing on.
func (p *Player) currentFloorID() int {
	// center X coordinate under the player’s body
	cx := p.X + p.Width/2

	// compute the row (y) of tiles from the player’s bottom edge
	ty := int((p.Y + p.Height) / float64(TileSize))

	// compute the column (x) of tiles from the center X
	tx := int(cx / float64(TileSize))

	// look up the tile ID in the current level’s grid
	return Levels[CurrentLevel].Tiles[ty][tx]
}

// Update handles everything that happens to the player each frame during active gameplay.
// It processes user input (movement and jumping), applies gravity and collision checks
// (ground, ceiling, and walls), and enforces screen boundaries and respawn logic if the
// player falls off the world. It also scans repeatedly for player/pumpkin collision to "pick them up",
// plays appropriate sounds (jump, landing, footsteps, pumpkin pickup, death), and manages
// interaction cooldowns so the player can only "interact" (press E) at specified intervals.
// Finally, it advances the correct animation frames—walking, idling, or jumping—based on
// movement state, ensuring the sprite and sounds stay in sync with the player’s actions.

func (p *Player) Update(
	jumpSnd *audio.Player, // plays when player jumps
	deathSnd *audio.Player, // plays when player falls off-screen
	stepSnds map[int]*audio.Player, // footstep sounds keyed by tile ID
	landingSnd *audio.Player, // plays when player lands on ground
	monsterDeathSnd *audio.Player, // plays when player stomps a monster
	pumpkinSnd *audio.Player, // plays when collecting a pumpkin
	pumpkinMissed bool, // indicates if last pumpkin interaction failed
) {
	// Track frames since last jump to enforce double-jump window
	p.framesSinceJump++

	// Tile size in world units, for converting between pixel coords and tile indices
	ts := float64(TileSize)

	// Determine desired horizontal speed from player input (A/D or ←/→)
	moveSpeed := 1.5
	intendedVX := readHorizontalInput(moveSpeed)

	// Update facing direction so sprite flips appropriately
	if intendedVX > 0 {
		p.facingRight = true
	} else if intendedVX < 0 {
		p.facingRight = false
	}

	// If the player falls below the bottom of the screen, trigger respawn
	if p.Y > float64(ScreenHeight) {
		deathSnd.Rewind()
		deathSnd.Play()
		p.Respawn()
		return // skip remaining physics and animation this frame
	}

	// Constrain horizontal position within screen bounds
	if p.X < 0 {
		p.X = 0
	}
	if p.X+p.Width > ScreenWidth {
		p.X = ScreenWidth - p.Width
	}

	// Apply gravity: accelerate downward, cap terminal velocity, then move
	p.VelY += 0.26
	if p.VelY > 3 {
		p.VelY = 3
	}
	p.Y += p.VelY

	// Ground collision detection: check the tile beneath the player's feet
	wasOn := p.OnGround
	cx := p.X + p.Width/2 // center of the player horizontally
	fy := p.Y + p.Height  // bottom edge of the player
	tx := int(cx / ts)    // tile column underfoot
	ty := int(fy / ts)    // tile row underfoot

	if isFloor(tx, ty) {
		// Snap the player to sit exactly on top of the floor tile
		p.Y = float64(ty)*ts - p.Height
		p.VelY = 0
		p.OnGround = true
	} else {
		p.OnGround = false
	}

	// Play landing sound only on the first frame touching ground
	if !wasOn && p.OnGround {
		landingSnd.Rewind()
		landingSnd.Play()
		p.idleIndex = 0
		p.idleTimer = p.idleDelay
	}

	// Ceiling collision: prevent head from embedding into solid tiles above
	headRow := int(p.Y / ts)
	if isFloor(tx, headRow) {
		p.Y = float64(headRow+1) * ts
		p.VelY = 0
	}

	// Horizontal movement and wall collision
	if p.OnGround {
		// When on ground, stop movement if next position collides with a wall
		nextX := p.X + intendedVX
		fx := nextX + p.Width/2
		fy2 := p.Y + p.Height - 1
		tx2 := int(fx / ts)
		ty2 := int(fy2 / ts)
		if isWall(tx2, ty2) {
			p.VelX = 0
		} else {
			p.VelX = intendedVX
		}
	} else {
		// In the air, allow horizontal velocity unimpeded
		p.VelX = intendedVX
	}
	p.X += p.VelX

	// Pumpkin collection: iterate over all tiles overlapping the player's bounding box
	x0 := int(p.X / ts)
	x1 := int((p.X + p.Width) / ts)
	y0 := int(p.Y / ts)
	y1 := int((p.Y + p.Height) / ts)
	for yy := y0; yy <= y1; yy++ {
		for xx := x0; xx <= x1; xx++ {
			// Skip outside the level grid
			if yy < 0 || yy >= len(Levels[CurrentLevel].Tiles) ||
				xx < 0 || xx >= len(Levels[CurrentLevel].Tiles[0]) {
				continue
			}
			// Tile ID 48 represents a pumpkin
			if Levels[CurrentLevel].Tiles[yy][xx] == 48 {
				p.Pumpkins++
				pumpkinSnd.Rewind()
				pumpkinSnd.Play()
				Levels[CurrentLevel].Tiles[yy][xx] = 0 // remove pumpkin from level
			}
		}
	}

	// Only proceed with movement animations and sounds if not mid-interaction
	if !p.interacting {
		// Footstep sounds: play periodically while walking on ground
		if p.OnGround && p.VelX != 0 {
			p.stepTimer--
			if p.stepTimer <= 0 {
				floorID := p.currentFloorID()
				if snd, ok := stepSnds[floorID]; ok {
					snd.Rewind()
					snd.Play()
				}
				p.stepTimer = stepInterval
			}
		} else {
			// Reset timer when standing still or airborne
			p.stepTimer = 0
		}

		// Handle jump input, including single and double jump logic
		p.handleJumpInput(jumpSnd)

		// Sprite animation: choose walking, idle, or jumping frames
		if p.OnGround {
			if p.VelX != 0 {
				// Walking animation: advance frames at walkFPS rate
				p.walkTimer--
				if p.walkTimer <= 0 {
					p.walkTimer = p.walkDelay
					p.frameIndex = (p.frameIndex + 1) % numFrames
				}
			} else {
				// Idle animation: advance frames at idleFPS rate
				p.idleTimer--
				if p.idleTimer <= 0 {
					p.idleTimer = p.idleDelay
					p.frameIndex = (p.frameIndex + 1) % numFrames
				}
			}
		} else {
			// In air: display the first jump frame and reset other timers
			p.frameIndex = 0
			p.walkTimer = p.walkDelay
			p.idleTimer = p.idleDelay
		}
	}
}

// Collision and helper utilities for detecting solid tiles, doing simple bounding-box checks,
// and exposing player state (facing direction, interaction flag).

// isWall returns true if the tile at (tx, ty) is one of the solid “wall” tiles
// that should stop horizontal movement or block passage.
func isWall(tx, ty int) bool {
	// get grid dimensions
	rows := len(Levels[CurrentLevel].Tiles)
	cols := len(Levels[CurrentLevel].Tiles[0])

	// out-of-bounds tiles are treated as empty space (no wall)
	if ty < 0 || ty >= rows || tx < 0 || tx >= cols {
		return false
	}

	// list of tile IDs considered solid walls
	switch Levels[CurrentLevel].Tiles[ty][tx] {
	case 10, 11, 27, 33, 37, 38, 39, 47, 49, 50, 51, 84, 90:
		return true
	default:
		return false
	}
}

// isFloor returns true for any tile that should block vertical movement,
// i.e. ground or ceiling. Currently, floor tiles are the same as walls.
func isFloor(tx, ty int) bool {
	return isWall(tx, ty)
}

// SolidAt checks whether any solid tile overlaps a given rectangle in world space.
// It converts the rectangle (x, y, w, h) into tile coordinates and scans each tile.
// Returns true as soon as it finds a wall tile under or beside the box.
func SolidAt(x, y, w, h float64) bool {
	ts := float64(TileSize)

	// compute tile ranges covered by the rectangle
	x0 := int(x / ts)
	x1 := int((x + w) / ts)
	y0 := int(y / ts)
	y1 := int((y + h) / ts)

	for ty := y0; ty <= y1; ty++ {
		for tx := x0; tx <= x1; tx++ {
			// skip any tile coordinate outside the level grid
			if ty < 0 || ty >= len(Levels[CurrentLevel].Tiles) ||
				tx < 0 || tx >= len(Levels[CurrentLevel].Tiles[0]) {
				continue
			}
			// if this tile is solid, we have a collision
			if isWall(tx, ty) {
				return true
			}
		}
	}
	return false
}

// Rect returns the player's axis-aligned bounding box in pixel coordinates.
// Useful for simple overlap tests with enemy rectangles or other objects.
func (e *Player) Rect() image.Rectangle {
	return image.Rect(
		int(e.X),
		int(e.Y),
		int(e.X+EnemyW),
		int(e.Y+EnemyH),
	)
}

// CollidesHeadOn checks if the player is landing on top of another rectangle r.
// It returns true only if the player is moving downward this frame,
// the bottom of the player was above r.Min.Y in the last frame,
// and their bounding boxes now overlap.
func (p *Player) CollidesHeadOn(r image.Rectangle) bool {
	pr := p.Rect()
	return p.VelY > 0 &&
		pr.Max.Y-int(p.VelY) <= r.Min.Y &&
		pr.Overlaps(r)
}

// FacingRight reports the current horizontal facing direction of the player.
// True means the player sprite should be drawn looking right.
func (p *Player) FacingRight() bool {
	return p.facingRight
}

// SetInteracting flips the internal interacting flag, indicating
// whether the player is currently performing the one-frame use animation.
func (p *Player) SetInteracting(v bool) {
	p.interacting = v
}
