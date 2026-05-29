// story.go — the StoryManager: the one object the main loop talks to.
//
// It knows three things: which NPCs stand where on each level, what each of
// them says, and how an open conversation behaves. The main loop only needs to
// call NewStoryManager once, then Update / DrawWorld / DrawUI each frame and
// check Active() to decide whether to freeze player input while talking.
//
// IMPORTANT: this file (and npc.go, dialogue.go) are the ONLY new files in the
// game package. Nothing here edits player.go, level.go, or main.go — the
// integration is a handful of call-sites the integrator adds.
package game

import (
	"io/fs"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font"
)

// talkReach is how close (in pixels, centre-to-centre) the giraffe must be to
// an NPC before pressing the talk key opens a conversation. A bit forgiving so
// you don't have to land on the exact pixel — covers standing beside them.
const talkReach = 34

// StoryManager owns all narrative state: the placed NPCs per level, the loaded
// sprites, the fonts, and the single conversation that may be open at a time.
type StoryManager struct {
	// sprites, loaded once, keyed by character.
	sprites map[who]*ebiten.Image

	// npcsByLevel[levelIndex] is the slice of characters standing on that level.
	npcsByLevel map[int][]*npc

	// fonts: bodyFace for the dialogue body + name, smallFace for the tiny
	// in-world "press E" prompt. Both are supplied by the caller.
	bodyFace  font.Face
	smallFace font.Face

	// the single live conversation. Only one NPC talks at a time.
	dlg dialogue

	// nearest tracks, per Update, which NPC (if any) the player is in range of,
	// so DrawWorld can highlight it and show the prompt without recomputing.
	nearLevel int
	nearIdx   int // index into npcsByLevel[nearLevel], or -1 if none

	// t is a steadily climbing time accumulator (seconds) driving the sprite
	// bob and the floating "!" wobble. Advanced once per Update.
	t float64

	// prevTalk debounces the talk key so holding E doesn't machine-gun through
	// every line in one frame — we only act on the rising edge (press, not hold).
	// The main loop passes the *current* key state; we remember the last one.
	prevTalk bool
}

// NewStoryManager loads the three NPC sprites from the embedded filesystem and
// stores the fonts the dialogue box will draw with. `face` is the body/name
// font (the caller can pass the same face it uses elsewhere); the in-world
// "press E" prompt reuses it too, since it draws into the small world buffer.
func NewStoryManager(fsys fs.FS, face font.Face) *StoryManager {
	m := &StoryManager{
		sprites: map[who]*ebiten.Image{
			whoElder:    loadNPCSprite(fsys, "assets/sprites/npc_elder.png"),
			whoCritter:  loadNPCSprite(fsys, "assets/sprites/npc_critter.png"),
			whoWanderer: loadNPCSprite(fsys, "assets/sprites/npc_wanderer.png"),
		},
		bodyFace:  face,
		smallFace: face,
		nearIdx:   -1,
	}
	m.buildPlacements()
	return m
}

// SetSmallFace lets the caller supply a smaller font for the in-world "press E"
// hint, if it has one (e.g. the game's 10pt interactionFont). Optional — if
// never called, the hint just uses the body font.
func (m *StoryManager) SetSmallFace(face font.Face) {
	m.smallFace = face
}

// groundTop is the y of an NPC's top-left when standing on the main ground
// surface: the ground tiles sit at world y ≈ 38*16 = 608, and a 16px-tall
// sprite's top is one tile above that.
const groundTop = 38*16 - 16 // = 592

// buildPlacements hardcodes which characters stand where on each of the four
// levels, in world pixels. Positions are chosen in the early/mid stretch of
// each map so the giraffe meets them naturally on the way through, and all sit
// on the main ground line. Levels in play order:
//
//	0 — Pumpkin Patch  (night)   1 — Rolling Start (day meadow)
//	2 — Gap Gauntlet   (dusk)    3 — Skyline Sprint (cave)
func (m *StoryManager) buildPlacements() {
	m.npcsByLevel = map[int][]*npc{
		// L0 Pumpkin Patch: the Elder greets you at the very start with the
		// core mystery and the neck hint; the Fox is a little further along
		// with the practical how-to-play tips.
		0: {
			m.place(whoElder, 150, groundTop, 0.0),
			m.place(whoCritter, 430, groundTop, 1.3),
		},
		// L1 Rolling Start: the Fox returns, looser and more confident in the
		// daylight; the Wanderer appears for the first time, quiet and uneasy.
		1: {
			m.place(whoCritter, 200, groundTop, 0.6),
			m.place(whoWanderer, 520, groundTop, 2.1),
		},
		// L2 Gap Gauntlet (dusk): the Fox is rattled now; the Wanderer's dread
		// sharpens into a warning about what waits below.
		2: {
			m.place(whoCritter, 180, groundTop, 0.9),
			m.place(whoWanderer, 470, groundTop, 1.7),
		},
		// L3 Skyline Sprint (cave): the Wanderer waits near the threshold; the
		// Elder is here too — somehow ahead of you — to close the loop.
		3: {
			m.place(whoWanderer, 160, groundTop, 0.4),
			m.place(whoElder, 440, groundTop, 1.1),
		},
	}
	// Settle each NPC onto the actual ground of its level, so they stand on the
	// real surface even on maps (like the original level 0) whose ground isn't
	// at the generated levels' standard row.
	for lvl, npcs := range m.npcsByLevel {
		for _, n := range npcs {
			n.y = settleToGround(lvl, n.x, n.y)
		}
	}
}

// settleToGround returns the top-left y at which an NPC at column x stands on
// the LOWEST solid surface in its column — i.e. the main ground floor, skipping
// any incidental platform overhead. This puts NPCs on the ground on the
// generated maps (row 38) and on the higher ground of the original level 0
// alike, rather than perched on a torii beam that happens to be above them.
func settleToGround(level int, x, startY float64) float64 {
	if level < 0 || level >= len(Levels) {
		return startY
	}
	tiles := Levels[level].Tiles
	col := int((x + 8) / float64(TileSize))
	for r := len(tiles) - 1; r >= 0; r-- {
		if col >= 0 && col < len(tiles[r]) && SolidTileID(tiles[r][col]) {
			return float64(r*TileSize - TileSize) // stand on top of that tile
		}
	}
	return startY
}

// place is a small constructor that pairs a character with its sprite at a
// world position and a bob phase.
func (m *StoryManager) place(id who, x, y, phase float64) *npc {
	return &npc{id: id, x: x, y: y, sprite: m.sprites[id], phase: phase}
}

// NPCsForLevel returns the characters standing on the given level (read-only
// view). Exposed so the integrator can, for example, count or inspect them; the
// drawing and talking are handled by DrawWorld / Update.
func (m *StoryManager) NPCsForLevel(level int) []*npc {
	return m.npcsByLevel[level]
}

// Update advances the open conversation and handles talk-key presses.
//
//   - It always ticks the typewriter so an open line keeps revealing.
//   - On the *rising edge* of talkPressed (press, not hold) it either advances
//     the open conversation, or — if none is open — opens the conversation of
//     the nearest in-range NPC.
//   - It records which NPC is in range so DrawWorld can highlight it.
//
// `px,py` is the player's world top-left. `talkPressed` is the current state of
// the talk key (e.g. ebiten.IsKeyPressed(ebiten.KeyE)); the manager handles the
// edge-detection itself, so the caller can pass the raw held state.
func (m *StoryManager) Update(level int, px, py float64, talkPressed bool) {
	m.t += 1.0 / 60.0 // assume Ebiten's fixed 60 TPS for the bob clock

	// keep the typewriter moving on whatever line is open
	m.dlg.tick()

	// find the nearest in-range NPC on this level (for highlight + open).
	m.nearLevel = level
	m.nearIdx = -1
	var bestD2 = math.MaxFloat64
	for i, n := range m.npcsByLevel[level] {
		if !n.near(px, py, talkReach) {
			continue
		}
		ncx, ncy := n.x+8, n.y+8
		pcx, pcy := px+spriteW/2, py+spriteH/2
		d2 := (ncx-pcx)*(ncx-pcx) + (ncy-pcy)*(ncy-pcy)
		if d2 < bestD2 {
			bestD2 = d2
			m.nearIdx = i
		}
	}

	// act only on the rising edge of the talk key
	justPressed := talkPressed && !m.prevTalk
	m.prevTalk = talkPressed
	if !justPressed {
		return
	}

	if m.dlg.open {
		m.dlg.advance() // step the conversation we're already in
		return
	}
	// not talking: open the nearest in-range NPC's conversation, if any
	if m.nearIdx >= 0 {
		n := m.npcsByLevel[level][m.nearIdx]
		m.dlg.start(n.id, linesFor(n.id, level))
	}
}

// DrawWorld draws the level's NPCs into the world buffer (the 640x360 surface
// that the main loop later scales by ZoomFactor). It applies the camera offset,
// adds the bob and floating "!", and — for the NPC currently in talk range —
// brightens the marker and shows the "press E" hint.
func (m *StoryManager) DrawWorld(screen *ebiten.Image, level int, camX, camY float64) {
	for i, n := range m.npcsByLevel[level] {
		highlight := level == m.nearLevel && i == m.nearIdx
		n.drawWorld(screen, camX, camY, m.t, highlight)
		// only show the prompt when in range AND not already mid-conversation
		if highlight && !m.dlg.open {
			n.drawPrompt(screen, camX, camY, m.smallFace)
		}
	}
}

// DrawUI draws the dialogue box at full screen scale (1280x720) when a
// conversation is open. Call this AFTER the world buffer has been scaled and
// blitted to the screen, so the box sits cleanly on top of everything.
func (m *StoryManager) DrawUI(screen *ebiten.Image) {
	w, h := screen.Bounds().Dx(), screen.Bounds().Dy()
	m.dlg.draw(screen, w, h, m.bodyFace, m.bodyFace)
}

// Active reports whether a dialogue box is currently open. The main loop checks
// this to pause player movement/jumping while the giraffe is talking, so input
// goes to advancing the conversation instead of walking off mid-sentence.
func (m *StoryManager) Active() bool {
	return m.dlg.open
}

// linesFor returns the script for a character on a specific level. An NPC can
// appear on several levels with evolving lines; this is the single place the
// writing lives. Keep lines short — roughly 40–60 characters so they sit on one
// row of the box without wrapping.
func linesFor(id who, level int) []string {
	switch id {

	// ---- Elder Giraffe: cryptic, kind, a mentor. Speaks in gentle riddles,
	// and is the one who frames the neck-extend ability as a matter of will
	// rather than anatomy. Bookends the journey (first level and last). ----
	case whoElder:
		switch level {
		case 0:
			return []string{
				"Ah. A young neck, and an old worry to match it.",
				"The patch is restless tonight. The dead do not wake idly.",
				"You will need pumpkins for the old gates. Gather them.",
				"And when the way runs out — reach. A neck is not measured in length.",
			}
		case 3:
			return []string{
				"You came further than your shadow thought you would.",
				"I went on ahead. Someone had to leave a lamp lit down here.",
				"The skeletons did not wake on their own. Something called them.",
				"Reach once more, little one. The end is only the dark before a door.",
			}
		}

	// ---- Fox: comic relief but earnest. Practical, fast-talking, gives the
	// player the actual mechanics (gates eat pumpkins, skeletons chase, you can
	// slash them). Gets steadily more spooked across the levels. ----
	case whoCritter:
		switch level {
		case 0:
			return []string{
				"Oh good, company! I was starting to talk to the gourds.",
				"Quick version: the gate at the end eats pumpkins. Bring five.",
				"Those rattly fellows? Get close and they chase. Charming.",
				"But you've got a sword. Time the swing and they go clatter.",
			}
		case 1:
			return []string{
				"Daylight! See, the meadow's not so bad with the sun up.",
				"Same rules: five pumpkins, the gate opens, off you trot.",
				"You're getting the hang of the swing. I almost didn't flinch.",
			}
		case 2:
			return []string{
				"Okay, the gaps out here are a bit much, I'll be honest.",
				"More bones than before, too. They're coming up from somewhere.",
				"Grab your pumpkins, mind the drop, and don't look down. Like me.",
			}
		}

	// ---- Hooded Traveler: melancholy, mysterious, foreshadows the cave and
	// the ending. Speaks softly and knows more than it says. ----
	case whoWanderer:
		switch level {
		case 1:
			return []string{
				"You walk toward the noise. Most things walk away from it.",
				"I have seen the cave at the road's end. I do not recommend it.",
				"Still. If anyone should go down there, perhaps it is the small.",
			}
		case 2:
			return []string{
				"Dusk already. The light leaves quicker the closer you get.",
				"Below us, the patch's roots run deep — and something nests there.",
				"It is not the skeletons you should fear. It is what frightened them.",
			}
		case 3:
			return []string{
				"So. The threshold. I could go no further than this.",
				"Whatever woke the dead is just ahead, in the dark you're carrying.",
				"Go gently, short one. Long necks see far. Short ones see clearly.",
			}
		}
	}

	// Fallback: an NPC placed on a level we forgot to write for. Better a quiet
	// line than a crash or an empty box.
	return []string{"...", "Safe travels, friend."}
}
