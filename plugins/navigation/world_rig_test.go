package navigation

import (
	"fmt"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

// navUnit is one unit of a navWorld: where its box stands, the cell it starts in and holds, its
// steering profile (nil: not steered), its order (nil: none), whether it carries navigation's
// markers from the start, and its owner (none: the virtual player's, an ally of every ownerless
// unit — one standing makes way for it).
type navUnit struct {
	box     plane.AABB
	at      cell.ID
	profile *steering.Steering
	order   *MoveOrder
	marks   bool
	owner   control.PlayerID
}

// navWorld is a world, a board and navigation over a cols x rows grid of open land, units a cell
// apart (CellSpacing), stepped as a game steps them: the world, the board, navigation.
type navWorld struct {
	grid      grid.Grid
	board     *board.Board
	occupancy *cell.SingleOccupancy
	nav       *Plugin
	ecs       *goke.ECS
	ids       []uid.UID64 // the units', in the order given
	step      func(goke.RunCtx, time.Duration)
}

// newNavWorld builds the world and puts units on it, each holding its cell; after are set up last,
// to build what a test reads. Lay the ground through board before the first tick.
func newNavWorld(t *testing.T, cols, rows, cellSize uint32, units []navUnit, after ...goke.System) *navWorld {
	t.Helper()
	nw := &navWorld{grid: grid.DefaultGrids{}.Square(cols, rows, cellSize), occupancy: &cell.SingleOccupancy{}}
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: cols * cellSize, Height: rows * cellSize},
		Entities: world.EntitiesCfg{MaxCount: max(len(units), 1), MinSize: 1, MaxSize: cellSize},
	})
	brd := board.NewPlugin(nw.grid, nw.occupancy, w)
	nw.board = brd.Res.Logic.Board
	nw.board.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	nw.nav = NewPlugin(brd, w, selection.NewPlugin(w)).WithSpacing(CellSpacing)
	if err := w.Carry(nw.nav); err != nil { // as the engine does with Use
		t.Fatal(err)
	}
	ctx := &stubInstallCtx{ecs: goke.New()}
	for _, install := range []func() error{
		func() error { return w.Install(ctx) },
		func() error { return brd.Install(ctx) },
		func() error { return nw.nav.Install(ctx) },
	} {
		if err := install(); err != nil {
			t.Fatal(err)
		}
	}
	kinds := make([]kind.ID, len(units))
	for i, u := range units {
		s := kind.Spec{
			comp.Load(func(u navUnit) world.Position { return world.Position{AABB: u.box} }),
			comp.Const(world.Velocity{}),
			comp.Load(func(u navUnit) unit.At { return unit.At{Cell: u.at} }),
		}
		if u.profile != nil {
			s = append(s, comp.Load(func(u navUnit) steering.Steering { return *u.profile }))
		}
		if u.order != nil {
			s = append(s, comp.Load(func(u navUnit) MoveOrder { return *u.order }))
		}
		if u.marks {
			s = append(s, comp.Marks[States]())
		}
		if u.owner != control.Nobody {
			s = append(s, comp.Tagged(owner.Of(u.owner)))
		}
		kind.Define[navUnit](w.Kinds(), fmt.Sprintf("u%d", i), s)
		k := kind.Named[navUnit](w.Kinds(), fmt.Sprintf("u%d", i))
		kinds[i] = k.ID()
		w.Seed(k.Entry(u))
	}
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	nw.ids = make([]uid.UID64, len(units))
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var base goke.Comp[world.Base]
		var at goke.Comp[unit.At]
		q := si.NewQueryBuilder(&base, &at).Build()
		for q.All(); q.Next(); {
			cur := q.Cursor()
			for i, id := range cur.IDs {
				nw.occupancy.Enter(at.Slice(cur)[i].Cell, id, cell.Land)
				for u, k := range kinds {
					if base.Slice(cur)[i].TypeID == k {
						nw.ids[u] = id
					}
				}
			}
		}
	}})
	ctx.ecs.Setup(append(systems, after...)...)
	nw.step = func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		brd.RunPlan(rc, d)
		nw.nav.RunPlan(rc, d)
		rc.Sync()
		w.Clock().Replay(rc, d)
	}
	ctx.ecs.SetPlan(nw.step)
	nw.ecs = ctx.ecs
	return nw
}
