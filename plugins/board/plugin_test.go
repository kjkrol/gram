package board

import (
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
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
		grid := DefaultGrids{}.Square(5, 5, 10).(*squareGrid)
		worldPlugin := world.NewPlugin(world.Config{
			Space:    world.SpaceCfg{Width: 100, Height: 100, Edges: tc.edges},
			Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
		})
		NewPlugin(grid, &SingleOccupancy{}, worldPlugin)

		if grid.WrapX != tc.wrapX || grid.WrapY != tc.wrapY {
			t.Errorf("edges %04b: grid wraps x=%v y=%v, want x=%v y=%v", tc.edges, grid.WrapX, grid.WrapY, tc.wrapX, tc.wrapY)
		}
	}
}

func newSeedTestPlugin(t *testing.T) (*Plugin, CellID) {
	t.Helper()
	grid := DefaultGrids{}.Square(5, 5, 10)
	worldPlugin := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 50, Height: 50},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	p := NewPlugin(grid, &SingleOccupancy{}, worldPlugin)
	p.CellKindDict().Create(
		CellKind{Name: Named("grass"), Cost: 1, Allows: Land},
		CellKind{Name: Named("wall"), Cost: 1, Solid: true},
	)
	cell, _ := grid.CellIndex(2, 2)
	return p, cell
}

func TestNewPlugin_RequiresACellAndAMoverOfEveryUnit(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 100, Height: 100}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 10, MaxSize: 10}})
	NewPlugin(DefaultGrids{}.Square(2, 2, 32), &MultipleOccupancy{}, w)
	defer func() {
		msg, _ := recover().(string)
		for _, want := range []string{"board requires board.Cell (the cell it starts in)", "board requires board.Mover (the domains it moves in)"} {
			if !strings.Contains(msg, want) {
				t.Errorf("panic %q does not mention %q", msg, want)
			}
		}
	}()

	w.Roster().Unit.Spec(comp.Const(world.Position{}))
	t.Error("the roster accepted a unit without a Cell and a Mover")
}

func TestPlugin_SeedPopulate_AppliesLayout(t *testing.T) {
	p, wallCell := newSeedTestPlugin(t)
	other, _ := p.Res.Logic.Board.CellIndex(0, 0)
	p.Seed(Layout{Default: "grass", Cells: []CellEntry{{Kind: "wall", Cell: wallCell}}})

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
	p, cell := newSeedTestPlugin(t)
	p.Seed(Layout{Default: "grass", Cells: []CellEntry{{Kind: "lava", Cell: cell}}})

	if err := p.Populate(); err == nil {
		t.Fatal("Populate: expected an error for unknown kind, got nil")
	}
	if got := p.Res.Logic.Board.Kind(cell).Name.String(); got != "" {
		t.Errorf("Kind(cell) = %q, want untouched terrain", got)
	}
}

// The board's atlas is the renderer's sheet, none before the renderer.
func TestPlugin_AtlasIsTheRenderersSheet(t *testing.T) {
	worldPlugin := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	p := NewPlugin(DefaultGrids{}.Square(5, 5, 10), &SingleOccupancy{}, worldPlugin)
	if p.Atlas() != nil {
		t.Fatal("an atlas before the renderer")
	}
	p.WithRenderer(nil)
	if p.Atlas() == nil || p.Atlas() != p.renderer.atlas {
		t.Error("the atlas is not the renderer's sheet")
	}
}
