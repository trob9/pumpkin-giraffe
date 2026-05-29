package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"time"

	"github.com/TRob9/pumpkin-giraffe/game"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
)

var mainMenuItems = []string{"Play", "How to Play", "Lore", "Settings", "Quit"}

// lorePages is the intro story shown on the Lore screen. Warm, a little wry,
// not juvenile. The in-level NPCs carry the rest of the tale.
var lorePages = [][]string{
	{
		"PUMPKIN GIRAFFE",
		"",
		"They say a giraffe is measured by its neck.",
		"Ours never got the memo. Stout of neck and",
		"long of nerve, he sets out anyway — because",
		"the patch has gone quiet, and quiet is wrong.",
	},
	{
		"THE TOLL",
		"",
		"The old portals between the fields still stand,",
		"and they still ask their price: pumpkins, paid",
		"in full. Gather what the patch will give you and",
		"the gates will let you through to whatever waits.",
	},
	{
		"THE WAKING",
		"",
		"Something stirred the bonefolk from their sleep.",
		"They wander the meadows now, and the bold ones",
		"give chase. A short neck, it turns out, ducks a",
		"skull's reach nicely — and a well-timed blade",
		"does the rest. Onward, then. The dusk won't wait.",
	},
}

// ---- key binding persistence ----

func keybindPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "keybinds.json"
	}
	return filepath.Join(dir, "pumpkin-giraffe", "keybinds.json")
}

func loadKeybinds() {
	b, err := os.ReadFile(keybindPath())
	if err != nil {
		return
	}
	var m map[string]int
	if json.Unmarshal(b, &m) != nil {
		return
	}
	for a, name := range game.ActionNames {
		if k, ok := m[name]; ok {
			game.Keys[a] = ebiten.Key(k)
		}
	}
}

func saveKeybinds() {
	m := map[string]int{}
	for a, name := range game.ActionNames {
		m[name] = int(game.Keys[a])
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	_ = os.MkdirAll(filepath.Dir(keybindPath()), 0o755)
	_ = os.WriteFile(keybindPath(), b, 0o644)
}

// startNewGame resets everything and drops into level 1.
func (g *Game) startNewGame() {
	game.Levels = loadAllLevels()
	game.CurrentLevel = 0
	applySpawn(0)
	g.player = game.NewPlayer(Assets, g.ui.NewMessage)
	g.pumpkins = nil
	g.pumpkinSpawned = false
	g.pumpkinMissed = false
	g.initialPumpkinDropped = false
	g.timerStopped = false
	g.accumulated = 0
	g.pauseStart = time.Time{}
	g.startTime = time.Now()
	g.endTime = time.Time{}
	g.resetAmbient()
	g.state = StatePlaying
}

// ---- update (input) ----

func menuNav() (up, down, enter, back bool) {
	up = inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW)
	down = inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS)
	enter = inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace)
	back = inpututil.IsKeyJustPressed(ebiten.KeyEscape)
	return
}

func (g *Game) updateMenu() {
	up, down, enter, _ := menuNav()
	n := len(mainMenuItems)
	if up {
		g.menuSel = (g.menuSel - 1 + n) % n
	}
	if down {
		g.menuSel = (g.menuSel + 1) % n
	}
	if enter {
		switch mainMenuItems[g.menuSel] {
		case "Play":
			g.startNewGame()
		case "How to Play":
			g.state = StateHowTo
		case "Lore":
			g.lorePage = 0
			g.state = StateLore
		case "Settings":
			g.setSel, g.rebindAction = 0, -1
			g.state = StateSettings
		case "Quit":
			os.Exit(0)
		}
	}
}

func (g *Game) updateSettings() {
	// Listening for a new key to bind.
	if g.rebindAction >= 0 {
		for _, k := range inpututil.AppendJustPressedKeys(nil) {
			if k == ebiten.KeyEscape {
				g.rebindAction = -1 // cancel
				return
			}
			game.Keys[game.Action(g.rebindAction)] = k
			g.rebindAction = -1
			saveKeybinds()
			return
		}
		return
	}
	up, down, enter, back := menuNav()
	n := len(game.ActionOrder)
	if up {
		g.setSel = (g.setSel - 1 + n) % n
	}
	if down {
		g.setSel = (g.setSel + 1) % n
	}
	if enter {
		g.rebindAction = int(game.ActionOrder[g.setSel])
	}
	if back {
		g.state = StateMenu
	}
}

func (g *Game) updateLore() {
	if inpututil.IsKeyJustPressed(ebiten.KeyRight) || inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		if g.lorePage < len(lorePages)-1 {
			g.lorePage++
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyLeft) || inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		if g.lorePage > 0 {
			g.lorePage--
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		g.state = StateMenu
	}
}

func (g *Game) updateSimpleScreen(GameState) {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) ||
		inpututil.IsKeyJustPressed(ebiten.KeyEnter) ||
		inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		g.state = StateMenu
	}
}

// ---- draw helpers ----

func centerText(screen *ebiten.Image, s string, face font.Face, y int, c color.Color) {
	b := text.BoundString(face, s)
	x := (WindowWidth - (b.Max.X - b.Min.X)) / 2
	text.Draw(screen, s, face, x, y, c)
}

var (
	colGold = color.RGBA{232, 198, 92, 255}
	colDim  = color.RGBA{150, 150, 165, 255}
	colHi   = color.RGBA{255, 255, 255, 255}
)

func (g *Game) drawMenu(screen *ebiten.Image) {
	screen.Fill(color.RGBA{18, 16, 30, 255})
	centerText(screen, "PUMPKIN GIRAFFE", buttonFont, 150, colGold)
	centerText(screen, "a short-necked odyssey", hudFont, 195, colDim)
	for i, item := range mainMenuItems {
		c := color.Color(colDim)
		label := item
		if i == g.menuSel {
			c = colHi
			label = "> " + item + " <"
		}
		centerText(screen, label, hudFont, 300+i*44, c)
	}
	centerText(screen, "Up / Down  -  select        Enter  -  confirm", hudFont, WindowHeight-50, colDim)
}

func (g *Game) drawSettings(screen *ebiten.Image) {
	screen.Fill(color.RGBA{18, 16, 30, 255})
	centerText(screen, "CONTROLS", buttonFont, 120, colGold)
	for i, a := range game.ActionOrder {
		y := 220 + i*48
		c := color.Color(colDim)
		if i == g.setSel {
			c = colHi
		}
		text.Draw(screen, game.ActionNames[a], hudFont, WindowWidth/2-260, y, c)
		keyLabel := "[ " + game.KeyName(game.Keys[a]) + " ]"
		if g.rebindAction == int(a) {
			keyLabel = "[ press a key... ]"
			c = colGold
		}
		text.Draw(screen, keyLabel, hudFont, WindowWidth/2+120, y, c)
	}
	centerText(screen, "Enter  -  rebind        Esc  -  back", hudFont, WindowHeight-50, colDim)
}

func (g *Game) drawLore(screen *ebiten.Image) {
	screen.Fill(color.RGBA{14, 12, 24, 255})
	page := lorePages[g.lorePage]
	for i, line := range page {
		c := color.Color(colHi)
		face := hudFont
		if i == 0 {
			c = colGold
			face = buttonFont
		}
		centerText(screen, line, face, 160+i*40, c)
	}
	centerText(screen, fmt.Sprintf("%d / %d", g.lorePage+1, len(lorePages)), hudFont, WindowHeight-90, colDim)
	centerText(screen, "Left / Right  -  pages        Esc  -  back", hudFont, WindowHeight-50, colDim)
}

func (g *Game) drawHowTo(screen *ebiten.Image) {
	screen.Fill(color.RGBA{18, 16, 30, 255})
	centerText(screen, "HOW TO PLAY", buttonFont, 110, colGold)
	lines := []string{
		"Collect pumpkins, then reach the glowing gate and",
		"pay its toll to pass to the next field.",
		"",
		"Move: " + game.KeyName(game.Keys[game.ActLeft]) + " / " + game.KeyName(game.Keys[game.ActRight]) +
			"   (arrow keys work too)",
		"Jump: " + game.KeyName(game.Keys[game.ActJump]) + "    Sprint: hold " + game.KeyName(game.Keys[game.ActSprint]),
		"Sword: " + game.KeyName(game.Keys[game.ActAttack]) + " — time it to cut down a skeleton.",
		"Push a boulder: hold " + game.KeyName(game.Keys[game.ActInteract]) + " and walk into it.",
		"Extend neck: hold " + game.KeyName(game.Keys[game.ActNeck]) + " — hook a ledge above, release to climb.",
		"",
		"Stomp skeletons from above, or dodge — a side hit",
		"costs a heart. Lose all three and the field resets.",
	}
	for i, l := range lines {
		text.Draw(screen, l, hudFont, 180, 190+i*38, colHi)
	}
	centerText(screen, "Esc  -  back", hudFont, WindowHeight-50, colDim)
}
