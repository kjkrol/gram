package field_test

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
)

var east = geom.NewVec(1, 0)

// wall is a solid kind that also cuts sight, as a wall of stone does.
var wall = boardtest.Wall

func TestGround_AWallColumnIsSolidAndAUnitCannotEnterIt(t *testing.T) {
	bw, _ := boardtest.SquareWorld(t, boardtest.Mover{Heading: east})
	walls := bw.Solid(world.Layers(cell.Land))
	if len(walls) != 14 {
		t.Fatalf("%d solid cells, want the 14 of the wall", len(walls))
	}
	var units []geom.AABB
	for tick := range 60 {
		bw.Tick()
		units = bw.Snapshot()
		bw.AssertClear(tick, units, walls)
	}
	if units[0].BottomRight.X < walls[0].TopLeft.X-1 {
		t.Errorf("unit ends at %v, never reached the wall at %v", units[0], walls[0].TopLeft.X)
	}
}

func TestGround_AUnitPushedByAnotherStaysOutOfTheWall(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(6, 16, boardtest.CellSize)
	cellAt := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	bw, _ := boardtest.SquareWorld(t, boardtest.Mover{Here: cellAt(2, 7)}, boardtest.Mover{Here: cellAt(1, 7), Heading: east})
	walls := bw.Solid(world.Layers(cell.Land))
	for tick := range 120 {
		bw.Tick()
		bw.AssertClear(tick, bw.Snapshot(), walls)
	}
	units := bw.Snapshot()
	if start := float64(boardtest.CellSize) + (boardtest.CellSize-boardtest.UnitSize)/2; units[1].TopLeft.X < start+5 {
		t.Errorf("the pusher never moved: %v behind %v", units[1], units[0])
	}
}

func TestGround_AHexIsCoveredAndKeepsAUnitOut(t *testing.T) {
	grid := grid.DefaultGrids{}.Hex(4, 4, boardtest.CellSize)
	hex, _ := grid.CellIndex(1, 1)
	start, _ := grid.CellAt(geom.NewVec(20, grid.CellCenter(hex).Y))
	bw := boardtest.NewWorld(t, grid, 320, 256, func(brd *board.Board) {
		brd.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
		brd.Set(hex, cell.Kind{Name: cell.Named("rock"), Solid: true})
	}, []boardtest.Mover{{Here: start, Heading: east}})
	walls := bw.Solid(world.Layers(cell.Land))
	if want := len(grid.CellBoxes(hex, nil)); len(walls) != want {
		t.Fatalf("%d solid boxes for one hex, want the %d covering it", len(walls), want)
	}
	for tick := range 90 {
		bw.Tick()
		bw.AssertClear(tick, bw.Snapshot(), walls)
	}
	units := bw.Snapshot()
	center := grid.CellCenter(hex)
	if units[0].BottomRight.X < center.X-math.Sqrt(3)/2*boardtest.CellSize-1 {
		t.Errorf("unit ends at %v, never reached the hex round %v", units[0], center)
	}
}

// Knocking a cell out of the wall opens it on the next tick, and the world gains no entity.
func TestGround_AGapKnockedInTheWallLetsAUnitThroughOnTheNextTick(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(6, 16, boardtest.CellSize)
	cellAt := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	bw, gap := boardtest.SquareWorld(t, boardtest.Mover{Here: cellAt(1, 7), Heading: east})
	bw.Tick()
	before := bw.World.Res.Telemetry.Count
	bw.Board.Res.Logic.Board.Set(gap, cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
	if got := len(bw.Solid(world.Layers(cell.Land))); got != 13 {
		t.Fatalf("%d solid cells after knocking one out, want 13", got)
	}
	for range 90 {
		bw.Tick()
	}
	if units := bw.Snapshot(); units[0].TopLeft.X < float64(4*boardtest.CellSize) {
		t.Errorf("unit ends at %v, want it through the gap", units[0])
	}
	if got := bw.World.Res.Telemetry.Count; got != before || got != 1 {
		t.Errorf("telemetry counts %d entities, %d before the change; want the one unit", got, before)
	}
}

func TestGround_AStrikeOnTheWallIsAContactWithTheTerrain(t *testing.T) {
	var hits []collision.Contact
	bw, gap := boardtest.SquareWorld(t, boardtest.Mover{Heading: east})
	for range 60 {
		bw.Tick()
		hits = append(hits, bw.Struck()...)
	}
	if len(hits) == 0 {
		t.Fatal("the unit drove into the wall and struck nothing")
	}
	h := hits[len(hits)-1]
	if !h.Terrain || h.Other != 0 || h.Cell != uint64(gap) || h.Normal != geom.NewVec(-1, 0) {
		t.Errorf("last contact %+v, want the terrain at cell %d, pushing west", h, gap)
	}
}

func TestGround_AWallThatVeilsCutsSight(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(6, 16, boardtest.CellSize)
	cellAt := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	observer := boardtest.Mover{Here: cellAt(1, 7), Sight: &vision.Sight{Facing: east, Radius: 300}, Eye: world.Eye{Angle: 2 * math.Pi / 8}}
	target := boardtest.Mover{Here: cellAt(5, 7)}

	bw, _ := boardtest.SquareWorld(t, observer, target)
	bw.Tick()
	if seen, ok := bw.Seen(); !ok || seen.Count != 0 {
		t.Errorf("saw %v through the wall, want nobody", seen.IDs[:seen.Count])
	}

	open := boardtest.NewWorld(t, grid, 6*boardtest.CellSize, 16*boardtest.CellSize, func(brd *board.Board) {
		brd.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
	}, []boardtest.Mover{observer, target})
	open.Tick()
	if seen, ok := open.Seen(); !ok || seen.Count != 1 {
		t.Errorf("saw %d across open ground, want 1", seen.Count)
	}
}

// Solid and Veil are apart: a fence stops walkers and hides nothing.
func TestGround_ASolidCellWithNoVeilLetsSightThrough(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(6, 16, boardtest.CellSize)
	cellAt := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	fence := func(brd *board.Board) {
		brd.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
		for y := uint32(1); y <= 14; y++ {
			brd.Set(cellAt(3, y), cell.Kind{Name: cell.Named("fence"), Cost: 1, Solid: true})
		}
	}
	observer := boardtest.Mover{Here: cellAt(1, 7), Sight: &vision.Sight{Facing: east, Radius: 300}, Eye: world.Eye{Angle: 2 * math.Pi / 8}}
	bw := boardtest.NewWorld(t, grid, 6*boardtest.CellSize, 16*boardtest.CellSize, fence, []boardtest.Mover{observer, {Here: cellAt(5, 7)}})
	bw.Tick()
	if seen, ok := bw.Seen(); !ok || seen.Count != 1 {
		t.Errorf("saw %d across the fence, want the target", seen.Count)
	}
	if got := len(bw.Solid(world.Layers(cell.Land))); got != 14 {
		t.Errorf("%d solid fence cells, want 14", got)
	}
}

// forestColumn is grass with a forest down column 3, veiling sight by veil, solid when solid.
func forestColumn(grid grid.Grid, veil float64, solid bool) func(*board.Board) {
	cellAt := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	return func(brd *board.Board) {
		brd.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
		for y := uint32(1); y <= 14; y++ {
			brd.Set(cellAt(3, y), cell.Kind{Name: cell.Named("forest"), Cost: 1, Allows: cell.Land, Solid: solid, Veil: veil, Veils: cell.Land})
		}
	}
}

func TestGround_AFullyVeiledCellOnlyBlocksSight(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(6, 16, boardtest.CellSize)
	cellAt := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	forest := forestColumn(grid, 1, false)
	observer := boardtest.Mover{Here: cellAt(1, 7), Sight: &vision.Sight{Facing: east, Radius: 300}, Eye: world.Eye{Angle: 2 * math.Pi / 8}}
	bw := boardtest.NewWorld(t, grid, 6*boardtest.CellSize, 16*boardtest.CellSize, forest, []boardtest.Mover{observer, {Here: cellAt(5, 7)}})
	bw.Tick()
	if seen, ok := bw.Seen(); !ok || seen.Count != 0 {
		t.Errorf("saw %v through the forest, want nobody", seen.IDs[:seen.Count])
	}
	if got := len(bw.Solid(world.Layers(cell.Land))); got != 0 {
		t.Errorf("%d solid forest cells, want none", got)
	}

	walker := boardtest.NewWorld(t, grid, 6*boardtest.CellSize, 16*boardtest.CellSize, forest, []boardtest.Mover{{Here: cellAt(1, 7), Heading: east}})
	for range 90 {
		walker.Tick()
	}
	if units := walker.Snapshot(); units[0].TopLeft.X < float64(4*boardtest.CellSize) {
		t.Errorf("unit ends at %v, want it past the forest column", units[0])
	}
}

// A forest column one cell (32) thick at Veil 0.6 costs 80 of reach: the target two cells past it
// is 165 away in budget terms and 117 as the crow flies.
func TestGround_AVeilDimsSightByItsDepth(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(6, 16, boardtest.CellSize)
	cellAt := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	forest := forestColumn(grid, 0.6, false)
	target := boardtest.Mover{Here: cellAt(5, 7)}
	look := func(radius float64, blockers world.Layers) uint8 {
		observer := boardtest.Mover{Here: cellAt(1, 7), Sight: &vision.Sight{Facing: east, Radius: radius, Blockers: blockers}, Eye: world.Eye{Angle: 2 * math.Pi / 8}}
		bw := boardtest.NewWorld(t, grid, 6*boardtest.CellSize, 16*boardtest.CellSize, forest, []boardtest.Mover{observer, target})
		bw.Tick()
		seen, ok := bw.Seen()
		if !ok {
			t.Fatal("no observer")
		}
		return seen.Count
	}

	if n := look(160, 0); n != 0 {
		t.Errorf("at 160 through the forest saw %d, want nobody", n)
	}
	if n := look(170, 0); n != 1 {
		t.Errorf("at 170 through the forest saw %d, want the target", n)
	}
	if n := look(160, world.Layers(cell.Air)); n != 1 {
		t.Errorf("at 160 looking over the forest from Air saw %d, want the target", n)
	}
}

// A Warcraft forest is solid and veiled at once: nobody walks in, sight is dimmed; cut down, it
// lets both through on the next tick.
func TestGround_AForestThatIsSolidAndVeiledStopsAndDimsUntilItIsCut(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(6, 16, boardtest.CellSize)
	cellAt := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	observer := boardtest.Mover{Here: cellAt(1, 3), Sight: &vision.Sight{Facing: east, Radius: 160}, Eye: world.Eye{Angle: 2 * math.Pi / 16}}
	bw := boardtest.NewWorld(t, grid, 6*boardtest.CellSize, 16*boardtest.CellSize, forestColumn(grid, 0.6, true),
		[]boardtest.Mover{observer, {Here: cellAt(5, 3)}, {Here: cellAt(1, 9), Heading: east}})
	walls := bw.Solid(world.Layers(cell.Land))
	for tick := range 60 {
		bw.Tick()
		bw.AssertClear(tick, bw.Snapshot(), walls)
	}
	if seen, _ := bw.Seen(); seen.Count != 0 {
		t.Errorf("saw %d through the forest at 160, want nobody", seen.Count)
	}

	for y := uint32(1); y <= 14; y++ {
		bw.Board.Res.Logic.Board.Set(cellAt(3, y), cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
	}
	for range 90 {
		bw.Tick()
	}
	if seen, _ := bw.Seen(); seen.Count != 1 {
		t.Errorf("saw %d across the cut forest, want the target", seen.Count)
	}
	if units := bw.Snapshot(); units[2].TopLeft.X < float64(4*boardtest.CellSize) {
		t.Errorf("the walker ends at %v, want it through where the forest stood", units[2])
	}
}

// The wall veils every layer, so it cuts sight whatever the Blockers.
func TestGround_AWallVeilingEveryLayerCutsSightFromEveryLayer(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(6, 16, boardtest.CellSize)
	cellAt := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	for _, blockers := range []world.Layers{0, world.Layers(cell.Land), world.Layers(cell.Air)} {
		observer := boardtest.Mover{Here: cellAt(1, 7), Sight: &vision.Sight{Facing: east, Radius: 300, Blockers: blockers}, Eye: world.Eye{Angle: 2 * math.Pi / 8}}
		bw, _ := boardtest.SquareWorld(t, observer, boardtest.Mover{Here: cellAt(5, 7)})
		bw.Tick()
		if seen, ok := bw.Seen(); !ok || seen.Count != 0 {
			t.Errorf("blockers %08b: saw %v through the wall, want nobody", blockers, seen.IDs[:seen.Count])
		}
	}
}

// A solid kind admitting Air keeps Land and Water out: it is solid for every layer but Air.
func TestGround_IsSolidForTheLayersItsKindKeepsOut(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(6, 16, boardtest.CellSize)
	cellAt := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	bw := boardtest.NewWorld(t, grid, 6*boardtest.CellSize, 16*boardtest.CellSize, func(brd *board.Board) {
		brd.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
		brd.Set(cellAt(3, 3), cell.Kind{Name: cell.Named("wall"), Cost: 1, Solid: true, Allows: cell.Air})
		brd.Set(cellAt(3, 8), cell.Kind{Name: cell.Named("rock"), Cost: 1, Solid: true})
	}, []boardtest.Mover{{Here: cellAt(1, 1)}})

	for _, c := range []struct {
		layers world.Layers
		want   int
	}{
		{world.Layers(cell.Land), 2},
		{world.Layers(cell.Water), 2},
		{world.Layers(cell.Air), 1},
		{0, 2},
	} {
		if got := len(bw.Solid(c.layers)); got != c.want {
			t.Errorf("layers %08b: %d solid cells, want %d", c.layers, got, c.want)
		}
	}
}

// A side of a solid cell is open where the neighbour is not solid for the entity: along a wall
// only its faces are open, so nothing is pushed along it.
func TestGround_OpensOnlyTheSidesFacingGroundTheEntityMayStandOn(t *testing.T) {
	bw, gap := boardtest.SquareWorld(t, boardtest.Mover{})
	w := bw.World.Res.Config.Space.Width
	var mid collision.FieldBox
	bw.Board.Cover().(collision.Field).Solid(world.Layers(cell.Land), geom.NewAABBAt(geom.NewVec(0, 7*boardtest.CellSize), float64(w), boardtest.CellSize), func(fb collision.FieldBox) bool {
		if fb.Cell == uint64(gap) {
			mid = fb
		}
		return true
	})
	if want := collide.Left | collide.Right; mid.Open != want {
		t.Errorf("the middle of the wall opens %04b, want only its left and right faces %04b", mid.Open, want)
	}
}

func TestGround_AVeiledHexCutsSightAcrossIt(t *testing.T) {
	grid := grid.DefaultGrids{}.Hex(6, 3, boardtest.CellSize)
	hex, _ := grid.CellIndex(2, 1)
	from, _ := grid.CellIndex(0, 1)
	to, _ := grid.CellIndex(4, 1)
	observer := boardtest.Mover{Here: from, Sight: &vision.Sight{Facing: east, Radius: 300}, Eye: world.Eye{Angle: 2 * math.Pi / 32}}
	look := func(veil float64) uint8 {
		bw := boardtest.NewWorld(t, grid, 400, 200, func(brd *board.Board) {
			brd.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
			brd.Set(hex, cell.Kind{Name: cell.Named("forest"), Cost: 1, Allows: cell.Land, Veil: veil})
		}, []boardtest.Mover{observer, {Here: to}})
		bw.Tick()
		seen, _ := bw.Seen()
		return seen.Count
	}
	if n := look(0); n != 1 {
		t.Fatalf("saw %d across open ground, want the target", n)
	}
	if n := look(1); n != 0 {
		t.Errorf("saw %d through a fully veiled hex, want nobody", n)
	}
	if n := look(0.2); n != 1 {
		t.Errorf("saw %d through a thin veil, want the target", n)
	}
}

// water is ground a walker does not stand on and nothing stops: not solid.
var water = cell.Kind{Name: cell.Named("water"), Cost: 1, Allows: cell.Water}

// A unit pushed by another towards water holds at the shore as at a wall: a push apart never puts
// it over ground that does not take it, and the one pushing is stopped as by the ground. Ground
// turning to water under a unit is no push: it stays there, nothing moves it out.
func TestGround_AUnitPushedByAnotherHoldsAtTheWater(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(6, 16, boardtest.CellSize)
	cellAt := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	bw := boardtest.NewWorld(t, grid, 6*boardtest.CellSize, 16*boardtest.CellSize, func(brd *board.Board) {
		brd.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
		for y := range uint32(16) {
			brd.Set(cellAt(3, y), water)
		}
	}, []boardtest.Mover{{Here: cellAt(2, 7), Brakes: true}, {Here: cellAt(1, 7), Heading: east}, {Here: cellAt(1, 12), Brakes: true}})
	shore := float64(3 * boardtest.CellSize)
	for tick := range 120 {
		bw.Tick()
		if units := bw.Snapshot(); units[0].BottomRight.X > shore+1e-6 {
			t.Fatalf("tick %d: the one pushed reaches %v, over the water from %v", tick, units[0].BottomRight.X, shore)
		}
	}
	units := bw.Snapshot()
	if units[0].BottomRight.X < shore-1 {
		t.Errorf("the one pushed stands at %v, want pushed up to the shore at %v", units[0], shore)
	}
	if units[1].BottomRight.X > units[0].TopLeft.X+1 {
		t.Errorf("the pusher at %v is into the one it pushes at %v, want it stopped by it", units[1], units[0])
	}

	still := units[2]
	bw.Board.Res.Logic.Board.Set(cellAt(1, 12), water)
	for range 30 {
		bw.Tick()
	}
	if got := bw.Snapshot()[2]; got != still {
		t.Errorf("the ground turned to water under a unit standing still, and it moved from %v to %v; want it left there", still, got)
	}
}
