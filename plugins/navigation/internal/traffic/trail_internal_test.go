package traffic

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
)

func TestTrail_HoldsAgainstOneUnderWayAheadOfTheUnit(t *testing.T) {
	g := grid.DefaultGrids{}.Square(4, 1, 32)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	finder := spacing.NewFinder(g, terrain, nil, spacing.OpenOccupancy{})
	east, west := geom.NewVec(1, 0), geom.NewVec(-1, 0)
	self := bodies.Body{ID: 7, At: geom.NewVec(40, 16), Half: geom.NewVec(3, 3), Moving: true,
		Way: east}
	// id 0, as a contact with the ground names
	ahead := bodies.Body{At: geom.NewVec(46, 16), Half: geom.NewVec(3, 3), Moving: true, Way: east}
	moving, queued, behind := ahead, ahead, ahead
	moving.Vel, queued.Trailing = east, true
	behind.At, behind.Vel = geom.NewVec(34, 16), east
	at := func(x float32) bodies.Body { b := moving; b.At = geom.NewVec(x, 16); return b }
	resting := collision.Contact{Normal: west, Resting: true}
	backward := collision.Contact{Normal: east, Resting: true}
	ground := collision.Contact{Normal: west, Terrain: true}
	for _, c := range []struct {
		name    string
		other   bodies.Body
		contact *collision.Contact // nil, touching nobody
		holds   bool
	}{
		{"resting against one moving ahead", moving, &resting, true},
		{"resting against one standing in a queue ahead", queued, &resting, true},
		{"resting against one standing that trails nobody", ahead, &resting, false},
		{"resting against one behind on the way", behind, &backward, false},
		{"one moving a body's width ahead, untouched", at(51), nil, true},
		{"one moving two bodies' widths ahead, untouched", at(58), nil, false},
		{"one moving two widths ahead, the ground alone touched", at(58), &ground, false},
	} {
		keep := spacing.NewBodyKeeping(finder, nil, nil)
		keep.Begin(func(dst []bodies.Body) []bodies.Body { return append(dst, self, c.other) })
		s := &System{keep: keep}
		w := &walker{id: self.ID, o: &order.MoveOrder{}, coll: &collision.Collider{}}
		if c.contact != nil {
			w.coll.Struck[0], w.coll.StruckCount = *c.contact, 1
		}
		if got := s.against(w, c.other.ID); got != c.holds {
			t.Errorf("%s: the trail holds %v, want %v", c.name, got, c.holds)
		}
	}
}

func TestBearing_AheadWithinAQuarterTurnOfTheRoutesWayOrOfTheFacingWithNone(t *testing.T) {
	s := &System{}
	// the way deg degrees round from east, a unit vector
	toward := func(deg float64) geom.Vec {
		r := deg * math.Pi / 180
		return geom.NewVec(float32(math.Cos(r)), float32(math.Sin(r)))
	}
	at := func(deg float64) geom.Vec { d := toward(deg); return geom.NewVec(10*d.X, 10*d.Y) }
	east, west := toward(0), toward(180)
	self := bodies.Body{Way: east, Facing: east}
	unrouted := bodies.Body{Facing: east}
	for _, c := range []struct {
		name               string
		self, other        bodies.Body
		ahead, along, same bool
	}{
		{"straight ahead, going my way", self,
			bodies.Body{At: at(0), Vel: east, Way: east}, true, true, true},
		{"40° off my way, going 40° off it", self,
			bodies.Body{At: at(40), Vel: toward(40), Way: toward(40)}, true, true, true},
		{"50° off my way, going 50° off it", self,
			bodies.Body{At: at(50), Way: toward(50)}, false, false, false},
		{"beside me, going my way", self, bodies.Body{At: at(90), Way: east}, false, false, true},
		{"behind me, coming at me", self,
			bodies.Body{At: at(180), Vel: west, Way: west}, false, false, false},
		{"ahead, with no route", self, bodies.Body{At: at(0), Vel: east}, true, true, false},
		{"no route of my own: ahead by my facing, facing alike", unrouted,
			bodies.Body{At: at(0), Vel: east, Facing: east}, true, true, true},
		{"no route of my own: ahead nose to nose", unrouted,
			bodies.Body{At: at(0), Facing: west}, true, false, false},
		{"no route of my own: behind me", unrouted,
			bodies.Body{At: at(180), Facing: east}, false, false, true},
	} {
		d := geom.NewVec(c.self.At.X-c.other.At.X, c.self.At.Y-c.other.At.Y)
		off := geom.NewVec(d.X/10, d.Y/10) // the way self leaves other, both 10 apart
		ahead, along, same := s.bearing(c.self, c.other, off)
		if ahead != c.ahead || along != c.along || same != c.same {
			t.Errorf("%s: ahead, along, same way %v %v %v, want %v %v %v", c.name, ahead, along,
				same, c.ahead, c.along, c.same)
		}
	}
}
