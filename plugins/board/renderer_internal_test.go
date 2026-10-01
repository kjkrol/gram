package board

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"testing"

	"github.com/kjkrol/aabbworld/geom"

	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/render"
)

type flatAtlas struct{}

func (flatAtlas) Atlas() *render.Image { return nil }

func (flatAtlas) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 1, 1 }

func (flatAtlas) White() (u, v float32) { return 0, 0 }

// lookMap is a flat Map drawing by a Look alone: nothing over the tiles, level ground.
type lookMap struct{ l Look }

func (m lookMap) Look() Look                                    { return m.l }
func (lookMap) Dressing() Dressing                              { return nil }
func (lookMap) Top(cell.ID) (corners [4]float32, level float32) { return corners, 0 }
func (lookMap) Climb(cell.ID, cell.ID, cell.Domain) float64     { return 1 }
func (lookMap) Least(cell.Domain) float64                       { return 1 }
func (lookMap) Heights() Heights                                { return nil }
func (lookMap) Slope(geom.Vec, geom.Vec, cell.Domain) float64   { return 1 }

// mapOf is a Map drawing by look.
func mapOf(look Look) func() Map { return func() Map { return lookMap{look} } }

// flatRenderer is a renderer of brd with the flat look.
func flatRenderer(brd *Board, state *RenderState) *Renderer {
	return newRenderer(brd, flatAtlas{}, state, mapOf(flatLook{}))
}

// compose is what r hands a frame through cam, by tier.
func compose(r *Renderer, cam camera.Camera) map[render.Tier]int {
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	out := map[render.Tier]int{}
	f.Each(func(tier render.Tier, _ float32, _ []render.Vertex) { out[tier]++ })
	return out
}

// outlinedTiles counts the tiles r hands a frame through cam that carry an outline.
func outlinedTiles(r *Renderer, cam camera.Camera) (tiles, outlined int) {
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	f.Each(func(tier render.Tier, _ float32, v []render.Vertex) {
		if tier != render.Ground {
			return
		}
		tiles++
		if v[0].Custom0 < 0 {
			outlined++
		}
	})
	return tiles, outlined
}

func TestRenderer_Compose_FlatLaysEveryVisibleCellOverItsBox(t *testing.T) {
	grid := DefaultGrids{}.Square(4, 4, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(cell.Kind{Cost: 1, Allows: cell.Land, Height: 0})
	if got := compose(flatRenderer(brd, &RenderState{}), icamera.NewFromSpace(128, 128, 0)); len(got) != 1 || got[render.Ground] != 16 {
		t.Errorf("composed %v, want the 16 cells on the ground and nothing else", got)
	}
}

func TestRenderer_Compose_OutlinesSquareTilesWhenCellsAreLargeEnough(t *testing.T) {
	grid := DefaultGrids{}.Square(4, 4, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	r := flatRenderer(brd, &RenderState{ShowGridLines: true})

	cam := icamera.NewFromSpace(128, 128, 0)
	if got := compose(r, cam); len(got) != 1 || got[render.Ground] != 16 {
		t.Errorf("composed %v, want the 16 tiles and nothing more", got)
	}
	if tiles, outlined := outlinedTiles(r, cam); outlined != tiles {
		t.Errorf("%d of %d tiles outlined, want all", outlined, tiles)
	}
	cam.SetViewport(20, 20) // the whole board on 20 pixels: a cell is 5 across
	cam.ZoomOut(32, 0, 0)
	if _, outlined := outlinedTiles(r, cam); outlined != 0 {
		t.Errorf("%d tiles outlined with cells a few pixels wide, want none", outlined)
	}
}

func TestRenderer_Compose_StrokesTheOutlinesOfHexCells(t *testing.T) {
	grid := DefaultGrids{}.Hex(3, 3, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	got := compose(flatRenderer(brd, &RenderState{ShowGridLines: true}), icamera.NewFromSpace(256, 256, 0))
	if got[gridTier] == 0 || got[render.Ground] != 9 {
		t.Errorf("composed %v, want the nine hexes and the lines round them", got)
	}
}

// A Look is asked for every visible cell, and a tile tells it its kind and what sways on it.
func TestRenderer_Compose_HandsTheLookEveryCell(t *testing.T) {
	grid := DefaultGrids{}.Square(4, 4, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	brd.heights = true
	wood, _ := grid.CellIndex(1, 1)
	brd.Set(wood, cell.Kind{Cost: 1, Allows: cell.Land, Height: 8, Sway: 1})
	var seen int
	var amount, rise float32
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *Tile) {
		seen++
		if t.ID == wood {
			amount, rise = t.Sway()
		}
	})
	r := newRenderer(brd, flatAtlas{}, &RenderState{}, mapOf(look))
	compose(r, icamera.NewFromSpace(128, 128, 0))
	if seen != 16 || amount != 1 || rise != 8 {
		t.Errorf("look saw %d cells, the wood swaying %v and standing %v high; want 16, 1 and the kind's 8", seen, amount, rise)
	}
}

// lookFn adapts a function to Look.
type lookFn func(f *render.Frame, cam camera.Camera, t *Tile)

func (fn lookFn) Cell(f *render.Frame, cam camera.Camera, t *Tile) { fn(f, cam, t) }

func TestTile_AFlatWorldIsDrawnAsItsSpritesAre(t *testing.T) {
	grid := DefaultGrids{}.Square(2, 2, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	var got render.Shade
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *Tile) { got = t.Light() })
	r := newRenderer(brd, flatAtlas{}, &RenderState{}, mapOf(look))
	compose(r, icamera.NewFromSpace(64, 64, 0))
	if got != render.Even(1) {
		t.Errorf("a flat world's tile is lit %v, want as drawn", got)
	}
}

// A flat board's tiles are drawn as they are: a sky over the board (plugins/atmosphere) is what
// lights them.
func TestRenderer_Compose_FlatTilesAreDrawnAsTheyAre(t *testing.T) {
	grid := DefaultGrids{}.Square(2, 2, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	r := newRenderer(brd, flatAtlas{}, &RenderState{}, mapOf(flatLook{}))
	var f render.Frame
	cam := icamera.NewFromSpace(64, 64, 0)
	f.Reset(cam)
	r.Compose(&f, cam)
	var v render.Vertex
	f.Each(func(_ render.Tier, _ float32, verts []render.Vertex) { v = verts[0] })
	if v.ColorR != 1 || v.ColorG != 1 || v.ColorB != 1 {
		t.Errorf("a flat world's tile is lit %v %v %v, want as it is", v.ColorR, v.ColorG, v.ColorB)
	}
}

// scaledCam is a camera drawing the board's two nearer rows at a pixel a world unit and the two
// further at a tenth of it, as a perspective draws what is further off smaller; its Zoom is 1.
type scaledCam struct{ camera.Camera }

func (scaledCam) ScaleAt(_, y, _ float32) float32 {
	if y < 64 {
		return 1
	}
	return 0.1
}

// The grid goes with each cell's own size on screen: through a camera whose scale changes over the
// screen, the cells near the eye are outlined and those too small to read far off are not.
func TestRenderer_Compose_OutlinesEachCellAsLargeAsItIsDrawn(t *testing.T) {
	grid := DefaultGrids{}.Square(4, 4, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	r := flatRenderer(brd, &RenderState{ShowGridLines: true})
	var f render.Frame
	cam := scaledCam{icamera.NewFromSpace(128, 128, 0)}
	f.Reset(cam)
	r.Compose(&f, cam)
	near, far := 0, 0 // outlined tiles in the two nearer rows, and further
	f.Each(func(tier render.Tier, _ float32, v []render.Vertex) {
		if tier != render.Ground || v[0].Custom0 >= 0 {
			return
		}
		if v[0].DstY < 64 {
			near++
		} else {
			far++
		}
	})
	if near != 8 || far != 0 {
		t.Errorf("outlined %d tiles in the nearer rows and %d further, want the 8 near ones: 32 pixels a cell there, 3 further", near, far)
	}
}
