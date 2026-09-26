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

func (flatAtlas) Atlas() *ebiten.Image                            { return nil }
func (flatAtlas) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 1, 1 }
func (flatAtlas) White() (u, v float32)                           { return 0, 0 }

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

// outlinedTiles counts the tiles r hands a frame through cam that carry an outline.
func outlinedTiles(r *Renderer, cam camera.Camera) (tiles, outlined int) {
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	f.Each(func(tier render.Tier, _ float32, v []ebiten.Vertex) {
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

// lightsOf composes brd from above in a world with heights under sun and gives each cell's light.
func lightsOf(brd *Board, sun world.Sun) map[CellID]render.Shade {
	brd.quasi3D = true
	out := map[CellID]render.Shade{}
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *Tile) { out[t.ID] = t.Light() })
	r := newRenderer(brd, flatAtlas{}, &RenderState{}, func() Look { return look }, func() world.Sun { return sun })
	compose(r, icamera.NewFromSpace(256, 256, 0))
	return out
}

func TestTile_LightFollowsTheSlopeOfTheGround(t *testing.T) {
	grid := DefaultGrids{}.Square(4, 4, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	hill, _ := grid.CellIndex(1, 1)
	brd.SetHeights(MeanOfCells(grid, func(c CellID) float64 {
		if c == hill {
			return 16
		}
		return 0
	}))
	sun := world.Sun{Dir: [3]float32{1, 0, 1}, Strength: 0.6, Ambient: 0.3} // from the east, 45° up
	lights := lightsOf(brd, sun)
	at := func(x, y uint32) render.Shade { c, _ := grid.CellIndex(x, y); return lights[c] }

	level := sun.Light(0, 0, 1)
	if far := at(3, 3); far != render.Even(level) {
		t.Errorf("level ground far from the hill is lit %v, want %v everywhere", far, level)
	}
	// the hill's east side falls away towards the sun, its west side rises away from it
	if east, west := at(2, 1)[0], at(0, 1)[1]; east <= level || west >= level {
		t.Errorf("the hill's sunny side is lit %v and its shady side %v, want above and below level %v", east, west, level)
	}
	// neighbouring tiles agree on the corner they share: the slope runs on without a seam
	if a, b := at(1, 1)[1], at(2, 1)[0]; a != b {
		t.Errorf("the corner the hill shares with its east neighbour is lit %v from one side and %v from the other", a, b)
	}
}

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

// wallInSun is a 6x3 board of level grass with a wall 10 tall at (3, 1), under a sun low in the
// east: its shadow falls 50 to the west.
func wallInSun(t *testing.T) (*Board, Grid, world.Sun) {
	t.Helper()
	grid := DefaultGrids{}.Square(6, 3, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	wall, _ := grid.CellIndex(3, 1)
	brd.Set(wall, CellKind{Cost: 1, Allows: Land, Solid: true, Height: 10})
	return brd, grid, world.Sun{Dir: [3]float32{1, 0, 0.2}, Strength: 0.6, Ambient: 0.3}
}

func TestTile_TheTerrainCastsItsShadowAwayFromTheSun(t *testing.T) {
	brd, grid, sun := wallInSun(t)
	lights := lightsOf(brd, sun)
	at := func(x uint32) render.Shade { c, _ := grid.CellIndex(x, 1); return lights[c] }
	lit, shade := sun.Light(0, 0, 1), sun.Ambient

	// the wall's west edge is at x 96: the grass right behind it is in shadow up to 50 away
	if got := at(2); got[1] != shade || got[0] != shade {
		t.Errorf("the grass behind the wall is lit %v, want its corners 32 and 0 from the wall in shadow %v", got, shade)
	}
	if got := at(1); got[1] != shade || got[0] != lit {
		t.Errorf("the next tile is lit %v, want 32 from the wall in shadow and 64 from it in the sun", got)
	}
	if got := at(4); got[0] != lit || got[1] != lit {
		t.Errorf("the grass on the sunny side is lit %v, want %v", got, lit)
	}
}

func TestTile_AShadowGoesWithWhatCastItAndWithTheSun(t *testing.T) {
	brd, grid, sun := wallInSun(t)
	brd.quasi3D = true
	var got map[CellID]render.Shade
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *Tile) { got[t.ID] = t.Light() })
	current := sun
	r := newRenderer(brd, flatAtlas{}, &RenderState{}, func() Look { return look }, func() world.Sun { return current })
	behind, _ := grid.CellIndex(2, 1)
	frame := func() render.Shade {
		got = map[CellID]render.Shade{}
		compose(r, icamera.NewFromSpace(192, 96, 0))
		return got[behind]
	}
	if frame()[1] != sun.Ambient {
		t.Fatal("no shadow behind the wall to begin with")
	}
	noon := world.Sun{Dir: [3]float32{0, 0, 1}, Strength: 0.6, Ambient: 0.3}
	current = noon
	if l := frame()[1]; l != noon.Light(0, 0, 1) {
		t.Errorf("under a sun overhead the grass behind the wall is lit %v, want %v: no shadow", l, noon.Light(0, 0, 1))
	}
	current = sun
	wall, _ := grid.CellIndex(3, 1)
	brd.Set(wall, CellKind{Cost: 1, Allows: Land})
	if l := frame()[1]; l != sun.Light(0, 0, 1) {
		t.Errorf("with the wall knocked down the grass is lit %v, want the full sun %v", l, sun.Light(0, 0, 1))
	}
	r.shadows = false
	brd.Set(wall, CellKind{Cost: 1, Allows: Land, Solid: true, Height: 10})
	if l := frame()[1]; l != sun.Light(0, 0, 1) {
		t.Errorf("with shadows off the grass behind the wall is lit %v, want the full sun", l)
	}
}
