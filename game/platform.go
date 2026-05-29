// Moving platforms: solid surfaces that slide back and forth on a fixed axis.
// They are rendered with the same wood-platform tiles the static maps use
// (left cap, middle spans, right cap) so they look native to the world.
package game

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

// Tile IDs used to draw a moving platform, matching the static-map grammar:
// a left cap, a repeated middle span, and a right cap.
const (
	platLeftCap  = 37
	platMid      = 39
	platRightCap = 38
)

// MovingPlatform is a one-way solid surface that ping-pongs between its origin
// and origin+range along a single axis. "One-way" means the player lands on it
// from above and rides it, but can jump up through it from below.
type MovingPlatform struct {
	X, Y         float64 // current top-left position in world pixels
	W, H         float64 // size in world pixels (H is always one tile)
	originX      float64 // start of the travel path
	originY      float64
	axis         string  // "x" for horizontal travel, "y" for vertical
	rng          float64 // how far it travels from the origin, in pixels
	speed        float64 // pixels per frame
	dir          float64 // current travel direction: +1 or -1
	prevX, prevY float64 // position last frame, used to carry a riding player
}

// NewMovingPlatform builds a platform at (x,y) with the given size and motion.
func NewMovingPlatform(x, y, w, h, rng, speed float64, axis string) *MovingPlatform {
	return &MovingPlatform{
		X: x, Y: y, W: w, H: h,
		originX: x, originY: y,
		axis:  axis,
		rng:   rng,
		speed: speed,
		dir:   1,
		prevX: x, prevY: y,
	}
}

// Update advances the platform one frame, bouncing at the ends of its path.
// It records the previous position first so DeltaX/DeltaY can report this
// frame's movement for carrying a player that's standing on it.
func (m *MovingPlatform) Update() {
	m.prevX, m.prevY = m.X, m.Y

	if m.axis == "y" {
		m.Y += m.dir * m.speed
		if m.Y > m.originY+m.rng {
			m.Y = m.originY + m.rng
			m.dir = -1
		} else if m.Y < m.originY {
			m.Y = m.originY
			m.dir = 1
		}
		return
	}

	// default: horizontal
	m.X += m.dir * m.speed
	if m.X > m.originX+m.rng {
		m.X = m.originX + m.rng
		m.dir = -1
	} else if m.X < m.originX {
		m.X = m.originX
		m.dir = 1
	}
}

// DeltaX is how far the platform moved horizontally this frame.
func (m *MovingPlatform) DeltaX() float64 { return m.X - m.prevX }

// DeltaY is how far the platform moved vertically this frame.
func (m *MovingPlatform) DeltaY() float64 { return m.Y - m.prevY }

// Rect returns the platform's bounding box for collision checks.
func (m *MovingPlatform) Rect() image.Rectangle {
	return image.Rect(int(m.X), int(m.Y), int(m.X+m.W), int(m.Y+m.H))
}

// Draw renders the platform across its width using cap/middle/cap tiles.
func (m *MovingPlatform) Draw(screen *ebiten.Image, camX, camY float64) {
	cols := int(m.W) / TileSize
	if cols <= 0 {
		cols = 1
	}
	for c := 0; c < cols; c++ {
		id := platMid
		if cols > 1 {
			if c == 0 {
				id = platLeftCap
			} else if c == cols-1 {
				id = platRightCap
			}
		}
		img := TileImage(id)
		if img == nil {
			continue
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(m.X+float64(c*TileSize)-camX, m.Y-camY)
		screen.DrawImage(img, op)
	}
}
