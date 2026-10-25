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

// the chunk keeps its length and its memory: only the table's version tells the slot changed hands
func TestIndex_ANewcomerTakingTheSlotOfOneGoneInTheSameStepIsFoundWhereItStands(t *testing.T) {
	p, things, ecs, q, base := spawnWorld(t, 0, 3)
	wm := p.module
	p.write.Spawn(
		things.Entry(thing{x: 100, y: 100, size: 10}),
		things.Entry(thing{x: 300, y: 100, size: 10}),
	)
	run(ecs, wm, func(*goke.CmdBuf) {})
	at := map[float32]uid.UID64{}
	for q.All(); q.Next(); {
		cursor := q.Cursor()
		for i, b := range base.Slice(cursor) {
			at[b.Pos.TopLeft.X] = cursor.IDs[i]
		}
	}
	finds := func(x float32) []uid.UID64 {
		var found []uid.UID64
		wm.space.Query(geom.NewAABBAt(geom.NewVec(x, 100), 10, 10), aabbworld.AnyCapability,
			func(id uid.UID64) { found = append(found, id) })
		return found
	}
	// a query builds the cells, which a later one reads until the space is told of a change
	if found := finds(300); !slices.Equal(found, []uid.UID64{at[300]}) {
		t.Fatalf("at 300 the space finds %v, want %v", found, at[300])
	}
	p.write.Spawn(things.Entry(thing{x: 800, y: 100, size: 10}))
	wm.commands.PutFrom(at[300], Despawn{})
	run(ecs, wm, func(*goke.CmdBuf) {})

	var newcomer uid.UID64
	for q.All(); q.Next(); {
		cursor := q.Cursor()
		for i, b := range base.Slice(cursor) {
			if b.Pos.TopLeft.X == 800 {
				newcomer = cursor.IDs[i]
			}
		}
	}
	for x, want := range map[float32][]uid.UID64{100: {at[100]}, 300: nil, 800: {newcomer}} {
		if found := finds(x); !slices.Equal(found, want) {
			t.Errorf("at %v the space finds %v, want %v", x, found, want)
		}
	}
}
