// Ambient atmosphere: drifting particles and a colour-grade tint layered over
// each level to give it a distinct time-of-day mood. Pure code/no assets — the
// particles wrap around the viewport and the tint is a translucent overlay.
package game

import (
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
)

// AmbientKind selects a level's atmosphere.
type AmbientKind int

const (
	AmbNight AmbientKind = iota // cool, sparse drifting motes (the original mood)
	AmbDay                      // bright; pale pollen floating up
	AmbDusk                     // warm; fireflies pulsing
	AmbCave                     // dark; embers rising, heavy vignette
)

type mote struct {
	x, y, vx, vy, size, phase, blink float64
}

// Ambient owns a level's particle field and lighting tint. The tint is stored
// as a normalised colour (0..1) plus an opacity, and drawn with PREMULTIPLIED
// alpha so a light tint stays subtle instead of additively blowing out the scene.
type Ambient struct {
	kind                   AmbientKind
	w, h                   float64
	motes                  []mote
	tintR, tintG, tintB    float32 // 0..1 tint colour
	tintA                  float32 // 0..1 opacity
	t                      float64
}

// NewAmbient builds the atmosphere for a viewport of w x h pixels.
func NewAmbient(kind AmbientKind, w, h int) *Ambient {
	a := &Ambient{kind: kind, w: float64(w), h: float64(h)}
	n := 0
	switch kind {
	case AmbDay:
		n = 26
		a.tintR, a.tintG, a.tintB, a.tintA = 1.0, 0.96, 0.78, 0.06 // faint warm
	case AmbDusk:
		n = 30
		a.tintR, a.tintG, a.tintB, a.tintA = 1.0, 0.55, 0.32, 0.16 // warm orange
	case AmbCave:
		n = 34
		a.tintR, a.tintG, a.tintB, a.tintA = 0.12, 0.09, 0.24, 0.40 // dark indigo
	default: // night
		n = 18
		a.tintR, a.tintG, a.tintB, a.tintA = 0.18, 0.27, 0.5, 0.12 // cool blue
	}
	for i := 0; i < n; i++ {
		a.motes = append(a.motes, a.spawn(rand.Float64()*a.h))
	}
	return a
}

func (a *Ambient) spawn(y float64) mote {
	m := mote{
		x:     rand.Float64() * a.w,
		y:     y,
		size:  1 + rand.Float64()*1.5,
		phase: rand.Float64() * math.Pi * 2,
		blink: 0.6 + rand.Float64()*1.4,
	}
	switch a.kind {
	case AmbDay: // pollen drifting gently up and to the right
		m.vx, m.vy = 0.15+rand.Float64()*0.2, -0.05-rand.Float64()*0.1
	case AmbDusk: // fireflies wandering
		m.vx, m.vy = (rand.Float64()-0.5)*0.3, (rand.Float64()-0.5)*0.2
		m.size = 1 + rand.Float64()
	case AmbCave: // embers rising
		m.vx, m.vy = (rand.Float64()-0.5)*0.15, -0.25-rand.Float64()*0.25
	default: // night motes barely drifting
		m.vx, m.vy = (rand.Float64()-0.5)*0.08, 0.04+rand.Float64()*0.06
	}
	return m
}

// Update drifts every particle, wrapping it around the viewport edges.
func (a *Ambient) Update() {
	a.t += 1.0 / 60
	for i := range a.motes {
		m := &a.motes[i]
		m.x += m.vx
		m.y += m.vy
		m.phase += 0.04 * m.blink
		if m.x < -2 {
			m.x = a.w + 2
		} else if m.x > a.w+2 {
			m.x = -2
		}
		if m.y < -2 {
			m.y = a.h + 2
		} else if m.y > a.h+2 {
			m.y = -2
		}
	}
}

func (a *Ambient) moteColor() (r, g, b float32) {
	switch a.kind {
	case AmbDay:
		return 1, 0.97, 0.8
	case AmbDusk:
		return 1, 0.92, 0.45
	case AmbCave:
		return 1, 0.55, 0.25
	default:
		return 0.8, 0.85, 1
	}
}

// Draw renders the particles then the lighting tint over the whole buffer.
func (a *Ambient) Draw(screen *ebiten.Image) {
	pix := neckPixelImage()
	cr, cg, cb := a.moteColor()
	for i := range a.motes {
		m := &a.motes[i]
		alpha := 0.55 + 0.45*math.Sin(m.phase) // twinkle / firefly pulse
		if alpha < 0.05 {
			continue
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(m.size, m.size)
		op.GeoM.Translate(m.x, m.y)
		op.ColorScale.Scale(cr, cg, cb, 1)
		op.ColorScale.ScaleAlpha(float32(alpha))
		screen.DrawImage(pix, op)
	}
	if a.tintA > 0 {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(a.w, a.h)
		// Premultiplied: RGB scaled by alpha so the overlay blends, not adds.
		op.ColorScale.Scale(a.tintR*a.tintA, a.tintG*a.tintA, a.tintB*a.tintA, a.tintA)
		screen.DrawImage(pix, op)
	}
}
