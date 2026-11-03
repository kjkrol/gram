package collision_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/internal/collisiontest"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
)

// two observers looking at (x, y) over a world w by h: the one a pixel a world unit, the other a
// quarter of a pixel seeing four times as far; under a ladder seeing from half a pixel a world
// unit, L0 within its sight of the point, within four times that the level its lower thresholds
// give a quarter of a pixel, L4 beyond
type observers [2]camera.Camera

var _ sector.Observers = (*observers)(nil)

func newObservers(w, h uint32, x, y float32) *observers {
	o := &observers{cameratest.NewFromSpace(w, h, 0), cameratest.NewFromSpace(w, h, 0)}
	for _, cam := range o {
		cam.SetViewport(2, 2)
	}
	o[1].ZoomOut(4, 0, 0)
	o[1].(interface{ See(factor float32) }).See(4)
	o.look(x, y)
	return o
}

// has both observers look at (x, y)
func (o *observers) look(x, y float32) {
	for _, cam := range o {
		cam.CenterOn(x, y, 0)
	}
}

func (o *observers) Observers(each func(cam camera.Camera)) {
	for _, cam := range o {
		each(cam)
	}
}

// far off is four sectors on from the near boxes: room for a sector a level coarser each between
func TestCollision_BoxesCollideEveryStepThroughL2AndRestFromL3(t *testing.T) {
	for _, c := range []struct {
		level   lod.Level
		scales  [lod.Levels - 1]float32
		sight   float32
		collide bool
	}{
		{lod.L1, [lod.Levels - 1]float32{0.5, 0.2}, 600, true},
		{lod.L2, [lod.Levels - 1]float32{0.5, 0.4, 0.2}, 600, true},
		{lod.L3, [lod.Levels - 1]float32{0.5, 0.4, 0.3}, 600, false},
		{lod.L4, [lod.Levels - 1]float32{0.5, 0.4, 0.3}, 400, false},
	} {
		t.Run(fmt.Sprintf("L%d", c.level), func(t *testing.T) {
			w := world.NewPlugin(world.Config{
				Space:    world.SpaceCfg{Width: 4096, Height: 512},
				Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 10, MaxSize: 20},
				Sectors:  world.SectorsCfg{Buckets: 2},
				LOD:      lod.Ladder{Scales: c.scales, Sight: c.sight},
			})
			at := newObservers(4096, 512, 100, 256)
			w.Write().Observe(at)
			m := motion.NewPlugin(w)
			coll := collision.NewPlugin(w, m)
			kind.Define[placed](w.Kinds(), "collider", kind.Spec{
				comp.Load(func(b placed) world.Position { return posAt(b.x, b.y, 10, 10) }),
				comp.Const(world.Velocity{}),
				comp.Const(collision.Collider{}),
			})
			colliders := kind.Named[placed](w.Kinds(), "collider")
			for _, x := range []float32{100, 105, 2200, 2205} {
				w.Write().Seed(colliders.Entry(placed{x: x, y: 250}))
			}
			if err := w.Populate(); err != nil {
				t.Fatal(err)
			}
			var base goke.Comp[world.Base]
			var collider goke.Comp[collision.Collider]
			var l1 goke.OptComp[sector.L1]
			var l2 goke.OptComp[sector.L2]
			var l3 goke.OptComp[sector.L3]
			var l4 goke.OptComp[sector.L4]
			var q *goke.Query
			ecs := collisiontest.Start(t, w, m, coll, goke.SystemFn{OnInit: func(si *goke.SysInit) {
				q = si.NewQueryBuilder(&base, &collider).Optional(&l1, &l2, &l3, &l4).Build()
			}})
			// whether the boxes about x touch and collide, and the level they lie at
			touching := func(x float32) (touches, collides bool, level lod.Level) {
				for q.All(); q.Next(); {
					cur := q.Cursor()
					for i, b := range base.Slice(cur) {
						if b.Pos.Center().X > x-20 && b.Pos.Center().X < x+30 {
							touches = touches || len(collider.Slice(cur)[i].Contacts()) > 0
							collides = collides || b.Caps&aabbworld.CanCollide != 0
							switch {
							case l1.Present(cur):
								level = lod.L1
							case l2.Present(cur):
								level = lod.L2
							case l3.Present(cur):
								level = lod.L3
							case l4.Present(cur):
								level = lod.L4
							}
						}
					}
				}
				return touches, collides, level
			}
			for range 2 {
				ecs.Tick(time.Second / 60)
			}
			if touches, collides, _ := touching(100); !touches || !collides {
				t.Errorf("near: touching %v, colliding %v; want both", touches, collides)
			}
			touches, collides, level := touching(2200)
			if level != c.level {
				t.Fatalf("far off at L%d, want L%d", level, c.level)
			}
			if touches != c.collide || collides != c.collide {
				t.Errorf("far off: touching %v, colliding %v; want %v", touches, collides,
					c.collide)
			}
			at.look(2200, 256)
			for range 2 {
				ecs.Tick(time.Second / 60)
			}
			if touches, collides, _ := touching(2200); !touches || !collides {
				t.Errorf("come near: touching %v, colliding %v; want both", touches, collides)
			}
		})
	}
}
