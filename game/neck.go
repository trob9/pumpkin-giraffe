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

	// neckBaseTop is the first sprite row (from the body's top) that the shoulder
	// mask starts covering. 0 means cover the whole original head box so no ear
	// tips or face pixels remain once the real head lifts off.
	neckBaseTop = 0.0
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
//	p      — the player, so we can read position/size and (during a hoist) move it.
//
// Returns true if the neck consumed control of the player this frame (i.e. a
// hoist is in progress), so the caller can skip conflicting movement if desired.
func (n *Neck) Update(qHeld bool, p *Player) {
	released := n.prevQ && !qHeld
	n.prevQ = qHeld

	// A hoist, once started, runs to completion regardless of Q.
	if n.hoisting {
		n.runHoist(p)
		return
	}

	if qHeld {
		// Grow the neck upward, clamped to max.
		n.length += neckGrowSpeed
		if n.length > neckMaxLen {
			n.length = neckMaxLen
		}
		// Re-evaluate the hook every frame as the head rises.
		n.evaluateHook(p)
		return
	}

	// Q not held this frame.
	if released && n.hooked && n.length > 0 {
		// Begin hoisting the body up to the hooked ledge.
		n.hoisting = true
		return
	}

	// No hook (or never extended): retract smoothly.
	n.hooked = false
	if n.length > 0 {
		n.length -= neckRetractSpeed
		if n.length < 0 {
			n.length = 0
		}
	}
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
	headBottom := headTop + neckHeadRows
	headLeft := p.X
	headRight := p.X + p.Width
	headCX := p.X + p.Width/2

	ts := float64(TileSize)

	// 1) Tile floors: scan the tile row at the head's top across the head's
	//    horizontal span. A solid tile whose TOP edge sits within the head box
	//    (and which has empty space directly above it) is a ledge we can hook.
	tyHead := int(headTop / ts)
	for _, ty := range []int{tyHead, tyHead + 1} {
		surfaceTop := float64(ty) * ts
		// The head must straddle this surface's top edge: head top at/above it,
		// head bottom at/below it.
		if surfaceTop < headTop-2 || surfaceTop > headBottom+2 {
			continue
		}
		for tx := int(headLeft / ts); tx <= int(headRight/ts); tx++ {
			if isWall(tx, ty) && !isWall(tx, ty-1) {
				n.hooked = true
				n.hookY = surfaceTop
				return
			}
		}
	}

	// 2) Moving platforms: head's center over the platform span and the head
	//    top straddling the platform's top edge.
	for _, mp := range Levels[CurrentLevel].Platforms {
		if headCX < mp.X || headCX > mp.X+mp.W {
			continue
		}
		if mp.Y >= headTop-2 && mp.Y <= headBottom+2 {
			n.hooked = true
			n.hookY = mp.Y
			return
		}
	}

	// 3) Boulders: same idea, hook over the top of a boulder.
	for _, b := range Levels[CurrentLevel].Boulders {
		if headCX < b.X || headCX > b.X+b.W {
			continue
		}
		if b.Y >= headTop-2 && b.Y <= headBottom+2 {
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

// Draw renders the extended neck and the lifted head on top of the body sprite.
// It is called from the player Draw path only when the neck is Active().
//
//	screen     — render target.
//	bodyImg    — the sprite frame currently chosen for the player (idle/walk/etc).
//	px, py     — the body's screen-space top-left (already camera-adjusted).
//
// The head is the top neckHeadRows of bodyImg, sub-imaged out and re-drawn at
// the top of the neck. The neck itself is a thin tan column with a 1px black
// outline, drawn between the body's neck-base and the lifted head.
func (n *Neck) Draw(screen *ebiten.Image, bodyImg *ebiten.Image, px, py float64) {
	if n.length <= 0.01 {
		return
	}

	// The neck column rises from just below the head region (the sprite's
	// natural neck pinch is around row neckHeadRows) up by n.length.
	// Centre the column horizontally on the sprite.
	colX := px + (16-neckWidth)/2
	colTopY := py - n.length             // top of the column = where head base goes
	colBottomY := py + float64(neckHeadRows) // overlap into the body a touch

	colH := colBottomY - colTopY
	if colH < 0 {
		colH = 0
	}

	pix := neckPixelImage()

	// First, mask the body's ORIGINAL head. The body sprite was already drawn in
	// full, so its own head/ears/face still sit at the top rows of the body. If we
	// left them, you'd see a second face peeking out at the shoulders once the
	// real head lifts away. We erase those top rows with a solid tan block (the
	// neck base / upper chest) plus a 1px black outline at the very top, so the
	// shoulders read as the bottom of the neck rather than a leftover head.
	maskTopY := py + neckBaseTop // skip the topmost rows that the sprite leaves transparent
	maskH := float64(neckHeadRows) - neckBaseTop
	if maskH > 0 {
		// black cap line across the shoulders
		opO := &ebiten.DrawImageOptions{}
		opO.GeoM.Scale(16, maskH)
		opO.GeoM.Translate(px, maskTopY)
		opO.ColorScale.ScaleWithColor(neckOutline)
		screen.DrawImage(pix, opO)
		// tan chest just inside the outline
		opT := &ebiten.DrawImageOptions{}
		opT.GeoM.Scale(14, maskH-1)
		opT.GeoM.Translate(px+1, maskTopY)
		opT.ColorScale.ScaleWithColor(neckTan)
		screen.DrawImage(pix, opT)
	}

	// Black outline column (1px wider each side).
	{
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(float64(neckWidth+2), colH)
		op.GeoM.Translate(colX-1, colTopY)
		op.ColorScale.ScaleWithColor(neckOutline)
		screen.DrawImage(pix, op)
	}
	// Tan fill column.
	{
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(float64(neckWidth), colH)
		op.GeoM.Translate(colX, colTopY)
		op.ColorScale.ScaleWithColor(neckTan)
		screen.DrawImage(pix, op)
	}

	// Draw the head (top neckHeadRows of the body sprite) at the top of the neck.
	headRect := image.Rect(0, 0, 16, neckHeadRows)
	head := bodyImg.SubImage(headRect).(*ebiten.Image)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(px, colTopY)
	screen.DrawImage(head, op)
}
