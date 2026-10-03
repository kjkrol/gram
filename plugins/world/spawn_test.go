package world

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/uid"
)

// thing is the row the spawn tests' kind spawns from: where it stands and how big it is.
type thing struct {
	x, y, size float64
}

// spawnWorld is a world of the edges given holding at most n entities, with one kind, thing,
// defined, set up and ready to tick; the counts are read off the module.
func spawnWorld(t *testing.T, edges aabbworld.Edges, n int) (*Plugin, kind.Of[thing], *goke.ECS, *goke.Query, *goke.Comp[Base]) {
	t.Helper()
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 1000, Height: 1000, Edges: edges},
		Entities: EntitiesCfg{MaxCount: n, MinSize: 5, MaxSize: 20},
	})
	things := kind.Define[thing](p.Kinds(), "thing", kind.Spec{
		comp.Load(func(r thing) Position { return Position{AABB: plane.NewAABB(geom.NewVec(r.x, r.y), r.size, r.size)} }),
		comp.Const(Velocity{}),
	})
	base := new(goke.Comp[Base])
	var q *goke.Query
	ecs := goke.New()
	ecs.Setup(append(p.module.SetupSystems(), goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(base).Build()
	}})...)
	p.module.RegSystems(ecs)
	return p, things, ecs, q, base
}

// A Spawn given is an entity in the world at the next step: in the ECS, in the space and counted.
func TestSpawn_MakesAnEntityOfTheKindAtTheNextStep(t *testing.T) {
	p, things, ecs, q, _ := spawnWorld(t, 0, 4)
	wm := p.module
	if !wm.commands.Put(control.Nobody, Spawn{Entry: things.Entry(thing{x: 100, y: 100, size: 10})}) {
		t.Fatal("the world carries no Spawn")
	}
	p.Spawn(things.Entry(thing{x: 300, y: 300, size: 10}))
	run(ecs, wm, func(*goke.CmdBuf) {})
	if n := len(living(q)); n != 2 {
		t.Fatalf("%d entities after the step, want the two spawned", n)
	}
	if wm.spawnedCount != 2 || wm.telemetry.Count != 2 {
		t.Errorf("spawned %d, telemetry %d, want 2 and 2", wm.spawnedCount, wm.telemetry.Count)
	}
	found := 0
	wm.space.Query(geom.NewAABB(geom.NewVec(90, 90), geom.NewVec(120, 120)), aabbworld.AnyCapability, func(uid.UID64) { found++ })
	if found != 1 {
		t.Errorf("the space finds %d pieces where the first was spawned, want it", found)
	}
}

// A Spawn is refused, with nothing made, for a row of another type, a size out of bounds, a box
// wholly past an open edge, and once the world is full; the kind's entries past the room are
// refused, those before it made.
func TestSpawn_RefusesWhatTheWorldDoesNotTake(t *testing.T) {
	p, things, ecs, q, _ := spawnWorld(t, aabbworld.OpenX, 3)
	wm := p.module
	others := kind.Define[int](p.Kinds(), "other", kind.Spec{
		comp.Const(Position{AABB: plane.NewAABB(geom.NewVec(1, 1), 10, 10)}),
		comp.Const(Velocity{}),
	})
	wm.commands.Put(control.Nobody, Spawn{Entry: others.Entry(7)})                                // defined after the set-up: no factory
	wm.commands.Put(control.Nobody, Spawn{Entry: things.Entry(thing{x: 100, y: 100, size: 50})})  // too big
	wm.commands.Put(control.Nobody, Spawn{Entry: things.Entry(thing{x: 1200, y: 100, size: 10})}) // past the open edge
	wm.commands.Put(control.Nobody, Spawn{Entry: things.Entry(thing{x: 100, y: 100, size: 10})})  // the first taken
	wm.commands.Put(control.Nobody, Spawn{Entry: things.Entry(thing{x: 200, y: 100, size: 10})})  // the second
	wm.commands.Put(control.Nobody, Spawn{Entry: things.Entry(thing{x: 300, y: 100, size: 10})})  // the third
	wm.commands.Put(control.Nobody, Spawn{Entry: things.Entry(thing{x: 400, y: 100, size: 10})})  // one too many
	wm.commands.Put(control.Nobody, Spawn{Entry: things.Entry(thing{x: 990, y: 100, size: 10})})  // full too
	run(ecs, wm, func(*goke.CmdBuf) {})
	if n := len(living(q)); n != 3 {
		t.Errorf("%d entities after the step, want the three the world has room for", n)
	}
	if wm.spawnedCount != 3 || wm.telemetry.Count != 3 {
		t.Errorf("spawned %d, telemetry %d, want 3 and 3", wm.spawnedCount, wm.telemetry.Count)
	}
}

// A despawn makes room at once: a spawn waiting in the same step takes it, and the next finds
// the world full again.
func TestSpawn_FillsTheRoomADespawnLeaves(t *testing.T) {
	p, things, ecs, q, _ := spawnWorld(t, 0, 1)
	wm := p.module
	p.Spawn(things.Entry(thing{x: 100, y: 100, size: 10}))
	run(ecs, wm, func(*goke.CmdBuf) {})
	var first uid.UID64
	for id := range living(q) {
		first = id
	}
	p.Spawn(things.Entry(thing{x: 200, y: 100, size: 10}))
	run(ecs, wm, func(cb *goke.CmdBuf) { wm.despawn(cb, first) })
	alive := living(q)
	if len(alive) != 1 || alive[first] {
		t.Fatalf("%v alive after the despawn, want the one spawned in the room left, not %v", alive, first)
	}
	p.Spawn(things.Entry(thing{x: 300, y: 100, size: 10}))
	run(ecs, wm, func(*goke.CmdBuf) {})
	if n := len(living(q)); n != 1 {
		t.Errorf("%d entities after a spawn into the full world, want 1 still", n)
	}
	if wm.spawnedCount != 1 || wm.telemetry.Count != 1 {
		t.Errorf("spawned %d, telemetry %d, want 1 and 1", wm.spawnedCount, wm.telemetry.Count)
	}
}

// A component added to a spawned entity in the same step lands with the step's sync.
func TestSpawn_AnEntitySpawnedThisStepTakesAComponentAtOnce(t *testing.T) {
	p, things, ecs, q, _ := spawnWorld(t, 0, 2)
	wm := p.module
	var outside goke.CompID
	var marked *goke.Query
	ecs.RegSys(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		outside = si.RegComp[Outside]()
		marked = si.NewQueryBuilder(new(goke.Comp[Base])).Include(goke.Include[Outside]()).Build()
	}})
	p.Spawn(things.Entry(thing{x: 100, y: 100, size: 10}))
	run(ecs, wm, func(*goke.CmdBuf) {})
	handle := ecs.RegSys(goke.SystemFn{OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
		for id := range living(q) {
			cb.AddOne(id, outside, Outside{})
		}
	}})
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(handle, d)
		ctx.Sync()
	})
	ecs.Tick(time.Millisecond)
	if n := len(living(marked)); n != 1 {
		t.Errorf("%d entities carry the component added, want the spawned one", n)
	}
}

// After a load the room is counted from what was loaded, so a full world stays full.
func TestSpawn_TheRoomIsCountedAfterALoad(t *testing.T) {
	p, things, ecs, _, _ := spawnWorld(t, 0, 2)
	wm := p.module
	p.Spawn(things.Entry(thing{x: 100, y: 100, size: 10}), things.Entry(thing{x: 200, y: 100, size: 10}))
	run(ecs, wm, func(*goke.CmdBuf) {})
	path := t.TempDir() + "/save.bin"
	ecs.Pause()
	if err := ecs.Save(path); err != nil {
		t.Fatal(err)
	}

	loaded := NewPlugin(Config{
		Space:    SpaceCfg{Width: 1000, Height: 1000},
		Entities: EntitiesCfg{MaxCount: 2, MinSize: 5, MaxSize: 20},
	})
	kinds := kind.Define[thing](loaded.Kinds(), "thing", kind.Spec{
		comp.Load(func(r thing) Position { return Position{AABB: plane.NewAABB(geom.NewVec(r.x, r.y), r.size, r.size)} }),
		comp.Const(Velocity{}),
	})
	ecs2 := goke.New()
	if err := ecs2.Load(path, loaded.module.LoadComps()...); err != nil {
		t.Fatal(err)
	}
	base := new(goke.Comp[Base])
	var q *goke.Query
	ecs2.Setup(append(loaded.module.SetupSystems(), loaded.module.PostLoad(), goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(base).Build()
	}})...)
	loaded.module.RegSystems(ecs2)
	if loaded.module.spawnedCount != 2 {
		t.Fatalf("after the load %d are counted as spawned, want the 2 loaded", loaded.module.spawnedCount)
	}
	loaded.Spawn(kinds.Entry(thing{x: 300, y: 100, size: 10}))
	run(ecs2, loaded.module, func(*goke.CmdBuf) {})
	if n := len(living(q)); n != 2 {
		t.Errorf("%d entities after a spawn into the loaded full world, want the 2 loaded alone", n)
	}
}

// At Setup a row of a size outside the bounds still panics, before any entity is made.
func TestSpawn_PopulateStillRefusesASizeOutOfBounds(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a seed of a size out of bounds went unrefused")
		}
	}()
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 1000, Height: 1000},
		Entities: EntitiesCfg{MaxCount: 2, MinSize: 5, MaxSize: 20},
	})
	things := kind.Define[thing](p.Kinds(), "thing", kind.Spec{
		comp.Load(func(r thing) Position { return Position{AABB: plane.NewAABB(geom.NewVec(r.x, r.y), r.size, r.size)} }),
		comp.Const(Velocity{}),
	})
	p.Seed(things.Entry(thing{x: 100, y: 100, size: 50}))
	if err := p.Populate(); err != nil {
		t.Fatal(err)
	}
	goke.New().Setup(p.module.SetupSystems()...)
}
