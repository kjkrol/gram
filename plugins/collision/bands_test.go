package collision_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/internal/collisiontest"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
)

type anchor struct{ x, y float32 }

func (a *anchor) Anchors(at func(x, y float32)) { at(a.x, a.y) }

var _ sector.Anchors = (*anchor)(nil)

func TestCollision_FarOffBoxesRestUntilTheBandComesNear(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 4096, Height: 512},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 10, MaxSize: 20},
		Sectors: world.SectorsCfg{
			Buckets: 2,
			Bands: []sector.Band{
				{Reach: 600},
				{Reach: 1200, Pace: 4},
			},
		},
	})
	at := &anchor{x: 100, y: 256}
	w.Write().Anchor(at)
	m := motion.NewPlugin(w)
	c := collision.NewPlugin(w, m)
	kind.Define[placed](w.Kinds(), "collider", kind.Spec{
		comp.Load(func(b placed) world.Position { return posAt(b.x, b.y, 10, 10) }),
		comp.Const(world.Velocity{}),
		comp.Const(collision.Collider{}),
	})
	colliders := kind.Named[placed](w.Kinds(), "collider")
	for _, x := range []float32{100, 105, 1200, 1205} {
		w.Write().Seed(colliders.Entry(placed{x: x, y: 250}))
	}
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	var base goke.Comp[world.Base]
	var coll goke.Comp[collision.Collider]
	var q *goke.Query
	ecs := collisiontest.Start(t, w, m, c, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&base, &coll).Build()
	}})
	// whether the boxes about x touch and collide
	touching := func(x float32) (touches, collides bool) {
		for q.All(); q.Next(); {
			cur := q.Cursor()
			for i, b := range base.Slice(cur) {
				if b.Pos.Center().X > x-20 && b.Pos.Center().X < x+30 {
					touches = touches || len(coll.Slice(cur)[i].Contacts()) > 0
					collides = collides || b.Caps&aabbworld.CanCollide != 0
				}
			}
		}
		return touches, collides
	}
	for range 2 {
		ecs.Tick(time.Second / 60)
	}
	if touches, collides := touching(100); !touches || !collides {
		t.Errorf("near: touching %v, colliding %v; want both", touches, collides)
	}
	if touches, collides := touching(1200); touches || collides {
		t.Errorf("in the second band: touching %v, colliding %v; want neither", touches, collides)
	}
	at.x = 1200
	for range 2 {
		ecs.Tick(time.Second / 60)
	}
	if touches, collides := touching(1200); !touches || !collides {
		t.Errorf("come near: touching %v, colliding %v; want both", touches, collides)
	}
}
