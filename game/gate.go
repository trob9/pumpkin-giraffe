// The end-of-level gate: a glowing portal that opens only when the giraffe is
// carrying enough pumpkins to pay its toll. Pumpkins are the game's currency —
// the gate spends them to let you through to the next level.
package game

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// Gate is a portal placed near the end of a level.
type Gate struct {
	X, Y, W, H float64
	Required   int     // pumpkins it costs to open
	Open       bool    // becomes true once paid and passed
	anim       float64 // animation phase
}

// NewGate builds a gate whose top-left is (x,y) with the given size and toll.
func NewGate(x, y, w, h float64, required int) *Gate {
	return &Gate{X: x, Y: y, W: w, H: h, Required: required}
}

// Rect returns the gate's bounding box (used to detect the giraffe entering it).
func (g *Gate) Rect() image.Rectangle {
	return image.Rect(int(g.X), int(g.Y), int(g.X+g.W), int(g.Y+g.H))
}

// Update advances the swirling animation.
func (g *Gate) Update() { g.anim += 0.06 }

// Draw renders the portal. When affordable it glows warm cyan-green and pulses
// invitingly; when you can't yet pay the toll it sits dim and violet.
func (g *Gate) Draw(screen *ebiten.Image, camX, camY float64, affordable bool) {
	pix := neckPixelImage()
	x0 := g.X - camX
	y0 := g.Y - camY

	// Energy fill: horizontal bands whose brightness ripples up the portal.
	for row := 0; row < int(g.H); row += 2 {
		ry := float64(row)
		wave := 0.5 + 0.5*math.Sin(g.anim*2+ry*0.18)
		var c color.RGBA
		if affordable {
			c = color.RGBA{
				uint8(40 + 60*wave),
				uint8(170 + 70*wave),
				uint8(150 + 80*wave),
				uint8(120 + 90*wave),
			}
		} else {
			c = color.RGBA{
				uint8(70 + 40*wave),
				uint8(40 + 20*wave),
				uint8(110 + 60*wave),
				uint8(70 + 50*wave),
			}
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(g.W-4, 2)
		op.GeoM.Translate(x0+2, y0+ry)
		op.ColorScale.ScaleWithColor(c)
		screen.DrawImage(pix, op)
	}

	// Stone-ish frame: two side pillars and a lintel.
	frame := color.RGBA{120, 110, 130, 255}
	drawBar := func(bx, by, bw, bh float64) {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(bw, bh)
		op.GeoM.Translate(bx, by)
		op.ColorScale.ScaleWithColor(frame)
		screen.DrawImage(pix, op)
	}
	drawBar(x0, y0, 3, g.H)          // left pillar
	drawBar(x0+g.W-3, y0, 3, g.H)    // right pillar
	drawBar(x0, y0, g.W, 3)          // lintel
}
