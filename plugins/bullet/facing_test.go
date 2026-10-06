package bullet_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/bullet"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Heading counts the ways round the circle from east against the clock — the screen's y grows
// down — and wraps whichever way the shot flies.
func TestFlight_Heading_CountsFromEastAgainstTheClock(t *testing.T) {
	for _, tc := range []struct {
		name string
		dir  geom.Vec
		n    int
		want int
	}{
		{"east", geom.NewVec(1, 0), 16, 0},
		{"north (up the screen)", geom.NewVec(0, -1), 16, 4},
		{"west", geom.NewVec(-1, 0), 16, 8},
		{"south (down the screen)", geom.NewVec(0, 1), 16, 12},
		{"north-east", geom.NewVec(1, -1), 8, 1},
		{"a way below east wraps round", geom.NewVec(1, 0.5), 16, 15},
		{"still, before the first step", geom.Vec{}, 16, 0},
	} {
		f := bullet.Flight{Dir: tc.dir}
		if got := f.Heading(tc.n); got != tc.want {
			t.Errorf("%s: Heading(%d) = %d, want %d", tc.name, tc.n, got, tc.want)
		}
	}
}

// Facing declares an ammo's directional twins — Headings hands them out, east first — and only
// then does the ammo count as faced.
func TestShots_Facing_DeclaresTheTwins(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 10},
	})
	shots := bullet.NewShots(w)
	shots.Define("bolt", bullet.Body{Size: 2, Speed: 10, Range: 50})
	if hs := shots.Named("bolt").Headings(); hs != nil {
		t.Fatalf("an ammo without a Facing has headings %v, want none", hs)
	}
	if rule := shots.Facing("bolt", 8); rule == nil {
		t.Fatal("Facing handed back no rule")
	}
	hs := shots.Named("bolt").Headings()
	if len(hs) != 8 {
		t.Fatalf("declared 8 ways, got %d twins", len(hs))
	}
	seen := map[render.SpriteID]bool{shots.Named("bolt").SpriteID(): true}
	for _, id := range hs {
		if seen[id] {
			t.Fatalf("twin %d shares a slot", id)
		}
		seen[id] = true
	}
}
