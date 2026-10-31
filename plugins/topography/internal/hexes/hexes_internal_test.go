package hexes

import (
	"slices"
	"testing"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/topography/internal/relief"
	"github.com/kjkrol/gram/plugins/world"
)

func TestGround_OutlinesThePrismsInTheBoardsGridLine(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 256, Height: 256},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 4},
		Heights:  true,
	})
	g := grid.DefaultGrids{}.Hex(4, 4, 20)
	b := board.NewPlugin(g, &cell.MultipleOccupancy{}, w)
	b.NewAtlas(32).Close()
	gridded := func() (bool, look.RenderState) { return false, *b.Res.Render }
	h := New(w, b, b.Read(0).Drawn(), nil, nil, gridded, nil, nil)
	cam := cameratest.NewFromSpace(256, 256, 0)
	want := board.DefaultGridLine(g)
	a := float32(want.Color.A) / 255
	line, width := h.gridOf(cam)
	if line[3] != a || line[0] != float32(want.Color.R)/255*a || width != want.Width {
		t.Errorf("the prisms outlined in %v, %v pixels wide; want the board's %v premultiplied",
			line, width, want)
	}
	cam.SetViewport(16, 16)
	cam.ZoomOut(32, 0, 0)
	if _, width := h.gridOf(cam); width != 0 {
		t.Errorf("cells a pixel across outlined %v pixels wide, want none", width)
	}
	cam = cameratest.NewFromSpace(256, 256, 0)
	b.Write().ShowGrid(false)
	if _, width := h.gridOf(cam); width != 0 {
		t.Errorf("the board's grid hidden, the prisms outlined %v pixels wide, want none", width)
	}
}

func TestGround_StandsEveryPrismToItsGroundWhateverItsKindsHeight(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 256, Height: 256},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 4},
		Heights:  true,
	})
	b := board.NewPlugin(grid.DefaultGrids{}.Hex(4, 4, 20), &cell.MultipleOccupancy{}, w)
	brd := b.Read(0).Board()
	forest, beside := brd.At(1, 1), brd.At(2, 1)
	b.Write().Board().Set(forest, cell.Kind{Name: cell.Named("forest"), Pace: 1, Height: 8})
	r := relief.New(brd, nil)
	r.SetCorners(forest, relief.Corners{3})
	r.SetCorners(beside, relief.Corners{5})
	gridded := func() (bool, look.RenderState) { return false, *b.Res.Render }
	h := New(w, b, b.Read(0).Drawn(), nil, func() *relief.Relief { return r }, gridded, nil, nil)
	h.lay()
	laid := func(c cell.ID) []float32 {
		centre := brd.CellCenter(c)
		for i := 0; i < len(h.cells); i += cellFloats {
			if h.cells[i] == centre.X && h.cells[i+1] == centre.Y {
				return h.cells[i : i+cellFloats]
			}
		}
		t.Fatalf("no prism laid at %v", centre)
		return nil
	}
	if top := laid(forest)[2]; top != 3 {
		t.Errorf("the forest stands to %v, want its ground's 3: a kind's Height is not drawn", top)
	}
	if sides := laid(beside)[4:10]; !slices.Contains(sides, 3) || slices.Contains(sides, 11) {
		t.Errorf("the forest's neighbour's sides go down to %v, want one to the forest's ground 3",
			sides)
	}
}

func TestGround_SetPutsItsOwnOverTheFramesWithoutAllocating(t *testing.T) {
	h := Ground{own: map[string]any{}}
	h.opts.Uniforms = map[string]any{}
	h.set("Eye", 1, 2, 3)
	h.opts.Uniforms["Eye"] = []float32{9, 9, 9}
	n := testing.AllocsPerRun(10, func() { h.set("Eye", 4, 5, 6) })
	if got := h.opts.Uniforms["Eye"].([]float32); n != 0 || got[0] != 4 || got[2] != 6 {
		t.Errorf("Eye set over another: %v allocations, %v; want none and [4 5 6]", n, got)
	}
}
