//Chooses and draws the correct player image each frame (idle, walk, jump or interact pose), 
//splits sprite sheets into frames, and advances the blink/idle cycle.
package game

import (
	"bytes"
	"image"
	"io/fs"

	_ "image/png"

	"github.com/hajimehoshi/ebiten/v2"
)

// Draw renders the player’s current visual representation each frame.
// It chooses between interaction, jump, walk, or idle sprites based on state,
// applies the camera offset, and advances the idle-cycle animation.
func (p *Player) Draw(screen *ebiten.Image, camX, camY float64) {
	// Sword slash effect, drawn in front regardless of blink/pose state.
	if p.Attacking() {
		p.drawSlash(screen, camX, camY)
	}

	// Blink while invulnerable after taking a hit: skip drawing on alternating
	// short intervals so the giraffe flickers.
	if p.invuln > 0 && (p.invuln/4)%2 == 1 {
		return
	}

	// Prepare drawing options and position at (X, Y) minus the camera’s offset
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(p.X-camX, p.Y-camY)

	// 1) Interaction has highest priority: show the use/“E” pose for one frame
	if p.interacting {
		if p.facingRight {
			screen.DrawImage(p.interactR, op)
		} else {
			screen.DrawImage(p.interactL, op)
		}
		return
	}

	// 2) Select the correct sprite based on movement state:
	var img *ebiten.Image
	switch {
	case !p.OnGround:
		// In air: use the single-frame jump image
		if p.facingRight {
			img = p.jumpR
		} else {
			img = p.jumpL
		}

	case p.VelX != 0:
		// Walking: pick the current frame from the walk animation slice
		if p.facingRight {
			img = p.walkR[p.frameIndex]
		} else {
			img = p.walkL[p.frameIndex]
		}

	default:
		// Standing still on the ground: use the idle animation slice
		if p.facingRight {
			img = p.idleR[p.idleIndex]
		} else {
			img = p.idleL[p.idleIndex]
		}
	}

	// 3) Optionally advance the idle animation timer so the player blinks or shifts stance
	p.idleTimer--
	if p.idleTimer <= 0 {
		p.idleTimer = p.idleDelay
		p.idleIndex = (p.idleIndex + 1) % idleFrames
	}

	// 4) Draw the player. When the neck is extended, the neck renderer draws the
	//    whole giraffe (body + neck column + lifted head) so we must NOT also draw
	//    the full sprite here, or its original head would show at the shoulders.
	//    When retracted, draw normally.
	if p.neck.Active() {
		// While the neck is extended, render the body from the IDLE frame rather
		// than the chosen pose. The jump pose tucks the eyes down into rows 4-5
		// where the raised arms also live, so no clean head split exists for it;
		// the idle frame keeps the whole head (eyes included) in the top 7 rows
		// with the arms down, which slices cleanly and is symmetric across facings.
		neckBody := p.idleR[0]
		if !p.facingRight {
			neckBody = p.idleL[0]
		}
		p.neck.Draw(screen, neckBody, 7, p.X-camX, p.Y-camY)
	} else {
		screen.DrawImage(img, op)
	}
}

// drawSlash renders the painted sword-slash crescent in front of the giraffe,
// picking the frame for how far through the swing we are.
func (p *Player) drawSlash(screen *ebiten.Image, camX, camY float64) {
	frames := p.slashR
	x := p.X + p.Width - 4
	if !p.facingRight {
		frames = p.slashL
		x = p.X - 12
	}
	if len(frames) == 0 {
		return
	}
	idx := p.AttackPhase() * len(frames) / attackDuration
	if idx >= len(frames) {
		idx = len(frames) - 1
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x-camX, p.Y-camY)
	screen.DrawImage(frames[idx], op)
}

// loadAnimationSheet reads a horizontal sprite sheet from the embedded FS,
// splits it into equal-width frames, and returns a slice of images.
// Useful for animations like walking or idling.
func loadAnimationSheet(fsys fs.FS, path string, frames int) []*ebiten.Image {
	// Read the raw PNG data from the embedded filesystem
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		panic(err)
	}

	// Decode into a Go image.Image, then wrap in an Ebiten Image
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		panic(err)
	}
	sheet := ebiten.NewImageFromImage(img)

	// Calculate each frame’s rectangle and extract it
	out := make([]*ebiten.Image, frames)
	for i := 0; i < frames; i++ {
		r := image.Rect(
			i*spriteW,     // left X of this frame
			0,             // top Y
			(i+1)*spriteW, // right X
			spriteH,       // bottom Y
		)
		out[i] = sheet.SubImage(r).(*ebiten.Image)
	}
	return out
}

// loadPoseSheet reads a single-frame sprite (e.g., jump or interact pose)
// from the embedded FS and returns it as an Ebiten Image.
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
