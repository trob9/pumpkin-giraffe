// Command art procedurally paints the game's pixel-art assets and writes them
// to the assets/ tree. Run from the repo root: go run ./tools/art
//
// Everything here is generated in code so the look stays cohesive: a small,
// warm, slightly-desaturated palette shared across sprites, plus atmospheric
// backgrounds built from vertical gradients smoothed with ordered (Bayer)
// dithering and layered silhouettes for depth.
//
// This file owns ALL asset generation. It does not touch game code.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"os"
	"path/filepath"
)

// ---------------------------------------------------------------------------
// Shared palette (matches existing giraffe + skull art)
// ---------------------------------------------------------------------------

var (
	cClear = color.RGBA{0, 0, 0, 0}
	cBlack = color.RGBA{0, 0, 0, 255}

	// Giraffe tan family (from player_idle_right.png)
	tan      = color.RGBA{181, 153, 59, 255}
	tanLight = color.RGBA{214, 188, 96, 255}
	tanDark  = color.RGBA{70, 48, 26, 255}

	// Bone family (from skull_walk_right.png)
	boneLight = color.RGBA{198, 196, 193, 255}
	boneMid   = color.RGBA{150, 148, 145, 255}
	boneShade = color.RGBA{73, 68, 66, 255}

	// Outline that is dark but not pure black, for softer silhouettes
	ink = color.RGBA{26, 22, 28, 255}
)

// ---------------------------------------------------------------------------
// Canvas helper
// ---------------------------------------------------------------------------

type canvas struct {
	img *image.RGBA
	w, h int
}

func newCanvas(w, h int) *canvas {
	return &canvas{img: image.NewRGBA(image.Rect(0, 0, w, h)), w: w, h: h}
}

func (c *canvas) set(x, y int, col color.RGBA) {
	if x < 0 || y < 0 || x >= c.w || y >= c.h {
		return
	}
	c.img.SetRGBA(x, y, col)
}

// blend draws col over whatever is already at (x,y), respecting col's alpha.
func (c *canvas) blend(x, y int, col color.RGBA) {
	if x < 0 || y < 0 || x >= c.w || y >= c.h {
		return
	}
	if col.A == 255 {
		c.img.SetRGBA(x, y, col)
		return
	}
	if col.A == 0 {
		return
	}
	bg := c.img.RGBAAt(x, y)
	a := float64(col.A) / 255
	out := color.RGBA{
		R: uint8(float64(col.R)*a + float64(bg.R)*(1-a)),
		G: uint8(float64(col.G)*a + float64(bg.G)*(1-a)),
		B: uint8(float64(col.B)*a + float64(bg.B)*(1-a)),
		A: 255,
	}
	c.img.SetRGBA(x, y, out)
}

func (c *canvas) at(x, y int) color.RGBA { return c.img.RGBAAt(x, y) }

func (c *canvas) fill(col color.RGBA) {
	for y := 0; y < c.h; y++ {
		for x := 0; x < c.w; x++ {
			c.img.SetRGBA(x, y, col)
		}
	}
}

func (c *canvas) save(path string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Println("mkdir:", err)
		os.Exit(1)
	}
	f, err := os.Create(path)
	if err != nil {
		fmt.Println("create:", err)
		os.Exit(1)
	}
	defer f.Close()
	if err := png.Encode(f, c.img); err != nil {
		fmt.Println("encode:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %-44s %dx%d\n", path, c.w, c.h)
}

// sub returns a sub-canvas painter offset by (ox,oy) so frame painters can
// draw in local 0..15 coordinates onto a wider sheet.
func (c *canvas) frame(ox, oy int) func(x, y int, col color.RGBA) {
	return func(x, y int, col color.RGBA) { c.set(ox+x, oy+y, col) }
}

// mirrorInto copies src horizontally-flipped into dst (same size).
func mirrorInto(dst, src *canvas) {
	for y := 0; y < src.h; y++ {
		for x := 0; x < src.w; x++ {
			dst.img.SetRGBA(src.w-1-x, y, src.at(x, y))
		}
	}
}

// ---------------------------------------------------------------------------
// Colour math
// ---------------------------------------------------------------------------

func lerp(a, b color.RGBA, t float64) color.RGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return color.RGBA{
		R: uint8(float64(a.R) + (float64(b.R)-float64(a.R))*t),
		G: uint8(float64(a.G) + (float64(b.G)-float64(a.G))*t),
		B: uint8(float64(a.B) + (float64(b.B)-float64(a.B))*t),
		A: 255,
	}
}

// 4x4 Bayer matrix for ordered dithering, normalised to (-0.5,0.5).
var bayer4 = [4][4]float64{
	{0, 8, 2, 10},
	{12, 4, 14, 6},
	{3, 11, 1, 9},
	{15, 7, 13, 5},
}

func bayerThreshold(x, y int) float64 {
	return (bayer4[y&3][x&3]+0.5)/16.0 - 0.5
}

// ditherGrad fills the whole canvas with a vertical gradient from top to
// bottom colour, using ordered dithering so the transition has no hard bands.
// stops are (position 0..1, colour) sorted by position.
type stop struct {
	p float64
	c color.RGBA
}

func gradientAt(stops []stop, t float64) color.RGBA {
	if t <= stops[0].p {
		return stops[0].c
	}
	last := stops[len(stops)-1]
	if t >= last.p {
		return last.c
	}
	for i := 0; i < len(stops)-1; i++ {
		a, b := stops[i], stops[i+1]
		if t >= a.p && t <= b.p {
			local := (t - a.p) / (b.p - a.p)
			return lerp(a.c, b.c, local)
		}
	}
	return last.c
}

// fillGradientDithered paints a vertical gradient with sub-pixel dithering.
// The dither nudges the sampling position by a fraction of one gradient step
// so adjacent rows interleave instead of stepping, killing visible banding.
func fillGradientDithered(c *canvas, stops []stop) {
	step := 1.0 / float64(c.h)
	for y := 0; y < c.h; y++ {
		base := float64(y) / float64(c.h)
		for x := 0; x < c.w; x++ {
			t := base + bayerThreshold(x, y)*step*1.6
			c.img.SetRGBA(x, y, gradientAt(stops, t))
		}
	}
}

// ---------------------------------------------------------------------------
// 1. SWORD SLASH effect  (48x16, 3 frames)
// ---------------------------------------------------------------------------

var (
	blade     = color.RGBA{220, 225, 235, 255}
	bladeHi   = color.RGBA{235, 248, 255, 255}
	cyan      = color.RGBA{180, 235, 245, 255}
	bladeEdge = color.RGBA{120, 140, 165, 255}
)

// drawArc paints a clean crescent swoosh into a frame. The blade sweeps around
// pivot (cx,cy) from angle a0 to a1. We render it as a signed-distance field:
// for every pixel near the arc we compute how far it is from the ideal arc
// centre-line and from the angular sweep, then colour by radial position
// (bright outer edge, cyan-white core) and fade the alpha toward the tail.
// This gives a solid, readable band with no random stipple gaps.
func drawArc(c *canvas, ox, oy int,
	cx, cy, radius, a0, a1, thickness, alpha float64, tailFade bool) {

	maxR := radius + thickness + 2
	for py := -2; py < 18; py++ {
		for px := -2; px < 18; px++ {
			dx := float64(px) - cx
			dy := float64(py) - cy
			dist := math.Hypot(dx, dy)
			if dist > maxR || dist < radius-thickness-2 {
				continue
			}
			ang := math.Atan2(dy, dx)
			// fraction along the sweep (0 at a0 tail .. 1 at a1 tip)
			f := (ang - a0) / (a1 - a0)
			if f < -0.04 || f > 1.04 {
				continue
			}
			if f < 0 {
				f = 0
			}
			if f > 1 {
				f = 1
			}
			// radial offset from the band's centre-line, normalised -1..1
			rad := (dist - radius) / thickness
			if rad < -1.15 || rad > 1.15 {
				continue
			}
			// coverage: 1 in the core, feathering to 0 at the rim (anti-alias)
			cover := 1.0 - math.Max(0, (math.Abs(rad)-0.55)/0.6)
			if cover <= 0 {
				continue
			}
			if cover > 1 {
				cover = 1
			}
			// taper the tail to a wisp: shrink thickness tolerance at low f
			if tailFade && math.Abs(rad) > 0.2+1.0*f {
				continue
			}
			// colour by radial position: faint outer edge -> bright cyan core.
			// Only use the dark edge tint where the band is solid (high cover),
			// otherwise it leaves stray dark speckles on the feathered rim.
			var col color.RGBA
			switch {
			case rad > 0.6:
				if cover > 0.7 {
					col = bladeEdge
				} else {
					col = blade
				}
			case rad > 0.15:
				col = blade
			case rad > -0.4:
				col = bladeHi
			default:
				col = cyan
			}
			// brightness ramps from tail (dim) to tip (bright)
			a := alpha * cover * (0.30 + 0.70*f)
			if a < 0.16 {
				continue // drop near-invisible specks for a clean silhouette
			}
			col.A = uint8(255 * math.Min(1, a))
			c.blend(ox+px, oy+py, col)
		}
	}
}

func paintSlash(side string) {
	sheet := newCanvas(48, 16)
	// pivot just off the left edge of each frame, roughly hand height
	cx, cy := 0.0, 8.0

	// Frame 0: wind-up — short bright arc up high
	drawArc(sheet, 0, 0, cx, cy, 12.5, -1.30, -0.30, 1.7, 0.9, true)
	// Frame 1: full committed slash — long bright arc sweeping top to bottom
	drawArc(sheet, 16, 0, cx, cy, 12.5, -1.35, 1.15, 2.2, 1.0, false)
	// inner motion streak reinforcing the sweep
	drawArc(sheet, 16, 0, cx, cy, 8.5, -1.1, 1.0, 1.2, 0.5, true)
	// Frame 2: trailing wisp — low, faint, tapering
	drawArc(sheet, 32, 0, cx, cy, 13.5, 0.20, 1.25, 1.4, 0.6, true)

	if side == "right" {
		sheet.save("assets/sprites/slash_right.png")
		return
	}
	// left = mirror each 16px frame in place (so frame order is preserved)
	out := newCanvas(48, 16)
	for fr := 0; fr < 3; fr++ {
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				out.img.SetRGBA(fr*16+(15-x), y, sheet.at(fr*16+x, y))
			}
		}
	}
	out.save("assets/sprites/slash_left.png")
}

// ---------------------------------------------------------------------------
// 2. SECOND ENEMY: charred, helmeted, red-eyed skull (64x16, 4 frames)
// ---------------------------------------------------------------------------

var (
	charLight = color.RGBA{120, 112, 110, 255} // ashen bone
	charMid   = color.RGBA{78, 70, 70, 255}
	charDark  = color.RGBA{44, 38, 40, 255}
	helmLight = color.RGBA{150, 70, 50, 255} // rusted iron-red helm
	helmMid   = color.RGBA{104, 46, 38, 255}
	helmDark  = color.RGBA{62, 28, 26, 255}
	eyeGlow   = color.RGBA{255, 80, 50, 255}
	eyeCore   = color.RGBA{255, 200, 120, 255}
)

// drawSkull2Frame paints one 16x16 frame of the armored skull. bob shifts the
// whole head vertically and phase animates the jaw + eye flicker so the walk
// cycle reads as a menacing bob.
func drawSkull2Frame(set func(x, y int, col color.RGBA), bob int, jaw int, flicker bool) {
	oy := bob
	// helmet dome (rows 1-4) — an iron cap over the cranium
	helm := func(x, y int, col color.RGBA) { set(x, y+oy, col) }
	bone := helm

	// cranium silhouette (rounded) rows 5..8
	// We'll lay the skull first then the helm on top.
	craniumRows := map[int][2]int{ // y -> [xstart,xend] inclusive
		4: {4, 11},
		5: {3, 12},
		6: {3, 12},
		7: {3, 12},
		8: {4, 11},
	}
	for y, span := range craniumRows {
		for x := span[0]; x <= span[1]; x++ {
			col := charMid
			if x <= span[0]+1 {
				col = charLight // left rim catch-light
			} else if x >= span[1]-1 {
				col = charDark
			}
			bone(x, y, col)
		}
	}
	// outline the cranium
	for y, span := range craniumRows {
		bone(span[0]-1, y, ink)
		bone(span[1]+1, y, ink)
	}

	// eye sockets — deep, with a glowing red ember
	for _, ex := range []int{5, 9} {
		for yy := 5; yy <= 6; yy++ {
			for xx := ex; xx <= ex+1; xx++ {
				bone(xx, yy, charDark)
			}
		}
		// ember
		bone(ex, 6, eyeGlow)
		bone(ex+1, 6, eyeGlow)
		if flicker {
			bone(ex+1, 5, eyeCore)
		} else {
			bone(ex, 5, eyeGlow)
		}
	}

	// nasal notch
	bone(7, 7, charDark)
	bone(8, 7, charDark)

	// brow ridge shadow above eyes for menace
	for x := 4; x <= 11; x++ {
		bone(x, 4, charDark)
	}

	// helmet over the top of the skull (rows 0..3) — rusted iron with a nasal guard
	helmRows := map[int][2]int{
		0: {5, 10},
		1: {3, 12},
		2: {3, 12},
		3: {3, 12},
	}
	for y, span := range helmRows {
		for x := span[0]; x <= span[1]; x++ {
			col := helmMid
			if y == 0 || x <= span[0]+1 {
				col = helmLight
			} else if x >= span[1]-1 {
				col = helmDark
			}
			helm(x, y, col)
		}
		helm(span[0]-1, y, ink)
		helm(span[1]+1, y, ink)
	}
	// helmet crest ridge (a darker centre line)
	for y := 0; y <= 3; y++ {
		helm(7, y, helmDark)
		helm(8, y, helmLight)
	}
	// nasal guard dropping between the eyes
	helm(7, 4, helmMid)
	helm(7, 5, helmDark)

	// jaw rows 9..12, animated open/closed by jaw offset
	jy := 9 + 0
	jawRows := map[int][2]int{
		jy:     {5, 10},
		jy + 1: {5, 10},
		jy + 2 + jaw: {6, 9},
	}
	for y, span := range jawRows {
		for x := span[0]; x <= span[1]; x++ {
			col := charMid
			if x <= span[0] {
				col = charLight
			} else if x >= span[1] {
				col = charDark
			}
			bone(x, y, col)
		}
		bone(span[0]-1, y, ink)
		bone(span[1]+1, y, ink)
	}
	// teeth (dark gaps)
	for x := 6; x <= 9; x += 1 {
		if x%2 == 0 {
			bone(x, jy, charDark)
			bone(x, jy+1, charDark)
		}
	}

	// crack across the cheek for the "charred/battle-damaged" look
	bone(11, 7, charDark)
	bone(12, 8, charDark)
}

func paintSkull2(side string) {
	sheet := newCanvas(64, 16)
	// 4-frame walk bob: down, mid, up, mid (jaw chatters)
	bobs := []int{1, 0, 0, 0}
	jaws := []int{0, 1, 0, 1}
	flick := []bool{true, false, true, false}
	for fr := 0; fr < 4; fr++ {
		set := sheet.frame(fr*16, 0)
		drawSkull2Frame(set, bobs[fr], jaws[fr], flick[fr])
	}
	if side == "right" {
		sheet.save("assets/sprites/skull2_walk_right.png")
		return
	}
	out := newCanvas(64, 16)
	for fr := 0; fr < 4; fr++ {
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				out.img.SetRGBA(fr*16+(15-x), y, sheet.at(fr*16+x, y))
			}
		}
	}
	out.save("assets/sprites/skull2_walk_left.png")
}

// ---------------------------------------------------------------------------
// 3. NPCs (16x16 each)
// ---------------------------------------------------------------------------

func outlineSilhouette(c *canvas) {
	// add ink outline around any non-transparent pixel that touches empty space
	type p struct{ x, y int }
	var edges []p
	for y := 0; y < c.h; y++ {
		for x := 0; x < c.w; x++ {
			if c.at(x, y).A != 0 {
				continue
			}
			// empty pixel adjacent to filled -> outline
			adj := false
			for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				nx, ny := x+d[0], y+d[1]
				if nx >= 0 && ny >= 0 && nx < c.w && ny < c.h && c.at(nx, ny).A != 0 {
					if c.at(nx, ny) != ink {
						adj = true
					}
				}
			}
			if adj {
				edges = append(edges, p{x, y})
			}
		}
	}
	for _, e := range edges {
		c.set(e.x, e.y, ink)
	}
}

// npcElder: a wise old giraffe in a robe, leaning on a glowing staff. Head
// sits left-of-centre with a long neck; the staff is held in a hand on the
// right edge and runs unbroken to the ground so nothing floats.
func paintElder() {
	c := newCanvas(16, 16)
	scarf := color.RGBA{150, 60, 70, 255}
	scarfHi := color.RGBA{186, 92, 100, 255}
	staff := color.RGBA{120, 86, 50, 255}
	staffHi := color.RGBA{152, 116, 70, 255}
	glow := color.RGBA{255, 224, 150, 255}
	robe := color.RGBA{120, 96, 58, 255}
	robeHi := color.RGBA{152, 124, 80, 255}
	robeDk := color.RGBA{84, 64, 38, 255}

	// --- staff (right side), unbroken from a glowing tip down to the floor ---
	for y := 4; y <= 15; y++ {
		c.set(12, y, staff)
		c.set(13, y, staffHi)
	}
	// glowing orb on top of the staff
	c.set(12, 3, glow)
	c.set(13, 3, glow)
	c.set(12, 2, color.RGBA{255, 244, 200, 255})
	c.set(13, 2, glow)
	c.set(11, 3, color.RGBA{255, 240, 180, 200})
	c.set(14, 3, color.RGBA{255, 240, 180, 200})

	// --- head (upper-left) ---
	head := [][3]int{ // y, xs, xe
		{1, 3, 7}, {2, 2, 8}, {3, 2, 8}, {4, 3, 8},
	}
	for _, h := range head {
		for x := h[1]; x <= h[2]; x++ {
			col := tan
			if x <= h[1]+1 {
				col = tanLight
			} else if x >= h[2]-1 {
				col = tanDark
			}
			c.set(x, h[0], col)
		}
	}
	// ossicones (giraffe horns) with little tufts
	c.set(3, 0, tanDark)
	c.set(7, 0, tanDark)
	// snout extends left a touch
	c.set(1, 3, tan)
	// kindly eye + pale aged brow
	c.set(5, 2, cBlack)
	c.set(5, 1, color.RGBA{230, 230, 230, 255})
	c.set(6, 1, color.RGBA{230, 230, 230, 255})

	// --- neck ---
	for y := 5; y <= 7; y++ {
		for x := 4; x <= 6; x++ {
			col := tan
			if x == 4 {
				col = tanLight
			} else if x == 6 {
				col = tanDark
			}
			c.set(x, y, col)
		}
	}
	c.set(5, 6, tanDark) // giraffe spot

	// --- scarf at base of neck ---
	for x := 3; x <= 8; x++ {
		c.set(x, 8, scarf)
	}
	for x := 4; x <= 7; x++ {
		c.set(x, 9, scarfHi)
	}
	c.set(3, 9, scarf)
	// scarf tail flicking left
	c.set(2, 9, scarf)
	c.set(2, 10, scarfHi)

	// --- robe body, widening to a stable base ---
	body := [][3]int{
		{10, 3, 8}, {11, 2, 9}, {12, 2, 9}, {13, 2, 10}, {14, 2, 10}, {15, 2, 10},
	}
	for _, b := range body {
		for x := b[1]; x <= b[2]; x++ {
			col := robe
			if x <= b[1]+1 {
				col = robeHi
			} else if x >= b[2]-1 {
				col = robeDk
			}
			c.set(x, b[0], col)
		}
	}
	// robe fold shadows for form
	c.set(5, 12, robeDk)
	c.set(5, 13, robeDk)
	c.set(7, 14, robeDk)

	// --- hand gripping the staff (bridges body to staff, no gap) ---
	c.set(11, 9, tan)
	c.set(11, 10, tanDark)
	c.set(10, 9, tanLight)

	outlineSilhouette(c)
	c.save("assets/sprites/npc_elder.png")
}

// npcCritter: a small friendly fox, warm orange with cream chest + big eyes.
func paintCritter() {
	c := newCanvas(16, 16)
	fox := color.RGBA{206, 122, 58, 255}
	foxHi := color.RGBA{232, 156, 88, 255}
	foxDark := color.RGBA{150, 80, 38, 255}
	cream := color.RGBA{238, 224, 196, 255}

	// ears
	for _, ex := range []int{4, 10} {
		c.set(ex, 2, fox)
		c.set(ex, 3, fox)
		c.set(ex+1, 3, foxDark)
	}
	c.set(4, 1, foxDark)
	c.set(10, 1, foxDark)

	// head
	head := [][3]int{
		{4, 4, 11}, {5, 3, 12}, {6, 3, 12}, {7, 4, 11}, {8, 5, 10},
	}
	for _, h := range head {
		for x := h[1]; x <= h[2]; x++ {
			col := fox
			if x <= h[1]+1 {
				col = foxHi
			} else if x >= h[2]-1 {
				col = foxDark
			}
			c.set(x, h[0], col)
		}
	}
	// cream cheeks/snout
	for x := 6; x <= 9; x++ {
		c.set(x, 7, cream)
	}
	c.set(7, 8, cream)
	c.set(8, 8, cream)
	// big friendly symmetric eyes (dark pupil + white shine, mirrored)
	eyeDk := color.RGBA{34, 26, 28, 255}
	c.set(5, 5, eyeDk)
	c.set(10, 5, eyeDk)
	c.set(5, 5, color.RGBA{255, 255, 255, 255}) // shine top-left of each eye
	c.set(10, 5, color.RGBA{255, 255, 255, 255})
	c.set(6, 5, eyeDk)
	c.set(9, 5, eyeDk)
	// nose
	c.set(7, 7, cBlack)
	c.set(8, 7, cBlack)

	// little body
	body := [][3]int{
		{9, 5, 10}, {10, 4, 11}, {11, 4, 11}, {12, 4, 11},
	}
	for _, b := range body {
		for x := b[1]; x <= b[2]; x++ {
			col := fox
			if x <= b[1] {
				col = foxHi
			} else if x >= b[2] {
				col = foxDark
			}
			c.set(x, b[0], col)
		}
	}
	// cream belly
	c.set(7, 11, cream)
	c.set(8, 11, cream)
	// paws
	c.set(5, 13, foxDark)
	c.set(6, 13, foxDark)
	c.set(9, 13, foxDark)
	c.set(10, 13, foxDark)
	// fluffy tail with cream tip
	c.set(12, 10, fox)
	c.set(13, 11, fox)
	c.set(13, 12, foxHi)
	c.set(12, 13, cream)
	c.set(13, 13, cream)

	outlineSilhouette(c)
	c.save("assets/sprites/npc_critter.png")
}

// npcWanderer: a hooded cloaked traveler, deep teal cloak, shadowed face,
// a glint of eyes under the hood.
func paintWanderer() {
	c := newCanvas(16, 16)
	cloak := color.RGBA{56, 84, 92, 255}
	cloakHi := color.RGBA{82, 116, 124, 255}
	cloakDark := color.RGBA{34, 52, 58, 255}
	face := color.RGBA{40, 34, 40, 255} // shadow under hood
	eye := color.RGBA{180, 235, 245, 255}

	// hood peak
	hood := [][3]int{
		{1, 6, 9}, {2, 5, 10}, {3, 4, 11}, {4, 4, 11},
	}
	for _, h := range hood {
		for x := h[1]; x <= h[2]; x++ {
			col := cloak
			if x <= h[1]+1 {
				col = cloakHi
			} else if x >= h[2]-1 {
				col = cloakDark
			}
			c.set(x, h[0], col)
		}
	}
	// shadowed face opening
	for y := 4; y <= 7; y++ {
		for x := 6; x <= 9; x++ {
			c.set(x, y, face)
		}
	}
	// glinting eyes
	c.set(6, 5, eye)
	c.set(9, 5, eye)
	// hood inner rim
	c.set(5, 4, cloakDark)
	c.set(10, 4, cloakDark)

	// cloak body widening to the ground
	body := [][3]int{
		{8, 4, 11}, {9, 3, 12}, {10, 3, 12}, {11, 3, 12},
		{12, 2, 13}, {13, 2, 13}, {14, 2, 13}, {15, 2, 13},
	}
	for _, b := range body {
		for x := b[1]; x <= b[2]; x++ {
			col := cloak
			if x <= b[1]+1 {
				col = cloakHi
			} else if x >= b[2]-1 {
				col = cloakDark
			}
			c.set(x, b[0], col)
		}
	}
	// center seam fold
	for y := 9; y <= 15; y++ {
		c.set(8, y, cloakDark)
	}
	// a clasp at the throat
	c.set(7, 8, color.RGBA{198, 170, 90, 255})
	c.set(8, 8, color.RGBA{160, 132, 64, 255})
	// hint of a walking staff / hand
	c.set(12, 11, color.RGBA{96, 70, 40, 255})
	c.set(12, 12, color.RGBA{96, 70, 40, 255})
	c.set(12, 13, color.RGBA{96, 70, 40, 255})

	outlineSilhouette(c)
	c.save("assets/sprites/npc_wanderer.png")
}

// ---------------------------------------------------------------------------
// 4. BACKGROUNDS (1280x720)
// ---------------------------------------------------------------------------

const bgW, bgH = 1280, 720

// silhouetteBand draws rolling hills / treeline as a filled silhouette using a
// summed-sine ground line. baseY is the average horizon for this band, amp the
// hill height, freqs the layered wave frequencies.
func silhouetteBand(c *canvas, col color.RGBA, baseY, amp float64, seed int64, jagged bool) {
	rng := rand.New(rand.NewSource(seed))
	phase1 := rng.Float64() * math.Pi * 2
	phase2 := rng.Float64() * math.Pi * 2
	phase3 := rng.Float64() * math.Pi * 2
	for x := 0; x < c.w; x++ {
		fx := float64(x)
		y := baseY
		y += math.Sin(fx/220+phase1) * amp
		y += math.Sin(fx/70+phase2) * amp * 0.35
		y += math.Sin(fx/28+phase3) * amp * 0.15
		if jagged {
			// add sharp tree-like spikes
			y -= math.Abs(math.Sin(fx/9+phase3)) * amp * 0.5
		}
		top := int(y)
		for yy := top; yy < c.h; yy++ {
			// subtle vertical shade within the band for form
			shade := lerp(col, lerp(col, cBlack, 0.25), float64(yy-top)/float64(c.h-top))
			c.blend(x, yy, shade)
		}
	}
}

func addStars(c *canvas, n int, seed int64, maxBright uint8, topFrac float64) {
	rng := rand.New(rand.NewSource(seed))
	for i := 0; i < n; i++ {
		x := rng.Intn(c.w)
		y := rng.Intn(int(float64(c.h) * topFrac))
		b := uint8(80) + uint8(rng.Intn(int(maxBright)-80))
		star := color.RGBA{b, b, uint8(int(b) + 8), 255}
		c.blend(x, y, star)
		// occasional twinkle (plus shape)
		if rng.Float64() < 0.18 {
			dim := color.RGBA{b / 2, b / 2, b/2 + 4, 255}
			c.blend(x-1, y, dim)
			c.blend(x+1, y, dim)
			c.blend(x, y-1, dim)
			c.blend(x, y+1, dim)
		}
	}
}

// softDisc draws a radial glow (sun / moon / torch / crystal) with smooth falloff.
func softDisc(c *canvas, cx, cy, r float64, core color.RGBA, glowReach float64) {
	maxR := r * glowReach
	for y := int(cy - maxR); y <= int(cy+maxR); y++ {
		for x := int(cx - maxR); x <= int(cx+maxR); x++ {
			d := math.Hypot(float64(x)-cx, float64(y)-cy)
			if d > maxR {
				continue
			}
			var a float64
			if d <= r {
				a = 1.0
			} else {
				a = 1.0 - (d-r)/(maxR-r)
				a = a * a // soften
			}
			col := core
			col.A = uint8(float64(core.A) * a)
			c.blend(x, y, col)
		}
	}
}

func paintMeadowDay() {
	c := newCanvas(bgW, bgH)
	fillGradientDithered(c, []stop{
		{0.0, color.RGBA{96, 156, 206, 255}},  // upper sky blue
		{0.45, color.RGBA{152, 196, 226, 255}}, // mid
		{0.7, color.RGBA{206, 226, 232, 255}},  // pale near horizon
		{1.0, color.RGBA{224, 232, 224, 255}},  // hazy meadow light
	})
	// soft sun glow upper-right
	softDisc(c, 1040, 150, 46, color.RGBA{255, 250, 226, 235}, 3.2)
	softDisc(c, 1040, 150, 28, color.RGBA{255, 255, 245, 255}, 1.0)

	// stylised clouds (soft horizontal lozenges, dithered edges)
	drawCloud(c, 300, 130, 120, 30, 0.9)
	drawCloud(c, 700, 90, 90, 22, 0.7)
	drawCloud(c, 980, 230, 70, 18, 0.6)
	drawCloud(c, 150, 250, 60, 16, 0.55)

	// parallax hill bands, far (desaturated, hazy) to near (richer green)
	silhouetteBand(c, color.RGBA{150, 178, 168, 255}, 430, 36, 11, false) // farthest, blue-green haze
	silhouetteBand(c, color.RGBA{118, 158, 110, 255}, 500, 44, 22, false)
	silhouetteBand(c, color.RGBA{86, 132, 74, 255}, 560, 52, 33, false)
	silhouetteBand(c, color.RGBA{58, 104, 56, 255}, 630, 40, 44, false) // nearest meadow
	c.save("assets/backgrounds/bg_meadow_day.png")
}

// drawCloud paints a soft white cloud built from several overlapping lobes so
// the silhouette is lumpy and natural rather than a single ellipse. Each pixel
// gets a smooth alpha from a summed radial falloff; a faint Bayer dither only
// nudges the very faint outer fringe so the rim is soft without looking dirty.
func drawCloud(c *canvas, cx, cy, w, h int, alpha float64) {
	// lobes: relative centre (lx,ly) and radius scale, in fractions of w/h
	lobes := []struct{ lx, ly, r float64 }{
		{-0.55, 0.15, 0.62}, {-0.15, -0.25, 0.85}, {0.25, -0.05, 0.78},
		{0.6, 0.2, 0.55}, {0.0, 0.35, 0.7},
	}
	for y := -h - 4; y <= h+4; y++ {
		for x := -w - 4; x <= w+4; x++ {
			// accumulate the strongest lobe contribution (soft-union)
			var best float64
			for _, lo := range lobes {
				dx := (float64(x) - lo.lx*float64(w)) / (float64(w) * lo.r)
				dy := (float64(y) - lo.ly*float64(h)) / (float64(h) * lo.r)
				d := dx*dx + dy*dy
				if d >= 1 {
					continue
				}
				v := 1 - d
				if v > best {
					best = v
				}
			}
			if best <= 0 {
				continue
			}
			a := best * alpha
			// soften the curve so the core is solid and the edge feathers
			a = a * a * (3 - 2*a)
			// only the faintest fringe gets dithered away, keeping it clean
			if a < 0.18 {
				if bayerThreshold(cx+x, cy+y)+0.5 > a/0.18 {
					continue
				}
			}
			// gentle top-lit shading: brighter up top, soft grey-blue underside
			top := float64(y+h) / float64(2*h) // 0 top .. 1 bottom
			col := lerp(color.RGBA{255, 255, 255, 255}, color.RGBA{206, 216, 230, 255}, top*0.8)
			col.A = uint8(255 * math.Min(1, a))
			c.blend(cx+x, cy+y, col)
		}
	}
}

func paintDusk() {
	c := newCanvas(bgW, bgH)
	fillGradientDithered(c, []stop{
		{0.0, color.RGBA{38, 30, 64, 255}},   // deep purple top
		{0.3, color.RGBA{86, 56, 96, 255}},   // purple
		{0.55, color.RGBA{176, 86, 96, 255}}, // rose
		{0.72, color.RGBA{230, 132, 84, 255}}, // orange band
		{0.85, color.RGBA{246, 178, 102, 255}}, // warm horizon
		{1.0, color.RGBA{120, 74, 78, 255}},  // dim foreground
	})
	// early stars only in the upper purple region
	addStars(c, 90, 7, 200, 0.4)
	// low sun near horizon
	softDisc(c, 760, 470, 60, color.RGBA{255, 226, 158, 200}, 3.0)
	softDisc(c, 760, 470, 34, color.RGBA{255, 244, 206, 255}, 1.0)
	// reflected light haze band on horizon
	for x := 0; x < c.w; x++ {
		for y := 455; y < 495; y++ {
			d := math.Abs(float64(x) - 760)
			a := math.Max(0, 1-d/640) * 0.18
			c.blend(x, y, color.RGBA{255, 210, 150, uint8(255 * a)})
		}
	}
	// silhouette treelines, darkening toward the foreground
	silhouetteBand(c, color.RGBA{86, 56, 74, 255}, 500, 30, 101, false)  // far ridge
	silhouetteBand(c, color.RGBA{44, 30, 46, 255}, 560, 40, 202, true)   // mid treeline
	silhouetteBand(c, color.RGBA{20, 14, 24, 255}, 650, 46, 303, true)   // near trees, near-black
	c.save("assets/backgrounds/bg_dusk.png")
}

func paintCave() {
	c := newCanvas(bgW, bgH)
	fillGradientDithered(c, []stop{
		{0.0, color.RGBA{6, 6, 12, 255}},      // near-black top
		{0.4, color.RGBA{14, 16, 34, 255}},    // deep blue
		{0.7, color.RGBA{26, 22, 52, 255}},    // violet
		{1.0, color.RGBA{34, 26, 58, 255}},    // slightly lit floor area
	})
	// distant warm torch glow, lower-right — irregular firelight, not a sun:
	// a broad soft halo plus a small flickery core offset upward.
	torchGlow(c, 1090, 500, color.RGBA{236, 130, 52, 255})
	// a second cooler distant glow left (mineral/crystal light)
	softDisc(c, 240, 520, 70, color.RGBA{70, 110, 190, 44}, 4.2)
	softDisc(c, 240, 520, 28, color.RGBA{96, 140, 210, 70}, 2.2)

	// rocky silhouettes: ceiling stalactites (top) and floor (bottom)
	rockTop(c, color.RGBA{8, 8, 18, 255}, 80, 60, 9)
	// floor silhouette
	silhouetteBand(c, color.RGBA{10, 10, 22, 255}, 600, 36, 55, true)

	// glowing crystals scattered on the floor silhouette
	crystals := [][3]int{
		{360, 612, 0}, {410, 626, 1}, {820, 600, 2}, {870, 618, 0}, {1180, 590, 1}, {120, 640, 2},
	}
	crystalCols := []color.RGBA{
		{120, 220, 235, 255}, // cyan
		{170, 140, 240, 255}, // violet
		{120, 240, 180, 255}, // teal-green
	}
	for _, cr := range crystals {
		drawCrystal(c, cr[0], cr[1], crystalCols[cr[2]])
	}
	// faint floating dust motes near glows
	addStars(c, 40, 99, 120, 1.0)
	c.save("assets/backgrounds/bg_cave.png")
}

// torchGlow paints distant firelight: a large soft warm halo with an
// elongated, slightly irregular shape (taller than wide, like rising heat)
// and a small dim flame core, so it reads as a torch rather than a sun disc.
func torchGlow(c *canvas, cx, cy int, col color.RGBA) {
	reach := 150.0
	for y := -int(reach * 1.3); y <= int(reach); y++ {
		for x := -int(reach); x <= int(reach); x++ {
			// elongate vertically + bias the falloff upward (flame licks up)
			fx := float64(x) / reach
			fy := float64(y) / (reach * 1.15)
			if y < 0 {
				fy *= 0.85 // glow extends further up
			}
			d := math.Sqrt(fx*fx + fy*fy)
			if d >= 1 {
				continue
			}
			a := (1 - d)
			a = a * a * 0.5 // dim, diffuse
			cc := col
			cc.A = uint8(255 * a)
			c.blend(cx+x, cy+y, cc)
		}
	}
	// small flame core, offset slightly up, brighter and yellower
	core := lerp(col, color.RGBA{255, 214, 140, 255}, 0.6)
	softDisc(c, float64(cx), float64(cy-6), 9, color.RGBA{core.R, core.G, core.B, 150}, 2.4)
	c.blend(cx, cy-6, color.RGBA{255, 230, 170, 220})
}

// rockTop draws a jagged stalactite ceiling silhouette descending from y=0.
func rockTop(c *canvas, col color.RGBA, baseDepth, amp float64, seed int64) {
	rng := rand.New(rand.NewSource(seed))
	p1 := rng.Float64() * 6
	p2 := rng.Float64() * 6
	for x := 0; x < c.w; x++ {
		fx := float64(x)
		d := baseDepth
		d += math.Sin(fx/120+p1) * amp
		d += math.Abs(math.Sin(fx/17+p2)) * amp * 1.4 // spikes downward
		bottom := int(d)
		for y := 0; y <= bottom; y++ {
			shade := lerp(col, lerp(col, cBlack, 0.4), 1-float64(y)/float64(bottom+1))
			c.blend(x, y, shade)
		}
	}
}

// drawCrystal paints a small faceted glowing crystal with a soft halo.
func drawCrystal(c *canvas, cx, cy int, col color.RGBA) {
	// halo
	softDisc(c, float64(cx), float64(cy-4), 6, color.RGBA{col.R, col.G, col.B, 90}, 3.5)
	// crystal body: a diamond/shard
	hi := lerp(col, color.RGBA{255, 255, 255, 255}, 0.5)
	dk := lerp(col, cBlack, 0.45)
	shape := [][2]int{
		{0, -8}, {-1, -6}, {0, -6}, {1, -6},
		{-1, -4}, {0, -4}, {1, -4},
		{-1, -2}, {0, -2}, {1, -2},
		{0, 0},
	}
	for _, s := range shape {
		x, y := cx+s[0], cy+s[1]
		col2 := col
		if s[0] < 0 {
			col2 = hi
		} else if s[0] > 0 {
			col2 = dk
		}
		c.blend(x, y, col2)
	}
	// bright tip
	c.blend(cx, cy-8, color.RGBA{255, 255, 255, 255})
}

// ---------------------------------------------------------------------------
// Hearts (kept: the generator originally painted these)
// ---------------------------------------------------------------------------

func paintHearts() {
	full := newCanvas(16, 16)
	empty := newCanvas(16, 16)
	red := color.RGBA{214, 64, 72, 255}
	redHi := color.RGBA{240, 120, 120, 255}
	redDk := color.RGBA{150, 40, 50, 255}
	emptyCol := color.RGBA{70, 50, 56, 255}
	emptyHi := color.RGBA{96, 72, 78, 255}

	// heart shape mask (1=fill)
	mask := []string{
		"................",
		"...##....##.....",
		"..####..####....",
		".##############.",
		".##############.",
		".##############.",
		".##############.",
		"..############..",
		"..############..",
		"...##########...",
		"....########....",
		".....######.....",
		"......####......",
		".......##.......",
		"................",
		"................",
	}
	for y, row := range mask {
		for x, ch := range row {
			if ch != '#' {
				continue
			}
			// shade: top-left highlight, bottom darker
			var col color.RGBA
			switch {
			case y <= 3 && x <= 7:
				col = redHi
			case y >= 9:
				col = redDk
			default:
				col = red
			}
			full.set(x, y, col)
			ecol := emptyCol
			if y <= 3 {
				ecol = emptyHi
			}
			empty.set(x, y, ecol)
		}
	}
	outlineSilhouette(full)
	outlineSilhouette(empty)
	// specular sparkle on full heart
	full.set(4, 3, color.RGBA{255, 220, 220, 255})
	full.set(5, 3, color.RGBA{255, 220, 220, 255})
	full.save("assets/ui/heart_full.png")
	empty.save("assets/ui/heart_empty.png")
}

// ---------------------------------------------------------------------------

func main() {
	// 1. slash
	paintSlash("right")
	paintSlash("left")
	// 2. second enemy
	paintSkull2("right")
	paintSkull2("left")
	// 3. NPCs
	paintElder()
	paintCritter()
	paintWanderer()
	// 4. backgrounds
	paintMeadowDay()
	paintDusk()
	paintCave()
	// hearts
	paintHearts()
	fmt.Println("done.")
}
