package main

import (
	"PumpkinGiraffe/game"
	aud "PumpkinGiraffe/game/audio" // aliased as "aud" because the ebitengine also has an "audio" package
	"PumpkinGiraffe/game/loader"

	"PumpkinGiraffe/game/ui"
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
)

//go:embed assets/tilesets/*.png
//go:embed assets/sprites/*.png
//go:embed assets/items/*.png
//go:embed assets/sfx/*.wav
//go:embed assets/music/*.wav
//go:embed assets/fonts/*.ttf
//go:embed levels/*.json
//go:embed levels/*.png
var Assets embed.FS

// THE COMMENTS ABOVE VAR ASSETS ARE BEING UTILISED AS CODE FOR THE EMBED LIBRARY, DO NOT DELETE THEM
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
	uiHelper        *ui.UI
)

func init() {
	// 1) Read the raw font file from our embedded assets.
	//    This pulls in the .ttf data so we can turn it into a usable font.
	data, err := Assets.ReadFile("assets/fonts/PressStart2P-Regular.ttf")
	if err != nil {
		// If we can’t find or load the font file, stop the program and log the error.
		log.Fatal(err)
	}

	// 2) Parse the binary font data into an OpenType font object.
	//    This step understands the TTF format and prepares it for sizing.
	tt, err := opentype.Parse(data)
	if err != nil {
		log.Fatal(err)
	}

	// 3) Create a font “face” for the HUD text.
	//    Here we pick size 14 points (a middle size) so our on-screen messages are easy to read.
	//    DPI (dots per inch) tells Go how to convert points into pixels on screen.
	//    HintingFull smooths out the font shapes for legibility at small sizes.
	hudFont, err = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    14,               // point size of the font
		DPI:     72,               // standard screen resolution
		Hinting: font.HintingFull, // use full hinting for sharper edges
	})
	if err != nil {
		log.Fatal(err)
	}

	// 4) Create a larger font face for buttons.
	//    Buttons need to stand out, so we bump up to 24 points.
	buttonFont, err = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    24,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}

	// 5) Create a smaller font face for brief interaction messages.
	//    These are tiny prompts or tooltips, so we go with 10 points.
	interactionFont, err = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    10,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}

	// 6) Initialize our UI helper object, giving it the small interaction font
	//    and a zoom factor (to scale UI elements up or down as needed).
	uiHelper = ui.NewUI(interactionFont, ZoomFactor)
}

type GameState int

const (
	StateTitle    GameState = iota // main menu before the player starts the game
	StatePlaying                   // game is active and running
	StatePaused                    // game is temporarily halted
	StateFinished                  // game has ended and results are shown
)

type Game struct {
	ui                    *ui.UI                // helper for drawing UI elements
	player                *game.Player          // the controllable player character
	pumpkins              []*game.Pumpkin       // all pumpkins currently in the world
	pumpkinSpawned        bool                  // whether any pumpkin has been spawned yet
	pumpkinMissed         bool                  // whether the player has missed catching a pumpkin that fell from the sky
	jumpSnd, deathSnd     *audio.Player         // sound effects for jump and death
	stepSnds              map[int]*audio.Player // footstep sounds mapped by step index
	landingSnd            *audio.Player         // sound effect when the player lands
	monsterDeathSnd       *audio.Player         // sound effect for monster’s death
	pumpkinSnd            *audio.Player         // sound effect for pumpkin-related events
	buffer                *ebiten.Image         // offscreen image buffer for drawing
	CameraX, CameraY      float64               // camera’s current X and Y offsets
	startTime             time.Time             // timestamp when gameplay began
	endTime               time.Time             // timestamp when gameplay ended
	timerStopped          bool                  // whether the game timer has been stopped
	state                 GameState             // current state: title, playing, paused, or finished
	prevEscape            bool                  // tracks previous Escape key state to toggle pause
	startButton           image.Rectangle       // screen area for the “Start” button
	restartButton         image.Rectangle       // screen area for the “Restart” button
	exitButton            image.Rectangle       // screen area for the “Exit” button
	initialPumpkinDropped bool                  // whether the first pumpkin drop has happened
	prevInteract          bool                  // tracks previous interact key state to debounce presses
	pauseStart            time.Time             // timestamp when the game was paused
	accumulated           time.Duration         // total time spent paused
	pumpkinSystem         *PumpkinSystem        // manager for spawning and updating pumpkins
	nextInteract          time.Time             // earliest time the next interaction is allowed
}

// NewGame constructs and wires up the Game, including the onInteract callback.
func NewGame(
	jumpSnd, deathSnd *audio.Player,
	stepSnds map[int]*audio.Player,
	landingSnd, monsterDeathSnd, pumpkinSnd *audio.Player,
) *Game {
	// calculate how big our offscreen buffer should be (in game units)
	bw := int(float64(WindowWidth) / ZoomFactor)
	bh := int(float64(WindowHeight) / ZoomFactor)

	g := &Game{
		// helper for drawing UI elements (menus, messages, etc.)
		ui: uiHelper,

		// list of all pumpkins in play (starts empty)
		pumpkins: nil,

		// has any pumpkin been spawned yet?
		pumpkinSpawned: false,

		// did the player miss catching a pumpkin?
		pumpkinMissed: false,

		// sound effect when the player jumps
		jumpSnd: jumpSnd,

		// sound effect when the player dies
		deathSnd: deathSnd,

		// footstep sounds, keyed by animation frame or step number
		stepSnds: stepSnds,

		// sound effect when the player lands on the ground
		landingSnd: landingSnd,

		// sound effect when a monster is defeated
		monsterDeathSnd: monsterDeathSnd,

		// sound effect for any pumpkin event (drop, catch, miss)
		pumpkinSnd: pumpkinSnd,

		// offscreen drawing surface we render the game to before scaling
		buffer: ebiten.NewImage(bw, bh),

		// camera horizontal offset in the world
		CameraX: 0,

		// camera vertical offset in the world
		CameraY: 0,

		// time when the game session started
		startTime: time.Now(),

		// time when the game session ended (zero until finished)
		endTime: time.Time{},

		// has the game timer been stopped (after finishing)?
		timerStopped: false,

		// current game state (title screen, playing, paused, or finished)
		state: StateTitle,

		// what the Escape key state was on the previous frame (for toggling pause)
		prevEscape: false,

		// defines the clickable area for the “Start” button
		startButton: image.Rect(btnX, btnY, btnX+btnW, btnY+btnH),

		// defines the clickable area for the “Restart” button (same size as Start)
		restartButton: image.Rect(btnX, btnY, btnX+btnW, btnY+btnH),

		// defines the clickable area for the “Exit” button (below Start/Restart)
		exitButton: image.Rect(btnX, btnY+btnH+20, btnX+btnW, btnY+btnH*2+20),

		// have we dropped the first pumpkin yet?
		initialPumpkinDropped: false,

		// what the interact key state was on the previous frame (to prevent repeats)
		prevInteract: false,

		// timestamp marking when the game was paused
		pauseStart: time.Time{},

		// total time accumulated while the game was paused
		accumulated: 0,

		// system that handles spawning and updating pumpkins
		pumpkinSystem: NewPumpkinSystem(),
	}

	// wire up the player so that whenever it triggers onInteract(message),
	// we call g.ui.NewMessage(message) to display it
	g.player = game.NewPlayer(
		Assets,
		g.ui.NewMessage, // callback for any interaction text
	)

	return g
}

// Update advances the game by one frame. It drives the interaction-text typewriter and clearing logic,
// handles global pause/resume on Escape, and routes clicks on the Title, Paused, and Finished screens.
// When in StatePlaying it will fall through to the main game logic (player, physics, enemies, etc.).
// Returns any error encountered during frame processing.

func (g *Game) Update() error {
	// 1) Pause and resume when Escape is pressed
	//    - Detect the instant the key goes down (edge detect)
	//    - Toggle between Playing and Paused states
	esc := ebiten.IsKeyPressed(ebiten.KeyEscape)
	if esc && !g.prevEscape {
		switch g.state {
		case StatePlaying:
			// record when we paused so we can exclude this time from the timer
			g.state = StatePaused
			g.pauseStart = time.Now()
		case StatePaused:
			// add paused duration to our accumulated pause time, then resume
			g.accumulated += time.Since(g.pauseStart)
			g.state = StatePlaying
		}
	}
	g.prevEscape = esc

	// 2) Advance any on-screen messages (they float/fade over time)
	//    Ebiten calls Update about 60 times per second
	g.ui.Update(time.Second / 60)

	// 3) Handle screens before actual gameplay
	switch g.state {
	case StateTitle:
		// look for a click on the Start button
		mx, my := ebiten.CursorPosition()
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) &&
			mx >= g.startButton.Min.X && mx <= g.startButton.Max.X &&
			my >= g.startButton.Min.Y && my <= g.startButton.Max.Y {

			g.state = StatePlaying
			g.startTime = time.Now() // begin the game timer
		}
		return nil // nothing else to do until we start

	case StatePaused, StateFinished:
		// in Paused or Finished, check Restart and Exit buttons
		mx, my := ebiten.CursorPosition()
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			pt := image.Pt(mx, my)

			if pt.In(g.restartButton) {
				// reset all gameplay flags and timers
				g.prevInteract = false
				g.initialPumpkinDropped = false
				g.pumpkinSpawned = false
				g.pumpkinMissed = false
				g.timerStopped = false
				g.accumulated = 0
				g.pauseStart = time.Time{}

				// restart the clock
				g.startTime = time.Now()
				g.endTime = time.Time{}

				// clear and recreate all pumpkins
				g.pumpkins = nil

				// make a brand-new player instance
				g.player = game.NewPlayer(
					Assets,
					g.ui.NewMessage,
				)

				// reload the level data from JSON
				lvlFull, err := loader.LoadLevelFS(Assets, "levels/test_level.json")
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
		return nil // skip the rest until we resume or restart
	}

	// 4) Main gameplay updates (only when state == StatePlaying)

	// 4a) Update pumpkin physics, spawning, catching, and missing logic
	g.pumpkinSystem.Update(g)

	// 4b) Update the player: movement, gravity, collisions, and animation
	g.player.Update(
		g.jumpSnd,
		g.deathSnd,
		g.stepSnds,
		g.landingSnd,
		g.monsterDeathSnd,
		g.pumpkinSnd,
		g.pumpkinMissed,
	)

	// 4c) Handle interaction key (E) with one-second cooldown
	use := ebiten.IsKeyPressed(ebiten.KeyE)
	now := time.Now()
	if use && !g.prevInteract && now.After(g.nextInteract) && g.player.IsBesideInteractableObject() {
		// make the player show the interact pose for one frame
		g.player.SetInteracting(true)

		// perform the interaction (e.g., drop or catch a pumpkin)
		g.player.TryInteract(game.InteractionContext{
			PumpkinMissed:  g.pumpkinMissed,
			SpawnPumpkin:   g.spawnPumpkinAt,
			InitialDropped: g.initialPumpkinDropped,
			PumpkinSpawned: g.pumpkinSpawned,
		})

		// enforce a one-second wait before the next interaction
		g.nextInteract = now.Add(1 * time.Second)
	} else {
		g.player.SetInteracting(false)
	}
	g.prevInteract = use

	// 4d) Update each enemy and check for collisions with the player
	lvl := game.Levels[game.CurrentLevel]
	alive := 0
	for _, en := range lvl.Enemies {
		en.Update()
		if !en.Alive {
			continue
		}
		alive++

		// if player overlaps enemy...
		pr, er := g.player.Rect(), en.Rect()
		if pr.Overlaps(er) {
			if g.player.CollidesHeadOn(er) {
				// stomped from above: defeat the enemy
				en.Alive = false
				g.monsterDeathSnd.Rewind()
				g.monsterDeathSnd.Play()
				g.player.VelY = -4 // bounce the player upward
			} else {
				// hit from the side: respawn the player
				g.player.Respawn()
			}
		}
	}

	// 4e) After catching 5 pumpkins, stop the timer and finish the game
	if !g.timerStopped && g.player.Pumpkins >= 5 {
		g.endTime = time.Now()
		g.timerStopped = true
		g.state = StateFinished
	}

	// 4f) Smooth camera follow: center on player but clamp to level bounds
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

// Draw renders the entire game screen each frame, choosing what to show
// depending on the current game state (title menu, paused screen, finished
// screen, or active gameplay). It handles UI overlays, buttons, world
// drawing, and the HUD (pumpkin count and timer).
func (g *Game) Draw(screen *ebiten.Image) {
	switch g.state {
	case StateTitle:
		// Title screen: clear to black and show instructions + Start button
		screen.Fill(color.Black)

		// Instruction text lines
		lines := []string{
			"You are a short-necked giraffe and need to collect 5 pumpkins as fast as you can.",
			"Try to be faster than your friends!",
			"Press E to interact with barrels.",
			"Kill monsters by jumping on their heads.",
			"This game rewards glitch abuse and curious minds.",
		}
		y := 100
		for _, l := range lines {
			// Measure text width so we can center it horizontally
			b := text.BoundString(hudFont, l)
			x := (WindowWidth - (b.Max.X - b.Min.X)) / 2
			text.Draw(screen, l, hudFont, x, y, color.White)
			y += 30
		}

		// Draw the START button background
		ebitenutil.DrawRect(screen,
			float64(g.startButton.Min.X), float64(g.startButton.Min.Y),
			float64(g.startButton.Dx()), float64(g.startButton.Dy()),
			color.RGBA{0, 0, 128, 200},
		)
		// Draw the START label centered in the button
		tb := "START"
		tbB := text.BoundString(buttonFont, tb)
		text.Draw(screen, tb, buttonFont,
			g.startButton.Min.X+(g.startButton.Dx()-tbB.Dx())/2,
			g.startButton.Min.Y+btnH/2+8,
			color.White,
		)
		return // nothing else to draw on title screen

	case StatePaused:
		// Paused screen: semi-transparent overlay + "PAUSED" text
		screen.Fill(color.RGBA{0, 0, 0, 180})
		text.Draw(screen, "PAUSED", buttonFont,
			WindowWidth/2-80, WindowHeight/2-100, color.White)

	case StateFinished:
		// Finished screen: show final time
		d := g.endTime.Sub(g.startTime)
		msg := fmt.Sprintf("You did it! Your time was %02d:%02d.%03d",
			int(d/time.Minute),
			int(d/time.Second)%60,
			int(d/time.Millisecond)%1000,
		)
		// Center the message horizontally
		b := text.BoundString(buttonFont, msg)
		x := (WindowWidth - (b.Max.X - b.Min.X)) / 2
		text.Draw(screen, msg, buttonFont, x, WindowHeight/2-60, color.White)
	}

	// Draw Restart and Exit buttons on both Paused and Finished screens
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

	// Only draw gameplay world and HUD if the game is active
	if g.state != StatePlaying {
		return
	}

	// --- Active gameplay drawing ---

	// 1) Clear the offscreen buffer
	g.buffer.Fill(color.Black)

	// 2) Draw the level (tiles, background, etc.) into the buffer
	game.DrawLevel(g.buffer, g.CameraX, g.CameraY)

	// 3) Draw the player sprite at its world position offset by the camera
	g.player.Draw(g.buffer, g.CameraX, g.CameraY)

	// 4) Draw all alive enemies
	lvl := game.Levels[game.CurrentLevel]
	for _, en := range lvl.Enemies {
		en.Draw(g.buffer, g.CameraX, g.CameraY)
	}

	// 5) Draw all alive pumpkins
	for _, p := range g.pumpkins {
		if !p.Alive {
			continue
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(p.X-g.CameraX, p.Y-g.CameraY)
		g.buffer.DrawImage(p.Image, op)
	}

	// 6) Scale the offscreen buffer and render it to the actual screen
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(ZoomFactor, ZoomFactor)
	screen.DrawImage(g.buffer, op)

	// 7) Draw any floating UI messages (e.g., interaction prompts)
	g.ui.Draw(
		screen,
		g.CameraX, g.CameraY, // camera offset for correct positioning
		g.player.X, g.player.Y, // player world position
		g.player.Height, // sprite height (for offset)
	)

	// 8) Draw HUD: pumpkin count in top-right
	text.Draw(screen, fmt.Sprintf("Pumpkins: %d", g.player.Pumpkins),
		hudFont, WindowWidth-200, 24, color.White,
	)

	// 9) Compute and draw the timer in the top-center
	var d time.Duration
	switch {
	case g.timerStopped:
		// game finished
		d = g.endTime.Sub(g.startTime) - g.accumulated
	case g.state == StatePaused:
		// paused but still on gameplay screen
		d = g.pauseStart.Sub(g.startTime) - g.accumulated
	default:
		// actively playing
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

// Layout tells Ebiten how large the game’s logical screen should be.
// Regardless of the actual window or display size, we render at WindowWidth×WindowHeight.
func (g *Game) Layout(w, h int) (int, int) {
	return WindowWidth, WindowHeight
}

// spawnPumpkinAt creates a new pumpkin just above the given world coordinates,
// adds it to our active pumpkin list, and records that at least one pumpkin is now flying.
func (g *Game) spawnPumpkinAt(px, py float64) {
	// place the pumpkin one tile above the specified point
	p := game.NewPumpkin(px, py-float64(game.TileSize))

	// add this pumpkin to our slice so Update/Draw will handle it
	g.pumpkins = append(g.pumpkins, p)

	// note that a pumpkin has been spawned (used for interaction logic)
	g.pumpkinSpawned = true
}

// clamp restricts a floating-point value v to remain within [lo, hi].
// If v is below lo, it returns lo; if v is above hi, it returns hi; otherwise, it returns v unchanged.
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// main is the entry point for Pumpkin Giraffe. It sets up graphics, audio, game data,
// and then starts the Ebiten game loop.
func main() {
	// ——— 1) Load graphics assets ———
	// Load the tile images from our embedded filesystem so we can draw the level.
	if err := game.LoadTilesetFS(Assets, "assets/tilesets/platformer.png"); err != nil {
		log.Fatal(err)
	}
	// Load any extra images used for interactive objects (e.g., barrels, pumpkins).
	game.LoadInteractableAssets(Assets)
	// Read the level layout from JSON so we know where walls, floors, and enemies go.
	lvlFull, err := loader.LoadLevelFS(Assets, "levels/test_level.json")
	if err != nil {
		log.Fatal(err)
	}
	// Store our single level in the global Levels slice.
	game.Levels = []game.Level{lvlFull}

	// ——— 2) Prepare sound effects ———
	// Create an audio context at our desired sample rate.
	audioCtx := audio.NewContext(SampleRate)
	// Load the jump and death sounds from embedded WAV files.
	jumpSnd := aud.LoadWAV(Assets, "assets/sfx/jump.wav")
	deathSnd := aud.LoadWAV(Assets, "assets/sfx/death.wav")
	// Footstep sounds keyed by which animation frame or tile type to play.
	stepSnds := map[int]*audio.Player{
		10: aud.LoadWAV(Assets, "assets/sfx/step_grass.wav"),
		27: aud.LoadWAV(Assets, "assets/sfx/step_grass.wav"),
		37: aud.LoadWAV(Assets, "assets/sfx/step_stone.wav"),
		38: aud.LoadWAV(Assets, "assets/sfx/step_stone.wav"),
		39: aud.LoadWAV(Assets, "assets/sfx/step_stone.wav"),
		33: aud.LoadWAV(Assets, "assets/sfx/step_stone.wav"),
		50: aud.LoadWAV(Assets, "assets/sfx/step_grass.wav"),
	}
	// Landing, monster-death, and pumpkin-related sound effects.
	landingSnd := aud.LoadWAV(Assets, "assets/sfx/land.wav")
	monsterDeathSnd := aud.LoadWAV(Assets, "assets/sfx/monster_death.wav")
	pumpkinSnd := aud.LoadWAV(Assets, "assets/sfx/pumpkin.wav")

	// ——— 3) Start background music on loop ———
	// Open the music file from embedded assets.
	f, err := Assets.Open("assets/music/firepot.wav")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	// Decode the WAV into an audio stream.
	d, err := wav.Decode(audioCtx, f)
	if err != nil {
		log.Fatal(err)
	}
	// Wrap it in an infinite loop so it never stops playing.
	loop := audio.NewInfiniteLoop(d, d.Length())
	bgm, err := audio.NewPlayer(audioCtx, loop)
	if err != nil {
		log.Fatal(err)
	}
	bgm.SetVolume(0.01) // turn it down so it's just a soft background track
	bgm.Play()

	// ——— 4) Configure the game window and initialize Game ———
	ebiten.SetWindowSize(WindowWidth, WindowHeight) // logical window size
	ebiten.SetWindowTitle("Pumpkin Giraffe")        // window title bar
	// Create our Game object, passing in all the sound effects.
	g := NewGame(jumpSnd, deathSnd, stepSnds, landingSnd, monsterDeathSnd, pumpkinSnd)
	g.ui = uiHelper // attach our UI helper so it can render messages

	// ——— 5) Run the game loop ———
	// This call will repeatedly call g.Update and g.Draw until the window closes.
	log.Fatal(ebiten.RunGame(g))
}
