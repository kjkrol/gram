package board_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

type keptBoard struct {
	g         grid.Grid
	w         *world.Plugin
	occupancy *cell.SingleOccupancy
	walker    kind.Of[cell.ID]
	ecs       *goke.ECS
	units     *goke.Query
	base      goke.Comp[world.Base]
	z         goke.Comp[world.Z]
	at        goke.Comp[unit.At]
	states    goke.Comp[tag.Tags[unit.States]]
}

func newKeptBoard(t *testing.T, seeded ...[2]uint32) *keptBoard {
	t.Helper()
	k := &keptBoard{g: grid.DefaultGrids{}.Square(5, 3, 10), occupancy: &cell.SingleOccupancy{}}
	k.w = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 50, Height: 30},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 1, MaxSize: 10},
		Heights:  true,
	})
	m := motion.NewPlugin(k.w)
	p := board.NewPlugin(k.g, k.occupancy, k.w).WithMotion(m)
	p.Write().Board().SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	west, _ := grid.Direction(k.g, -1, 0)
	east, _ := grid.Direction(k.g, 1, 0)
	decks := board.NewEntities(p, board.Shape{Size: 10}, func(c cell.ID) cell.ID { return c })
	decks.Define("deck", comp.Const(cell.Floor{
		Allows:    cell.Land,
		Height:    10,
		Thickness: 1,
		Links:     1<<west | 1<<east,
		Width:     6,
	}))
	units := board.NewUnits(p, board.Shape{Size: 2, Height: 2},
		func(c cell.ID) geom.Vec { return k.g.CellCenter(c) })
	units.Define("walker", unit.Mover{Domain: cell.Land}, steering.Steering{MaxSpeed: 10})
	k.walker = units.Named("walker")
	for x := range uint32(3) {
		k.w.Write().Seed(decks.Named("deck").Entry(k.g.At(x+1, 1)))
	}
	for _, at := range seeded {
		k.w.Write().Seed(k.walker.Entry(k.g.At(at[0], at[1])))
	}
	ctx := boardtest.NewInstallCtx()
	if err := k.w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := k.w.Populate(); err != nil {
		t.Fatal(err)
	}
	k.ecs = ctx.ECS()
	k.ecs.Setup(append(ctx.Systems(), goke.SystemFn{OnInit: func(si *goke.SysInit) {
		k.units = si.NewQueryBuilder(&k.base, &k.z, &k.at, &k.states).Build()
	}})...)
	k.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		k.w.RunPlan(rc, d)
		m.RunPlan(rc, d)
		p.RunPlan(rc, d)
		rc.Sync()
		k.w.Write().Clock().Replay(rc, d)
	})
	return k
}

func (k *keptBoard) tick() { k.ecs.Tick(time.Second / 60) }

func (k *keptBoard) each(f func(uid.UID64, *world.Base, *world.Z, *unit.At,
	*tag.Tags[unit.States])) {
	for k.units.All(); k.units.Next(); {
		cur := k.units.Cursor()
		bases, zs, ats, states := k.base.Slice(cur), k.z.Slice(cur), k.at.Slice(cur),
			k.states.Slice(cur)
		for i, id := range cur.IDs {
			f(id, &bases[i], &zs[i], &ats[i], &states[i])
		}
	}
}

func (k *keptBoard) holders(c cell.ID) []uid.UID64 {
	var ids []uid.UID64
	k.occupancy.Holders(c, func(e uid.UID64, _ cell.Domain, _ bool) { ids = append(ids, e) })
	return ids
}

func TestOccupancy_HoldsAUnitsCellOnTheLevelOfItsFeet(t *testing.T) {
	k := newKeptBoard(t, [2]uint32{2, 1})
	mid := k.g.At(2, 1)
	standAt := func(alt float64) {
		k.each(func(_ uid.UID64, _ *world.Base, z *world.Z, _ *unit.At, _ *tag.Tags[unit.States]) {
			z.Altitude = alt
		})
		k.tick()
	}
	const other = 99
	standAt(10)
	if k.occupancy.CanEnter(mid, true, other, cell.Land) ||
		!k.occupancy.CanEnter(mid, false, other, cell.Land) {
		t.Error("a unit standing on the deck holds its cell's ground, not the deck")
	}
	standAt(0)
	if !k.occupancy.CanEnter(mid, true, other, cell.Land) ||
		k.occupancy.CanEnter(mid, false, other, cell.Land) {
		t.Error("a unit fallen off the deck into its cell still holds the deck, not the ground")
	}
}

func TestOccupancy_HoldsTheCellOfAUnitSpawnedInTheGame(t *testing.T) {
	k := newKeptBoard(t)
	k.tick()
	c := k.g.At(2, 0)
	k.w.Write().Spawn(k.walker.Entry(c))
	k.tick()
	if got := k.holders(c); len(got) != 1 {
		t.Fatalf("the cell a unit was spawned on is held by %v, want the unit alone", got)
	}
	k.each(func(id uid.UID64, _ *world.Base, _ *world.Z, at *unit.At, _ *tag.Tags[unit.States]) {
		if at.Cell != c || k.holders(c)[0] != id {
			t.Errorf("unit %d is At %v, the cell held by %v", id, at.Cell, k.holders(c))
		}
	})
}

func TestOccupancy_FollowsAUnitMovedByNoneWhoKeepsItsCell(t *testing.T) {
	k := newKeptBoard(t, [2]uint32{1, 0})
	from := k.g.At(1, 0)
	to := k.g.At(2, 0)
	k.tick()
	k.each(func(_ uid.UID64, b *world.Base, _ *world.Z, _ *unit.At, _ *tag.Tags[unit.States]) {
		b.Pos.AABB = boardtest.CellBox(k.g, to, 2)
	})
	k.tick()
	k.each(func(id uid.UID64, _ *world.Base, _ *world.Z, at *unit.At, s *tag.Tags[unit.States]) {
		if at.Cell != to {
			t.Errorf("the unit moved is At %v, want the cell it stands on, %v", at.Cell, to)
		}
		if !s.Has(unit.Entered) {
			t.Error("the unit moved into another cell has not entered it")
		}
		if got := k.holders(to); len(got) != 1 || got[0] != id {
			t.Errorf("the cell it stands on is held by %v, want it", got)
		}
	})
	if got := k.holders(from); len(got) != 0 {
		t.Errorf("the cell it left is still held by %v", got)
	}
}

func TestOccupancy_LeavesAUnitInACellItHolds(t *testing.T) {
	k := newKeptBoard(t, [2]uint32{1, 0})
	from := k.g.At(1, 0)
	to := k.g.At(2, 0)
	k.tick()
	k.each(func(id uid.UID64, b *world.Base, _ *world.Z, _ *unit.At, _ *tag.Tags[unit.States]) {
		k.occupancy.Enter(to, false, id, cell.Land)
		b.Pos.AABB = boardtest.CellBox(k.g, to, 2)
	})
	k.tick()
	k.each(func(_ uid.UID64, _ *world.Base, _ *world.Z, at *unit.At, _ *tag.Tags[unit.States]) {
		if at.Cell != from {
			t.Errorf("a unit in the middle of its step is At %v, want %v", at.Cell, from)
		}
	})
	if len(k.holders(from)) != 1 || len(k.holders(to)) != 1 {
		t.Errorf("the cells of its step are held by %v and %v, want it in both",
			k.holders(from), k.holders(to))
	}
}
