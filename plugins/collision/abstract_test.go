package collision_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/internal/collisiontest"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/uid"
)

func TestCollision_ABodyOnTheAbstractGraphPushesNothingAndIsPushedByNothing(t *testing.T) {
	for _, abstract := range []bool{false, true} {
		w := world.NewPlugin(world.Config{
			Space:    world.SpaceCfg{Width: 1000, Height: 1000},
			Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
			Heights:  true,
		})
		m := motion.NewPlugin(w)
		c := collision.NewPlugin(w, m).WithField(&wallField{
			box: geom.NewAABBAt(geom.NewVec(200, 0), 32, 1000), band: collision.Everywhere})
		// in the wall, and overlapping the other
		spec := kind.Spec{
			comp.Load(func(r placed) world.Position { return posAt(r.x, 500, 10, 10) }),
			comp.Const(world.Velocity{}),
			comp.Const(collision.Collider{}),
			comp.Const(collision.Physics{}),
			comp.Const(world.Z{Height: 10}),
		}
		if abstract {
			spec = append(spec, comp.Const(sector.Abstract{}))
		}
		kind.Define[placed](w.Kinds(), "far", spec)
		kind.Define[placed](w.Kinds(), "other", kind.Spec{
			comp.Load(func(r placed) world.Position { return posAt(r.x, 500, 10, 10) }),
			comp.Const(world.Velocity{}),
			comp.Const(collision.Collider{}),
			comp.Const(collision.Physics{}),
			comp.Const(world.Z{Height: 10}),
		})
		w.Write().Seed(kind.Named[placed](w.Kinds(), "far").Entry(placed{x: 195}),
			kind.Named[placed](w.Kinds(), "other").Entry(placed{x: 188}))
		if err := w.Populate(); err != nil {
			t.Fatal(err)
		}
		var base goke.Comp[world.Base]
		var collider goke.Comp[collision.Collider]
		var q *goke.Query
		ecs := collisiontest.Start(t, w, m, c, goke.SystemFn{OnInit: func(si *goke.SysInit) {
			q = si.NewQueryBuilder(&base, &collider).Build()
		}})
		for range 3 {
			ecs.Tick(time.Second / 60)
		}
		moved, touched := 0, 0
		for q.All(); q.Next(); {
			cur := q.Cursor()
			for i, b := range base.Slice(cur) {
				if x := b.Pos.TopLeft.X; x != 195 && x != 188 {
					moved++
				}
				touched += len(collider.Slice(cur)[i].Contacts())
			}
		}
		if abstract && (moved != 0 || touched != 0) {
			t.Errorf("marked: %d moved, %d contacts; want none pushed, none touching", moved,
				touched)
		}
		if !abstract && (moved == 0 || touched == 0) {
			t.Errorf("unmarked: %d moved, %d contacts; want them pushed apart, touching", moved,
				touched)
		}
	}
}

func TestCollision_ABodyGoneOnTheAbstractGraphRestsAgainstNoOne(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
	})
	m := motion.NewPlugin(w)
	c := collision.NewPlugin(w, m)
	kind.Define[placed](w.Kinds(), "box", kind.Spec{
		comp.Load(func(r placed) world.Position { return posAt(r.x, 500, 10, 10) }),
		comp.Const(world.Velocity{}),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{}),
	})
	boxes := kind.Named[placed](w.Kinds(), "box")
	// overlapping, then pushed apart to rest side by side
	w.Write().Seed(boxes.Entry(placed{x: 190}), boxes.Entry(placed{x: 198}))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	var base goke.Comp[world.Base]
	var collider goke.Comp[collision.Collider]
	var q *goke.Query
	ecs := collisiontest.Start(t, w, m, c, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&base, &collider).Build()
	}})
	// the contacts, and the later of the two, whose resting pair the other keeps
	contacts := func() (n int, later uid.UID64) {
		for q.All(); q.Next(); {
			cur := q.Cursor()
			for i, id := range cur.IDs {
				n += len(collider.Slice(cur)[i].Contacts())
				later = max(later, id)
			}
		}
		return n, later
	}
	for range 3 {
		ecs.Tick(time.Second / 60)
	}
	n, later := contacts()
	if n == 0 {
		t.Fatal("the boxes never met")
	}
	var marked goke.CompID
	marking := ecs.RegSys(goke.SystemFn{
		OnInit: func(si *goke.SysInit) { marked = si.RegComp[sector.Abstract]() },
		OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
			cb.AddOne(later, marked, sector.Abstract{})
		},
	})
	mark := true
	step := collisiontest.Step(w, m, c)
	ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		step(rc, d)
		if mark {
			rc.Run(marking, d)
			rc.Sync()
			mark = false
		}
	})
	for range 3 {
		ecs.Tick(time.Second / 60)
	}
	if n, _ := contacts(); n != 0 {
		t.Errorf("%d contacts with one of the two on the abstract graph, want none", n)
	}
}
