package terrain_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
	"github.com/kjkrol/gram/render"
)

// Over a square grid and over a hex one the plugin draws the ground Direct at the Ground tier — a
// mesh, prisms — and the board's Look lays no tile. G is bound to nothing.
func TestPlugin_TheGroundOnTheGPUTakesTheTilesPlace(t *testing.T) {
	w := topotest.NewWorld(0)
	b, _ := topotest.LevelBoard(w)
	p := topography.NewPlugin(w, b, topography.Config{Cell: 32, HeightUnit: 1})
	r, ok := p.Renderer().(render.Direct)
	if !ok || r.Tier() != render.Ground {
		t.Fatalf("the renderer is %T, want a render.Direct at the Ground tier", p.Renderer())
	}
	if p.Look() != look.Nothing {
		t.Error("over a square grid the board's Look still lays tiles")
	}
	for _, bd := range p.DefaultBindings() {
		if players.Written(bd.Trigger) == "G" {
			t.Error("G is still bound")
		}
	}

	w2 := topotest.NewWorld(0)
	hex := board.NewPlugin(grid.DefaultGrids{}.Hex(4, 4, 16), &cell.MultipleOccupancy{}, w2)
	hex.Res.Logic.Board.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	prisms := topography.NewPlugin(w2, hex, topography.Config{Cell: 32, HeightUnit: 1})
	if r, ok := prisms.Renderer().(render.Direct); !ok || r.Tier() != render.Ground {
		t.Errorf("over a hex grid the renderer is %T, want a render.Direct at the Ground tier", prisms.Renderer())
	}
	if prisms.Look() != look.Nothing {
		t.Error("over a hex grid the board's Look still lays tiles")
	}
}
