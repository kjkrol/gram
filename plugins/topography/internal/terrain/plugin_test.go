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

func TestPlugin_TheGroundOnTheGPUTakesTheTilesPlace(t *testing.T) {
	w := topotest.NewWorld(0)
	b, _ := topotest.LevelBoard(w)
	p := topography.NewPlugin(w, b, topography.Config{Cell: 32, HeightUnit: 1})
	if d := topotest.Drawn(t, p); len(d) != 1 || !drawnOpaque(d[0]) {
		t.Fatalf("the topography draws %v, want one render.Direct drawing the opaque ground", d)
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
	hex.Board().SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	prisms := topography.NewPlugin(w2, hex, topography.Config{Cell: 32, HeightUnit: 1})
	if d := topotest.Drawn(t, prisms); len(d) != 1 || !drawnOpaque(d[0]) {
		t.Errorf("over a hex grid the topography draws %v, want one render.Direct drawing the "+
			"opaque ground", d)
	}
	if prisms.Look() != look.Nothing {
		t.Error("over a hex grid the board's Look still lays tiles")
	}
}

func drawnOpaque(l render.Source) bool {
	d, ok := l.(render.Direct)
	if !ok {
		return false
	}
	var f render.Frame
	d.Compose(&f, nil)
	opaque := false
	f.Places(func(p render.Pass, _ render.At) { opaque = opaque || p == render.Opaque })
	return opaque
}
