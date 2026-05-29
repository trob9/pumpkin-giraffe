// Implements the giraffe's "neck extend" ability.
//
// Holding Q grows the neck UPWARD over time, lifting only the head above the
// body (the body stays put). If the rising head clears the top edge of a solid
// surface (tile floor or moving platform) and is horizontally over/beside it,
// the head becomes "hooked". Releasing Q while hooked hoists the body up so the
// player stands on that surface; releasing without a hook smoothly retracts the
// neck.
//
// All state lives here in a Neck value so game/player.go only needs one field,
// one Update call, and one Draw call. When Q isn't pressed and the neck is fully
// retracted, this is completely inert: no behaviour change, nothing drawn.
package game

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	// neckMaxLen is how far (in pixels) the head can rise above its normal
	// position when the neck is fully extended.
	neckMaxLen = 44.0

	// neckGrowSpeed / neckRetractSpeed are pixels-per-frame for smooth lerp-like
	// growth and retraction so the neck never snaps.
	neckGrowSpeed    = 1.6
	neckRetractSpeed = 2.4

	// neckHeadRows is the number of pixel rows at the top of the 16x16 sprite
	// that count as the "head" (ears + face). Below this the sprite narrows into
	// the natural neck, so lifting these rows reads as the head rising.
	neckHeadRows = 7

	// neckWidth is the drawn width (px) of the thin neck column. The giraffe's
	// natural throat is ~2px wide, so we keep it slim to stay in proportion.
	neckWidth = 4

	// neckHoistSpeed is how fast (px/frame) the body rises during the hoist.
	neckHoistSpeed = 3.0
)

// neckTan and neckOutline are sampled from the giraffe sprite: the golden body
// colour and the black outline. Used to draw the procedural neck column.
var (
	neckTan     = color.RGBA{181, 153, 59, 255}
	neckOutline = color.RGBA{0, 0, 0, 255}
)

// neckColumn is a small reusable 1x1 white image we tint+stretch to draw the
// neck. Built lazily on first use.
var neckPixel *ebiten.Image

func neckPixelImage() *ebiten.Image {
	if neckPixel == nil {
		neckPixel = ebiten.NewImage(1, 1)
		neckPixel.Fill(color.White)
	}
	return neckPixel
}

// Neck holds all the per-frame state for the extend/hook/hoist ability.
type Neck struct {
	// length is the current animated neck length in pixels (0 = retracted).
	length float64

	// hooked is true once the extended head has cleared a surface edge.
	hooked bool
	// hookY is the world-space top of the surface the head hooked onto (the Y
	// the player's feet should end up on when hoisted).
	hookY float64

	// hoisting is true while the body is animating upward onto the hooked ledge.
	hoisting bool

	// prevQ remembers last frame's Q state to detect the release edge.
	prevQ bool

	// headImg / headBaseY cache the head sub-image and its draw offset, refreshed
	// each frame by the player Draw path so we render the correct facing/frame.
}

// Active reports whether the neck is doing anything at all this frame. When
// false, the Draw path skips all neck rendering so normal gameplay is untouched.
func (n *Neck) Active() bool {
	return n.length > 0.01 || n.hoisting
}

// Update advances the neck state machine for one frame.
//
//	qHeld  — whether the Q key is currently held.
//	p      — the player, so we can read position/size and move it while
//	         anchored (hanging) or hoisting.
//
// Returns true when the neck has taken control of the player this frame — while
// hanging by the head from a ledge, or while hoisting up onto it — so the caller
// skips the normal physics that would otherwise drag the player back down.
func (n *Neck) Update(qHeld bool, p *Player) bool {
	released := n.prevQ && !qHeld
	n.prevQ = qHeld

	// A hoist, once started, runs to completion regardless of Q.
	if n.hoisting {
		n.runHoist(p)
		return true
	}

	if qHeld {
		// Grow the neck upward until it catches on something; once hooked the
		// length is frozen so the giraffe hangs stably by the head.
		if !n.hooked {
			n.length += neckGrowSpeed
			if n.length > neckMaxLen {
				n.length = neckMaxLen
			}
			n.evaluateHook(p)
		}
		if n.hooked {
			// Anchor: the head's chin rests on the ledge lip and the body hangs
			// below by the neck's length. Kill all momentum so we don't slip off
			// or slide past — you hang here until you release Q to climb up.
			p.VelX = 0
			p.VelY = 0
			p.OnGround = false
			p.Y = n.hookY - neckHeadRows + n.length
			return true
		}
		return false // extending but not yet caught: normal physics (rise/fall)
	}

	// Q released.
	if released && n.hooked && n.length > 0 {
		n.hoisting = true // climb up onto the ledge
		return true
	}

	// No hook (or never extended): retract smoothly.
	n.hooked = false
	if n.length > 0 {
		n.length -= neckRetractSpeed
		if n.length < 0 {
			n.length = 0
		}
	}
	return false
}

// headWorldTop returns the current world-space Y of the top of the head given
// the neck length. The head normally sits at the top of the sprite (p.Y); the
// neck lifts it by n.length.
func (n *Neck) headWorldTop(p *Player) float64 {
	return p.Y - n.length
}

// evaluateHook checks whether the raised head has cleared the top edge of a
// solid surface and is horizontally over/just beside it. If so it records the
// surface top in hookY and sets hooked.
//
// The "head" is a small box at the top of the neck, the width of the sprite,
// neckHeadRows tall. We consider it hooked when its top is at or just above a
// surface's top edge AND it horizontally overlaps that surface — i.e. the head
// is resting over the lip of a ledge.
func (n *Neck) evaluateHook(p *Player) {
	headTop := n.headWorldTop(p)
	chin := headTop + neckHeadRows // bottom of the head — the part that catches
	headCX := p.X + p.Width/2
	ts := float64(TileSize)

	// The chin catches a lip when it has risen to roughly the ledge's top edge:
	// from a touch below (catching just as it approaches) to well above (once
	// it's cleared the lip it stays caught). Generous so you don't have to time
	// the jump perfectly — d = how far the chin sits below the ledge top.
	const catchBelow = 8.0  // chin still this far below the lip → catches early
	const catchAbove = 30.0 // chin up to this far above the lip → still catches
	catches := func(surfaceTop float64) bool {
		// Only ledges ABOVE the body can be hooked — never the floor the giraffe
		// is standing on (which sits below the body, at the feet). Without this
		// the chin would "catch" the ground underneath and refuse to rise.
		if surfaceTop > p.Y {
			return false
		}
		d := chin - surfaceTop
		return d <= catchBelow && d >= -catchAbove
	}

	// 1) Tile ledges: require the head's CENTRE to be over a standable tile (solid
	//    with empty space above), not merely beside its edge — otherwise the head
	//    magnetises onto a ledge the instant you pass near its side. You have to
	//    bring your head over the top surface for the chin to catch.
	tcx := int(headCX / ts)
	tyLo := int((chin-catchAbove)/ts) - 1
	tyHi := int((chin+catchBelow)/ts) + 1
	for ty := tyLo; ty <= tyHi; ty++ {
		if !isWall(tcx, ty) || isWall(tcx, ty-1) {
			continue // not a hookable ledge surface
		}
		if surfaceTop := float64(ty) * ts; catches(surfaceTop) {
			n.hooked = true
			n.hookY = surfaceTop
			return
		}
	}

	// 2) Moving platforms: head centred over the span, chin catching the top.
	for _, mp := range Levels[CurrentLevel].Platforms {
		if headCX >= mp.X && headCX <= mp.X+mp.W && catches(mp.Y) {
			n.hooked = true
			n.hookY = mp.Y
			return
		}
	}

	// 3) Boulders.
	for _, b := range Levels[CurrentLevel].Boulders {
		if headCX >= b.X && headCX <= b.X+b.W && catches(b.Y) {
			n.hooked = true
			n.hookY = b.Y
			return
		}
	}

	n.hooked = false
}

// runHoist animates the body rising until the player's feet rest on the hooked
// surface, then resets the neck to its normal retracted state.
func (n *Neck) runHoist(p *Player) {
	// Target Y so the player's feet (Y+Height) sit on the hooked surface top.
	targetY := n.hookY - p.Height

	// Move the body up; as the body rises, the neck visually shortens so the
	// head stays put at the ledge.
	p.Y -= neckHoistSpeed
	if n.length > 0 {
		n.length -= neckHoistSpeed
		if n.length < 0 {
			n.length = 0
		}
	}

	// Kill downward velocity so gravity doesn't fight the hoist.
	p.VelY = 0

	if p.Y <= targetY {
		// Land cleanly on the ledge.
		p.Y = targetY
		p.VelY = 0
		p.OnGround = true
		p.ridingPlatform = nil
		// Reset neck state fully.
		n.length = 0
		n.hooked = false
		n.hoisting = false
	}
}

// Draw renders the whole giraffe while the neck is extended: the lower body in
// place, a thin neck column rising from the shoulders, and the head lifted by
// n.length. It is called INSTEAD of the normal sprite draw (player_sprite.go),
// so there is no original head left at the shoulders to mask — the old
// full-width mask (which showed as two boxes either side of the neck) is gone.
//
//	bodyImg  — the current sprite frame (idle/walk/jump).
//	headRows — how many top rows of THIS frame are the head. Differs by pose: in
//	           idle/walk the head fills the top 7 rows; in the jump pose the arms
//	           are raised into rows 4+, so only rows 0-3 are head. A full-width
//	           row slice at the right height is clean for both facings (no left/
//	           right column asymmetry) and never lifts the arms.
//	px, py   — the body's screen-space top-left (already camera-adjusted). We
//	           slice with the frame's OWN bounds origin so walk frames (which live
//	           at x-offsets in their sheet) slice correctly instead of empty.
func (n *Neck) Draw(screen *ebiten.Image, bodyImg *ebiten.Image, headRows int, px, py float64) {
	if n.length <= 0.01 {
		return
	}
	b := bodyImg.Bounds()
	mnx, mny := b.Min.X, b.Min.Y
	h := float64(headRows)

	// 1) Lower body (everything below the head, incl. raised arms) in place.
	body := bodyImg.SubImage(image.Rect(mnx, mny+headRows, mnx+16, mny+16)).(*ebiten.Image)
	opBody := &ebiten.DrawImageOptions{}
	opBody.GeoM.Translate(px, py+h)
	screen.DrawImage(body, opBody)

	// 2) Thin neck column between the lifted head and the body's neck-base.
	headTopY := py - n.length
	neckTopY := headTopY + h - 1
	neckBotY := py + h + 1
	if neckH := neckBotY - neckTopY; neckH > 0 {
		pix := neckPixelImage()
		opO := &ebiten.DrawImageOptions{}
		opO.GeoM.Scale(6, neckH)
		opO.GeoM.Translate(px+5, neckTopY)
		opO.ColorScale.ScaleWithColor(neckOutline)
		screen.DrawImage(pix, opO)

		opT := &ebiten.DrawImageOptions{}
		opT.GeoM.Scale(4, neckH)
		opT.GeoM.Translate(px+6, neckTopY)
		opT.ColorScale.ScaleWithColor(neckTan)
		screen.DrawImage(pix, opT)
	}

	// 3) The head (top headRows of the current frame) lifted up by n.length.
	head := bodyImg.SubImage(image.Rect(mnx, mny, mnx+16, mny+headRows)).(*ebiten.Image)
	opHead := &ebiten.DrawImageOptions{}
	opHead.GeoM.Translate(px, headTopY)
	screen.DrawImage(head, opHead)
}
