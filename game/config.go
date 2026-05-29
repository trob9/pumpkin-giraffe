// Centralised, rebindable key configuration. Every gameplay input reads its key
// from here so the settings screen can remap controls. Arrow keys and right
// shift stay hard-wired as always-available alternates for movement/jump so the
// game is never unplayable regardless of custom binds.
package game

import "github.com/hajimehoshi/ebiten/v2"

// Action is a rebindable game action.
type Action int

const (
	ActLeft Action = iota
	ActRight
	ActJump
	ActSprint
	ActAttack
	ActNeck
	ActInteract
)

// ActionOrder is the display order in the settings screen.
var ActionOrder = []Action{ActLeft, ActRight, ActJump, ActSprint, ActAttack, ActNeck, ActInteract}

// ActionNames are human labels for the settings screen.
var ActionNames = map[Action]string{
	ActLeft:     "Move Left",
	ActRight:    "Move Right",
	ActJump:     "Jump",
	ActSprint:   "Sprint",
	ActAttack:   "Sword Slash",
	ActNeck:     "Extend Neck",
	ActInteract: "Interact / Push",
}

// Keys holds the current bindings. Mutated by the settings screen, persisted by
// the caller. Defaults match the classic layout.
var Keys = map[Action]ebiten.Key{
	ActLeft:     ebiten.KeyA,
	ActRight:    ebiten.KeyD,
	ActJump:     ebiten.KeySpace,
	ActSprint:   ebiten.KeyShiftLeft,
	ActAttack:   ebiten.KeyF,
	ActNeck:     ebiten.KeyQ,
	ActInteract: ebiten.KeyE,
}

// Down reports whether the key bound to an action is held.
func Down(a Action) bool { return ebiten.IsKeyPressed(Keys[a]) }

// KeyName returns a display name for a key (e.g. "A", "Space", "Shift").
func KeyName(k ebiten.Key) string {
	s := k.String()
	// ebiten key names are like "KeyA" / "KeyShiftLeft"; trim the "Key" prefix.
	if len(s) > 3 && s[:3] == "Key" {
		s = s[3:]
	}
	return s
}
