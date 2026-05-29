# Pumpkin Giraffe

A tiny speedrunning platformer about a short-necked giraffe, a fistful of pumpkins, and a clock that never stops ticking. Built in Go with [Ebiten](https://ebitengine.org/).

![Pumpkin Giraffe gameplay](docs/screenshot.png)

## The point of the game

Get to the end as fast as humanly possible.

A timer starts the instant the level loads and runs in the top-centre of the screen down to the millisecond (`00:05.030`). When you reach the goal, the game freezes your final time and throws up a **"You did it! Your time was MM:SS.mmm"** — that number is your run. Beat it. Then beat it again.

Everything in the level is built around shaving frames:

- **Sprint** (hold Shift) multiplies your speed by 2.5×. You'll want it held basically the whole run, but it makes platforms and pits far less forgiving — the classic speedrun trade of control for pace.
- **Pumpkins** (the orange clusters on the ledges) are optional. Grabbing one bumps the `Pumpkins` counter top-right and plays a satisfying *pop*, but it costs you time. Pure speed runs ignore them; 100% runs detour for every last one. Two different races, same level.
- **Skull enemies** patrol the platforms. Touch one and you die and respawn — a death is a run-killer, so they double as risk gates on the fast lines.

So there are really two ways to play: **any%** (touch nothing, just reach the goal in the lowest time) and **100%** (collect every pumpkin *and* keep the time low). The fun is that the fastest route and the greediest route are almost never the same.

## Controls

| Action | Keys |
|---|---|
| Move left / right | `A` / `D` or `←` / `→` |
| Sprint (2.5× speed) | hold `Shift` |
| Jump | `Space` or `↑` |
| Interact (barrels etc.) | `E` |
| Quit | `Esc` |

## Run it

Grab the latest build, or build it yourself — it's a single self-contained binary with every sprite, sound, and the music baked in (no loose asset files to ship).

```bash
git clone https://github.com/TRob9/pumpkin-giraffe.git
cd pumpkin-giraffe
go build -o pumpkin-giraffe .
./pumpkin-giraffe        # Windows: .\pumpkin-giraffe.exe
```

You'll need [Go](https://go.dev/dl/) 1.24 or newer. The first build downloads Ebiten and its dependencies automatically.

## A note on the giraffe

He has a short neck. Yes, on purpose. A giraffe who can't reach the high leaves has to be good at *something* — so he jumps.

## Tech

- **Language:** Go (1.24+)
- **Engine:** [Ebiten](https://ebitengine.org/) v2 — pure-Go 2D game library
- **Assets:** embedded into the binary via Go's `//go:embed`, so the exe runs anywhere with nothing alongside it
- **Resolution:** 1280×720 logical, scaled to the window

---

*A hobby project. Pull requests and faster times both welcome.*
