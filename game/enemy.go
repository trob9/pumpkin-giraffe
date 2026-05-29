// Defines the Enemy type and its behaviour: a floating skull that patrols a
// stretch, but lunges into a chase when the giraffe wanders into its alert
// range, and gives up if the giraffe gets far enough away. Two variants exist —
// a steady Patroller and a faster, longer-sighted Chaser.
package game

import (
	"bytes"
	"image"
	_ "image/png"
	"io/fs"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	EnemyW, EnemyH      = 16, 16 // enemy sprite width & height in pixels
	PatrolDistanceTiles = 2      // how many tiles a patroller walks before turning
	PatrolSpeed         = 0.5    // patrol horizontal speed (world units per frame)
	AnimFPS             = 8      // animation frames per second
)

// EnemyKind selects an enemy's behaviour/appearance.
type EnemyKind int

const (
	KindPatroller EnemyKind = iota // steady back-and-forth, short sight
	KindChaser                     // faster, longer sight, hunts the giraffe
)

// Enemy is a floating skull. It patrols between startX/endX until the giraffe
// enters its alert range, then chases; it returns to patrolling once the
// giraffe is far enough away again.
type Enemy struct {
	X, Y              float64
	dir               float64 // facing/move direction: +1 right, -1 left
	startX, endX      float64 // patrol bounds
	kind              EnemyKind
	alerted           bool
	alertRange        float64 // horizontal sight distance to start a chase
	patrolSpeed       float64
	chaseSpeed        float64
	framesLeft        []*ebiten.Image
	framesRight       []*ebiten.Image
	idx, timer, delay int
	Alive             bool
}

// loadEnemySheet loads a horizontal sprite sheet of EnemyW-wide frames. It tries
// `primary` first and falls back to `fallback` if the primary file is missing —
// so a variant can ship before its dedicated art exists.
func loadEnemySheet(fsys fs.FS, primary, fallback string) []*ebiten.Image {
	data, err := fs.ReadFile(fsys, primary)
	if err != nil {
		data, err = fs.ReadFile(fsys, fallback)
	}
	if err != nil {
		panic("reading enemy sheet " + primary + ": " + err.Error())
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		panic("decoding " + primary + ": " + err.Error())
	}
	eb := ebiten.NewImageFromImage(img)
	count := eb.Bounds().Dx() / EnemyW
	if count < 1 {
		count = 1
	}
	out := make([]*ebiten.Image, count)
	for i := 0; i < count; i++ {
		out[i] = eb.SubImage(image.Rect(i*EnemyW, 0, (i+1)*EnemyW, EnemyH)).(*ebiten.Image)
	}
	return out
}

// newEnemy is the shared constructor; NewEnemyFS / NewChaserFS wrap it.
func newEnemy(x, y float64, kind EnemyKind, right, left []*ebiten.Image) *Enemy {
	e := &Enemy{
		X:           x,
		Y:           y - EnemyH, // place so the sprite's bottom rests at y
		dir:         -1,
		startX:      x,
		endX:        x + PatrolDistanceTiles*TileSize,
		kind:        kind,
		framesRight: right,
		framesLeft:  left,
		delay:       60 / AnimFPS,
		timer:       60 / AnimFPS,
		Alive:       true,
	}
	switch kind {
	case KindChaser:
		e.alertRange = 110
		e.patrolSpeed = 0.55
		e.chaseSpeed = 1.05
	default:
		e.alertRange = 52
		e.patrolSpeed = PatrolSpeed
		e.chaseSpeed = 0.85
	}
	return e
}

// NewEnemyFS builds a Patroller skull at the tile bottom (x,y).
func NewEnemyFS(fsys fs.FS, x, y float64) *Enemy {
	r := loadEnemySheet(fsys, "assets/sprites/skull_walk_right.png", "assets/sprites/skull_walk_right.png")
	l := loadEnemySheet(fsys, "assets/sprites/skull_walk_left.png", "assets/sprites/skull_walk_left.png")
	return newEnemy(x, y, KindPatroller, r, l)
}

// NewChaserFS builds a Chaser. It uses the dedicated skull2 art if present and
// falls back to the standard skull art otherwise.
func NewChaserFS(fsys fs.FS, x, y float64) *Enemy {
	r := loadEnemySheet(fsys, "assets/sprites/skull2_walk_right.png", "assets/sprites/skull_walk_right.png")
	l := loadEnemySheet(fsys, "assets/sprites/skull2_walk_left.png", "assets/sprites/skull_walk_left.png")
	return newEnemy(x, y, KindChaser, r, l)
}

// Update advances the enemy one frame given the giraffe's position. It toggles
// alert state on horizontal+vertical proximity, chases when alerted, otherwise
// patrols, and always advances the walk animation.
func (e *Enemy) Update(px, py float64) {
	if !e.Alive {
		return
	}

	dx := px - e.X
	dist := math.Abs(dx)
	vdist := math.Abs(py - e.Y)

	if e.alerted {
		// Give up the chase once the giraffe is well out of range.
		if dist > e.alertRange*2.2 || vdist > 90 {
			e.alerted = false
		}
	} else if dist < e.alertRange && vdist < 60 {
		e.alerted = true
	}

	if e.alerted {
		if dx > 1 {
			e.dir = 1
		} else if dx < -1 {
			e.dir = -1
		}
		e.X += e.dir * e.chaseSpeed
		if e.X < 0 {
			e.X = 0
		}
		if e.X > float64(ScreenWidth-EnemyW) {
			e.X = float64(ScreenWidth - EnemyW)
		}
	} else {
		e.X += e.dir * e.patrolSpeed
		if e.X < e.startX {
			e.X, e.dir = e.startX, 1
		}
		if e.X > e.endX {
			e.X, e.dir = e.endX, -1
		}
	}

	if n := len(e.framesRight); n > 0 {
		e.timer--
		if e.timer <= 0 {
			e.timer = e.delay
			e.idx = (e.idx + 1) % n
		}
	}
}

// Draw renders the enemy with the correct facing frame.
func (e *Enemy) Draw(screen *ebiten.Image, camX, camY float64) {
	if !e.Alive {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(e.X-camX, e.Y-camY)
	if e.dir > 0 {
		screen.DrawImage(e.framesRight[e.idx], op)
	} else {
		screen.DrawImage(e.framesLeft[e.idx], op)
	}
}

// Rect returns the enemy's bounding box for AABB collision checks.
func (e *Enemy) Rect() image.Rectangle {
	return image.Rect(int(e.X), int(e.Y), int(e.X+EnemyW), int(e.Y+EnemyH))
}
