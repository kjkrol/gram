package navigation

import (
	"fmt"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// an ownerless unit is the virtual player's, an ally of every other ownerless unit
type navUnit struct {
	box     plane.AABB
	at      cell.ID
	profile *steering.Steering
	order   *MoveOrder
	marks   bool
	owner   command.Issuer
}

type navWorld struct {
	grid      grid.Grid
	board     testBoard
	occupancy *cell.SingleOccupancy
	nav       *Plugin
	ecs       *goke.ECS
	ids       []uid.UID64 // the units', in the order given
	step      func(goke.RunCtx, time.Duration)
}

// after are set up last, to build what a test reads

// testBoard is a board as the tests lay and read it at once
type testBoard struct {
	*board.BoardRead
	*board.BoardWrite
}

// the board of p as the tests lay and read it
func boardOf(p *board.Plugin) testBoard { return testBoard{p.Read(0).Board(), p.Write().Board()} }

func newNavWorld(t *testing.T, cols, rows, cellSize uint32, units []navUnit, after ...goke.System) *navWorld {
	t.Helper()
	return newNavWorldWith(t, cols, rows, cellSize, units, nil, after...)
}

// prepare is handed the world and the board before they are installed
func newNavWorldWith(t *testing.T, cols, rows, cellSize uint32, units []navUnit,
	prepare func(*world.Plugin, *board.Plugin), after ...goke.System) *navWorld {
	t.Helper()
	return newNavWorldIn(t, cols, rows, cellSize, units, world.SectorsCfg{}, prepare, after...)
}

// the world cut into sectors as given
func newNavWorldIn(t *testing.T, cols, rows, cellSize uint32, units []navUnit,
	sectors world.SectorsCfg, prepare func(*world.Plugin, *board.Plugin),
	after ...goke.System) *navWorld {
	t.Helper()
	nw := &navWorld{grid: grid.DefaultGrids{}.Square(cols, rows, cellSize), occupancy: &cell.SingleOccupancy{}}
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: cols * cellSize, Height: rows * cellSize},
		Entities: world.EntitiesCfg{MaxCount: len(units) + 8, MinSize: 1, MaxSize: cellSize},
		Heights:  true,
		Sectors:  sectors,
	})
	m := motion.NewPlugin(w)
	brd := board.NewPlugin(nw.grid, nw.occupancy, w).WithMotion(m)
	nw.board = testBoard{brd.Read(0).Board(), brd.Write().Board()}
	nw.board.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	sel := selection.NewPlugin(w)
	nw.nav = NewPlugin(brd, w, m, sel, driving.NewPlugin(w, m, sel)).WithSpacing(CellSpacing)
	if prepare != nil {
		prepare(w, brd)
	}
	if err := w.Carry(nw.nav); err != nil { // as the engine does with Use
		t.Fatal(err)
	}
	ctx := &stubInstallCtx{ecs: goke.New()}
	for _, install := range []func() error{
		func() error { return w.Install(ctx) },
		func() error { return m.Install(ctx) },
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
			s = append(s, comp.Marks[unit.States]())
		}
		if u.owner != command.Nobody {
			s = append(s, comp.Tagged(owner.Of(u.owner)))
		}
		kind.Define[navUnit](w.Kinds(), fmt.Sprintf("u%d", i), s)
		k := kind.Named[navUnit](w.Kinds(), fmt.Sprintf("u%d", i))
		kinds[i] = k.ID()
		w.Write().Seed(k.Entry(u))
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
				nw.occupancy.Enter(at.Slice(cur)[i].Cell, false, id, cell.Land)
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
		m.RunPlan(rc, d)
		brd.RunPlan(rc, d)
		nw.nav.RunPlan(rc, d)
		rc.Sync()
		w.Write().Clock().Replay(rc, d)
	}
	ctx.ecs.SetPlan(nw.step)
	nw.ecs = ctx.ecs
	return nw
}
