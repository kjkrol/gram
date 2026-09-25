package board

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/render"
)

type flatAtlas struct{}

func (flatAtlas) Atlas() *ebiten.Image                            { return nil }
func (flatAtlas) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 1, 1 }
func (flatAtlas) White() (u, v float32)                           { return 0, 0 }

// compose is what r hands a frame through cam, by tier.
func compose(r *Renderer, cam camera.Camera) map[render.Tier]int {
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	out := map[render.Tier]int{}
	f.Each(func(tier render.Tier, _ float32, _ []ebiten.Vertex) { out[tier]++ })
	return out
}

func TestSlopeShade_LightsTheTileFromTheUpperLeft(t *testing.T) {
	level := slopeShade([4]float32{5, 5, 5, 5})
	if level != shadeLevel {
		t.Errorf("a level tile is shaded %v, want %v", level, shadeLevel)
	}
	if rising := slopeShade([4]float32{0, 10, 0, 10}); rising >= level { // rises towards the right, away from the light
		t.Errorf("a tile rising to the right is shaded %v, want darker than level %v", rising, level)
	}
	if facing := slopeShade([4]float32{10, 0, 10, 0}); facing <= level { // rises towards the left, into the light
		t.Errorf("a tile rising to the left is shaded %v, want brighter than level %v", facing, level)
	}
}

func TestRenderer_Compose_OneTilePerVisibleCellAtItsAltitude(t *testing.T) {
	grid := DefaultGrids{}.Square(4, 4, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	c, _ := grid.CellIndex(1, 1)
	brd.SetHeights(MeanOfCells(grid, func(at CellID) float64 {
		if at == c {
			return 10
		}
		return 0
	}))
	cam := icamera.NewFromSpaceWithConfig(128, 128, 0, camera.Config{Projection: camera.Isometric{Cell: 32, HeightUnit: 1}})
	r := newRenderer(brd, flatAtlas{}, &RenderState{})

	if got := compose(r, cam); got[render.Ground] != 16 || len(got) != 1 {
		t.Errorf("composed %v, want the 16 cells on the ground: the hill slopes into its neighbours, no faces", got)
	}

	wall, _ := grid.CellIndex(2, 2)
	brd.Set(wall, CellKind{Cost: 1, Allows: Land, Solid: true, Height: 8})
	if got := compose(r, cam); got[render.Ground] != 18 {
		t.Errorf("composed %v, want two more for the wall's faces down to the ground", got)
	}

	flat := newRenderer(brd, flatAtlas{}, &RenderState{})
	if got := compose(flat, icamera.NewFromSpace(128, 128, 0)); got[render.Ground] != 16 {
		t.Errorf("top-down composed %v, want the 16 cells alone: no faces from above", got)
	}

	// A point 10 up is drawn HeightUnit·10 above the ground under it.
	flatX, flatY := cam.Project(48, 48, 0)
	hillX, hillY := cam.Project(48, 48, 10)
	if hillX != flatX || hillY != flatY-10 {
		t.Errorf("a point 10 up is drawn at (%v, %v), the ground under it at (%v, %v); want 10 higher", hillX, hillY, flatX, flatY)
	}
}

func TestRenderer_Compose_LaysTheGridOverTheTilesWhenCellsAreLargeEnough(t *testing.T) {
	grid := DefaultGrids{}.Square(4, 4, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	r := newRenderer(brd, flatAtlas{}, &RenderState{ShowGridLines: true})

	cam := icamera.NewFromSpace(128, 128, 0)
	if got := compose(r, cam); got[gridTier] != 4*16 || got[render.Ground] != 16 {
		t.Errorf("composed %v from above, want 16 tiles and four grid lines each over them", got)
	}
	iso := icamera.NewFromSpaceWithConfig(128, 128, 0, camera.Config{Projection: camera.Isometric{Cell: 32, HeightUnit: 1}})
	if got := compose(r, iso); got[gridTier] != 2*16 {
		t.Errorf("composed %v through an isometric camera, want the two near edges of each cell", got)
	}
	cam.SetViewport(20, 20) // the whole board on 20 pixels: a cell is 5 across
	cam.ZoomOut(32, 0, 0)
	if got := compose(r, cam); got[gridTier] != 0 {
		t.Errorf("composed %v with cells a few pixels wide, want no grid", got)
	}
}
