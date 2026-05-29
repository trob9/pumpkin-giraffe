package game

import (
	"image"
	"testing"
)

// newTestPlayer builds a Player with just the fields the logic tests need,
// bypassing sprite loading (which requires the embedded asset FS).
func newTestPlayer() *Player {
	return &Player{X: 100, Y: 100, Width: spriteW, Height: spriteH, Health: MaxHealth, facingRight: true}
}

func TestHurtAndInvuln(t *testing.T) {
	p := newTestPlayer()
	if dead := p.Hurt(1); dead || p.Health != MaxHealth-1 {
		t.Fatalf("first hit: dead=%v health=%d (want false, %d)", dead, p.Health, MaxHealth-1)
	}
	if !p.Invulnerable() {
		t.Fatal("should be invulnerable right after a hit")
	}
	// A second hit while invulnerable is a no-op.
	if dead := p.Hurt(1); dead || p.Health != MaxHealth-1 {
		t.Fatalf("hit during invuln changed health to %d", p.Health)
	}
	// Clear invuln and drain the rest.
	p.invuln = 0
	p.Hurt(1)
	p.invuln = 0
	if dead := p.Hurt(1); !dead || p.Health > 0 {
		t.Fatalf("final hit: dead=%v health=%d (want true, <=0)", dead, p.Health)
	}
}

func TestLoseHeartIgnoresInvuln(t *testing.T) {
	p := newTestPlayer()
	p.invuln = 999 // even fully invulnerable...
	if p.LoseHeart() || p.Health != MaxHealth-1 {
		t.Fatalf("LoseHeart should drain a heart regardless of invuln (health=%d)", p.Health)
	}
}

func TestAttackHitboxTiming(t *testing.T) {
	p := newTestPlayer()
	if _, ok := p.AttackHitbox(); ok {
		t.Fatal("no hitbox when not attacking")
	}
	// Simulate the active window of a swing.
	p.attackTimer = attackDuration - attackActiveLo - 1 // elapsed just past activeLo
	hb, ok := p.AttackHitbox()
	if !ok {
		t.Fatal("expected an active hitbox mid-swing")
	}
	if hb.Min.X < int(p.X) { // facing right → hitbox in front (to the right)
		t.Fatalf("right-facing hitbox should be to the right, got %v", hb)
	}
	// Before the active window opens, no hitbox.
	p.attackTimer = attackDuration - 1 // elapsed = 1 < attackActiveLo
	if _, ok := p.AttackHitbox(); ok {
		t.Fatal("hitbox should be inactive in the wind-up")
	}
	// Left-facing hitbox is to the left.
	p.facingRight = false
	p.attackTimer = attackDuration - attackActiveLo - 1
	hb, _ = p.AttackHitbox()
	if hb.Max.X > int(p.X)+1 {
		t.Fatalf("left-facing hitbox should be to the left, got %v", hb)
	}
}

func TestEnemyChaseToggles(t *testing.T) {
	e := newEnemy(200, 116, KindChaser, nil, nil)
	e.Alive = true
	// far away → stays on patrol (not alerted)
	e.Update(900, 116)
	if e.alerted {
		t.Fatal("enemy should not alert to a distant player")
	}
	// player within alert range and similar height → becomes alerted and moves toward
	startX := e.X
	e.Update(e.X-30, e.Y) // player just to the left, in range
	if !e.alerted {
		t.Fatal("enemy should alert to a nearby player")
	}
	if e.X >= startX {
		t.Fatalf("alerted enemy should move toward the player (left); x went %v -> %v", startX, e.X)
	}
	// player runs far away → de-aggros
	for i := 0; i < 5; i++ {
		e.Update(1200, e.Y)
	}
	if e.alerted {
		t.Fatal("enemy should give up the chase when the player is far")
	}
}

func TestGateToll(t *testing.T) {
	g := NewGate(64, 64, 32, 48, 4)
	if g.Required != 4 {
		t.Fatalf("gate required = %d, want 4", g.Required)
	}
	want := image.Rect(64, 64, 96, 112)
	if g.Rect() != want {
		t.Fatalf("gate rect = %v, want %v", g.Rect(), want)
	}
}
