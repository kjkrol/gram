package collision_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/internal/collisiontest"
	"github.com/kjkrol/gram/plugins/world"
)

// raised is one entity of a Z test: where it stands along x and the heights it spans, none for
// no Z at all.
type raised struct {
	x float64
	z *world.Z
}

// zRun ticks two overlapping boxes spanning the heights given once, in a world with heights, and
// reports whether they met and how far apart their left edges ended.
func zRun(t *testing.T, a, b raised) (met bool, gap float64) {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
		Heights:  true,
	})
	var stats collision.ContactStats
	c := collision.NewPlugin(w).WithStats(&stats)
	spec := kind.Spec{
		comp.Load(func(r raised) world.Position { return posAt(r.x, 500, 10, 10) }),
		comp.Const(world.Velocity{}),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{}),
	}
	define := func(name string, r raised) kind.Of[raised] {
		own := spec
		if r.z != nil {
			own = append(append(kind.Spec{}, spec...), comp.Const(*r.z))
		}
		return kind.Define[raised](w.Kinds(), name, own)
	}
	w.Seed(define("a", a).Entry(a), define("b", b).Entry(b))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	var base goke.Comp[world.Base]
	var q *goke.Query
	ecs := collisiontest.Start(t, w, c, goke.SystemFn{OnInit: func(si *goke.SysInit) { q = si.NewQueryBuilder(&base).Build() }})
	ecs.Tick(time.Second / 60)

	var lefts []float64
	for q.All(); q.Next(); {
		for _, b := range base.Slice(q.Cursor()) {
			lefts = append(lefts, b.Pos.TopLeft.X)
		}
	}
	return stats.Counter > 0, max(lefts[0], lefts[1]) - min(lefts[0], lefts[1])
}

func z(altitude, height float64) *world.Z { return &world.Z{Altitude: altitude, Height: height} }

// In a world with heights two overlapping boxes meet only where the heights they span do: a hawk
// passes over a walker, a crate on a platform touches nothing under it, and whatever says no
// height — no Z, a Height of 0 — stands at every height and meets everything.
func TestZ_TouchOnlyWhereTheBandsOverlap(t *testing.T) {
	cases := map[string]struct {
		a, b  *world.Z
		touch bool
	}{
		"on the same ground": {z(0, 20), z(0, 20), true},
		"one over the other": {z(0, 20), z(40, 10), false},
		"overlapping bands":  {z(0, 20), z(15, 20), true},
		"meeting at an edge": {z(0, 10), z(10, 10), false},
		"one without a Z":    {nil, z(40, 10), true},
		"one without height": {z(0, 0), z(40, 10), true},
		"short on a step":    {z(0, 2), z(12, 2), false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			met, gap := zRun(t, raised{x: 100, z: tc.a}, raised{x: 104, z: tc.b})
			if met != tc.touch {
				t.Errorf("met = %v, want %v", met, tc.touch)
			}
			if pushedApart := gap >= 10; pushedApart != tc.touch {
				t.Errorf("gap after the tick = %v, want pushed apart = %v", gap, tc.touch)
			}
		})
	}
}

// The bands of a flat world: everything, so Band.Meets never vetoes there.
func TestBand_OfNothingIsEverywhere(t *testing.T) {
	if got := collision.BandOf(nil); got != collision.Everywhere {
		t.Errorf("BandOf(nil) = %v, want Everywhere", got)
	}
	if got := collision.BandOf(&world.Z{Altitude: 5}); got != collision.Everywhere {
		t.Errorf("BandOf of no Height = %v, want Everywhere", got)
	}
	if got := collision.BandOf(&world.Z{Altitude: 5, Height: 3}); got != (collision.Band{Bottom: 5, Top: 8}) {
		t.Errorf("BandOf(5, 3) = %v, want [5, 8]", got)
	}
	if !collision.Everywhere.Meets(collision.Band{Bottom: 40, Top: 50}) {
		t.Error("Everywhere meets nothing")
	}
}
