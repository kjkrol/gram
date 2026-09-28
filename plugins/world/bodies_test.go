package world_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/kind"
	"github.com/kjkrol/gram/plugins/world/kind/comp"
	"github.com/kjkrol/uid"
)

// bodyRig is a world with one system that spawns bodies of a reserved kind on demand.
type bodyRig struct {
	w      *world.Plugin
	ecs    *goke.ECS
	typeID kind.ID
	bodies *world.Bodies
	tag    goke.Comp[struct{ Rock bool }]
	base   goke.Comp[world.Base]
	look   goke.OptComp[world.Appearance]
	query  *goke.Query
	spawn  []plane.AABB
}

func newBodyRig(t *testing.T, maxCount int) *bodyRig {
	t.Helper()
	r := &bodyRig{}
	r.w = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 400, Height: 400},
		Entities: world.EntitiesCfg{MaxCount: maxCount, MinSize: 10, MaxSize: 10},
	})
	r.typeID = r.w.Kinds().Reserve("terrain")
	ctx := &installCtx{ecs: goke.New()}
	if err := r.w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		r.bodies = r.w.NewBodies(si, r.typeID, &r.tag)
		r.query = si.NewQueryBuilder(&r.base).Optional(&r.look).Build()
	}})
	ctx.ecs.Setup(systems...)
	spawner := ctx.ecs.RegSys(goke.SystemFn{OnUpdate: func(*goke.CmdBuf, time.Duration) {
		r.bodies.Spawn(r.spawn, func(i int, _ uid.UID64, cursor *goke.Cursor) { r.tag.Slice(cursor)[i].Rock = true })
		r.spawn = nil
	}})
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		rc.Run(spawner, d)
		rc.Sync()
		r.w.RunPlan(rc, d)
		r.w.Clock().Replay(rc, d)
	})
	r.ecs = ctx.ecs
	return r
}

func (r *bodyRig) tick() { r.ecs.Tick(time.Second / 60) }

// entities returns every entity's id, base and whether it is drawn.
func (r *bodyRig) entities() (ids []uid.UID64, bases []world.Base, drawn []bool) {
	for r.query.All(); r.query.Next(); {
		cur := r.query.Cursor()
		ids = append(ids, cur.IDs...)
		bases = append(bases, r.base.Slice(cur)...)
		for range cur.IDs {
			drawn = append(drawn, r.look.Present(cur))
		}
	}
	return
}

func box(x, y, w, h float64) plane.AABB { return plane.NewAABB(geom.NewVec(x, y), w, h) }

func TestBodies_SpawnPlacesUndrawnEntitiesOfTheReservedKindFreeOfSizeBounds(t *testing.T) {
	r := newBodyRig(t, 4)
	r.spawn = []plane.AABB{box(0, 0, 300, 20), box(50, 100, 5, 5)}
	r.tick()

	ids, bases, drawn := r.entities()
	if len(ids) != 2 {
		t.Fatalf("%d entities, want 2", len(ids))
	}
	for i, b := range bases {
		if b.TypeID != r.typeID {
			t.Errorf("body %d has TypeID %d, want the reserved %d", i, b.TypeID, r.typeID)
		}
		if drawn[i] {
			t.Errorf("body %d carries Appearance, want none", i)
		}
	}
	if got := r.w.Res.Telemetry.Count; got != 2 {
		t.Errorf("telemetry counts %d, want 2", got)
	}
	found := 0
	r.w.Space().Query(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(400, 400)), 0xFF, func(uid.UID64) { found++ })
	if found != 2 {
		t.Errorf("the space indexes %d bodies, want 2", found)
	}
}

func TestBodies_RemoveFreesTheBudgetSpawnOverItPanics(t *testing.T) {
	r := newBodyRig(t, 2)
	r.spawn = []plane.AABB{box(0, 0, 10, 10), box(20, 0, 10, 10)}
	r.tick()
	ids, _, _ := r.entities()

	remover := r.ecs.RegSys(goke.SystemFn{OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) { r.bodies.Remove(cb, ids[0]) }})
	r.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		rc.Run(remover, d)
		rc.Sync()
		r.w.RunPlan(rc, d)
		r.w.Clock().Replay(rc, d)
	})
	r.tick()
	if left, _, _ := r.entities(); len(left) != 1 {
		t.Fatalf("%d entities after Remove, want 1", len(left))
	}
	if got := r.w.Res.Telemetry.Count; got != 1 {
		t.Errorf("telemetry counts %d after Remove, want 1", got)
	}

	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, "MaxCount") {
			t.Errorf("spawning past MaxCount panicked with %q, want a MaxCount message", msg)
		}
	}()
	spawner := r.ecs.RegSys(goke.SystemFn{OnUpdate: func(*goke.CmdBuf, time.Duration) {
		r.bodies.Spawn([]plane.AABB{box(0, 50, 10, 10), box(0, 70, 10, 10)}, func(int, uid.UID64, *goke.Cursor) {})
	}})
	r.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) { rc.Run(spawner, d) })
	r.tick()
}

func TestKinds_ReserveTakesAnIDLikeDefineAndRefusesADuplicateName(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	if got := w.Kinds().Reserve("terrain"); got != 0 {
		t.Errorf("first Reserve got ID %d, want 0", got)
	}
	rock := kind.Define[struct{}](w.Kinds(), "rock", kind.Spec{
		comp.Const(world.Position{AABB: box(0, 0, 10, 10)}),
		comp.Const(world.Velocity{}),
	})
	if rock.ID() != 1 {
		t.Errorf("Define after Reserve got ID %d, want 1", rock.ID())
	}
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, "twice") {
			t.Errorf("Reserve of a taken name panicked with %q, want a defined-twice message", msg)
		}
	}()
	w.Kinds().Reserve("rock")
}

// installCtx is the plugin.Installer a Stage would hand over, minus the engine.
type installCtx struct {
	ecs     *goke.ECS
	pending []func() []goke.System
}

func (c *installCtx) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}
func (c *installCtx) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *installCtx) RegSys(factory func() goke.System) goke.Runnable { return c.ecs.RegSys(factory()) }
func (c *installCtx) ECS() *goke.ECS                                  { return c.ecs }
