package navigation

import (
	"fmt"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

// The scene of the reported bug: a wall column on the left, a red unit standing still, a blue
// unit above it ordered below it and, on its way past, ordered back to where it started.
type unitRow struct {
	start   board.CellID
	target  board.CellID
	ordered bool
}

// turnaroundWorld is world + collision + board + navigation over a 3x4 board, as the demo runs them.
type turnaroundWorld struct {
	t     *testing.T
	grid  board.Grid
	ecs   *goke.ECS
	nav   *Plugin
	cell  goke.Comp[board.Cell]
	base  goke.Comp[world.Base]
	order goke.OptComp[MoveOrder]
	q     *goke.Query
	blue  uid.UID64
}

func newTurnaroundWorld(t *testing.T, collide bool) *turnaroundWorld {
	t.Helper()
	const size = 32
	tw := &turnaroundWorld{t: t, grid: board.DefaultGrids{}.Square(3, 4, size)}
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 3 * size, Height: 4 * size},
		Entities: world.EntitiesCfg{MaxCount: 2, MinSize: 22, MaxSize: 22},
	})
	occupancy := &board.SingleOccupancy{}
	brd := board.NewPlugin(tw.grid, occupancy, w)
	brd.Res.Logic.Board.SetAll(board.CellKind{Cost: 2, Allows: board.Land}) // grass, as in the demo
	for y := uint32(0); y < 4; y++ {
		brd.Res.Logic.Board.Set(tw.at(0, y), board.CellKind{Cost: 1, Solid: true})
	}
	sel := selection.NewPlugin(w)
	tw.nav = NewPlugin(brd, w, sel)
	var c *collision.Plugin
	if collide {
		c = collision.NewPlugin(w)
	}

	ctx := &stubInstallCtx{ecs: goke.New()}
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := brd.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if c != nil {
		if err := c.Install(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.nav.Install(ctx); err != nil {
		t.Fatal(err)
	}

	spec := func(ordered bool) kind.Spec {
		s := kind.Spec{
			comp.Load(func(u unitRow) world.Position { return world.Position{AABB: board.CellAABB(tw.grid, u.start, 22)} }),
			comp.Const(world.Velocity{}),
			comp.Const(steering.Steering{MaxSpeed: 64, Accel: 128, V0: 32, TurnRate: 0.15}),
			comp.Load(func(u unitRow) board.Cell { return board.Cell{ID: u.start} }).
				WithEffect(func(c board.Cell, id uid.UID64) { occupancy.Enter(c.ID, id, board.Land) }),
			comp.Const(collision.Collider{}),
			comp.Const(collision.Physics{}),
		}
		if ordered {
			s = append(s, comp.Tagged(sel.Tags().Selectable, sel.Tags().Selected),
				comp.Load(func(u unitRow) MoveOrder { return MoveOrder{Target: u.target} }))
		}
		return s
	}
	red := kind.Define[unitRow](w.Kinds(), "red", spec(false))
	blue := kind.Define[unitRow](w.Kinds(), "blue", spec(true))
	w.Seed(red.Entry(unitRow{start: tw.at(1, 2)}), blue.Entry(unitRow{start: tw.at(1, 1), target: tw.at(1, 3), ordered: true}))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}

	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		tw.q = si.NewQueryBuilder(&tw.cell, &tw.base).Optional(&tw.order).Build()
	}})
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		if c != nil {
			c.RunPlan(rc, d)
		}
		tw.nav.RunPlan(rc, d)
		rc.Sync()
		w.Clock().Replay(rc, d)
	})
	tw.ecs = ctx.ecs
	for tw.q.All(); tw.q.Next(); {
		cur := tw.q.Cursor()
		for i, id := range cur.IDs {
			if tw.order.Slice(cur) != nil && tw.base.Slice(cur)[i].TypeID == blue.ID() {
				tw.blue = id
			}
		}
	}
	return tw
}

func (tw *turnaroundWorld) at(x, y uint32) board.CellID { c, _ := tw.grid.CellIndex(x, y); return c }

// blueState is the blue unit's cell, its order if any, and its route.
func (tw *turnaroundWorld) blueState() (cell board.CellID, mt *MoveOrder) {
	for tw.q.All(); tw.q.Next(); {
		cur := tw.q.Cursor()
		for i, id := range cur.IDs {
			if id != tw.blue {
				continue
			}
			cell = tw.cell.Slice(cur)[i].ID
			if orders := tw.order.Slice(cur); orders != nil {
				o := orders[i]
				mt = &o
			}
		}
	}
	return
}

// runTurnaround orders blue back home after `after` ticks on its way down and reports how it went.
func runTurnaround(t *testing.T, collide bool, after int) (replans int, err string) {
	t.Helper()
	tw := newTurnaroundWorld(t, collide)
	a := tw.at(1, 1)

	for range after {
		tw.ecs.Tick(time.Second / 60)
	}
	if _, mt := tw.blueState(); mt == nil {
		return 0, "done" // already arrived below red: nothing to turn around from
	}
	tw.nav.moves.Add(control.Nobody, MoveTo{Cell: a})
	var last []board.CellID
	for tick := range 60 * 10 {
		tw.ecs.Tick(time.Second / 60)
		cell, mt := tw.blueState()
		if mt == nil {
			if cell != a {
				return replans, fmt.Sprintf("tick %d: order done at %v, want %v", tick, cell, a)
			}
			return replans, ""
		}
		steps := append([]board.CellID(nil), mt.Path.Steps[:mt.Path.Length]...)
		if !equalSteps(steps, last) {
			replans++
			last = steps
		}
	}
	return replans, fmt.Sprintf("never got back to %v in 10 s", a)
}

func sweepTurnaround(t *testing.T, collide bool) {
	t.Helper()
	worst := 0
	for after := 1; after < 60*8; after++ {
		replans, err := runTurnaround(t, collide, after)
		if err == "done" {
			break
		}
		if err != "" {
			t.Fatalf("ordered home after %d ticks: %s (%d re-plans)", after, err, replans)
		}
		worst = max(worst, replans)
	}
	if worst > 3 {
		t.Errorf("the worst turnaround took %d re-plans, want a handful at most", worst)
	}
}

func TestNavigation_TurnaroundBesideAWallGetsHome(t *testing.T) {
	sweepTurnaround(t, true)
}

func TestNavigation_TurnaroundBesideAWallGetsHome_NoCollision(t *testing.T) {
	sweepTurnaround(t, false)
}
