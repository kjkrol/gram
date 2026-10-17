package look_test

import (
	"image/color"
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type flatAtlas struct{}

func (flatAtlas) Atlas() *render.Image { return nil }

func (flatAtlas) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 1, 1 }

func (flatAtlas) White() (u, v float32) { return 0, 0 }

type lookMap struct{ l look.Look }

func (m lookMap) Look() look.Look                               { return m.l }
func (lookMap) Dressing() look.Dressing                         { return nil }
func (lookMap) Top(cell.ID) (corners [4]float32, level float32) { return corners, 0 }
func (lookMap) Climb(cell.ID, cell.ID, cell.Domain) float64     { return 1 }
func (lookMap) Least(cell.Domain) float64                       { return 1 }
func (lookMap) Heights() ground.Heights                         { return nil }
func (lookMap) Slope(geom.Vec, geom.Vec, cell.Domain) float64   { return 1 }

func mapOf(l look.Look) look.Map { return lookMap{l} }

func newRenderer(
	b look.Board, atlas render.AtlasSource, state look.RenderState, m look.Map,
) *look.Renderer {
	r := look.NewRenderer(b, atlas, m, world.SpaceCfg{})
	*r.State() = state
	return r
}

func flatRenderer(brd *board.Board, state look.RenderState) *look.Renderer {
	return newRenderer(brd, flatAtlas{}, state, mapOf(look.FlatLook()))
}

// how many pieces lie at each height
func compose(r *look.Renderer, cam camera.Camera) map[float32]int {
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	out := map[float32]int{}
	f.Each(func(at render.At, _ []render.Vertex) { out[at.Z()]++ })
	return out
}

// the grid on in a line width pixels wide
func gridOf(line color.RGBA, width float32) look.RenderState {
	return look.RenderState{ShowGridLines: true, GridLine: line, GridWidth: width}
}

var black = color.RGBA{A: 255}

// the lines of the grid composed through cam
func gridLinesOf(r *look.Renderer, cam camera.Camera) [][]render.Vertex {
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	var out [][]render.Vertex
	f.Each(func(at render.At, v []render.Vertex) {
		if at.Z() == look.GridLift {
			out = append(out, append([]render.Vertex(nil), v...))
		}
	})
	return out
}

func TestRenderer_Compose_FlatLaysEveryVisibleCellOverItsBox(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	brd := board.NewBoard(grid)
	brd.SetAll(cell.Kind{Cost: 1, Allows: cell.Land, Height: 0})
	got := compose(flatRenderer(brd, look.RenderState{}), cameratest.NewFromSpace(128, 128, 0))
	if len(got) != 1 || got[0] != 16 {
		t.Errorf("composed %v, want the 16 cells on the ground and nothing else", got)
	}
}

func TestRenderer_Compose_DrawsEverySideOfASquareGridOnceWhereCellsAreLargeEnough(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	brd := board.NewBoard(grid)
	brd.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	r := flatRenderer(brd, gridOf(black, 1))
	cam := cameratest.NewFromSpace(128, 128, 0)
	if got := compose(r, cam); got[0] != 16 || got[look.GridLift] != 40 {
		t.Errorf("composed %v, want the 16 tiles and the 40 sides of their cells", got)
	}
	cam.SetViewport(20, 20) // the whole board on 20 pixels: a cell is 5 across
	cam.ZoomOut(32, 0, 0)
	if n := len(gridLinesOf(r, cam)); n != 0 {
		t.Errorf("%d lines drawn with cells a few pixels wide, want none", n)
	}
}

func TestRenderer_Compose_DrawsEverySideOfAHexGridOnce(t *testing.T) {
	grid := grid.DefaultGrids{}.Hex(3, 3, 32)
	brd := board.NewBoard(grid)
	brd.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	got := compose(flatRenderer(brd, gridOf(black, 1)), cameratest.NewFromSpace(256, 256, 0))
	if got[0] != 9 || got[look.GridLift] != 38 {
		t.Errorf("composed %v, want the nine hexes and the 38 sides of their cells", got)
	}
}

func TestRenderer_Compose_DrawsTheGridInItsLine(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(2, 2, 32)
	brd := board.NewBoard(grid)
	brd.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	red := color.RGBA{R: 200, A: 128}
	lines := gridLinesOf(flatRenderer(brd, gridOf(red, 3)), cameratest.NewFromSpace(64, 64, 0))
	if len(lines) != 12 {
		t.Fatalf("%d lines, want the 12 sides of four cells", len(lines))
	}
	for _, v := range lines {
		across := float32(math.Hypot(float64(v[0].DstX-v[2].DstX), float64(v[0].DstY-v[2].DstY)))
		r, a := float64(v[0].ColorR), float64(v[0].ColorA)
		if math.Abs(r-200.0/255*128/255) > 1e-6 || math.Abs(a-128.0/255) > 1e-6 || across != 4 {
			t.Fatalf("a line in %v %v %v %v, %v pixels across; want the red premultiplied, 3 "+
				"pixels and its fading ones", v[0].ColorR, v[0].ColorG, v[0].ColorB, v[0].ColorA,
				across)
		}
	}
	none := flatRenderer(brd, gridOf(red, 0))
	if n := len(gridLinesOf(none, cameratest.NewFromSpace(64, 64, 0))); n != 0 {
		t.Errorf("%d lines in a line no pixel wide, want none", n)
	}
}

func TestRenderer_Compose_AMaterialsCellKeepsItsGrid(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(2, 2, 32)
	brd := board.NewBoard(grid)
	brd.SetAll(cell.Kind{Name: cell.Named("lava"), Cost: 1, Allows: cell.Land})
	r := flatRenderer(brd, gridOf(black, 1))
	r.Shade(map[cell.Name]render.MaterialID{cell.Named("lava"): 1})
	got := compose(r, cameratest.NewFromSpace(64, 64, 0))
	if got[0] != 0 || got[look.GridLift] != 12 {
		t.Errorf("composed %v, want no tile but the 12 sides of the four cells", got)
	}
}

func TestRenderer_Compose_DrawsEachSideOfAPartlySeenHexBoardOnce(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(8, 8, 20)
	brd := board.NewBoard(g)
	brd.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	var seen []cell.ID
	lk := lookFn(func(_ *render.Frame, _ camera.Camera, t *look.Tile) { seen = append(seen, t.ID) })
	r := newRenderer(brd, flatAtlas{}, gridOf(black, 1), mapOf(lk))
	cam := cameratest.NewFromSpace(400, 400, 0)
	cam.SetViewport(150, 110)
	cam.MoveTo(70, 50)
	type side [4]int32
	sideOf := func(ax, ay, bx, by float64) side {
		a := [2]int32{int32(math.Round(ax * 10)), int32(math.Round(ay * 10))}
		b := [2]int32{int32(math.Round(bx * 10)), int32(math.Round(by * 10))}
		if a[0] > b[0] || a[0] == b[0] && a[1] > b[1] {
			a, b = b, a
		}
		return side{a[0], a[1], b[0], b[1]}
	}
	drawn := map[side]int{}
	ox, oy := cam.FromScreen(0, 0)
	for _, v := range gridLinesOf(r, cam) {
		// a line's ends are the middles of its quad's short sides, on screen from (ox, oy)
		ax, ay := (v[0].DstX+v[2].DstX)/2+ox, (v[0].DstY+v[2].DstY)/2+oy
		bx, by := (v[1].DstX+v[3].DstX)/2+ox, (v[1].DstY+v[3].DstY)/2+oy
		drawn[sideOf(float64(ax), float64(ay), float64(bx), float64(by))]++
	}
	want := map[side]bool{}
	for _, c := range seen {
		o := g.CellOutline(c, nil)
		for i, p := range o {
			q := o[(i+1)%len(o)]
			want[sideOf(p.X, p.Y, q.X, q.Y)] = true
		}
	}
	if len(seen) == 0 || len(seen) == 64 {
		t.Fatalf("%d of the 64 cells seen, want some of them", len(seen))
	}
	for s, n := range drawn {
		if n != 1 || !want[s] {
			t.Errorf("the side %v drawn %d times, want once, and only a seen cell's", s, n)
		}
	}
	if len(drawn) != len(want) {
		t.Errorf("%d sides drawn, want the %d of the %d cells seen", len(drawn), len(want),
			len(seen))
	}
}

func TestRenderer_Compose_HandsTheLookEveryCell(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	brd := board.NewBoard(grid)
	brd.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	wood := grid.At(1, 1)
	brd.Set(wood, cell.Kind{Cost: 1, Allows: cell.Land, Height: 8, Sway: 1})
	var seen int
	var amount, rise float32
	lk := lookFn(func(_ *render.Frame, _ camera.Camera, t *look.Tile) {
		seen++
		if t.ID == wood {
			amount, rise = t.Sway()
		}
	})
	r := newRenderer(brd, flatAtlas{}, look.RenderState{}, mapOf(lk))
	compose(r, cameratest.NewFromSpace(128, 128, 0))
	if seen != 16 || amount != 1 || rise != 8 {
		t.Errorf("look saw %d cells, the wood swaying %v and standing %v high; want 16, 1 and the kind's 8", seen, amount, rise)
	}
}

type lookFn func(f *render.Frame, cam camera.Camera, t *look.Tile)

func (fn lookFn) Cell(f *render.Frame, cam camera.Camera, t *look.Tile) { fn(f, cam, t) }

func TestTile_AFlatWorldIsDrawnAsItsSpritesAre(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(2, 2, 32)
	brd := board.NewBoard(grid)
	brd.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	var got render.Shade
	lk := lookFn(func(_ *render.Frame, _ camera.Camera, t *look.Tile) { got = t.Light() })
	r := newRenderer(brd, flatAtlas{}, look.RenderState{}, mapOf(lk))
	compose(r, cameratest.NewFromSpace(64, 64, 0))
	if got != render.Even(1) {
		t.Errorf("a flat world's tile is lit %v, want as drawn", got)
	}
}

func TestRenderer_Compose_FlatTilesAreDrawnAsTheyAre(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(2, 2, 32)
	brd := board.NewBoard(grid)
	brd.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	r := newRenderer(brd, flatAtlas{}, look.RenderState{}, mapOf(look.FlatLook()))
	var f render.Frame
	cam := cameratest.NewFromSpace(64, 64, 0)
	f.Reset(cam)
	r.Compose(&f, cam)
	var v render.Vertex
	f.Each(func(_ render.At, verts []render.Vertex) { v = verts[0] })
	if v.ColorR != 1 || v.ColorG != 1 || v.ColorB != 1 {
		t.Errorf("a flat world's tile is lit %v %v %v, want as it is", v.ColorR, v.ColorG, v.ColorB)
	}
}

type scaledCam struct{ camera.Camera }

func (scaledCam) ScaleAt(_, y, _ float32) float32 {
	if y < 64 {
		return 1
	}
	return 0.1
}

func TestRenderer_Compose_DrawsEachCellsGridAsLargeAsItIsDrawn(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	brd := board.NewBoard(grid)
	brd.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	r := flatRenderer(brd, gridOf(black, 1))
	lines := gridLinesOf(r, scaledCam{cameratest.NewFromSpace(128, 128, 0)})
	for _, v := range lines {
		if v[0].DstY > 66 && v[1].DstY > 66 {
			t.Fatalf("a line at %v down the screen, want none past the two nearer rows", v[0].DstY)
		}
	}
	if len(lines) != 22 {
		t.Errorf("%d lines, want the 22 sides of the 8 cells in the nearer rows: 32 pixels a "+
			"cell there, 3 further", len(lines))
	}
}
