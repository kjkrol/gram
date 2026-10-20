package world

import (
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/uid"
)

type indexMark struct{}

func TestIndex_AMigrationInAStepLeavesTheSpaceFindingEveryEntityWhereItStands(t *testing.T) {
	wm := testWorld()
	ids, ecs, q, base := despawnWorld(t, wm, 3)
	at := []geom.Vec{geom.NewVec(100, 100), geom.NewVec(300, 300), geom.NewVec(500, 500)}
	for q.All(); q.Next(); {
		cursor := q.Cursor()
		bases := base.Slice(cursor)
		for i, id := range cursor.IDs {
			bases[i].Pos.AABB = plane.NewAABB(at[slices.Index(ids, id)], 10, 10)
		}
	}
	var mark goke.CompID
	handle := ecs.RegSys(goke.SystemFn{
		OnInit: func(si *goke.SysInit) { mark = si.RegComp[indexMark]() },
		// the first one leaves its table, the last moving into its slot
		OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) { cb.AddOne(ids[0], mark, indexMark{}) },
	})
	step := func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(handle, d)
		ctx.Sync()
	}
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		wm.RunPlan(ctx, d)
		wm.clock.Simulate(ctx, step)
		wm.clock.Replay(ctx, d)
	})
	ecs.Tick(time.Millisecond)

	for k, id := range ids {
		var found []uid.UID64
		wm.space.Query(geom.NewAABBAt(at[k], 10, 10), aabbworld.AnyCapability,
			func(id uid.UID64) { found = append(found, id) })
		if !slices.Equal(found, []uid.UID64{id}) {
			t.Errorf("at %v the space finds %v, want %v alone", at[k], found, id)
		}
	}
	var everywhere []uid.UID64
	wm.space.Query(geom.NewAABBAt(geom.NewVec(0, 0), 1000, 1000), aabbworld.AnyCapability,
		func(id uid.UID64) { everywhere = append(everywhere, id) })
	slices.Sort(everywhere)
	if want := slices.Sorted(slices.Values(ids)); !slices.Equal(everywhere, want) {
		t.Errorf("the space finds %v, want each of %v once", everywhere, ids)
	}
}
