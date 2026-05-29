# Pumpkin Giraffe

A short-necked giraffe platformer with a little soul to it. Gather pumpkins to
pay the old portals' tolls, cut down the bonefolk that woke in the patch, and
journey from a starlit night, through a meadow morning and a burning dusk, down
into the cave at the end of it all. Built in Go with [Ebiten](https://ebitengine.org/).

![Pumpkin Giraffe gameplay](docs/screenshot.png)

## The loop

Each field is a level. Gather the pumpkins scattered across it, then reach the
glowing **gate** at the far end — it opens once you're carrying enough pumpkins
to pay its toll, and whisks you to the next field (healing you to full hearts on
the way). Clear all the fields against a running speedrun clock.

You have **three hearts**. A skeleton's touch from the side costs one and knocks
you back (with a moment of invulnerability); fall off the world and it costs one
too. Lose all three and the field resets — but the clock keeps ticking.

## What's out there

- **Skeletons.** Patrollers pace their stretch; **chasers** (the rusted,
  red-eyed ones) hunt you down if you stray into their sight, and give up if you
  get far enough away. Stomp one from above to pop it and bounce, or **time a
  sword slash** to cut it down. Mistime the swing and you'll eat a hit.
- **The gate.** A portal at each field's end. It glows warm when you can afford
  it, cold when you can't. Pumpkins are the key.
- **Moving platforms, boulders, torii.** Ride the platforms (they're solid both
  ways — mind your head), push a boulder under an out-of-reach ledge to make a
  step, and climb the torii gates that span the gaps.
- **NPCs.** An old long-necked Elder, an earnest Fox, and a hooded Wanderer
  stand along the way. Walk up and press your interact key to talk — the Elder
  carries the lore, the Fox the tips, the Wanderer the dread.

## Tricks

- **Sprint** — hold to move at 2.5×.
- **Dash** — double-tap a direction for a quick burst (short cooldown).
- **Double-jump chain** — land on a ledge near the peak of a jump and jump again
  for a boosted launch.
- **Extend the neck** (the giraffe's party trick) — hold the neck key and the
  neck grows upward; hook the head over a ledge above you, release, and it hoists
  your whole body up. A short neck, it turns out, has its uses.

## Controls

All controls are **rebindable** in **Settings**. Defaults:

| Action | Key |
|---|---|
| Move | `A` / `D` (or arrow keys) |
| Jump | `Space` (or `↑`) |
| Sprint | hold `Shift` |
| Sword slash | `F` |
| Extend neck | `Q` |
| Interact / push boulder / talk | `E` |
| Dash | double-tap a direction |
| Pause | `Esc` · Main menu (from pause) | `M` |

The main menu (keyboard: `↑`/`↓` + `Enter`) has **Play**, **How to Play**,
**Lore**, **Settings** (rebind keys), and **Quit**.

## Run it

A single self-contained binary — every sprite, sound, background and the music
are baked in, so it runs anywhere with nothing alongside it.

```bash
git clone https://github.com/TRob9/pumpkin-giraffe.git
cd pumpkin-giraffe
go build -o pumpkin-giraffe .
./pumpkin-giraffe        # Windows: .\pumpkin-giraffe.exe
```

You'll need [Go](https://go.dev/dl/) 1.24 or newer; the first build pulls Ebiten
automatically.

## Tech

- **Language:** Go (1.24+)
- **Engine:** [Ebiten](https://ebitengine.org/) v2 — pure-Go 2D
- **Assets:** embedded via `//go:embed`. Sprites, backgrounds, hearts, the slash,
  the enemy variants and NPCs are procedurally painted by `tools/art`; the four
  levels are authored by `tools/genlevels`, which also runs an arc-based
  reachability solver so no pumpkin is ever placed out of reach.
- **Resolution:** 1280×720, rendered at a 2× pixel scale.

---

*A hobby project. Pull requests and faster times both welcome.*
