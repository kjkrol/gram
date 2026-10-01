package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/uid"
)

// riverBoard is a 5x3 board with water across the middle row, except a ford at column 4.
func riverBoard() (board.Grid, *board.TerrainMap) {
	grid := board.DefaultGrids{}.Square(5, 3, 10)
	terrain := board.NewTerrainMap()
	terrain.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
	for x := uint32(0); x < 4; x++ {
		c, _ := grid.CellIndex(x, 1)
		terrain.Set(c, cell.Kind{Name: cell.Named("water"), Cost: 1, Allows: cell.Water})
	}
	return grid, terrain
}

func TestFindPath_KeepsEachDomainToItsOwnGround(t *testing.T) {
	grid, terrain := riverBoard()
	pf := newPathFinder(grid, terrain, nil, &board.MultipleOccupancy{})
	at := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }

	path, ok := pf.findPath(uid.UID64(1), cell.Land, at(0, 0), at(0, 2))
	if !ok {
		t.Fatal("a land unit found no way round the river")
	}
	for _, step := range path.Steps[:path.Length] {
		if terrain.Kind(step).Name.String() == "water" {
			t.Fatalf("a land unit's route crosses water at %v", step)
		}
	}
	if path.Length < 4 {
		t.Errorf("route of %d steps, want the detour through the ford", path.Length)
	}

	boat, ok := pf.findPath(uid.UID64(2), cell.Water, at(0, 1), at(3, 1))
	if !ok || boat.Length != 3 {
		t.Fatalf("a boat along the river: ok=%v, %d steps, want 3 straight", ok, boat.Length)
	}
	if _, ok := pf.findPath(uid.UID64(2), cell.Water, at(0, 1), at(0, 0)); ok {
		t.Error("a boat found a route onto grass")
	}

	amphibious := cell.Land | cell.Water
	if direct, ok := pf.findPath(uid.UID64(3), amphibious, at(0, 0), at(0, 2)); !ok || direct.Length != 2 {
		t.Errorf("an amphibious unit: ok=%v, %d steps, want 2 straight across", ok, direct.Length)
	}
}

func TestCommandSystem_Update_IgnoresATargetTheUnitsDomainMayNotEnter(t *testing.T) {
	grid := board.DefaultGrids{}.Square(10, 1, 10)
	terrain := board.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	start, _ := grid.CellIndex(0, 0)
	lake, _ := grid.CellIndex(8, 0)
	terrain.Set(lake, cell.Kind{Name: cell.Named("water"), Cost: 1, Allows: cell.Water})

	moves := &control.Queue[MoveTo]{}
	cmds := newMoveCommandSystem(newPathFinder(grid, terrain, nil, &board.SingleOccupancy{}), moves, &control.Queue[LookAt]{}, selTags.Selected)

	var at goke.Comp[board.At]
	var pos goke.Comp[world.Base]
	var mover goke.Comp[board.Mover]
	var selected goke.Comp[tag.Tags[selection.Family]]
	var order goke.OptComp[MoveOrder]
	var readQuery *goke.Query

	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&at, &pos, &mover, &selected)
		f.Create(1)
		f.Next()
		selected.Slice(&f.Cursor)[0] = selectedMarks
		at.Slice(&f.Cursor)[0] = board.At{Cell: start}
		pos.Slice(&f.Cursor)[0].Pos = world.Position{AABB: board.CellAABB(grid, start, 8)}
		mover.Slice(&f.Cursor)[0] = board.Mover{Domain: cell.Land}
		readQuery = si.NewQueryBuilder().Optional(&order).Build()
		cmds.Init(si)
	}})
	handle := ecs.RegSys(cmds)
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(handle, d)
		ctx.Sync()
	})

	moves.Add(control.Nobody, MoveTo{Cell: lake})
	ecs.Tick(time.Second)

	for readQuery.All(); readQuery.Next(); {
		if order.Present(readQuery.Cursor()) {
			t.Fatal("a land unit was ordered onto water")
		}
	}
}

func TestFindPath_PricesTheRouteForTheUnitsDomain(t *testing.T) {
	const frost = cell.Domain(1 << 3)
	grid := board.DefaultGrids{}.Square(3, 3, 10)
	terrain := board.NewTerrainMap()
	terrain.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land | frost})
	at := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	// The middle row is snow: slow for anyone on foot, a highway for the frost-born.
	for x := range uint32(3) {
		terrain.Set(at(x, 1), cell.Kind{Name: cell.Named("snow"), Cost: 5, Allows: cell.Land | frost}.Costing(frost, 0.2))
	}
	pf := newPathFinder(grid, terrain, nil, &board.MultipleOccupancy{})

	walker, _ := pf.findPath(uid.UID64(1), cell.Land, at(0, 0), at(2, 0))
	for _, step := range walker.Steps[:walker.Length] {
		if terrain.Kind(step).Name.String() == "snow" {
			t.Fatalf("a walker's route dips into the snow at %v", step)
		}
	}
	witch, _ := pf.findPath(uid.UID64(2), cell.Land|frost, at(0, 0), at(2, 0))
	onSnow := 0
	for _, step := range witch.Steps[:witch.Length] {
		if terrain.Kind(step).Name.String() == "snow" {
			onSnow++
		}
	}
	if onSnow == 0 {
		t.Error("the witch's route never takes the snow she is fast on")
	}
}
