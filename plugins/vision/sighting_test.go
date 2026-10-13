package vision_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
)

func moving(x, y float64, dir geom.Vec) *world.Base {
	return &world.Base{Pos: world.Position{AABB: plane.NewAABB(geom.NewVec(x, y), 10, 10)}, Vel: world.Velocity{Dir: dir, Value: 1}}
}

func TestSighting_ClosingAndNobody(t *testing.T) {
	east, west, north := geom.NewVec(1, 0), geom.NewVec(-1, 0), geom.NewVec(0, -1)
	for name, c := range map[string]struct {
		mine, theirs geom.Vec
		at           float64 // the other's x; the observer stands at 0
		none         bool
		want         bool
	}{
		"heading at it":       {mine: east, theirs: north, at: 100, want: true},
		"it heads at me":      {mine: north, theirs: west, at: 100, want: true},
		"head on":             {mine: east, theirs: west, at: 100, want: true},
		"both their own ways": {mine: north, theirs: north, at: 100},
		"going apart":         {mine: west, theirs: east, at: 100},
		"on one spot":         {mine: east, theirs: west, at: 0},
		"nobody in view":      {mine: east, none: true},
	} {
		s := vision.Sighting{Base: moving(0, 0, c.mine)}
		if !c.none {
			s.Seen = []vision.Seen{{Base: moving(c.at, 0, c.theirs)}}
		}
		if got := vision.Closing().Holds(s); got != c.want {
			t.Errorf("%s: Closing = %v, want %v", name, got, c.want)
		}
		if got := vision.SeesNobody().Holds(s); got != c.none {
			t.Errorf("%s: Nobody = %v, want %v", name, got, c.none)
		}
	}
}
