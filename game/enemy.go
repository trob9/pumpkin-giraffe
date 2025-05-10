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
	EnemyW, EnemyH      = 16, 16
	PatrolDistanceTiles = 2
	PatrolSpeed         = 0.5
	AnimFPS             = 8
)

type Enemy struct {
	X, Y              float64
	dir               float64
	startX            float64
	endX              float64
	framesLeft        []*ebiten.Image
	framesRight       []*ebiten.Image
	idx, timer, delay int
	Alive             bool
}

// NewEnemyFS loads its sprite sheets from the provided fs.FS (e.g. your Assets)
// at paths "assets/sprites/skull_walk_right.png" and "..._left.png".
func NewEnemyFS(fsys fs.FS, x, y float64) *Enemy {
	// helper to load and slice a sheet
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
			r := image.Rect(i*EnemyW, 0, (i+1)*EnemyW, EnemyH)
			out[i] = ebImg.SubImage(r).(*ebiten.Image)
		}
		return out
	}

	// we assume your sprites live under assets/sprites in your embed.FS:
	rightFrames := loadSheet(filepath.ToSlash("assets/sprites/skull_walk_right.png"))
	leftFrames := loadSheet(filepath.ToSlash("assets/sprites/skull_walk_left.png"))

	delay := 60 / AnimFPS

	return &Enemy{
		X:           x,
		Y:           y - EnemyH, // put bottom of sprite on the tile
		dir:         -1,
		startX:      x,
		endX:        x + PatrolDistanceTiles*TileSize,
		framesRight: rightFrames,
		framesLeft:  leftFrames,
		delay:       delay,
		timer:       delay,
		Alive:       true,
	}
}

func (e *Enemy) Update() {
	if !e.Alive {
		return
	}
	// patrol movement
	e.X += e.dir * PatrolSpeed
	if e.X < e.startX {
		e.X, e.dir = e.startX, 1
	}
	if e.X > e.endX {
		e.X, e.dir = e.endX, -1
	}
	// animation timer
	e.timer--
	if e.timer <= 0 {
		e.timer = e.delay
		e.idx = (e.idx + 1) % len(e.framesRight)
	}
}

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

// Rect returns the enemy's AABB for collision.
func (e *Enemy) Rect() image.Rectangle {
	return image.Rect(
		int(e.X), int(e.Y),
		int(e.X+EnemyW), int(e.Y+EnemyH),
	)
}
