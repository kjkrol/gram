package collision_test

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/internal/collisiontest"
	"github.com/kjkrol/gram/plugins/world"
)

// wallField is a Field of one solid cell, a wall standing in the band given, open all round.
type wallField struct {
	box  geom.AABB
	band collision.Band
}

func (f *wallField) Solid(_ world.Layers, band collision.Band, box geom.AABB, visit func(collision.FieldBox) bool) {
	if !f.band.Meets(band) || !box.Intersects(f.box) {
		return
	}
	visit(collision.FieldBox{Box: f.box, Cell: 1, Open: collide.Left | collide.Right | collide.Top | collide.Bottom})
}

func (f *wallField) Overhang(world.Layers, geom.AABB) float64 { return 0 }

// A wall of a height stops only what stands in its band: a walker on the ground is held at its
// face, a flyer over its top goes on; a wall of no height stands at every height.
func TestZ_TheGroundStopsOnlyWhatStandsInItsBand(t *testing.T) {
	wall := func(top float64) *wallField {
		return &wallField{box: geom.NewAABBAt(geom.NewVec(200, 0), 32, 1000), band: collision.Band{Bottom: math.Inf(-1), Top: top}}
	}
	cases := map[string]struct {
		field *wallField
		z     world.Z
		held  bool
	}{
		"a walker at the foot of the wall": {wall(30), world.Z{Altitude: 0, Height: 20}, true},
		"a flyer over the wall":            {wall(30), world.Z{Altitude: 40, Height: 10}, false},
		"a flyer under a wall of no height": {&wallField{box: geom.NewAABBAt(geom.NewVec(200, 0), 32, 1000), band: collision.Everywhere},
			world.Z{Altitude: 40, Height: 10}, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			w := world.NewPlugin(world.Config{
				Space:    world.SpaceCfg{Width: 1000, Height: 1000},
				Entities: world.EntitiesCfg{MaxCount: 2, MinSize: 10, MaxSize: 10},
				Heights:  true,
			})
			c := collision.NewPlugin(w).WithField(tc.field)
			kind.Define[raised](w.Kinds(), "runner", kind.Spec{
				comp.Load(func(r raised) world.Position { return posAt(r.x, 500, 10, 10) }),
				comp.Const(world.Velocity{Dir: geom.NewVec(1, 0), Value: 300}),
				comp.Const(collision.Collider{}),
				comp.Const(collision.Physics{}),
				comp.Const(tc.z),
			})
			runner := kind.Named[raised](w.Kinds(), "runner")
			w.Seed(runner.Entry(raised{x: 180}))
			if err := w.Populate(); err != nil {
				t.Fatal(err)
			}
			var base goke.Comp[world.Base]
			var q *goke.Query
			ecs := collisiontest.Start(t, w, c, goke.SystemFn{OnInit: func(si *goke.SysInit) { q = si.NewQueryBuilder(&base).Build() }})
			for range 30 {
				ecs.Tick(time.Second / 60)
			}
			var right float64
			for q.All(); q.Next(); {
				for _, b := range base.Slice(q.Cursor()) {
					right = b.Pos.BottomRight.X
				}
			}
			if held := right <= 200+1e-9; held != tc.held {
				t.Errorf("the runner's right edge ended at %v: held at the wall = %v, want %v", right, held, tc.held)
			}
		})
	}
}

// In a flat world a Z is never asked: two boxes one over the other meet as any two do.
func TestZ_AFlatWorldIgnoresIt(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
	})
	var stats collision.ContactStats
	c := collision.NewPlugin(w).WithStats(&stats)
	var base goke.Comp[world.Base]
	var coll goke.Comp[collision.Collider]
	var zs goke.Comp[world.Z]
	// the kinds refuse a Z in a flat world, so the two are made by hand, one over the other
	ecs := collisiontest.Start(t, w, c, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&base, &coll, &zs)
		f.Create(2)
		for f.Next() {
			for i := range f.IDs {
				base.Slice(&f.Cursor)[i] = world.Base{Pos: posAt(100+4*float64(i), 500, 10, 10)}
				zs.Slice(&f.Cursor)[i] = world.Z{Altitude: 40 * float64(i), Height: 10}
			}
		}
	}})
	ecs.Tick(time.Second / 60)
	if stats.Counter == 0 {
		t.Error("the two never met: a flat world asked their Z")
	}
}
