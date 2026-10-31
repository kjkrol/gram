package hexes

import (
	"math"
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/topography/internal/relief"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render/gputest"
)

func TestGround_OutlinesTheCellsInTheBoardsGridLine(t *testing.T) {
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
		t.Errorf("the cells outlined in %v, %v pixels wide; want the board's %v premultiplied",
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
		t.Errorf("the board's grid hidden, the cells outlined %v pixels wide, want none", width)
	}
}

func TestGround_LaysEveryCellToItsCornersWhateverItsKindsHeight(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 256, Height: 256},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 4},
		Heights:  true,
	})
	b := board.NewPlugin(grid.DefaultGrids{}.Hex(4, 4, 20), &cell.MultipleOccupancy{}, w)
	brd := b.Read(0).Board()
	forest := brd.At(1, 1)
	b.Write().Board().Set(forest, cell.Kind{Name: cell.Named("forest"), Pace: 1, Height: 8})
	r := relief.New(brd, nil)
	r.SetHeights(func(p geom.Vec) float32 { return p.X/10 + p.Y/20 })
	r.SetCorners(forest, relief.Corners{3})
	gridded := func() (bool, look.RenderState) { return false, *b.Res.Render }
	h := New(w, b, b.Read(0).Drawn(), nil, func() *relief.Relief { return r }, gridded, nil, nil)
	h.lay()
	if len(h.cells) != cellFloats*brd.CellCount() {
		t.Fatalf("%d floats laid, want %d a cell", len(h.cells), cellFloats)
	}
	brd.EachCell(func(c cell.ID) {
		o, _ := brd.Ordinal(c)
		laid := h.cells[cellFloats*o : cellFloats*(o+1)]
		corners, level, _ := r.Hex(c)
		centre := brd.CellCenter(c)
		if laid[0] != centre.X || laid[1] != centre.Y || laid[2] != level {
			t.Errorf("%v laid at %v, %v to %v; want at its centre %v to its level %v", c, laid[0],
				laid[1], laid[2], centre, level)
		}
		if !slices.Equal(laid[4:10], corners[:]) {
			t.Errorf("%v's corners laid at %v, want the relief's %v", c, laid[4:10], corners)
		}
		// edge k from corner k to k+1, the cell across it centred at the edge's middle twice over
		outline, rim := brd.CellOutline(c, nil), 0
		for k, p := range outline {
			q := outline[(k+1)%6]
			if _, ok := brd.CellAt(geom.NewVec(p.X+q.X-centre.X, p.Y+q.Y-centre.Y)); !ok {
				rim |= 1 << k
			}
		}
		if laid[10] != float32(rim) || laid[11] != float32(o) {
			t.Errorf("%v laid with the board's edge along %b, ordinal %v; want %b, %v", c,
				int(laid[10]), laid[11], rim, o)
		}
	})
	o, _ := brd.Ordinal(forest)
	if top := h.cells[cellFloats*o+2]; top != 3 {
		t.Errorf("the forest stands to %v, want its ground's 3: a kind's Height is not drawn", top)
	}
}

func TestGround_FacesEveryCornerTheWayTheGroundSlopesRoundIt(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 256, Height: 256},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 4},
		Heights:  true,
	})
	b := board.NewPlugin(grid.DefaultGrids{}.Hex(5, 4, 20), &cell.MultipleOccupancy{}, w)
	r := relief.New(b.Read(0).Board(), nil)
	// a plane rising a tenth east and a fifth south faces so at every corner, the edge's too
	r.SetHeights(func(p geom.Vec) float32 { return p.X/10 + p.Y/5 })
	gridded := func() (bool, look.RenderState) { return false, *b.Res.Render }
	h := New(w, b, b.Read(0).Drawn(), nil, func() *relief.Relief { return r }, gridded, nil, nil)
	h.face()
	n := relief.HexCount(5, 4)
	if h.facing.Bounds().Dx()*h.facing.Bounds().Dy() < n || len(h.pix) < 4*n {
		t.Fatalf("a facing of %v for %d corners", h.facing.Bounds(), n)
	}
	l := math.Sqrt(0.01 + 0.04 + 1)
	want := [3]byte{unit(-0.1 / l), unit(-0.2 / l), unit(1 / l)}
	for i := range n {
		if got := [3]byte(h.pix[4*i : 4*i+3]); got != want {
			t.Fatalf("corner %d faces %v, want the plane's %v", i, got, want)
		}
	}
}

func TestFans_Compile(t *testing.T) {
	gputest.Need(t)
	if err := fans.Compile(); err != nil {
		t.Error(err)
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
