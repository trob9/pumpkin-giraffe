package main

import (
	"embed"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"

	"PumpkinGiraffe/game"
)

//go:embed assets/tilesets/*.png
//go:embed assets/sprites/*.png
//go:embed assets/sfx/*.wav
//go:embed assets/music/*.wav
//go:embed assets/fonts/*.ttf
//go:embed levels/*.json
//go:embed levels/*.png
var Assets embed.FS

const (
	SampleRate     = 48000
	WindowWidth    = 1280
	WindowHeight   = 720
	ZoomFactor     = 2.0
	pumpkinGravity = 0.04
	pumpkinMaxFall = 2.0

	btnW = 200
	btnH = 80
)

var (
	// center the Start button
	btnX = (WindowWidth - btnW) / 2
	btnY = (WindowHeight - btnH) / 2

	hudFont         font.Face // for the blurb, HUD & timer
	buttonFont      font.Face // for button labels
	interactionFont font.Face
)

func init() {
	// load single TTF, make two faces
	data, err := Assets.ReadFile("assets/fonts/PressStart2P-Regular.ttf")
	if err != nil {
		log.Fatal(err)
	}
	tt, err := opentype.Parse(data)
	if err != nil {
		log.Fatal(err)
	}
	// HUD: larger so the blurb is readable
	hudFont, err = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    14,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}
	// Buttons: even bigger
	buttonFont, err = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    24,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}
	// Interaction messages: small size 10
	interactionFont, err = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    10,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}
}

type GameState int

const (
	StateTitle GameState = iota
	StatePlaying
	StatePaused
	StateFinished
)

type Game struct {
	ui                    *UI
	player                *game.Player
	pumpkins              []*game.Pumpkin
	pumpkinSpawned        bool
	pumpkinMissed         bool
	jumpSnd, deathSnd     *audio.Player
	stepSnds              map[int]*audio.Player
	landingSnd            *audio.Player
	monsterDeathSnd       *audio.Player
	pumpkinSnd            *audio.Player
	buffer                *ebiten.Image
	CameraX, CameraY      float64
	startTime             time.Time
	endTime               time.Time
	timerStopped          bool
	state                 GameState
	prevEscape            bool
	startButton           image.Rectangle
	restartButton         image.Rectangle
	exitButton            image.Rectangle
	initialPumpkinDropped bool
	prevInteract          bool
	pauseStart            time.Time
	accumulated           time.Duration
	interactionText       string    // full message
	interactionRunes      int       // how many runes to draw
	interactionStart      time.Time // when NewMessage was called
}

func (g *Game) SpawnInsidePumpkin() {
	if !g.pumpkinSpawned && g.pumpkinMissed {
		g.spawnPumpkinAt(390, 0)
		g.pumpkinSpawned = true
		g.pumpkinMissed = false
		g.NewMessage("Wow, there’s a pumpkin inside!")
	}
}

func (g *Game) RedropPumpkin() {
	if g.pumpkinMissed && !g.pumpkinSpawned {
		g.spawnPumpkinAt(390, 0)
		g.pumpkinSpawned = true
		g.pumpkinMissed = false
		g.NewMessage("…is that something falling from the sky?")
	}
}

// NewGame constructs and wires up the Game, including the onInteract callback.
func NewGame(
	jumpSnd, deathSnd *audio.Player,
	stepSnds map[int]*audio.Player,
	landingSnd *audio.Player,
	monsterDeathSnd *audio.Player,
	pumpkinSnd *audio.Player,
) *Game {
	// compute buffer size once
	bw := int(float64(WindowWidth) / ZoomFactor)
	bh := int(float64(WindowHeight) / ZoomFactor)

	// 1) create the Game instance (player wired in step 2)
	g := &Game{
		ui:                    NewUI(),
		pumpkins:              nil,
		pumpkinSpawned:        false,
		pumpkinMissed:         false,
		jumpSnd:               jumpSnd,
		deathSnd:              deathSnd,
		stepSnds:              stepSnds,
		landingSnd:            landingSnd,
		monsterDeathSnd:       monsterDeathSnd,
		pumpkinSnd:            pumpkinSnd,
		buffer:                ebiten.NewImage(bw, bh),
		CameraX:               0,
		CameraY:               0,
		startTime:             time.Now(),
		endTime:               time.Time{},
		timerStopped:          false,
		state:                 StateTitle,
		prevEscape:            false,
		startButton:           image.Rect(btnX, btnY, btnX+btnW, btnY+btnH),
		restartButton:         image.Rect(btnX, btnY, btnX+btnW, btnY+btnH),
		exitButton:            image.Rect(btnX, btnY+btnH+20, btnX+btnW, btnY+btnH*2+20),
		initialPumpkinDropped: false,
		prevInteract:          false,
		pauseStart:            time.Time{},
		accumulated:           0,

		// ← initialize your interaction text storage
		interactionText: "",
	}

	// 2) wire the player with the onInteract callback
	//    whenever the player calls onInteract(msg), it will invoke g.NewMessage(msg)
	g.player = game.NewPlayer(
		Assets,
		g.NewMessage,         // onInteract
		g.SpawnInsidePumpkin, // hidden‐inside pumpkin (tile 90)
		g.RedropPumpkin,      // sky‐drop re‐drop (tile 84)
	)

	return g
}

func (g *Game) Update() error {
	// ————— Interaction text typewriter & auto-clear —————
	if g.interactionText != "" {
		elapsed := time.Since(g.interactionStart)
		totalRunes := len([]rune(g.interactionText))
		revealCount := int(elapsed / (50 * time.Millisecond))
		if revealCount > totalRunes {
			revealCount = totalRunes
		}
		g.interactionRunes = revealCount
		if elapsed > 3*time.Second {
			g.interactionText = ""
		}
	}

	// Global Escape → pause/resume
	esc := ebiten.IsKeyPressed(ebiten.KeyEscape)
	if esc && !g.prevEscape {
		switch g.state {
		case StatePlaying:
			g.state = StatePaused
			g.pauseStart = time.Now()
		case StatePaused:
			g.accumulated += time.Since(g.pauseStart)
			g.state = StatePlaying
		}
	}
	g.prevEscape = esc

	// Handle non‐Playing states
	switch g.state {
	case StateTitle:
		mx, my := ebiten.CursorPosition()
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) &&
			mx >= g.startButton.Min.X && mx <= g.startButton.Max.X &&
			my >= g.startButton.Min.Y && my <= g.startButton.Max.Y {
			g.state = StatePlaying
			g.startTime = time.Now()
		}
		return nil

	case StatePaused, StateFinished:
		mx, my := ebiten.CursorPosition()
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			pt := image.Pt(mx, my)
			if pt.In(g.restartButton) {
				// reset flags
				g.prevInteract = false
				g.initialPumpkinDropped = false
				g.pumpkinSpawned = false
				g.pumpkinMissed = false
				g.timerStopped = false
				g.accumulated = 0
				g.pauseStart = time.Time{}

				// reset clock
				g.startTime = time.Now()
				g.endTime = time.Time{}

				// clear pumpkins
				g.pumpkins = nil

				// recreate player
				g.player = game.NewPlayer(
					Assets,
					g.NewMessage,
					g.SpawnInsidePumpkin,
					g.RedropPumpkin,
				)

				// reload level
				lvlFull, err := game.LoadLevelFS(Assets, "levels/test_level.json")
				if err != nil {
					log.Fatal(err)
				}
				game.Levels[game.CurrentLevel] = lvlFull

				g.state = StatePlaying
			}
			if pt.In(g.exitButton) {
				os.Exit(0)
			}
		}
		return nil
	}

	// StatePlaying:
	// 1) Pumpkin physics: gravity, catch, miss
	lvl := game.Levels[game.CurrentLevel]
	for i := 0; i < len(g.pumpkins); i++ {
		pk := g.pumpkins[i]
		// gravity
		pk.VelY += pumpkinGravity
		if pk.VelY > pumpkinMaxFall {
			pk.VelY = pumpkinMaxFall
		}
		pk.Y += pk.VelY

		// caught?
		if g.player.Rect().Overlaps(pk.Rect()) {
			g.player.Pumpkins++
			g.pumpkinSnd.Rewind()
			g.pumpkinSnd.Play()
			g.pumpkins = append(g.pumpkins[:i], g.pumpkins[i+1:]...)
			i--
			g.pumpkinSpawned = false
			g.pumpkinMissed = false
			continue
		}

		// missed?
		bottom := float64(len(lvl.Tiles)) * float64(game.TileSize)
		if pk.Y > bottom {
			g.pumpkins = append(g.pumpkins[:i], g.pumpkins[i+1:]...)
			i--
			g.pumpkinMissed = true
			g.pumpkinSpawned = false
			continue
		}
	}

	// 2) Player update (pass in current pumpkinMissed flag)
	g.player.Update(
		g.jumpSnd,
		g.deathSnd,
		g.stepSnds,
		g.landingSnd,
		g.monsterDeathSnd,
		g.pumpkinSnd,
		g.pumpkinMissed,
	)

	// 3) Enemy update & collisions
	alive := 0
	for _, en := range lvl.Enemies {
		en.Update()
		if !en.Alive {
			continue
		}
		alive++
		pr, er := g.player.Rect(), en.Rect()
		if pr.Overlaps(er) {
			if g.player.CollidesHeadOn(er) {
				en.Alive = false
				g.monsterDeathSnd.Rewind()
				g.monsterDeathSnd.Play()
				g.player.VelY = -4
			} else {
				g.player.Respawn()
			}
		}
	}

	// 4) Initial pumpkin drop when last enemy dies
	if alive == 0 && !g.initialPumpkinDropped {
		g.spawnPumpkinAt(390, 0)
		g.initialPumpkinDropped = true
		g.pumpkinSpawned = true
	}

	// 5) Stop timer at 5 pumpkins
	if !g.timerStopped && g.player.Pumpkins >= 5 {
		g.endTime = time.Now()
		g.timerStopped = true
	}

	// 6) Camera follow
	bw := float64(g.buffer.Bounds().Dx())
	bh := float64(g.buffer.Bounds().Dy())
	g.CameraX = clamp(
		g.player.X-bw/2+g.player.Width/2,
		0,
		float64(len(lvl.Tiles[0])*game.TileSize)-bw,
	)
	g.CameraY = clamp(
		g.player.Y-bh/2+g.player.Height/2,
		0,
		float64(len(lvl.Tiles)*game.TileSize)-bh,
	)

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	switch g.state {
	case StateTitle:
		screen.Fill(color.Black)
		// blurb
		lines := []string{
			"You are a short-necked giraffe and need to collect 5 pumpkins as fast as you can.",
			"Try to be faster than your friends!",
			"Press E to interact with barrels.",
			"Kill monsters by jumping on their heads.",
			"This game rewards glitch abuse and curious minds.",
		}
		y := 100
		for _, l := range lines {
			b := text.BoundString(hudFont, l)
			x := (WindowWidth - (b.Max.X - b.Min.X)) / 2
			text.Draw(screen, l, hudFont, x, y, color.White)
			y += 30
		}
		// Start button
		ebitenutil.DrawRect(screen,
			float64(g.startButton.Min.X), float64(g.startButton.Min.Y),
			float64(g.startButton.Dx()), float64(g.startButton.Dy()),
			color.RGBA{0, 0, 128, 200},
		)
		tb := "START"
		tbB := text.BoundString(buttonFont, tb)
		text.Draw(screen, tb, buttonFont,
			g.startButton.Min.X+(g.startButton.Dx()-tbB.Dx())/2,
			g.startButton.Min.Y+btnH/2+8,
			color.White,
		)
		return

	case StatePaused:
		// dark overlay
		screen.Fill(color.RGBA{0, 0, 0, 180})
		text.Draw(screen, "PAUSED", buttonFont,
			WindowWidth/2-80, WindowHeight/2-100, color.White)

	case StateFinished:
		d := g.endTime.Sub(g.startTime)
		msg := fmt.Sprintf("You did it! Your time was %02d:%02d.%03d",
			int(d/time.Minute),
			int(d/time.Second)%60,
			int(d/time.Millisecond)%1000,
		)
		b := text.BoundString(buttonFont, msg)
		x := (WindowWidth - (b.Max.X - b.Min.X)) / 2
		text.Draw(screen, msg, buttonFont, x, WindowHeight/2-60, color.White)
	}

	// draw Restart/Exit buttons in both Paused/Finished
	ebitenutil.DrawRect(screen,
		float64(g.restartButton.Min.X), float64(g.restartButton.Min.Y),
		float64(g.restartButton.Dx()), float64(g.restartButton.Dy()),
		color.RGBA{0, 0, 128, 200},
	)
	text.Draw(screen, "RESTART", buttonFont,
		g.restartButton.Min.X+(g.restartButton.Dx()-text.BoundString(buttonFont, "RESTART").Dx())/2,
		g.restartButton.Min.Y+btnH/2+8,
		color.White,
	)
	ebitenutil.DrawRect(screen,
		float64(g.exitButton.Min.X), float64(g.exitButton.Min.Y),
		float64(g.exitButton.Dx()), float64(g.exitButton.Dy()),
		color.RGBA{128, 0, 0, 200},
	)
	text.Draw(screen, "EXIT", buttonFont,
		g.exitButton.Min.X+(g.exitButton.Dx()-text.BoundString(buttonFont, "EXIT").Dx())/2,
		g.exitButton.Min.Y+btnH/2+8,
		color.White,
	)

	if g.state != StatePlaying {
		return
	}

	// --- regular play draw ---
	g.buffer.Fill(color.Black)
	game.DrawLevel(g.buffer, g.CameraX, g.CameraY)
	g.player.Draw(g.buffer, g.CameraX, g.CameraY)

	lvl := game.Levels[game.CurrentLevel]
	for _, en := range lvl.Enemies {
		en.Draw(g.buffer, g.CameraX, g.CameraY)
	}
	for _, p := range g.pumpkins {
		if !p.Alive {
			continue
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(p.X-g.CameraX, p.Y-g.CameraY)
		g.buffer.DrawImage(p.Image, op)
	}

	// 1) scale & blit world
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(ZoomFactor, ZoomFactor)
	screen.DrawImage(g.buffer, op)

	// 2) floating interaction text (screen-space)
	// 2) floating interaction text (screen-space)
	if g.interactionText != "" && g.interactionRunes > 0 {
		runes := []rune(g.interactionText)
		msg := string(runes[:g.interactionRunes])
		b := text.BoundString(interactionFont, msg)
		w := b.Max.X - b.Min.X

		// world-space center above the player, including camera offset
		worldX := (g.player.X - g.CameraX) + g.player.Width/2
		worldY := (g.player.Y - g.CameraY) - 8

		// to screen-space
		screenX := worldX*ZoomFactor - float64(w)/2
		screenY := worldY * ZoomFactor

		text.Draw(screen, msg, interactionFont, int(screenX), int(screenY), color.White)
	}

	// 3) HUD & timer
	text.Draw(screen, fmt.Sprintf("Pumpkins: %d", g.player.Pumpkins),
		hudFont, WindowWidth-200, 24, color.White,
	)

	var d time.Duration
	if g.timerStopped {
		d = g.endTime.Sub(g.startTime) - g.accumulated
	} else if g.state == StatePaused {
		d = g.pauseStart.Sub(g.startTime) - g.accumulated
	} else {
		d = time.Now().Sub(g.startTime) - g.accumulated
	}
	timerStr := fmt.Sprintf("%02d:%02d.%03d",
		int(d/time.Minute),
		int(d/time.Second)%60,
		int(d/time.Millisecond)%1000,
	)
	tb := text.BoundString(hudFont, timerStr)
	x := (WindowWidth - tb.Dx()) / 2
	text.Draw(screen, timerStr, hudFont, x, 32, color.White)
}

func (g *Game) Layout(w, h int) (int, int) { return WindowWidth, WindowHeight }

func (g *Game) spawnPumpkinAt(px, py float64) {
	p := game.NewPumpkin(px, py-float64(game.TileSize))
	g.pumpkins = append(g.pumpkins, p)
}

func (g *Game) NewMessage(msg string) {
	log.Println("NewMessage:", msg)
	g.interactionText = msg
	g.interactionRunes = 0
	g.interactionStart = time.Now()
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func loadWAV(ctx *audio.Context, path string) *audio.Player {
	f, err := Assets.Open(path)
	if err != nil {
		log.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	d, err := wav.Decode(ctx, f)
	if err != nil {
		log.Fatalf("decode %s: %v", path, err)
	}
	p, err := audio.NewPlayer(ctx, d)
	if err != nil {
		log.Fatalf("new player %s: %v", path, err)
	}
	return p
}

func main() {
	// — load tileset & level (now reading from embed.FS) —
	if err := game.LoadTilesetFS(Assets, "assets/tilesets/platformer.png"); err != nil {
		log.Fatal(err)
	}

	lvlFull, err := game.LoadLevelFS(Assets, "levels/test_level.json")
	if err != nil {
		log.Fatal(err)
	}
	game.Levels = []game.Level{lvlFull}

	// SOUND DEPENDENCY (ADD SOUND PATHS HERE)
	audioCtx := audio.NewContext(SampleRate)
	jumpSnd := loadWAV(audioCtx, "assets/sfx/jump.wav")
	deathSnd := loadWAV(audioCtx, "assets/sfx/death.wav")
	stepSnds := map[int]*audio.Player{
		10: loadWAV(audioCtx, "assets/sfx/step_grass.wav"),
		27: loadWAV(audioCtx, "assets/sfx/step_grass.wav"),
		37: loadWAV(audioCtx, "assets/sfx/step_stone.wav"),
		38: loadWAV(audioCtx, "assets/sfx/step_stone.wav"),
		39: loadWAV(audioCtx, "assets/sfx/step_stone.wav"),
		33: loadWAV(audioCtx, "assets/sfx/step_stone.wav"),
		50: loadWAV(audioCtx, "assets/sfx/step_grass.wav"),
	}
	landingSnd := loadWAV(audioCtx, "assets/sfx/land.wav")
	monsterDeathSnd := loadWAV(audioCtx, "assets/sfx/monster_death.wav") // SOUND DEPENDENCY (ADD SOUND HERE)
	pumpkinSnd := loadWAV(audioCtx, "assets/sfx/pumpkin.wav")

	// — background music (from embed.FS) —
	f, err := Assets.Open("assets/music/firepot.wav")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	d, err := wav.Decode(audioCtx, f)
	if err != nil {
		log.Fatal(err)
	}
	loop := audio.NewInfiniteLoop(d, d.Length())
	bgm, err := audio.NewPlayer(audioCtx, loop)
	if err != nil {
		log.Fatal(err)
	}
	bgm.SetVolume(0.01)
	bgm.Play()

	// — start your Ebiten game —
	ebiten.SetWindowSize(WindowWidth, WindowHeight)
	ebiten.SetWindowTitle("Pumpkin Giraffe")
	g := NewGame(jumpSnd, deathSnd, stepSnds, landingSnd, monsterDeathSnd, pumpkinSnd) // SOUND DEPENDENCY (ADD SOUND HERE)
	log.Fatal(ebiten.RunGame(g))
}
