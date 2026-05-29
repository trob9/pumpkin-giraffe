// Lightweight transient visual effects — currently a "poof" burst when an enemy
// is defeated, so kills read with a bit of pop instead of the sprite just
// vanishing. Pure code, no assets; particles expand and fade then expire.
package game

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

const poofLife = 22 // frames a poof lives

// Effect is one short-lived burst of particles at a world position.
type Effect struct {
	x, y float64
	age  int
}

// NewPoof creates a defeat burst centred at (x, y) in world pixels.
func NewPoof(x, y float64) *Effect { return &Effect{x: x, y: y} }

// Update advances the effect; Done reports when it should be removed.
func (e *Effect) Update()    { e.age++ }
func (e *Effect) Done() bool { return e.age >= poofLife }

// Draw renders the expanding, fading ring of bone-white motes.
func (e *Effect) Draw(screen *ebiten.Image, camX, camY float64) {
	pix := neckPixelImage()
	t := float64(e.age) / poofLife
	radius := 2 + t*9          // expand outward
	alpha := float32(1 - t)    // fade out
	size := 2.0 - t            // shrink each mote slightly
	if alpha <= 0 || size <= 0 {
		return
	}
	const n = 8
	for i := 0; i < n; i++ {
		ang := float64(i) / n * 2 * math.Pi
		px := e.x + math.Cos(ang)*radius - camX
		py := e.y + math.Sin(ang)*radius - camY
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(size, size)
		op.GeoM.Translate(px, py)
		op.ColorScale.ScaleWithColor(color.RGBA{220, 215, 225, 255})
		op.ColorScale.ScaleAlpha(alpha)
		screen.DrawImage(pix, op)
	}
}
