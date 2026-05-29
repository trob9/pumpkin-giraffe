// Pushable boulders: solid blocks that obey gravity and can be shoved
// horizontally by the player. Push one into a pit to make a step, or line it up
// under an out-of-reach ledge and jump off it. A puzzle element introduced on
// the later levels.
package game

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

// boulderTile is the tileset graphic used to draw a boulder.
const boulderTile = 71

// Boulder is a 1-tile solid block affected by gravity. The player can stand on
// its top and push it left/right when standing beside it on the ground.
type Boulder struct {
	X, Y float64
	W, H float64
	VelY float64
}

// NewBoulder creates a boulder whose top-left starts at (x, y). It will fall to
// the first solid tile beneath it.
func NewBoulder(x, y float64) *Boulder {
	return &Boulder{X: x, Y: y, W: float64(TileSize), H: float64(TileSize)}
}

// Rect returns the boulder's bounding box.
func (b *Boulder) Rect() image.Rectangle {
	return image.Rect(int(b.X), int(b.Y), int(b.X+b.W), int(b.Y+b.H))
}

// Update applies gravity and rests the boulder on the first solid tile below it.
func (b *Boulder) Update() {
	ts := float64(TileSize)
	b.VelY += 0.26
	if b.VelY > 3 {
		b.VelY = 3
	}
	b.Y += b.VelY

	cx := b.X + b.W/2
	footY := b.Y + b.H
	tx := int(cx / ts)
	ty := int(footY / ts)
	if isWall(tx, ty) {
		b.Y = float64(ty)*ts - b.H
		b.VelY = 0
	}
}

// CanMove reports whether the boulder can shift by dx without hitting a solid
// tile or leaving the world.
func (b *Boulder) CanMove(dx float64) bool {
	ts := float64(TileSize)
	newX := b.X + dx
	if newX < 0 || newX+b.W > ScreenWidth {
		return false
	}
	edge := newX
	if dx > 0 {
		edge = newX + b.W - 1
	}
	midY := b.Y + b.H/2
	tx := int(edge / ts)
	ty := int(midY / ts)
	return !isWall(tx, ty)
}

// Draw renders the boulder offset by the camera.
func (b *Boulder) Draw(screen *ebiten.Image, camX, camY float64) {
	img := TileImage(boulderTile)
	if img == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(b.X-camX, b.Y-camY)
	screen.DrawImage(img, op)
}
