package traffic

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/world"
)

func TestTrail_HoldsAgainstOneUnderWayWhoLeadsTheUnit(t *testing.T) {
	g := grid.DefaultGrids{}.Square(4, 1, 32)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	finder := spacing.NewFinder(g, terrain, nil, spacing.OpenOccupancy{})
	east, west := geom.NewVec(1, 0), geom.NewVec(-1, 0)
	point, here := g.CellCenter(g.At(3, 0)), geom.NewVec(40, 16)
	// id 0, as a contact with the ground names
	ahead := bodies.Body{At: geom.NewVec(46, 16), Half: geom.NewVec(3, 3), Moving: true}
	moving, queued, behind := ahead, ahead, ahead
	moving.Vel, queued.Trailing = east, true
	behind.At, behind.Vel = geom.NewVec(34, 16), east
	resting := collision.Contact{Normal: west, Resting: true}
	for _, c := range []struct {
		name    string
		other   bodies.Body
		contact collision.Contact
		holds   bool
	}{
		{"resting against one moving ahead", moving, resting, true},
		{"resting against one standing in a queue ahead", queued, resting, true},
		{"resting against one standing that trails nobody", ahead, resting, false},
		{"resting against one behind on the way", behind,
			collision.Contact{Normal: east, Resting: true}, false},
		{"against the ground alone", moving, collision.Contact{Normal: west, Terrain: true}, false},
	} {
		keep := spacing.NewBodyKeeping(finder, nil, nil)
		keep.Begin(func(dst []bodies.Body) []bodies.Body { return append(dst, c.other) })
		s := &System{keep: keep}
		w := &walker{o: &order.MoveOrder{At: point}, coll: &collision.Collider{StruckCount: 1},
			m: bodies.Member{Facing: east, Pos: world.Position{
				AABB: plane.NewAABB(geom.NewVec(here.X-3, here.Y-3), 6, 6)}}}
		w.coll.Struck[0] = c.contact
		if got := s.against(w, c.other.ID); got != c.holds {
			t.Errorf("%s: the trail holds %v, want %v", c.name, got, c.holds)
		}
	}
}

func TestLeads_NearerTheOrdersPointOrWithNoneAheadFacingAlike(t *testing.T) {
	s := &System{}
	east, west := geom.NewVec(1, 0), geom.NewVec(-1, 0)
	point := geom.NewVec(100, 0)
	self := bodies.Body{At: geom.NewVec(40, 0), Facing: east, Point: point}
	unpointed := self
	unpointed.Point = geom.Vec{}
	for _, c := range []struct {
		name  string
		self  bodies.Body
		other bodies.Body
		holds bool
	}{
		{"nearer the point, facing away from it", self,
			bodies.Body{At: geom.NewVec(46, 0), Facing: west}, true},
		{"farther from the point", self, bodies.Body{At: geom.NewVec(34, 0), Facing: east}, false},
		{"as far from the point", self, bodies.Body{At: geom.NewVec(40, 0), Facing: east}, false},
		{"no point, ahead facing alike", unpointed,
			bodies.Body{At: geom.NewVec(46, 0), Facing: east}, true},
		{"no point, ahead nose to nose", unpointed,
			bodies.Body{At: geom.NewVec(46, 0), Facing: west}, false},
		{"no point, behind facing alike", unpointed,
			bodies.Body{At: geom.NewVec(34, 0), Facing: east}, false},
	} {
		d := geom.NewVec(c.self.At.X-c.other.At.X, c.self.At.Y-c.other.At.Y)
		if got := s.leads(c.other, c.self, d); got != c.holds {
			t.Errorf("%s: leads %v, want %v", c.name, got, c.holds)
		}
	}
}
