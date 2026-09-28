package board

import (
	"image/color"
	"sync/atomic"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// stripes is a Parallel Dressing for tests: it lights every tile by its cell and lays a stripe
// over it, counting the tiles warmed and the workers asked for; a worker keeps a scratch of its
// own and the tiles it dressed.
type stripes struct {
	warmed  atomic.Int32
	readied int
	workers int
	dressed []int
}

var _ Parallel = (*stripes)(nil)

func (*stripes) Begin(*render.Frame, camera.Camera)                {}
func (*stripes) Sheet(atlas render.AtlasSource) render.AtlasSource { return atlas }
func (*stripes) Base(t *Tile) render.SpriteID                      { return t.Sprite() }
func (*stripes) FaceLight(*Tile, int, int) render.Light            { return render.Light{1, 1, 1} }
func (*stripes) Covers(*Tile) bool                                 { return false }
func (*stripes) Light(t *Tile) render.Shade                        { return render.Even(float32(t.ID%7) / 7) }
func (s *stripes) Dress(f *render.Frame, cam camera.Camera, t *Tile, x0, y0, x1, y1, depth float32) {
	ax, ay := cam.Project(x0, y0, 0)
	bx, by := cam.Project(x1, y1, 0)
	f.Line(render.Ground+5, depth, ax, ay, bx, by, 1, color.RGBA{R: uint8(t.ID), A: 255})
	s.dressed = append(s.dressed, int(t.ID))
}
func (s *stripes) Warm(*Tile) { s.warmed.Add(1) }
func (s *stripes) Ready()     { s.readied++ }
func (s *stripes) Worker(k int) Dressing {
	s.workers = max(s.workers, k+1)
	return &stripes{}
}

// stripedMap is a flat Map dressed by stripes, drawn by look.
type stripedMap struct {
	lookMap
	d *stripes
}

func (m stripedMap) Dressing() Dressing { return m.d }

// piecesOf composes r through cam and gives every piece of the frame in order.
func piecesOf(r *Renderer, cam camera.Camera) (out [][]ebiten.Vertex, n int) {
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	f.Each(func(_ render.Tier, _ float32, v []ebiten.Vertex) {
		out = append(out, append([]ebiten.Vertex(nil), v...))
	})
	return out, f.Len()
}

// Under a Parallel Dressing and a ParallelLook the tiles are warmed first, then dressed on several
// goroutines at once, and the picture is the one a single goroutine draws, piece for piece.
func TestRenderer_Compose_WorkersDrawWhatOneGoroutineDoes(t *testing.T) {
	grid := DefaultGrids{}.Square(16, 16, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	cam := icamera.NewFromSpace(512, 512, 0)
	one, many := &stripes{}, &stripes{}
	serial := newRenderer(brd, flatAtlas{}, &RenderState{ShowGridLines: true}, func() Map { return stripedMap{lookMap{flatLook{}}, one} })
	serial.Workers(1)
	parallel := newRenderer(brd, flatAtlas{}, &RenderState{ShowGridLines: true}, func() Map { return stripedMap{lookMap{flatLook{}}, many} })
	parallel.Workers(4)
	want, wantN := piecesOf(serial, cam)
	got, gotN := piecesOf(parallel, cam)
	if many.warmed.Load() != 256 || many.readied != 1 || many.workers < 2 || one.warmed.Load() != 0 {
		t.Fatalf("%d tiles warmed, readied %d times, for %d workers; %d warmed drawing on one goroutine; want all 256 once for several, none for one", many.warmed.Load(), many.readied, many.workers, one.warmed.Load())
	}
	if len(got) != len(want) || gotN != wantN || len(want) < 512 {
		t.Fatalf("%d pieces (%d counted) from several goroutines, %d (%d) from one", len(got), gotN, len(want), wantN)
	}
	for i := range want {
		for k := range want[i] {
			if got[i][k] != want[i][k] {
				t.Fatalf("piece %d vertex %d is %+v, want %+v", i, k, got[i][k], want[i][k])
			}
		}
	}
	if len(many.dressed) != 0 || len(one.dressed) != 256 {
		t.Errorf("the dressing itself dressed %d tiles under workers and %d alone; want none and all", len(many.dressed), len(one.dressed))
	}
}

// A Look that is no ParallelLook keeps the tiles on the frame's goroutine, warmed by nobody.
func TestRenderer_Compose_APlainLookKeepsOneGoroutine(t *testing.T) {
	grid := DefaultGrids{}.Square(16, 16, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	d := &stripes{}
	seen := 0
	look := lookFn(func(f *render.Frame, cam camera.Camera, tile *Tile) {
		seen++
		tile.Dress(f, cam, tile.X0, tile.Y0, tile.X1, tile.Y1, 0)
	})
	r := newRenderer(brd, flatAtlas{}, &RenderState{}, func() Map { return stripedMap{lookMap{look}, d} })
	if _, n := piecesOf(r, icamera.NewFromSpace(512, 512, 0)); n != 256 || seen != 256 || d.warmed.Load() != 0 || d.workers != 0 {
		t.Errorf("%d pieces, the look saw %d tiles, %d warmed for %d workers; want 256, 256, none and none", n, seen, d.warmed.Load(), d.workers)
	}
}

// The board reads every cell's cover once for all (Ready) and anew when a cell changes.
func TestBoard_ReadyReadsTheCoverOnceAndAnewWhenACellChanges(t *testing.T) {
	grid := DefaultGrids{}.Square(8, 1, 10)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	forest := CellKind{Cost: 1, Allows: Land, Veil: 0.5, Veils: Land}
	c3, _ := grid.CellIndex(3, 0)
	c6, _ := grid.CellIndex(6, 0)
	brd.Set(c3, forest)
	walk := func() (stretches [][2]float64) {
		brd.Walk(geom.NewVec(0, 5), geom.NewVec(1, 0), 80, world.Layers(Land), func(near, far, _, _, tau float64) bool {
			stretches = append(stretches, [2]float64{near, far})
			return true
		})
		return stretches
	}
	brd.Ready()
	if got := walk(); len(got) != 1 || got[0] != [2]float64{30, 40} {
		t.Fatalf("walked through %v, want the forest cell from 30 to 40", got)
	}
	brd.Set(c6, forest)
	if got := walk(); len(got) != 2 || got[1] != [2]float64{60, 70} {
		t.Errorf("walked through %v after a second forest, want it from 60 to 70 too", got)
	}
}
