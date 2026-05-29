// npc.go — the talkable characters that stand in the world.
//
// An NPC is a small, mostly-static thing: a 16x16 sprite that stands at a
// fixed world position, bobs gently so it reads as "alive", and shows a little
// floating "!" plus a "press E" hint when the giraffe wanders close. All of the
// *conversation* lives elsewhere (story.go owns the script, dialogue.go owns the
// box); this file is just the body that stands there and the drawing of it.
package game

import (
	"bytes"
	"image"
	"image/color"
	_ "image/png" // register PNG decoder for image.Decode
	"io/fs"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
)

// who identifies which character a placement is, so the StoryManager can look
// up the right sprite and the right block of dialogue lines for it.
type who int

const (
	whoElder    who = iota // the wise long-necked giraffe
	whoCritter             // the earnest, chatty fox
	whoWanderer            // the melancholy hooded traveler
)

// speakerName is the label drawn at the top of the dialogue box for each
// character. Kept here next to the who-constants so the two never drift apart.
func (w who) speakerName() string {
	switch w {
	case whoElder:
		return "Elder Giraffe"
	case whoCritter:
		return "Fox"
	case whoWanderer:
		return "Hooded Traveler"
	default:
		return ""
	}
}

// npc is one placed character on one level: a sprite, where it stands, and a
// per-instance bob phase so several NPCs on screen don't bob in lockstep.
type npc struct {
	id    who           // which character this is (drives sprite + script)
	x, y  float64       // world position of the sprite's top-left, in pixels
	sprite *ebiten.Image // the 16x16 frame to draw
	phase float64       // bob phase offset so each NPC bobs independently
}

// npcBox is the 16x16 footprint of an NPC in world space, used for the
// "is the player near enough to talk?" proximity test.
func (n *npc) rect() image.Rectangle {
	return image.Rect(int(n.x), int(n.y), int(n.x+16), int(n.y+16))
}

// near reports whether the player's centre is within `reach` pixels of the
// NPC's centre. We compare centres (not edges) so the trigger feels fair from
// either side and doesn't depend on which way the giraffe is facing.
func (n *npc) near(px, py, reach float64) bool {
	ncx, ncy := n.x+8, n.y+8
	pcx, pcy := px+spriteW/2, py+spriteH/2
	dx, dy := ncx-pcx, ncy-pcy
	return dx*dx+dy*dy <= reach*reach
}

// loadNPCSprite reads a single-frame 16x16 PNG from the embedded FS. It mirrors
// loadPoseSheet in player_sprite.go but lives here so npc.go is self-contained
// and we never touch the existing file.
func loadNPCSprite(fsys fs.FS, path string) *ebiten.Image {
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

// drawWorld renders this NPC into the world buffer (the 640x360 surface that is
// later scaled 2x). It applies the camera offset, adds a gentle vertical bob,
// and floats a small "!" above the head so the player notices there's someone
// to talk to. `t` is a steadily increasing time value (seconds) shared by all
// NPCs so the bob is smooth and frame-rate independent.
func (n *npc) drawWorld(screen *ebiten.Image, camX, camY, t float64, highlight bool) {
	// Bob: a slow sine wave, ~1.5px peak-to-trough. The phase offset spreads
	// multiple NPCs out so they don't all rise and fall together.
	bob := math.Sin(t*2.2+n.phase) * 0.75

	sx := n.x - camX
	sy := n.y - camY + bob

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(sx, sy)
	screen.DrawImage(n.sprite, op)

	// Floating "!" marker above the head. It bobs with the body (so it rides
	// along) plus its own faster wobble so it draws the eye. We draw it as a
	// tiny hand-built glyph rather than text, because the world buffer is only
	// 640x360 and the game's font is far too large at this scale.
	mx := sx + 7              // roughly centred over the 16px-wide sprite
	my := sy - 6 + math.Sin(t*4)*0.6

	markCol := color.RGBA{0xFF, 0xE8, 0x6A, 0xff} // warm yellow
	if highlight {
		markCol = color.RGBA{0xFF, 0xFF, 0xFF, 0xff} // brighten when in range
	}
	// the stalk of the "!" (3px tall) and its dot below
	for dy := 0; dy < 3; dy++ {
		screen.Set(int(mx), int(my)+dy, markCol)
	}
	screen.Set(int(mx), int(my)+4, markCol)
}

// drawPrompt draws a subtle "press E" hint just under the NPC when the giraffe
// is standing close enough to talk. It is drawn in the SAME world buffer as the
// sprite (so it scales with the world), using the supplied small font face. We
// keep it dim so it reads as a hint, not a shout.
func (n *npc) drawPrompt(screen *ebiten.Image, camX, camY float64, face font.Face) {
	const hint = "press E"
	b := text.BoundString(face, hint)
	sx := n.x - camX + 8 - float64(b.Dx())/2 // centre under the sprite
	sy := n.y - camY + 16 + float64(b.Dy()) + 2
	// soft drop-shadow then the hint, so it stays legible over any tile
	text.Draw(screen, hint, face, int(sx)+1, int(sy)+1, color.RGBA{0, 0, 0, 200})
	text.Draw(screen, hint, face, int(sx), int(sy), color.RGBA{0xEA, 0xEA, 0xEA, 0xff})
}
