//Implements a simple on-screen message system (floating, fading text) 
//and helper functions for drawing UI elements (buttons, prompts) at the proper scale.
package ui

import (
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
)

const (
	// defaultCharDelay is the time between revealing each character.
	defaultCharDelay = 30 * time.Millisecond
	// defaultDisplay is how long the full message stays on screen after typing finishes.
	defaultDisplay = 3 * time.Second
)

// Message manages a single floating text notification with
// typewriter-style reveal and automatic hide after a lifespan.
type Message struct {
	Full      string        // the complete text to display
	Index     int           // how many runes of Full are currently visible
	StartedAt time.Time     // when the message began revealing
	CharDelay time.Duration // delay between showing each character
	Life      time.Duration // how long to keep full text visible
	Active    bool          // whether the message is currently shown
}

// Start begins a new message sequence, resetting timers and activation.
func (m *Message) Start(text string, charDelay, display time.Duration) {
	m.Full = text
	m.Index = 0
	m.StartedAt = time.Now() // reset reveal timer
	m.CharDelay = charDelay
	m.Life = display
	m.Active = true // mark as visible
}

// Update should be called each frame (or on a fixed tick).
// It advances the reveal by one character when enough time has passed,
// and once fully revealed, it deactivates the message after its life expires.
func (m *Message) Update(_ time.Duration) {
	if !m.Active {
		return
	}
	elapsed := time.Since(m.StartedAt)
	runes := []rune(m.Full)

	// 1) Reveal phase: while not fully typed out
	if m.Index < len(runes) {
		// Check if enough time has passed to show the next rune
		if elapsed >= m.CharDelay*time.Duration(m.Index+1) {
			m.Index++
		}
		return
	}

	// 2) Display phase: fully revealed, wait for Life to elapse
	totalTypeTime := m.CharDelay * time.Duration(len(runes))
	if elapsed >= totalTypeTime+m.Life {
		m.Active = false // hide message
	}
}

// Text returns the substring of Full that should currently be shown.
func (m *Message) Text() string {
	runes := []rune(m.Full)
	if m.Index > len(runes) {
		return m.Full
	}
	return string(runes[:m.Index])
}

// UI ties together the Message logic with font rendering and positioning.
// It renders a floating label above the player’s head in the game world.
type UI struct {
	msg  Message   // the current message being displayed
	face font.Face // font used for drawing text
	zoom float64   // scale factor from world coords to screen pixels
}

// NewUI creates a UI manager given a font face and a world-to-screen zoom factor.
func NewUI(face font.Face, zoom float64) *UI {
	return &UI{face: face, zoom: zoom}
}

// NewMessage requests a new floating message using default timings.
// It will type out and then auto-hide after defaultDisplay.
func (ui *UI) NewMessage(s string) {
	ui.msg.Start(s, defaultCharDelay, defaultDisplay)
}

// Update drives the internal message state. Call once per frame.
func (ui *UI) Update(dt time.Duration) {
	ui.msg.Update(dt)
}

// CanTrigger reports whether the UI is ready to show another message.
// It returns true if no message is active or the previous one has fully expired.
func (ui *UI) CanTrigger() bool {
	if !ui.msg.Active {
		return true
	}
	elapsed := time.Since(ui.msg.StartedAt)
	runes := []rune(ui.msg.Full)
	totalType := ui.msg.CharDelay * time.Duration(len(runes))
	// Only allow new message after typing + display durations have passed
	return elapsed > totalType+ui.msg.Life
}

// Draw renders the current message above the player's head.
// It computes screen coordinates from world positions, centers the text,
// and draws only the portion revealed so far.
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
	bounds := text.BoundString(ui.face, str)

	// Convert world coordinates to screen pixels using zoom and camera offsets
	sx := (playerX - cameraX) * ui.zoom
	sy := (playerY - cameraY) * ui.zoom

	// Position text centered above the player's head, with a small gap
	x := sx - float64(bounds.Dx())/2
	y := sy - (playerHeight*ui.zoom)/2 - float64(bounds.Dy()) - 4

	// Draw the visible substring in white
	text.Draw(screen, str, ui.face, int(x), int(y), color.White)
}
