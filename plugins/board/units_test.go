package board_test

import (
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
)

type recruit struct{ start cell.ID }

// unitsWorld is world + collision + board with one kind made by Units, populated and set up.
func unitsWorld(t *testing.T, define func(units *board.Units[recruit]) kind.Of[recruit]) (*goke.ECS, *world.Plugin, grid.Grid) {
	t.Helper()
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 20, MaxSize: 20},
	})
	c := collision.NewPlugin(w)
	brd := board.NewPlugin(grid, &cell.MultipleOccupancy{}, w).WithCollision(c)
	brd.Res.Logic.Board.SetAll(cell.Kind{Cost: 1, Allows: cell.Land | cell.Water})
	units := board.NewUnits[recruit](brd, board.Shape{Size: 20}, func(r recruit) geom.Vec { return grid.CellCenter(r.start) })
	k := define(units)

	ctx := boardtest.NewInstallCtx()
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := brd.Install(ctx); err != nil {
		t.Fatal(err)
	}
	start, _ := grid.CellIndex(2, 1)
	w.Seed(k.Entry(recruit{start: start}))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	systems := ctx.Systems()
	ctx.ECS().Setup(systems...)
	return ctx.ECS(), w, grid
}

func TestUnits_DeriveThePositionAndTheCellFromOnePoint(t *testing.T) {
	ecs, _, grid := unitsWorld(t, func(units *board.Units[recruit]) kind.Of[recruit] {
		return units.Define("recruit", unit.Mover{Domain: cell.Water}, steering.Steering{MaxSpeed: 10})
	})
	var base goke.Comp[world.Base]
	var at goke.Comp[unit.At]
	var mover goke.Comp[unit.Mover]
	var layers goke.Comp[world.Layers]
	var steer goke.Comp[steering.Steering]
	var collider goke.Comp[collision.Collider]
	var q *goke.Query
	ecs.RegSys(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&base, &at, &mover, &layers, &steer, &collider).Build()
	}})

	found := 0
	for q.All(); q.Next(); {
		cur := q.Cursor()
		for i := range cur.IDs {
			found++
			want, _ := grid.CellIndex(2, 1)
			if at.Slice(cur)[i].Cell != want {
				t.Errorf("cell = %v, want the cell under the position, %v", at.Slice(cur)[i].Cell, want)
			}
			if c := base.Slice(cur)[i].Pos.Center(); c != grid.CellCenter(want) {
				t.Errorf("position centre = %v, want the cell's centre %v", c, grid.CellCenter(want))
			}
			if mover.Slice(cur)[i].Domain != cell.Water || layers.Slice(cur)[i] != world.Layers(cell.Water) {
				t.Errorf("mover %v, layers %08b, want both from Water", mover.Slice(cur)[i], layers.Slice(cur)[i])
			}
			if steer.Slice(cur)[i].MaxSpeed != 10 {
				t.Errorf("steering %+v, want the profile given", steer.Slice(cur)[i])
			}
		}
	}
	if found != 1 {
		t.Errorf("found %d units carrying every derived component and the collision defaults, want 1", found)
	}
}

func TestUnits_AUnitOffTheBoardPanicsWhenSpawned(t *testing.T) {
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, "off the board") {
			t.Errorf("panic %q, want one about standing off the board", msg)
		}
	}()
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 20, MaxSize: 20},
	})
	brd := board.NewPlugin(grid, &cell.MultipleOccupancy{}, w)
	units := board.NewUnits[recruit](brd, board.Shape{Size: 20}, func(recruit) geom.Vec { return geom.NewVec(-50, -50) })
	k := units.Define("stray", unit.Mover{Domain: cell.Land}, steering.Steering{})
	ctx := boardtest.NewInstallCtx()
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := brd.Install(ctx); err != nil {
		t.Fatal(err)
	}
	w.Seed(k.Entry(recruit{}))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	systems := ctx.Systems()
	ctx.ECS().Setup(systems...) // Setup runs the queued spawn, where the At is read off the position
	t.Error("a unit off the board spawned")
}
