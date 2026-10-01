package board_test

import (
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/uid"
)

// testInstallCtx is a minimal plugin.Installer for tests that call Install directly.
type testInstallCtx struct {
	ecs     *goke.ECS
	pending []func() []goke.System
}

func (c *testInstallCtx) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(si *goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}
func (c *testInstallCtx) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *testInstallCtx) RegSys(factory func() goke.System) goke.Runnable {
	return c.ecs.RegSys(factory())
}
func (c *testInstallCtx) ECS() *goke.ECS { return c.ecs }

func (c *testInstallCtx) flush() {
	var systems []goke.System
	for _, produce := range c.pending {
		systems = append(systems, produce()...)
	}
	c.ecs.Setup(systems...)
}

func TestKind_LoadWithEffect_EntersOccupancyOnSpawn(t *testing.T) {
	sqGrid := board.DefaultGrids{}.Square(5, 5, 10)
	occupancy := &board.SingleOccupancy{}
	target, _ := sqGrid.CellIndex(2, 2)

	cfg := world.Config{
		Space:    world.SpaceCfg{Width: 50, Height: 50},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 8, MaxSize: 8},
	}
	plugin := world.NewPlugin(cfg)
	placement := world.NewGridPlacement(50, 50, 8)
	unit := kind.Define[cell.ID](plugin.Kinds(), "unit", kind.Spec{
		comp.Const(placement.Place(0, 1)),
		comp.Const(world.Velocity{}),
		comp.Load(func(c cell.ID) board.At { return board.At{Cell: c} }).
			WithEffect(func(c board.At, id uid.UID64) { occupancy.Enter(c.Cell, id, cell.Land) }),
	})
	plugin.Seed(unit.Entry(target))
	if err := plugin.Populate(); err != nil {
		t.Fatalf("Populate: %v", err)
	}

	ctx := &testInstallCtx{ecs: goke.New()}
	if err := plugin.Install(ctx); err != nil {
		t.Fatalf("Install: %v", err)
	}

	var at goke.Comp[board.At]
	var q *goke.Query
	ctx.pending = append(ctx.pending, func() []goke.System {
		return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
			q = si.NewQueryBuilder(&at).Build()
		}}}
	})
	ctx.flush()

	q.All()
	found := false
	for q.Next() {
		cur := q.Cursor()
		cells := at.Slice(cur)
		for i := range cur.IDs {
			found = true
			if cells[i].Cell != target {
				t.Errorf("At.Cell = %v, want %v", cells[i].Cell, target)
			}
		}
	}
	if !found {
		t.Fatal("expected the spawned entity to exist")
	}
	if occupancy.CanEnter(target, uid.UID64(999), cell.Land) {
		t.Error("expected occupancy.Enter to have claimed the target cell for the spawned entity")
	}
}
