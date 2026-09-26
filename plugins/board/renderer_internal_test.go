package board

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type flatAtlas struct{}

func (flatAtlas) Atlas() *ebiten.Image { return nil }

func (flatAtlas) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 1, 1 }

func (flatAtlas) White() (u, v float32) { return 0, 0 }

// flatRenderer is a renderer of brd with the flat look.
func flatRenderer(brd *Board, state *RenderState) *Renderer {
	return newRenderer(brd, flatAtlas{}, state, func() Look { return flatLook{} }, func() world.Sun { return world.DefaultSun })
}

// compose is what r hands a frame through cam, by tier.
func compose(r *Renderer, cam camera.Camera) map[render.Tier]int {
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	out := map[render.Tier]int{}
	f.Each(func(tier render.Tier, _ float32, _ []ebiten.Vertex) { out[tier]++ })
	return out
}

// outlinedTiles counts the tiles r hands a frame through cam and the outlines laid over them.
func outlinedTiles(r *Renderer, cam camera.Camera) (tiles, outlined int) {
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	f.Each(func(tier render.Tier, _ float32, v []ebiten.Vertex) {
		if tier != render.Ground {
			return
		}
		switch {
		case v[0].ColorA <= 1.5:
			tiles++
		case v[0].Custom0 < 0: // an overlay drawing an outline
			outlined++
		}
	})
	return tiles, outlined
}

func TestRenderer_Compose_FlatLaysEveryVisibleCellOverItsBox(t *testing.T) {
	grid := DefaultGrids{}.Square(4, 4, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land, Height: 0})
	if got := compose(flatRenderer(brd, &RenderState{}), icamera.NewFromSpace(128, 128, 0)); len(got) != 1 || got[render.Ground] != 16 {
		t.Errorf("composed %v, want the 16 cells on the ground and nothing else", got)
	}
}

func TestRenderer_Compose_OutlinesSquareTilesWhenCellsAreLargeEnough(t *testing.T) {
	grid := DefaultGrids{}.Square(4, 4, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	r := flatRenderer(brd, &RenderState{ShowGridLines: true})

	cam := icamera.NewFromSpace(128, 128, 0)
	if got := compose(r, cam); len(got) != 1 || got[render.Ground] != 32 {
		t.Errorf("composed %v, want the 16 tiles and their outlines and nothing more", got)
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
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	got := compose(flatRenderer(brd, &RenderState{ShowGridLines: true}), icamera.NewFromSpace(256, 256, 0))
	if got[gridTier] == 0 || got[render.Ground] != 9 {
		t.Errorf("composed %v, want the nine hexes and the lines round them", got)
	}
}

// A Look is asked for every visible cell, and a tile tells it the heights it needs.
func TestRenderer_Compose_HandsTheLookEveryCellWithItsHeights(t *testing.T) {
	grid := DefaultGrids{}.Square(4, 4, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	hill, _ := grid.CellIndex(1, 1)
	brd.Set(hill, CellKind{Cost: 1, Allows: Land, Height: 8})
	var seen int
	var hillTop, besideHill [4]float32
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *Tile) {
		seen++
		if t.ID == hill {
			hillTop, _ = t.Top()
		}
		if left, _ := grid.CellIndex(0, 1); t.ID == left {
			besideHill = t.Beside(1, 0)
		}
	})
	r := newRenderer(brd, flatAtlas{}, &RenderState{}, func() Look { return look }, func() world.Sun { return world.DefaultSun })
	compose(r, icamera.NewFromSpace(128, 128, 0))
	if seen != 16 || hillTop != [4]float32{8, 8, 8, 8} || besideHill != hillTop {
		t.Errorf("look saw %d cells, the hill's top %v and beside it %v; want 16 and the kind's 8 everywhere", seen, hillTop, besideHill)
	}
}

// lookFn adapts a function to Look.
type lookFn func(f *render.Frame, cam camera.Camera, t *Tile)

func (fn lookFn) Cell(f *render.Frame, cam camera.Camera, t *Tile) { fn(f, cam, t) }

func TestTile_AFlatWorldIsDrawnAsItsSpritesAre(t *testing.T) {
	grid := DefaultGrids{}.Square(2, 2, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	var got render.Shade
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *Tile) { got = t.Light() })
	r := newRenderer(brd, flatAtlas{}, &RenderState{}, func() Look { return look }, func() world.Sun { return world.DefaultSun })
	compose(r, icamera.NewFromSpace(64, 64, 0))
	if got != render.Even(1) {
		t.Errorf("a flat world's tile is lit %v, want as drawn", got)
	}
}

func TestFlatLook_SwaysWhatSways(t *testing.T) {
	grid := DefaultGrids{}.Square(4, 4, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	brd.quasi3D = true
	r := flatRenderer(brd, &RenderState{})
	cam := icamera.NewFromSpace(128, 128, 0)
	var air world.Weather
	r.weather = func() world.Weather { return air }

	tree, _ := grid.CellIndex(1, 1)
	brd.Set(tree, CellKind{Cost: 1, Allows: Land, Height: 8, Sway: 1})
	at := func() float32 { // where the tree's top is drawn
		var f render.Frame
		f.Reset(cam)
		r.Compose(&f, cam)
		x := float32(-1)
		f.Each(func(_ render.Tier, _ float32, v []ebiten.Vertex) {
			if x < 0 && v[0].DstY > 28 && v[0].DstY < 36 && v[0].DstX > 28 && v[0].DstX < 44 { // cell (1, 1)
				x = v[0].DstX
			}
		})
		return x
	}
	calm := at()
	air.Wind = [2]float32{40, 0}
	if blown := at(); blown <= calm {
		t.Errorf("in an east wind the tree's top is drawn at x %v, in the calm at %v; want it leaning east", blown, calm)
	}
}
