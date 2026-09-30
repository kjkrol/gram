package main

import (
	"testing"

	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/players/owner"
)

// The demo sets itself up without a window: its plugins install, its keys bind without a clash,
// its giants spawn within the world's sizes.
func TestDemo_StartsWithoutAWindow(t *testing.T) {
	d := NewDemo()
	if err := engine.NewEngine(d).Init(); err != nil {
		t.Fatal(err)
	}
	if d.stage.world == nil || d.stage.topography == nil {
		t.Fatal("the demo's world or topography was not made")
	}
}

// The island has two sides: the player at the keyboard and a rival without one, each with walkers
// of its own kind.
func TestDemo_ThePlayerAndARivalOwnWalkersOfTheirOwn(t *testing.T) {
	d := NewDemo()
	if err := engine.NewEngine(d).Init(); err != nil {
		t.Fatal(err)
	}
	s := d.stage
	if s.player.Owner() != owner.Of(1) || s.rival.Owner() != owner.Of(2) {
		t.Errorf("the player owns by %v, the rival by %v; want players 1 and 2", s.player.Owner(), s.rival.Owner())
	}
	if s.rivals.SpriteID() == s.unit.SpriteID() {
		t.Error("the rival's walkers are drawn as the player's")
	}
}
