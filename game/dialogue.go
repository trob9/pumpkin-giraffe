// dialogue.go — the conversation box: its state machine and its on-screen look.
//
// A conversation is a list of lines spoken by one NPC. The box reveals the
// current line one character at a time (a "typewriter"), and a press of the
// talk key either (a) snaps the current line fully open if it's still typing,
// or (b) advances to the next line, or (c) closes the box if that was the last
// line. The StoryManager owns one of these and feeds it presses.
package game

import (
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
)

// charDelay is the time between revealing each character of a line. Slightly
// faster than the floating-message UI (which uses 30ms) because dialogue lines
// are longer and we don't want the player waiting.
const charDelay = 22 * time.Millisecond

// dialogue holds the state of one open conversation.
type dialogue struct {
	speaker who       // who is talking (drives the name label)
	lines   []string  // the full set of lines for this conversation
	line    int       // index of the line currently being shown
	shown   int       // how many runes of the current line are revealed
	started time.Time // when the current line began revealing
	open    bool      // whether the box is currently visible
}

// start opens the box on the first line of a fresh conversation.
func (d *dialogue) start(speaker who, lines []string) {
	if len(lines) == 0 {
		return // nothing to say — don't open an empty box
	}
	d.speaker = speaker
	d.lines = lines
	d.line = 0
	d.shown = 0
	d.started = time.Now()
	d.open = true
}

// tick advances the typewriter for the current line. Call once per frame while
// the box is open. It reveals one more rune each time enough real time has
// passed, which keeps the reveal speed steady regardless of frame rate.
func (d *dialogue) tick() {
	if !d.open {
		return
	}
	runes := []rune(d.lines[d.line])
	if d.shown >= len(runes) {
		return // current line already fully shown; wait for an advance press
	}
	elapsed := time.Since(d.started)
	// how many runes *should* be visible by now
	want := int(elapsed / charDelay)
	if want > len(runes) {
		want = len(runes)
	}
	d.shown = want
}

// fullyShown reports whether the current line has finished typing out.
func (d *dialogue) fullyShown() bool {
	return d.shown >= len([]rune(d.lines[d.line]))
}

// advance is called when the talk key is pressed while the box is open.
//   - If the current line is still typing, snap it fully open (impatient
//     players can skip the typewriter).
//   - Otherwise move to the next line, or close the box if this was the last.
func (d *dialogue) advance() {
	if !d.open {
		return
	}
	if !d.fullyShown() {
		d.shown = len([]rune(d.lines[d.line])) // snap to full
		return
	}
	if d.line < len(d.lines)-1 {
		d.line++
		d.shown = 0
		d.started = time.Now()
		return
	}
	d.open = false // that was the last line
}

// visibleText returns the portion of the current line revealed so far.
func (d *dialogue) visibleText() string {
	runes := []rune(d.lines[d.line])
	if d.shown > len(runes) {
		return d.lines[d.line]
	}
	return string(runes[:d.shown])
}

// --- Rendering ---------------------------------------------------------------

// Box geometry, in SCREEN pixels (the box is drawn after the world is scaled up,
// so it works in the full 1280x720 space). A wide, short panel hugging the
// bottom edge — the classic visual-novel / RPG dialogue footprint.
const (
	boxMargin = 48  // gap from the screen's left/right/bottom edges
	boxHeight = 150 // height of the panel
	boxPadX   = 28  // inner horizontal padding for text
	boxPadY   = 26  // inner top padding for the speaker name
)

// draw renders the dialogue box at screen scale. `face` is the body-text font,
// `nameFace` the (smaller) font for the speaker label. `w`,`h` are the screen
// dimensions so the box positions itself relative to the bottom edge.
func (d *dialogue) draw(screen *ebiten.Image, w, h int, face, nameFace font.Face) {
	if !d.open {
		return
	}

	bx := boxMargin
	by := h - boxMargin - boxHeight
	bw := w - boxMargin*2
	bh := boxHeight

	// 1) A soft dark backing so the box lifts off the world behind it. We draw
	//    a slightly larger, very dark rectangle first as a "shadow", then the
	//    main panel on top — cheap depth without any image assets.
	fillRect(screen, bx-4, by-4, bw+8, bh+8, color.RGBA{0, 0, 0, 120})
	fillRect(screen, bx, by, bw, bh, color.RGBA{18, 16, 34, 235}) // deep indigo, near-opaque

	// 2) A thin warm border, drawn as four edge strips. The warm tone ties the
	//    box to the pumpkin theme without being garish.
	border := color.RGBA{0xC9, 0x8A, 0x3C, 0xff}
	const t = 2                                  // border thickness
	fillRect(screen, bx, by, bw, t, border)      // top
	fillRect(screen, bx, by+bh-t, bw, t, border) // bottom
	fillRect(screen, bx, by, t, bh, border)      // left
	fillRect(screen, bx+bw-t, by, t, bh, border) // right

	// 3) Speaker name, in the warm border colour, near the top-left.
	name := d.speaker.speakerName()
	text.Draw(screen, name, nameFace, bx+boxPadX, by+boxPadY, border)

	// 4) A faint divider rule under the name to separate it from the body.
	fillRect(screen, bx+boxPadX, by+boxPadY+10, bw-boxPadX*2, 1, color.RGBA{0x6A, 0x5A, 0x46, 0xff})

	// 5) The typewritered body, word-wrapped to the box width and drawn as one
	//    or more rows. Wrapping means the writing isn't forced into a single
	//    cramped line — the PressStart2P pixel font is very wide, so a sentence
	//    naturally flows onto two rows inside the panel.
	body := d.visibleText()
	tx := bx + boxPadX
	ty := by + boxPadY + 40
	lineH := face.Metrics().Height.Ceil() + 6 // row spacing with a little air
	innerW := bw - boxPadX*2
	for i, row := range wrapText(face, body, innerW) {
		ry := ty + i*lineH
		text.Draw(screen, row, face, tx+1, ry+1, color.RGBA{0, 0, 0, 180})
		text.Draw(screen, row, face, tx, ry, color.RGBA{0xF2, 0xEC, 0xDE, 0xff})
	}

	// 6) A blinking advance arrow in the bottom-right once the line is fully
	//    revealed, signalling "press E for more / to close".
	if d.fullyShown() {
		// blink ~twice a second using wall-clock time
		if (time.Now().UnixMilli()/400)%2 == 0 {
			drawArrow(screen, bx+bw-boxPadX-10, by+bh-boxPadY+6, border)
		}
	}
}

// dlgPixel is a shared 1x1 white image scaled to draw rectangles, so fillRect
// allocates nothing per call (it's invoked ~10x/frame while a box is open).
var dlgPixel *ebiten.Image

// fillRect draws a solid filled rectangle by scaling a shared 1x1 pixel and
// tinting it — no per-call image allocation.
func fillRect(dst *ebiten.Image, x, y, w, h int, c color.Color) {
	if dlgPixel == nil {
		dlgPixel = ebiten.NewImage(1, 1)
		dlgPixel.Fill(color.White)
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(w), float64(h))
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.ScaleWithColor(c)
	dst.DrawImage(dlgPixel, op)
}

// wrapText breaks s into rows that each fit within maxW pixels for the given
// face, splitting only on spaces so words stay whole. It is fed the *visible*
// (partly-typed) text each frame, so the wrap follows the typewriter live.
func wrapText(face font.Face, s string, maxW int) []string {
	if s == "" {
		return []string{""}
	}
	var rows []string
	var line string
	for _, word := range splitSpaces(s) {
		try := word
		if line != "" {
			try = line + " " + word
		}
		if text.BoundString(face, try).Dx() <= maxW {
			line = try
			continue
		}
		// word doesn't fit on the current row: flush and start a new one
		if line != "" {
			rows = append(rows, line)
		}
		line = word
	}
	if line != "" {
		rows = append(rows, line)
	}
	if len(rows) == 0 {
		rows = []string{""}
	}
	return rows
}

// splitSpaces splits on single spaces while preserving a trailing empty token
// when the visible text ends mid-space, so the wrap doesn't jump as the
// typewriter crosses a word boundary. Standard library strings.Fields would
// collapse runs and lose that nuance; this keeps it simple and dependency-free.
func splitSpaces(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

// drawArrow paints a small downward-pointing triangle (the "more" indicator).
func drawArrow(dst *ebiten.Image, x, y int, c color.Color) {
	// a little 9px-wide triangle, filled row by row
	for row := 0; row < 5; row++ {
		half := 4 - row
		fillRect(dst, x-half, y+row, half*2+1, 1, c)
	}
}
