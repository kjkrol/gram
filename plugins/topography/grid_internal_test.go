package topography

import (
	"image/color"
	"testing"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

func TestBoardSurface_DrawsTheGridInTheBoardsLine(t *testing.T) {
	gputest.Need(t)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 4},
		Heights:  true,
	})
	b := board.NewPlugin(grid.DefaultGrids{}.Square(4, 4, 32), &cell.MultipleOccupancy{}, w)
	b.Read(0).CellKinds().Define("grass", cell.Kind{Pace: 1, Allows: cell.Land})
	grass, _ := b.Read(0).CellKinds().Get("grass")
	b.Write().Board().SetAll(grass)
	p := NewPlugin(w, b, nil, Config{})
	a := b.NewAtlas(32)
	a.Add("grass", render.Solid(color.RGBA{G: 160, A: 255}))
	a.Grid(board.Line{Color: color.RGBA{R: 255, A: 255}, Width: 3})
	a.Close()
	out := boardSurface{p}.Surface()
	red := [4]float32{1, 0, 0, 1}
	if out.Grid != look.MinGridCell || out.GridLine != red ||
		out.GridWidth != 3 {
		t.Errorf("the ground's grid from %v pixels a cell in %v, %v wide; want from %v in the "+
			"board's red, 3", out.Grid, out.GridLine, out.GridWidth, look.MinGridCell)
	}
	b.Write().ShowGrid(false)
	if out := (boardSurface{p}).Surface(); out.Grid != 0 || out.GridWidth != 0 {
		t.Errorf("the board's grid hidden, the ground's is from %v pixels a cell, %v wide; want "+
			"none", out.Grid, out.GridWidth)
	}
}

func TestMiniature_IsTheBoardPaintedWhereTheGroundIs(t *testing.T) {
	gputest.Need(t)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 4},
		Heights:  true,
	})
	b := board.NewPlugin(grid.DefaultGrids{}.Square(4, 4, 32), &cell.MultipleOccupancy{}, w)
	b.Read(0).CellKinds().Define("grass", cell.Kind{Pace: 1, Allows: cell.Land})
	grass, _ := b.Read(0).CellKinds().Get("grass")
	b.Write().Board().SetAll(grass)
	p := NewPlugin(w, b, nil, Config{})
	if img, _ := p.write.Miniature(); img != nil {
		t.Error("a miniature before the board's atlas is closed")
	}
	a := b.NewAtlas(32)
	a.Add("grass", render.Solid(color.RGBA{G: 160, A: 255}))
	a.Close()
	img, perUnit := p.write.Miniature()
	if img == nil || perUnit <= 0 {
		t.Fatalf("the miniature is %v at %v a unit, want the painter's", img, perUnit)
	}
	b2 := img.Bounds()
	if got := float32(b2.Dx()) / perUnit; got < 128 {
		t.Errorf("the miniature spans %v world units, want the world's 128", got)
	}
}
