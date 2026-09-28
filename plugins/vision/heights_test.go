package vision_test

import (
	"math"
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
)

func z(altitude, height float64) *world.Z { return &world.Z{Altitude: altitude, Height: height} }

// eyed is an eastward look from eye above its bottom.
func eyed(eye float64) *look {
	return &look{Sight: vision.Sight{Facing: geom.NewVec(1.0, 0.0), Radius: 300}, Eye: world.Eye{Height: eye, Angle: math.Pi / 4}}
}

// A wall 10 tall 100 ahead and a walker 240 ahead: a walker looking from 1.5 sees the wall alone,
// a hawk 40 up sees over it.
func TestHeights_AWalkerIsStoppedByTheWallAHawkLooksOver(t *testing.T) {
	quasi := &relief{}
	wallAndWalker := []spawn{
		{x: 100, y: 0, size: 60, z: z(0, 10)},
		{x: 240, y: 0, z: z(0, 2)},
	}
	_, seen, _ := sceneIn(t, quasi, append([]spawn{{x: 0, y: 0, z: z(0, 2), sight: eyed(1.5)}}, wallAndWalker...)...)
	if seen[0].Count != 1 || seen[0].Dists[0] > 100 {
		t.Errorf("the walker saw %d at %v, want the wall alone", seen[0].Count, seen[0].Dists[:seen[0].Count])
	}
	_, seen, _ = sceneIn(t, quasi, append([]spawn{{x: 0, y: 0, z: z(40, 2), sight: eyed(1)}}, wallAndWalker...)...)
	if seen[0].Count != 2 {
		t.Errorf("the hawk saw %d, want the wall and the walker behind it", seen[0].Count)
	}
}

// plateau is ground 12 high for x in [50, 100), 0 elsewhere.
type plateau struct{}

func (plateau) At(p geom.Vec) float64 {
	if p.X >= 50 && p.X < 100 {
		return 12
	}
	return 0
}
func (plateau) Step() float64 { return 10 }

func TestHeights_AHillHidesTheLowlandFromAWalkerAndNotFromAHawk(t *testing.T) {
	onHill := &relief{ground: plateau{}, step: 10}
	target := spawn{x: 240, y: 0, z: z(0, 2)}
	_, seen, outlines := sceneIn(t, onHill, spawn{x: 0, y: 0, z: z(0, 2), sight: eyed(1.5), outline: true}, target)
	if seen[0].Count != 0 {
		t.Errorf("the walker saw %d past the hill, want nothing", seen[0].Count)
	}
	if b := outlines[0].Shadows[outlines[0].Count/2][0]; b.From > 100 || b.To < 240 {
		t.Errorf("the walker's shadow ahead is %+v, want the ground from the hill past the target out of sight", b)
	}
	_, seen, _ = sceneIn(t, onHill, spawn{x: 0, y: 0, z: z(40, 2), sight: eyed(1)}, target)
	if seen[0].Count != 1 {
		t.Errorf("the hawk saw %d past the hill, want the target", seen[0].Count)
	}
}

func TestHeights_AFlatWorldRefusesAnEyeAndAWorldWithHeightsRefusesBlockers(t *testing.T) {
	expect := func(t *testing.T, want string, run func()) {
		t.Helper()
		defer func() {
			if msg, _ := recover().(string); !strings.Contains(msg, want) {
				t.Errorf("panic %q, want one mentioning %q", msg, want)
			}
		}()
		run()
		t.Errorf("no panic, want one mentioning %q", want)
	}
	expect(t, "Eye.Height", func() { scene(t, spawn{x: 0, y: 0, sight: eyed(1.5)}) })
	blocked := eastward(math.Pi/8, 300)
	blocked.Blockers = 1
	expect(t, "Sight.Blockers", func() { sceneIn(t, &relief{}, spawn{x: 0, y: 0, sight: blocked}) })
}

// A walker on the plateau, 20 short of its edge, looking east: in a world with heights the view
// reaches its full radius. The lowland just past the edge is below the line from its eye, a
// shadow; farther on the line has dropped below the lowland and it is seen again.
func TestHeights_AViewReachesItsRadiusWithTheGroundOutOfSightAsShadows(t *testing.T) {
	onPlateau := &relief{ground: plateau{}, step: 10}
	_, _, outlines := sceneIn(t, onPlateau, spawn{x: 70, y: 0, z: z(12, 2), sight: eyed(1.5), outline: true})
	o := outlines[0]
	mid := int(o.Count) / 2
	for i := range int(o.Count) {
		if o.Depths[i] != 300 {
			t.Fatalf("depth %d = %v, want the full radius 300 at every angle", i, o.Depths[i])
		}
	}
	b := o.Shadows[mid][0]
	if b == (vision.Band{}) || b.From < 20 || b.From > 45 || b.To < 150 || b.To > 200 {
		t.Errorf("shadow straight ahead = %+v, want one from just past the edge 25 away to where the lowland is seen again, about 180", b)
	}
	if o.Shadows[mid][1] != (vision.Band{}) {
		t.Errorf("a second shadow %+v straight ahead, want the lowland seen to the radius", o.Shadows[mid][1])
	}
}

func TestHeights_AFlatWorldDrawsNoShadows(t *testing.T) {
	_, _, outlines := scene(t,
		spawn{x: 0, y: 0, sight: eastward(math.Pi/8, 300), outline: true},
		spawn{x: 100, y: 0, size: 60}, // a wall: the reach stops at it, as before
	)
	o := outlines[0]
	for i := range int(o.Count) {
		if o.Shadows[i][0] != (vision.Band{}) {
			t.Fatalf("a flat world's outline has shadow %+v at angle %d, want none", o.Shadows[i][0], i)
		}
	}
	if mid := o.Depths[o.Count/2]; mid > 100 {
		t.Errorf("reach straight ahead %v, want it cut at the wall", mid)
	}
}

// On a world with a scale the ground sinks under an eye's level as far off as it lies: a walker
// 240 off on level ground is out of sight past a walker's horizon — at 4 km a world unit, about 74
// off for an eye 1.5 up — and in sight of a hawk 40 up, and of anyone on a flat world.
func TestHeights_WhatLiesPastTheHorizonIsOutOfSight(t *testing.T) {
	target := spawn{x: 240, y: 0, z: z(0, 2)}
	walker := spawn{x: 0, y: 0, z: z(0, 2), sight: eyed(1.5)}
	if _, seen, _ := sceneIn(t, &relief{}, walker, target); seen[0].Count != 1 {
		t.Fatalf("on a flat world the walker saw %d, want the target", seen[0].Count)
	}
	earth := &relief{scale: world.Scale{Metres: 4000}}
	if _, seen, _ := sceneIn(t, earth, walker, target); seen[0].Count != 0 {
		t.Errorf("the walker saw %d past its horizon, want nothing", seen[0].Count)
	}
	hawk := spawn{x: 0, y: 0, z: z(40, 2), sight: eyed(1)}
	if _, seen, _ := sceneIn(t, earth, hawk, target); seen[0].Count != 1 {
		t.Errorf("the hawk 40 up saw %d, want the target within its horizon", seen[0].Count)
	}
}
