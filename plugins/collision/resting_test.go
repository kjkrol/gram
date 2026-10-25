package collision_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/internal/collisiontest"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/world"
)

type walkerRow struct {
	x    float32
	mass float32
	vel  float32
}

// a walker runs east into a post of infinite mass, stops against it and stands there
func TestResting_APairPartedStillMeetsWhileItTouches(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 2, MinSize: 10, MaxSize: 10},
	})
	var orders heards
	if err := w.Carry(&orders); err != nil {
		t.Fatal(err)
	}
	m := motion.NewPlugin(w)
	c := collision.NewPlugin(w, m)
	if err := c.Hook(
		act.Branch[collision.Meeting]{
			Name: "met",
			When: cond.Self.If(cond.Always),
			Then: act.Self.Do(heard{Rule: "met"}),
		},
		act.Branch[collision.Struck]{
			Name: "struck",
			When: cond.Self.If(cond.Always),
			Then: act.Self.Do(heard{Rule: "struck"}),
		}); err != nil {
		t.Fatal(err)
	}
	kind.Define[walkerRow](w.Kinds(), "thing", kind.Spec{
		comp.Load(func(r walkerRow) world.Position { return posAt(r.x, 100, 10, 10) }),
		comp.Load(func(r walkerRow) world.Velocity {
			return world.Velocity{Dir: geom.NewVec(1, 0), Value: r.vel}
		}),
		comp.Const(collision.Collider{}),
		comp.Load(func(r walkerRow) collision.Physics { return collision.Physics{Mass: r.mass} }),
	})
	thing := kind.Named[walkerRow](w.Kinds(), "thing")
	w.Write().Seed(thing.Entry(walkerRow{x: 100, mass: 1, vel: 120}), thing.Entry(walkerRow{x: 116, mass: f32.Inf(1)}))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	var base goke.Comp[world.Base]
	var collider goke.Comp[collision.Collider]
	var q *goke.Query
	ecs := collisiontest.Start(t, w, m, c, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&base, &collider).Build()
	}})
	for range 8 { // into the post and held at it
		ecs.Tick(time.Second / 60)
	}
	for q.All(); q.Next(); {
		bases := base.Slice(q.Cursor())
		for i := range bases {
			bases[i].Vel.Value = 0 // the walker stops pressing
		}
	}
	orders.given()
	for range 3 {
		ecs.Tick(time.Second / 60)
	}
	met, struck := 0, 0
	for _, h := range orders.given() {
		switch h.Command.Rule {
		case "met":
			met++
		case "struck":
			struck++
		}
	}
	if met != 3 {
		t.Errorf("standing against the post for three ticks, met %d times; want once a tick", met)
	}
	if struck != 0 {
		t.Errorf("standing against the post, struck %d times; want none: resting is no strike", struck)
	}
	resting := 0
	for q.All(); q.Next(); {
		for _, col := range collider.Slice(q.Cursor()) {
			for _, ct := range col.Contacts() {
				if ct.Resting && ct.Impact == 0 {
					resting++
				}
			}
		}
	}
	if resting != 2 {
		t.Errorf("%d resting contacts recorded, want one a side", resting)
	}
	// walking away, the pair parts
	for q.All(); q.Next(); {
		bases := base.Slice(q.Cursor())
		for i := range bases {
			if bases[i].Pos.TopLeft.X < 110 {
				bases[i].Vel = world.Velocity{Dir: geom.NewVec(-1, 0), Value: 120}
			}
		}
	}
	ecs.Tick(time.Second / 60)
	orders.given()
	ecs.Tick(time.Second / 60)
	for _, h := range orders.given() {
		if h.Command.Rule == "met" {
			t.Errorf("walked away, still met")
		}
	}
}

// a walker pressed into a post every tick meets it once a tick: the pair pushed apart is not
// rested on top
func TestResting_APairRecordedThisTickIsNotRestedAgain(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 2, MinSize: 10, MaxSize: 10},
	})
	var orders heards
	if err := w.Carry(&orders); err != nil {
		t.Fatal(err)
	}
	m := motion.NewPlugin(w)
	c := collision.NewPlugin(w, m)
	if err := c.Hook(act.Branch[collision.Meeting]{
		Name: "met",
		When: cond.Self.If(cond.Always),
		Then: act.Self.Do(heard{Rule: "met"}),
	}); err != nil {
		t.Fatal(err)
	}
	kind.Define[walkerRow](w.Kinds(), "thing", kind.Spec{
		comp.Load(func(r walkerRow) world.Position { return posAt(r.x, 100, 10, 10) }),
		comp.Load(func(r walkerRow) world.Velocity {
			return world.Velocity{Dir: geom.NewVec(1, 0), Value: r.vel}
		}),
		comp.Const(collision.Collider{}),
		comp.Load(func(r walkerRow) collision.Physics { return collision.Physics{Mass: r.mass} }),
	})
	thing := kind.Named[walkerRow](w.Kinds(), "thing")
	w.Write().Seed(
		thing.Entry(walkerRow{x: 100, mass: 1, vel: 120}),
		thing.Entry(walkerRow{x: 116, mass: f32.Inf(1)}),
	)
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	var base goke.Comp[world.Base]
	var collider goke.Comp[collision.Collider]
	var q *goke.Query
	ecs := collisiontest.Start(t, w, m, c, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&base, &collider).Build()
	}})
	press := func() { // the push takes the walker's speed; given back, it overlaps again
		for q.All(); q.Next(); {
			bases := base.Slice(q.Cursor())
			for i := range bases {
				if bases[i].Pos.TopLeft.X < 110 {
					bases[i].Vel = world.Velocity{Dir: geom.NewVec(1, 0), Value: 120}
				}
			}
		}
	}
	for range 8 {
		press()
		ecs.Tick(time.Second / 60)
	}
	orders.given()
	for range 3 {
		press()
		ecs.Tick(time.Second / 60)
	}
	met := 0
	for _, h := range orders.given() {
		if h.Command.Rule == "met" {
			met++
		}
	}
	if met != 3 {
		t.Errorf("pressed into the post for three ticks, met %d times; want once a tick", met)
	}
	for q.All(); q.Next(); {
		cur := q.Cursor()
		for i, col := range collider.Slice(cur) {
			contacts := col.Contacts()
			if len(contacts) != 1 {
				t.Errorf("%v holds %d contacts, want the one push", cur.IDs[i], len(contacts))
				continue
			}
			if contacts[0].Resting {
				t.Errorf("%v's contact is at rest, want the push", cur.IDs[i])
			}
		}
	}
}

type offMiddle struct{ x, y, side, vel, mass float32 }

// a small box resting off the middle of a big one's face leaves it straight off that face
func TestResting_TheNormalIsTheFaceTouched(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 2, MinSize: 6, MaxSize: 28},
	})
	m := motion.NewPlugin(w)
	c := collision.NewPlugin(w, m)
	kind.Define[offMiddle](w.Kinds(), "box", kind.Spec{
		comp.Load(func(r offMiddle) world.Position { return posAt(r.x, r.y, r.side, r.side) }),
		comp.Load(func(r offMiddle) world.Velocity {
			return world.Velocity{Dir: geom.NewVec(1, 0), Value: r.vel}
		}),
		comp.Const(collision.Collider{}),
		comp.Load(func(r offMiddle) collision.Physics { return collision.Physics{Mass: r.mass} }),
	})
	box := kind.Named[offMiddle](w.Kinds(), "box")
	// the middles 21 across and 9 down: the line between them slants by 23°
	w.Write().Seed(
		box.Entry(offMiddle{x: 100, y: 100, side: 6, vel: 120, mass: 1}),
		box.Entry(offMiddle{x: 110, y: 80, side: 28, mass: f32.Inf(1)}),
	)
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	var base goke.Comp[world.Base]
	var collider goke.Comp[collision.Collider]
	var q *goke.Query
	ecs := collisiontest.Start(t, w, m, c, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&base, &collider).Build()
	}})
	for range 8 {
		ecs.Tick(time.Second / 60)
	}
	for q.All(); q.Next(); {
		bases := base.Slice(q.Cursor())
		for i := range bases {
			bases[i].Vel.Value = 0
		}
	}
	ecs.Tick(time.Second / 60)
	for q.All(); q.Next(); {
		cur := q.Cursor()
		for i, b := range base.Slice(cur) {
			want := geom.NewVec(-1, 0)
			if b.Pos.Size.X != 6 {
				want = geom.NewVec(1, 0)
			}
			contacts := collider.Slice(cur)[i].Contacts()
			if len(contacts) != 1 || !contacts[0].Resting || contacts[0].Normal != want {
				t.Errorf("the box of side %v rests by %+v, want one contact at rest leaving by %v",
					b.Pos.Size.X, contacts, want)
			}
		}
	}
}
