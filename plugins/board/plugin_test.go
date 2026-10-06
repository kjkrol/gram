package board

import (
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/world"
)

func TestNewPlugin_SetsEachGridAxisFromTheWorldsEdges(t *testing.T) {
	for _, tc := range []struct {
		edges        aabbworld.Edges
		wrapX, wrapY bool
	}{
		{0, false, false},
		{aabbworld.Torus, true, true},
		{aabbworld.WrapX | aabbworld.OpenY, true, false},
		{aabbworld.WrapY, false, true},
	} {
		g := grid.DefaultGrids{}.Square(5, 5, 10)
		worldPlugin := world.NewPlugin(world.Config{
			Space:    world.SpaceCfg{Width: 100, Height: 100, Edges: tc.edges},
			Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
		})
		NewPlugin(g, &cell.SingleOccupancy{}, worldPlugin)

		if sq, _ := grid.ShapeOf(g); sq.WrapX != tc.wrapX || sq.WrapY != tc.wrapY {
			t.Errorf("edges %04b: grid wraps x=%v y=%v, want x=%v y=%v", tc.edges, sq.WrapX, sq.WrapY, tc.wrapX, tc.wrapY)
		}
	}
}

func newSeedTestPlugin(t *testing.T) (*Plugin, cell.ID) {
	t.Helper()
	grid := grid.DefaultGrids{}.Square(5, 5, 10)
	worldPlugin := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 50, Height: 50},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	p := NewPlugin(grid, &cell.SingleOccupancy{}, worldPlugin)
	p.CellKinds().Define("grass", cell.Kind{Cost: 1, Allows: cell.Land})
	p.CellKinds().Define("wall", cell.Kind{Cost: 1, Solid: true})
	cell, _ := grid.CellIndex(2, 2)
	return p, cell
}

func TestNewPlugin_RequiresACellAndAMoverOfEveryUnit(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 100, Height: 100}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 10, MaxSize: 10}})
	NewPlugin(grid.DefaultGrids{}.Square(2, 2, 32), &cell.MultipleOccupancy{}, w)
	defer func() {
		msg, _ := recover().(string)
		for _, want := range []string{"board requires unit.At (the cell it starts in)", "board requires unit.Mover (the domains it moves in)"} {
			if !strings.Contains(msg, want) {
				t.Errorf("panic %q does not mention %q", msg, want)
			}
		}
	}()

	w.Roster().Unit.Spec(comp.Const(world.Position{}))
	t.Error("the roster accepted a unit without an unit.At and a unit.Mover")
}

func TestPlugin_SeedPopulate_AppliesLayout(t *testing.T) {
	p, wallCell := newSeedTestPlugin(t)
	other, _ := p.Res.Logic.Board.CellIndex(0, 0)
	p.Seed(Layout{Default: "grass", Cells: []cell.Entry{{Kind: "wall", Cell: wallCell}}})

	if err := p.Populate(); err != nil {
		t.Fatalf("Populate: %v", err)
	}
	if got := p.Res.Logic.Board.Kind(wallCell).Name.String(); got != "wall" {
		t.Errorf("Kind(wallCell) = %q, want %q", got, "wall")
	}
	if got := p.Res.Logic.Board.Kind(other).Name.String(); got != "grass" {
		t.Errorf("Kind(other) = %q, want %q (Default)", got, "grass")
	}
}

func TestPlugin_Populate_UnknownKindChangesNothing(t *testing.T) {
	p, lava := newSeedTestPlugin(t)
	p.Seed(Layout{Default: "grass", Cells: []cell.Entry{{Kind: "lava", Cell: lava}}})

	if err := p.Populate(); err == nil {
		t.Fatal("Populate: expected an error for unknown kind, got nil")
	}
	if got := p.Res.Logic.Board.Kind(lava).Name.String(); got != "" {
		t.Errorf("Kind(lava) = %q, want untouched terrain", got)
	}
}

// The board has an atlas once it draws, none before the renderer.
func TestPlugin_HasAnAtlasOnceItDraws(t *testing.T) {
	worldPlugin := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	p := NewPlugin(grid.DefaultGrids{}.Square(5, 5, 10), &cell.SingleOccupancy{}, worldPlugin)
	if p.Atlas() != nil {
		t.Fatal("an atlas before the renderer")
	}
	p.WithRenderer(nil)
	if p.Atlas() == nil {
		t.Error("no atlas after the renderer: given none, it draws from the board's own")
	}
}
