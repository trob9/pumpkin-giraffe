package ui

import (
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
)

const (
	defaultCharDelay = 30 * time.Millisecond
	defaultDisplay   = 3 * time.Second
)

// Message handles the typewriter reveal and auto-hide logic.
type Message struct {
	Full      string
	Index     int
	StartedAt time.Time // renamed
	CharDelay time.Duration
	Life      time.Duration
	Active    bool
}

// Start initializes a new message.
func (m *Message) Start(text string, charDelay, display time.Duration) {
	m.Full = text
	m.Index = 0
	m.StartedAt = time.Now()
	m.CharDelay = charDelay
	m.Life = display
	m.Active = true
}

// Update advances the reveal by at most one rune per call, based on elapsed time.
func (m *Message) Update(_ time.Duration) {
	if !m.Active {
		return
	}
	elapsed := time.Since(m.StartedAt)

	// If we aren't fully revealed yet, check if it's time for the next rune.
	runes := []rune(m.Full)
	if m.Index < len(runes) {
		// has enough time passed for one more char?
		if elapsed >= m.CharDelay*time.Duration(m.Index+1) {
			m.Index++
		}
		return
	}

	// Fully revealed: check if display duration has passed.
	if elapsed >= m.CharDelay*time.Duration(len(runes))+m.Life {
		m.Active = false
	}
}

// Text returns the currently visible substring.
func (m *Message) Text() string {
	runes := []rune(m.Full)
	if m.Index > len(runes) {
		return m.Full
	}
	return string(runes[:m.Index])
}

// UI renders floating messages above the player.
type UI struct {
	msg  Message
	face font.Face
	zoom float64
}

// NewUI constructs a UI with the given font face and zoom.
func NewUI(face font.Face, zoom float64) *UI {
	return &UI{face: face, zoom: zoom}
}

// NewMessage triggers a typewriter message.
func (ui *UI) NewMessage(s string) {
	ui.msg.Start(s, defaultCharDelay, defaultDisplay)
}

// Update drives the message; call once per frame.
func (ui *UI) Update(dt time.Duration) {
	ui.msg.Update(dt)
}

// CanTrigger reports whether enough time has passed (typed + display)
// so it’s OK to show a new message.
func (ui *UI) CanTrigger() bool {
	if !ui.msg.Active {
		return true
	}
	elapsed := time.Since(ui.msg.StartedAt)
	runes := []rune(ui.msg.Full)
	totalType := ui.msg.CharDelay * time.Duration(len(runes))
	return elapsed > totalType+ui.msg.Life
}

// Draw renders the message above the player's head.
func (ui *UI) Draw(
	screen *ebiten.Image,
	cameraX, cameraY,
	playerX, playerY,
	playerHeight float64,
) {
	if !ui.msg.Active {
		return
	}
	str := ui.msg.Text()
	b := text.BoundString(ui.face, str)

	// world→screen
	sx := (playerX - cameraX) * ui.zoom
	sy := (playerY - cameraY) * ui.zoom

	// center text above head
	x := sx - float64(b.Dx())/2
	y := sy - (playerHeight*ui.zoom)/2 - float64(b.Dy()) - 4

	text.Draw(screen, str, ui.face, int(x), int(y), color.White)
}
