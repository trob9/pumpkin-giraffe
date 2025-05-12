package ui

import (
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font/basicfont"
)

const (
	defaultCharDelay = 20 * time.Millisecond
	defaultDisplay   = 4 * time.Second
)

// Message handles the typewriter reveal and auto-hide.
type Message struct {
	full      string
	index     int
	timer     time.Duration
	charDelay time.Duration
	life      time.Duration
	active    bool
}

func (m *Message) Start(text string, charDelay, display time.Duration) {
	m.full = text
	m.index = 0
	m.timer = 0
	m.charDelay = charDelay
	m.life = display
	m.active = true
}

func (m *Message) Update(dt time.Duration) {
	if !m.active {
		return
	}
	if m.index < len(m.full) {
		m.timer += dt
		for m.timer >= m.charDelay && m.index < len(m.full) {
			m.timer -= m.charDelay
			m.index++
		}
	} else {
		m.life -= dt
		if m.life <= 0 {
			m.active = false
		}
	}
}

func (m *Message) Text() string {
	return m.full[:m.index]
}

// UI is your on-screen UI helper.
type UI struct {
	msg Message
}

// NewUI builds your UI helper.
func NewUI() *UI {
	return &UI{}
}

// NewMessage triggers a new typewriter message.
func (ui *UI) NewMessage(s string) {
	ui.msg.Start(s, defaultCharDelay, defaultDisplay)
}

// Update must be called every frame with your dt.
func (ui *UI) Update(dt time.Duration) {
	ui.msg.Update(dt)
}

// Draw, given the player’s position & height, will render the message above their head.
func (ui *UI) Draw(screen *ebiten.Image, playerX, playerY, playerHeight float64) {
	if !ui.msg.active {
		return
	}
	str := ui.msg.Text()
	b := text.BoundString(basicfont.Face7x13, str)
	x := playerX - float64(b.Dx()/2)
	y := playerY - playerHeight/2 - 6 // tweak “-6” to space above your sprite
	text.Draw(screen, str, basicfont.Face7x13, int(x), int(y), color.White)
}
