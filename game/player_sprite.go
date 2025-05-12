package game

import (
	"bytes"
	"image"
	"io/fs"

	_ "image/png"

	"github.com/hajimehoshi/ebiten/v2"
)

// DrawSprite draws the player's current animation frame based on movement and interaction state.
func (p *Player) Draw(screen *ebiten.Image, camX, camY float64) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(p.X-camX, p.Y-camY)

	// Interaction override
	if p.interacting {
		if p.facingRight {
			screen.DrawImage(p.interactR, op)
		} else {
			screen.DrawImage(p.interactL, op)
		}
		return
	}

	// Determine correct sprite frame
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
	default:
		if p.facingRight {
			img = p.idleR[p.idleIndex]
		} else {
			img = p.idleL[p.idleIndex]
		}
	}
	// Advance idle animation frame (optional)
	p.idleTimer--
	if p.idleTimer <= 0 {
		p.idleTimer = p.idleDelay
		p.idleIndex = (p.idleIndex + 1) % idleFrames
	}

	screen.DrawImage(img, op)
}

// loadSheet loads an animation sprite sheet (a single row of frames) from the given path,
// splits it into 'frames' equal-width images, and returns them as a slice.
//
// Used for multi-frame animations like walk and idle cycles.
func loadAnimationSheet(fsys fs.FS, path string, frames int) []*ebiten.Image {
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

// loadPoseSheet loads a single-frame sprite  (Used for standalone sprites like jump or interact poses.) from the given path in the provided filesystem.
func loadPoseSheet(fsys fs.FS, path string) *ebiten.Image {

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
