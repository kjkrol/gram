package collision_test

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/internal/collisiontest"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
)

func TestBounce_AWallTurnsNoUnitAHandSteers(t *testing.T) {
	heading := geom.NewVec(math.Cos(2*math.Pi/180), math.Sin(2*math.Pi/180))
	hands := map[string]*steering.Driven{
		"by hand":                {Ahead: 1},
		"by a hand off the keys": {},
		"by nobody":              nil,
	}
	for name, hand := range hands {
		driven := hand != nil
		t.Run(name, func(t *testing.T) {
			w := world.NewPlugin(world.Config{
				Space:    world.SpaceCfg{Width: 1000, Height: 1000},
				Entities: world.EntitiesCfg{MaxCount: 2, MinSize: 10, MaxSize: 10},
			})
			wall := &wallField{
				box:  geom.NewAABBAt(geom.NewVec(200, 0), 32, 1000),
				band: collision.Everywhere,
			}
			m := motion.NewPlugin(w)
			c := collision.NewPlugin(w, m).WithField(wall)
			spec := kind.Spec{
				comp.Load(func(r raised) world.Position { return posAt(r.x, 500, 10, 10) }),
				comp.Const(world.Velocity{Dir: heading, Value: 300}),
				comp.Const(collision.Collider{}),
				comp.Const(collision.Physics{}),
			}
			if driven {
				spec = append(spec, comp.Const(*hand))
			}
			kind.Define[raised](w.Kinds(), "walker", spec)
			w.Write().Seed(kind.Named[raised](w.Kinds(), "walker").Entry(raised{x: 180}))
			if err := w.Populate(); err != nil {
				t.Fatal(err)
			}
			var base goke.Comp[world.Base]
			var q *goke.Query
			ecs := collisiontest.Start(t, w, m, c, goke.SystemFn{OnInit: func(si *goke.SysInit) {
				q = si.NewQueryBuilder(&base).Build()
			}})
			for range 10 {
				ecs.Tick(time.Second / 60)
			}
			var dir geom.Vec
			for q.All(); q.Next(); {
				for _, b := range base.Slice(q.Cursor()) {
					dir = b.Vel.Dir
				}
			}
			kept := math.Abs(dir.X-heading.X) < 1e-9 && math.Abs(dir.Y-heading.Y) < 1e-9
			if kept != driven {
				t.Errorf("after the wall it faces %v: kept its heading %v = %v, want %v",
					dir, heading, kept, driven)
			}
		})
	}
}
