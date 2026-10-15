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

func eyed(eye float64) *look {
	return &look{Sight: vision.Sight{Facing: geom.NewVec(1.0, 0.0), Radius: 300}, Eye: world.Eye{Height: eye, Angle: math.Pi / 4}}
}

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

func TestHeights_AFlatWorldRefusesAnEye(t *testing.T) {
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
	narrow := eastward(math.Pi/8, 300)
	narrow.Eye.Vertical = 1
	expect(t, "Eye.Vertical", func() { scene(t, spawn{x: 0, y: 0, sight: narrow}) })
}

func TestHeights_BlockersLookOverAWallOnAnotherLayer(t *testing.T) {
	for name, tc := range map[string]struct {
		blockers world.Layers
		want     uint8
	}{
		"blockers zero, the wall cuts":     {want: 1},
		"the wall on a blocking layer":     {blockers: 1, want: 1},
		"the wall on another layer is not": {blockers: 2, want: 2},
	} {
		t.Run(name, func(t *testing.T) {
			walker := eyed(1.5)
			walker.Blockers = tc.blockers
			_, seen, _ := sceneIn(t, &relief{},
				spawn{x: 0, y: 0, z: z(0, 2), sight: walker},
				spawn{x: 100, y: 0, size: 60, z: z(0, 10), layers: 1},
				spawn{x: 240, y: 0, z: z(0, 2)},
			)
			if seen[0].Count != tc.want || seen[0].Dists[0] > 100 {
				t.Errorf("saw %d at %v, want %d with the wall first", seen[0].Count,
					seen[0].Dists[:seen[0].Count], tc.want)
			}
		})
	}
}

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
		spawn{x: 100, y: 0, size: 60}, // a wall
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

// at 4 km a world unit the horizon of an eye 1.5 up is about 74 off
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

// a ridge 4 wide, which a sample every 10 from an eye at x 5 passes over
type ridge struct{}

func (ridge) At(p geom.Vec) float64 {
	if d := math.Abs(p.X - 150); d < 2 {
		return 20 * (1 - d/2)
	}
	return 0
}
func (ridge) Step() float64 { return 10 }

type profiled struct{ ridge }

func (r profiled) Along(origin, dir geom.Vec, length float64, visit func(d, alt float64) bool) {
	if dir.X > 0 {
		for _, x := range []float64{148, 150, 152} {
			d := (x - origin.X) / dir.X
			if d > 0 && d < length && !visit(d, r.At(geom.NewVec(x, origin.Y+dir.Y*d))) {
				return
			}
		}
	}
	visit(length, r.At(geom.NewVec(origin.X+dir.X*length, origin.Y+dir.Y*length)))
}

func TestHeights_AGroundIsReadAlongItsProfile(t *testing.T) {
	walker := spawn{x: 0, y: 0, z: z(0, 2), sight: eyed(1.5)}
	behind := spawn{x: 240, y: 0, z: z(0, 2)}
	_, seen, _ := sceneIn(t, &relief{ground: ridge{}, step: 10}, walker, behind)
	if seen[0].Count != 1 {
		t.Fatalf("sampled every 10 the walker saw %d, want the one behind the ridge", seen[0].Count)
	}
	_, seen, _ = sceneIn(t, &relief{ground: profiled{}, step: 10}, walker, behind)
	if seen[0].Count != 0 {
		t.Errorf("read along its profile the walker saw %d past the ridge, want none",
			seen[0].Count)
	}
}

func TestHeights_TheVerticalFieldLimitsWhatIsSeenUpAndDown(t *testing.T) {
	level := spawn{x: 100, y: 0, z: z(0, 2)}
	high := spawn{x: 200, y: 0, z: z(150, 2)} // 37° up from the eye
	narrow := func(tilt float64) *look {
		l := eyed(1.5)
		l.Eye.Vertical, l.Eye.Tilt = math.Pi/3, tilt
		return l
	}
	eye := func(tilt float64) spawn { return spawn{x: 0, y: 0, z: z(0, 2), sight: narrow(tilt)} }
	_, seen, _ := sceneIn(t, &relief{}, eye(0), level, high)
	if seen[0].Count != 1 || seen[0].Dists[0] > 150 {
		t.Errorf("looking level, the eye saw %d at %v, want the walker at its level alone",
			seen[0].Count, seen[0].Dists[:seen[0].Count])
	}
	_, seen, _ = sceneIn(t, &relief{}, eye(math.Pi/3), level, high)
	if seen[0].Count != 1 || seen[0].Dists[0] < 150 {
		t.Errorf("looking up 60°, the eye saw %d at %v, want the one 150 up alone",
			seen[0].Count, seen[0].Dists[:seen[0].Count])
	}
	_, seen, _ = sceneIn(t, &relief{}, spawn{x: 0, y: 0, z: z(0, 2), sight: eyed(1.5)}, level, high)
	if seen[0].Count != 2 {
		t.Errorf("without a Vertical the eye saw %d, want both", seen[0].Count)
	}
}
