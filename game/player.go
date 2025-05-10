package game

import (
	"bytes"
	"embed"
	"image"
	_ "image/png"
	"io/fs"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
)

const (
	ScreenWidth      = 1280
	ScreenHeight     = 720
	stepInterval     = 20
	doubleJumpWindow = 24
	normalJumpVel    = -6.5
	doubleJumpVel    = normalJumpVel * 1.5

	idleFPS    = 3
	walkFPS    = 7
	walkFrames = 4
	numFrames  = 4
	idleFrames = 2 // <-- two frames in your idle sheets
	spriteW    = 16
	spriteH    = 16
	spriteDir  = "assets/sprites/"
)

type Player struct {
	// movement
	X, Y            float64
	VelX, VelY      float64
	Width, Height   float64
	OnGround        bool
	stepTimer       int
	framesSinceJump int

	// pumpkin count
	Pumpkins int

	// animation resources
	walkR, walkL []*ebiten.Image
	idleR, idleL []*ebiten.Image
	jumpR, jumpL *ebiten.Image
	interactR    *ebiten.Image
	interactL    *ebiten.Image

	// animation state
	frameIndex, walkTimer, walkDelay int
	idleIndex, idleTimer, idleDelay  int
	facingRight                      bool

	// interaction callbacks & state
	interacting     bool
	onInteract      func(msg string)
	onPumpkinSpawn  func()
	onPumpkinRedrop func()

	prevUse bool
}

var (
	spawnX float64 = 64
	spawnY float64 = 300
)

func loadSheet(fsys fs.FS, path string, frames int) []*ebiten.Image {
	data, err := fs.ReadFile(fsys, path) // use fs.ReadFile
	if err != nil {
		panic(err)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		panic(err)
	}
	sheet := ebiten.NewImageFromImage(img)
	out := make([]*ebiten.Image, frames)
	for i := 0; i < frames; i++ {
		r := image.Rect(i*spriteW, 0, (i+1)*spriteW, spriteH)
		out[i] = sheet.SubImage(r).(*ebiten.Image)
	}
	return out
}

func loadImage(fsys fs.FS, path string) *ebiten.Image {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		panic(err)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		panic(err)
	}
	return ebiten.NewImageFromImage(img)
}
func NewPlayer(
	assets embed.FS,
	onInteract func(string),
	onPumpkinSpawn func(),
	onPumpkinRedrop func(),
) *Player {
	loadSheet := func(path string, frames int) []*ebiten.Image {
		data, err := assets.ReadFile(path) // ← read from your embed.FS
		if err != nil {
			panic(err)
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			panic(err)
		}
		sheet := ebiten.NewImageFromImage(img)
		out := make([]*ebiten.Image, frames)
		for i := 0; i < frames; i++ {
			r := image.Rect(i*spriteW, 0, (i+1)*spriteW, spriteH)
			out[i] = sheet.SubImage(r).(*ebiten.Image)
		}
		return out
	}

	walkR := loadSheet(spriteDir+"player_walk_right.png", walkFrames)
	walkL := loadSheet(spriteDir+"player_walk_left.png", walkFrames)
	idleR := loadSheet(spriteDir+"player_idle_right.png", idleFrames)
	idleL := loadSheet(spriteDir+"player_idle_left.png", idleFrames)

	loadImage := func(path string) *ebiten.Image {
		data, err := assets.ReadFile(path)
		if err != nil {
			panic(err)
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			panic(err)
		}
		return ebiten.NewImageFromImage(img)
	}

	// single-frame jumps
	jr := loadImage(spriteDir + "player_jump_right.png")
	jl := loadImage(spriteDir + "player_jump_left.png")
	interactR := loadImage(spriteDir + "player_interact_right.png")
	interactL := loadImage(spriteDir + "player_interact_left.png")

	walkDelay := 60 / walkFPS
	idleDelay := (60 / idleFPS) * 15

	return &Player{
		X:               spawnX,
		Y:               spawnY,
		Width:           spriteW,
		Height:          spriteH,
		stepTimer:       stepInterval,
		framesSinceJump: doubleJumpWindow + 1,

		walkR:     walkR,
		walkL:     walkL,
		idleR:     idleR,
		idleL:     idleL,
		jumpR:     jr,
		jumpL:     jl,
		interactR: interactR,
		interactL: interactL,

		walkDelay:       walkDelay,
		walkTimer:       walkDelay,
		idleDelay:       idleDelay,
		idleTimer:       idleDelay,
		facingRight:     true,
		onInteract:      onInteract,
		onPumpkinSpawn:  onPumpkinSpawn,
		onPumpkinRedrop: onPumpkinRedrop,
		prevUse:         false,
	}
}

func (p *Player) Respawn() {
	p.X, p.Y = spawnX, spawnY
	p.VelX, p.VelY = 0, 0
	p.OnGround = false
	p.framesSinceJump = doubleJumpWindow + 1
}

func (p *Player) currentFloorID() int {
	cx := p.X + p.Width/2
	ty := int((p.Y + p.Height) / float64(TileSize))
	tx := int(cx / float64(TileSize))
	return Levels[CurrentLevel].Tiles[ty][tx]
}

func (p *Player) Update(
	jumpSnd *audio.Player,
	deathSnd *audio.Player,
	stepSnds map[int]*audio.Player,
	landingSnd *audio.Player,
	monsterDeathSnd *audio.Player,
	pumpkinSnd *audio.Player, // SOUND DEPENDENCY (ADD MORE SOUND PARAMETERS TO ACCEPT IF YOU'RE ADDING MORE SOUND)
	pumpkinMissed bool,
) {
	p.framesSinceJump++
	ts := float64(TileSize)
	moveSpeed := 1.5

	// 0) respawn if fallen
	if p.Y > float64(ScreenHeight) {
		deathSnd.Rewind()
		deathSnd.Play()
		p.Respawn()
		return
	}

	// --- INPUT ---
	var intendedVX float64
	if ebiten.IsKeyPressed(ebiten.KeyLeft) || ebiten.IsKeyPressed(ebiten.KeyA) {
		intendedVX = -moveSpeed
	} else if ebiten.IsKeyPressed(ebiten.KeyRight) || ebiten.IsKeyPressed(ebiten.KeyD) {
		intendedVX = moveSpeed
	}
	if intendedVX < 0 {
		p.facingRight = false
	} else if intendedVX > 0 {
		p.facingRight = true
	}

	// --- X BOUNDS ---
	if p.X < 0 {
		p.X = 0
	}
	if p.X+p.Width > ScreenWidth {
		p.X = ScreenWidth - p.Width
	}

	// PUMPKINS START HERE - PUMPKINS START HERE - PUMPKINS START HERE - PUMPKINS START HERE - PUMPKINS START HERE - PUMPKINS START HERE - PUMPKINS START HERE
	// --- INTERACT LOGIC & POSE ---
	pressed := ebiten.IsKeyPressed(ebiten.KeyE)
	nowUse := pressed && !p.prevUse
	p.prevUse = pressed

	// Do we show the interact sprite?  Yes if E is down and there's a barrel beside us:
	{
		ts := float64(TileSize)
		ty := int((p.Y + p.Height/2) / ts)
		var tx int
		if p.facingRight {
			tx = int((p.X + p.Width + 1) / ts)
		} else {
			tx = int((p.X - 1) / ts)
		}
		barrelBeside := ty >= 0 && ty < len(Levels[CurrentLevel].Tiles) &&
			tx >= 0 && tx < len(Levels[CurrentLevel].Tiles[0]) &&
			Levels[CurrentLevel].Tiles[ty][tx] == 11
		p.interacting = pressed && barrelBeside
	}

	// Only on the rising edge do we inspect above‐tile and spawn/message:
	if nowUse {
		ts := float64(TileSize)
		ty := int((p.Y + p.Height/2) / ts)
		var tx int
		if p.facingRight {
			tx = int((p.X + p.Width + 1) / ts)
		} else {
			tx = int((p.X - 1) / ts)
		}

		above := ty - 1
		switch {
		case above >= 1 && Levels[CurrentLevel].Tiles[above][tx] == 84:
			if pumpkinMissed {
				p.onInteract("…is that something falling from the sky?")
				p.onPumpkinRedrop()
			} else {
				p.onInteract("I wonder if something will fall into this barrel just for me...")
			}

		case above >= 1 && Levels[CurrentLevel].Tiles[above][tx] == 90:
			Levels[CurrentLevel].Tiles[above][tx] = 48
			p.onInteract("Wow, there’s a pumpkin inside!")
			p.onPumpkinSpawn()

		default:
			p.onInteract("There’s nothing inside...")
		}
	}

	// PUMPKINS END HERE - PUMPKINS END HERE - PUMPKINS END HERE - PUMPKINS END HERE - PUMPKINS END HERE - PUMPKINS END HERE - PUMPKINS END HERE - PUMPKINS END HERE - PUMPKINS END HERE - PUMPKINS END HERE -
	// --- GRAVITY & V ---
	p.VelY += 0.26
	if p.VelY > 3 {
		p.VelY = 3
	}
	p.Y += p.VelY

	// --- GROUND COLLISION & LAND ---
	wasOn := p.OnGround
	cx := p.X + p.Width/2
	fy := p.Y + p.Height
	tx := int(cx / ts)
	ty := int(fy / ts)
	if isFloor(tx, ty) {
		p.Y = float64(ty)*ts - p.Height
		p.VelY = 0
		p.OnGround = true
	} else {
		p.OnGround = false
	}
	if !wasOn && p.OnGround {
		landingSnd.Rewind()
		landingSnd.Play()
		p.idleIndex = 0
		p.idleTimer = p.idleDelay
	}

	// --- CEILING ---
	headY := p.Y
	ty = int(headY / ts)
	if isFloor(tx, ty) {
		p.Y = float64(ty+1) * ts
		p.VelY = 0
	}

	// --- HORIZONTAL & WALL ---
	if p.OnGround {
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
		p.VelX = intendedVX
	}
	p.X += p.VelX

	// --- PUMPKINS ---
	x0 := int(p.X / ts)
	x1 := int((p.X + p.Width) / ts)
	y0 := int(p.Y / ts)
	y1 := int((p.Y + p.Height) / ts)
	for yy := y0; yy <= y1; yy++ {
		for xx := x0; xx <= x1; xx++ {
			if yy < 0 || yy >= len(Levels[CurrentLevel].Tiles) ||
				xx < 0 || xx >= len(Levels[CurrentLevel].Tiles[0]) {
				continue
			}
			if Levels[CurrentLevel].Tiles[yy][xx] == 48 {
				p.Pumpkins++
				pumpkinSnd.Rewind()
				pumpkinSnd.Play()
				Levels[CurrentLevel].Tiles[yy][xx] = 0
			}
		}
	}

	/* p.interacting = false
	if ebiten.IsKeyPressed(ebiten.KeyE) {
		// check the tile just beside you, in the direction you face:
		ty := int((p.Y + p.Height/2) / float64(TileSize))
		var tx int
		if p.facingRight {
			tx = int((p.X + p.Width + 1) / float64(TileSize))
		} else {
			tx = int((p.X - 1) / float64(TileSize))
		}
		if tx >= 0 && ty >= 0 && tx < len(Levels[CurrentLevel].Tiles[0]) &&
			ty < len(Levels[CurrentLevel].Tiles) &&
			Levels[CurrentLevel].Tiles[ty][tx] == 11 {
			p.interacting = true
			nowInteracting := tx >= 0 && ty >= 0 && tx < len(Levels[CurrentLevel].Tiles[0]) &&
				ty < len(Levels[CurrentLevel].Tiles) &&
				Levels[CurrentLevel].Tiles[ty][tx] == 11

			if nowInteracting && !p.prevInteracting {
				if p.onInteract != nil {
				}
			}

		}
	}*/
	if !p.interacting {
		// --- FOOTSTEPS ---
		if p.OnGround && p.VelX != 0 {
			p.stepTimer--
			if p.stepTimer <= 0 {
				id := p.currentFloorID()
				if snd, ok := stepSnds[id]; ok {
					snd.Rewind()
					snd.Play()
				}
				p.stepTimer = stepInterval
			}
		} else {
			p.stepTimer = 0
		}

		// --- JUMP ---
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
		}

		// --- IDLE ANIM (when stopped on ground) ---
		// ANIMATION: separate walk vs. idle
		if p.OnGround {
			if p.VelX != 0 {
				// walk‐cycle
				p.walkTimer--
				if p.walkTimer <= 0 {
					p.walkTimer = p.walkDelay
					p.frameIndex = (p.frameIndex + 1) % numFrames
				}
			} else {
				// idle‐cycle
				p.idleTimer--
				if p.idleTimer <= 0 {
					p.idleTimer = p.idleDelay
					p.frameIndex = (p.frameIndex + 1) % numFrames
				}
			}
		} else {
			// in air, just show jump frame (reset timers if you like)
			p.frameIndex = 0
			p.walkTimer = p.walkDelay
			p.idleTimer = p.idleDelay
		}
	}
}

func (p *Player) Draw(screen *ebiten.Image, camX, camY float64) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(p.X-camX, p.Y-camY)

	if p.interacting {
		if p.facingRight {
			screen.DrawImage(p.interactR, op)
		} else {
			screen.DrawImage(p.interactL, op)
		}
		return
	}

	var img *ebiten.Image
	switch {
	case !p.OnGround:
		if p.facingRight {
			img = p.jumpR
		} else {
			img = p.jumpL
		}
	case p.VelX != 0:
		if p.facingRight {
			img = p.walkR[p.frameIndex]
		} else {
			img = p.walkL[p.frameIndex]
		}
	default: // idle
		if p.facingRight {
			img = p.idleR[p.idleIndex]
		} else {
			img = p.idleL[p.idleIndex]
		}
	}
	p.idleTimer--
	if p.idleTimer <= 0 {
		p.idleTimer = p.idleDelay
		p.idleIndex = (p.idleIndex + 1) % idleFrames
	}

	screen.DrawImage(img, op)
}

// isWall blocks horizontal movement
func isWall(tx, ty int) bool {
	rows := len(Levels[CurrentLevel].Tiles)
	cols := len(Levels[CurrentLevel].Tiles[0])
	if ty < 0 || ty >= rows || tx < 0 || tx >= cols {
		return false
	}
	switch Levels[CurrentLevel].Tiles[ty][tx] {
	case 10, 11, 27, 33, 37, 38, 39, 47, 49, 50, 51:
		return true
	default:
		return false
	}
}

// isFloor blocks vertical movement (ground & ceiling)
func isFloor(tx, ty int) bool {
	return isWall(tx, ty)
}

// solidAt reports whether any solid tile overlaps the given AABB.
func SolidAt(x, y, w, h float64) bool {
	ts := float64(TileSize)
	x0 := int(x / ts)
	x1 := int((x + w) / ts)
	y0 := int(y / ts)
	y1 := int((y + h) / ts)

	for ty := y0; ty <= y1; ty++ {
		for tx := x0; tx <= x1; tx++ {
			if ty < 0 || ty >= len(Levels[CurrentLevel].Tiles) ||
				tx < 0 || tx >= len(Levels[CurrentLevel].Tiles[0]) {
				continue
			}
			if isWall(tx, ty) {
				return true
			}
		}
	}
	return false
}

// Rect for simple AABB collision
func (e *Player) Rect() image.Rectangle {
	return image.Rect(
		int(e.X), int(e.Y),
		int(e.X+EnemyW), int(e.Y+EnemyH),
	)
}
func (p *Player) CollidesHeadOn(r image.Rectangle) bool {
	pr := p.Rect()
	// only if moving downward and last frame the bottom was above r.Min.Y
	return p.VelY > 0 &&
		pr.Max.Y-int(p.VelY) <= r.Min.Y &&
		pr.Overlaps(r)
}

// FacingRight reports which way the player is currently facing.
func (p *Player) FacingRight() bool {
	return p.facingRight
}
