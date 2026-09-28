package world

import (
	"github.com/kjkrol/aabbworld/geom"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/uid"
)

// despawnWorld spawns n entities and returns their ids with a live Position query.
func despawnWorld(t *testing.T, wm *module, n int) ([]uid.UID64, *goke.ECS, *goke.Query, *goke.Comp[Base]) {
	t.Helper()
	wm.populate(testKind(spawnerTestPos(), Velocity{}), make([]any, n))

	base := new(goke.Comp[Base])
	var q *goke.Query
	var ids []uid.UID64
	ecs := goke.New()
	ecs.Setup(append(wm.SetupSystems(), goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(base).Build()
		q.All()
		for q.Next() {
			ids = append(ids, q.Cursor().IDs...)
		}
	}})...)
	wm.RegSystems(ecs)
	return ids, ecs, q, base
}

// run ticks ecs once with act followed by the world's own plan.
func run(ecs *goke.ECS, wm *module, act func(*goke.CmdBuf)) {
	handle := ecs.RegSys(goke.SystemFn{OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) { act(cb) }})
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(handle, d)
		ctx.Sync()
		wm.RunPlan(ctx, d)
		wm.clock.Replay(ctx, d)
	})
	ecs.Tick(time.Millisecond)
}

func living(q *goke.Query) map[uid.UID64]bool {
	found := map[uid.UID64]bool{}
	q.All()
	for q.Next() {
		for _, id := range q.Cursor().IDs {
			found[id] = true
		}
	}
	return found
}

// indexed reports which of the space's entities a probe over the whole world still finds.
func indexed(space *aabbworld.Space) map[uid.UID64]bool {
	at := spawnerTestPos()
	box := geom.NewAABB(
		geom.NewVec(at.TopLeft.X-100, at.TopLeft.Y-100),
		geom.NewVec(at.TopLeft.X+at.Size.X+100, at.TopLeft.Y+at.Size.Y+100),
	)
	found := map[uid.UID64]bool{}
	space.Query(box, aabbworld.AnyCapability, func(id uid.UID64) {
		found[id] = true
	})
	return found
}

func TestDespawn_TakesTheEntityOutOfBothTheECSAndTheIndex(t *testing.T) {
	wm := testWorld()
	ids, ecs, q, _ := despawnWorld(t, wm, 3)

	gone := ids[1]
	run(ecs, wm, func(cb *goke.CmdBuf) { wm.despawn(cb, gone) })
	run(ecs, wm, func(*goke.CmdBuf) {})

	if alive := living(q); alive[gone] || len(alive) != 2 {
		t.Errorf("entities left = %v, want the two that were not despawned", alive)
	}
	if seen := indexed(wm.space); seen[gone] {
		t.Errorf("the spatial index still offers %v — a probe would keep finding a ghost", gone)
	}
	if wm.telemetry.Count != 2 {
		t.Errorf("telemetry.Count = %d, want 2", wm.telemetry.Count)
	}
}

func TestDespawn_TwiceInATickCountsOnce(t *testing.T) {
	wm := testWorld()
	ids, ecs, _, _ := despawnWorld(t, wm, 3)

	run(ecs, wm, func(cb *goke.CmdBuf) {
		wm.despawn(cb, ids[0])
		wm.despawn(cb, ids[0])
	})

	if wm.telemetry.Count != 2 {
		t.Errorf("telemetry.Count = %d, want 2 — the second ask was for an entity already gone", wm.telemetry.Count)
	}
}
