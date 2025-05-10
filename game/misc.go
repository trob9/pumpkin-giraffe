// game/pumpkin.go
package game

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

// Pumpkin is your collectible
type Pumpkin struct {
	X, Y   float64
	VelY   float64
	Width  float64
	Height float64
	Image  *ebiten.Image
	Alive  bool
}

// NewPumpkin returns a fresh pumpkin that will fall from (x,y).
func NewPumpkin(x, y float64) *Pumpkin {
	return &Pumpkin{
		X:      x,
		Y:      y,
		VelY:   0,
		Width:  float64(TileSize),
		Height: float64(TileSize),
		Image:  tileImages[48-1], // reuse tile 48 graphic
		Alive:  true,
	}
}

// Rect returns its current AABB.
func (p *Pumpkin) Rect() image.Rectangle {
	return image.Rect(
		int(p.X), int(p.Y),
		int(p.X+p.Width), int(p.Y+p.Height),
	)
}
