//Defines the Enemy type (position, patrol range, animation frames) 
//and its Update/Draw methods for walking-skull patrol logic and sprite animation.
package game

import (
	"bytes"
	"image"
	_ "image/png"
	"io/fs"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	EnemyW, EnemyH      = 16, 16 // enemy sprite width & height in pixels
	PatrolDistanceTiles = 2      // how many tiles an enemy walks before turning
	PatrolSpeed         = 0.5    // horizontal speed in world units per frame
	AnimFPS             = 8      // animation frames per second
)

// Enemy represents a walking skull that patrols back and forth.
type Enemy struct {
	X, Y              float64         // world position (top-left of sprite)
	dir               float64         // current horizontal direction: +1 for right, -1 for left
	startX            float64         // leftmost patrol x
	endX              float64         // rightmost patrol x
	framesLeft        []*ebiten.Image // animation frames facing left
	framesRight       []*ebiten.Image // animation frames facing right
	idx, timer, delay int             // animation index, countdown timer, and per-frame delay
	Alive             bool            // whether this enemy should update & draw
}

// NewEnemyFS loads left/right walk animations from the embedded FS,
// positions the sprite so its bottom aligns on the tile at (x,y),
// and sets up patrol boundaries.
func NewEnemyFS(fsys fs.FS, x, y float64) *Enemy {
	// helper to load a horizontal sprite sheet and split into frames
	loadSheet := func(relPath string) []*ebiten.Image {
		data, err := fs.ReadFile(fsys, relPath)
		if err != nil {
			panic("reading " + relPath + ": " + err.Error())
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			panic("decoding " + relPath + ": " + err.Error())
		}
		ebImg := ebiten.NewImageFromImage(img)
		count := ebImg.Bounds().Dx() / EnemyW
		out := make([]*ebiten.Image, count)
		for i := 0; i < count; i++ {
			// slice each frame by fixed width
			r := image.Rect(i*EnemyW, 0, (i+1)*EnemyW, EnemyH)
			out[i] = ebImg.SubImage(r).(*ebiten.Image)
		}
		return out
	}

	// load frames for both directions
	rightFrames := loadSheet(filepath.ToSlash("assets/sprites/skull_walk_right.png"))
	leftFrames := loadSheet(filepath.ToSlash("assets/sprites/skull_walk_left.png"))

	// compute how many update calls between animation frame switches
	delay := 60 / AnimFPS

	return &Enemy{
		X:           x,
		Y:           y - EnemyH, // place so that sprite’s bottom rests at y
		dir:         -1,         // start moving left
		startX:      x,
		endX:        x + PatrolDistanceTiles*TileSize,
		framesRight: rightFrames,
		framesLeft:  leftFrames,
		delay:       delay,
		timer:       delay,
		Alive:       true,
	}
}

// Update moves the enemy along its patrol range and advances animation frames.
func (e *Enemy) Update() {
	if !e.Alive {
		return // skip if dead
	}

	// move in current direction, then bounce at patrol edges
	e.X += e.dir * PatrolSpeed
	if e.X < e.startX {
		e.X, e.dir = e.startX, 1 // hit left edge, turn right
	}
	if e.X > e.endX {
		e.X, e.dir = e.endX, -1 // hit right edge, turn left
	}

	// animation: count down, then advance to next sprite frame
	e.timer--
	if e.timer <= 0 {
		e.timer = e.delay
		e.idx = (e.idx + 1) % len(e.framesRight)
	}
}

// Draw renders the enemy using the correct frame and facing direction.
func (e *Enemy) Draw(screen *ebiten.Image, camX, camY float64) {
	if !e.Alive {
		return // don't draw if dead
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(e.X-camX, e.Y-camY)
	// choose left or right animation slice based on direction sign
	if e.dir > 0 {
		screen.DrawImage(e.framesRight[e.idx], op)
	} else {
		screen.DrawImage(e.framesLeft[e.idx], op)
	}
}

// Rect returns the enemy’s bounding box for simple AABB collision checks.
func (e *Enemy) Rect() image.Rectangle {
	return image.Rect(
		int(e.X),
		int(e.Y),
		int(e.X+EnemyW),
		int(e.Y+EnemyH),
	)
}
