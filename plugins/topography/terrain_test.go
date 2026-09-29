package topography_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/render"
)

// Over a square grid the plugin draws the ground Direct at the Ground tier and the board's Look
// lays no tile; off one there is no such renderer and the tiles are laid. G is bound to nothing.
func TestPlugin_TheTerrainTakesTheTilesPlaceOverASquareGrid(t *testing.T) {
	w := newWorld(0)
	b, _ := levelBoard(w)
	p := topography.NewPlugin(w, b, topography.Config{Cell: 32, HeightUnit: 1, Isometric: true})
	r, ok := p.Renderer().(render.Direct)
	if !ok || r.Tier() != render.Ground {
		t.Fatalf("the renderer is %T, want a render.Direct at the Ground tier", p.Renderer())
	}
	if p.Look() != board.Nothing {
		t.Error("over a square grid the board's Look still lays tiles")
	}
	for _, bd := range p.DefaultBindings() {
		if players.Written(bd.Trigger) == "G" {
			t.Error("G is still bound")
		}
	}

	w2 := newWorld(0)
	hex := board.NewPlugin(board.DefaultGrids{}.Hex(4, 4, 16), &board.MultipleOccupancy{}, w2)
	hex.Res.Logic.Board.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	tiles := topography.NewPlugin(w2, hex, topography.Config{Cell: 32, HeightUnit: 1, Isometric: true})
	if tiles.Renderer() != nil {
		t.Errorf("off a square grid the renderer is %T, want nil", tiles.Renderer())
	}
	if tiles.Look() == board.Nothing {
		t.Error("off a square grid the board's Look lays no tile")
	}
}
