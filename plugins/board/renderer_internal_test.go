package board

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

type flatAtlas struct{}

func (flatAtlas) Atlas() *ebiten.Image                            { return nil }
func (flatAtlas) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 1, 1 }

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

func TestRenderer_Submit_OneQuadPerVisibleCellAtItsAltitude(t *testing.T) {
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
	cam := camera.NewFromSpaceWithConfig(128, 128, 0, camera.Config{Projection: camera.Isometric{Cell: 32, HeightUnit: 1}})
	r := newRenderer(cam, brd, flatAtlas{}, &RenderState{})

	var sink render.Sink
	r.Submit(&sink)
	if sink.Len() != 16 {
		t.Errorf("submitted %d quads, want the 16 cells: the hill slopes into its neighbours, no faces", sink.Len())
	}

	wall, _ := grid.CellIndex(2, 2)
	brd.Set(wall, CellKind{Cost: 1, Allows: Land, Solid: true, Height: 8})
	sink = render.Sink{}
	r.Submit(&sink)
	if sink.Len() != 18 {
		t.Errorf("submitted %d quads, want two more for the wall's faces down to the ground", sink.Len())
	}

	flat := newRenderer(camera.NewFromSpace(128, 128, 0), brd, flatAtlas{}, &RenderState{})
	sink = render.Sink{}
	flat.Submit(&sink)
	if sink.Len() != 16 {
		t.Errorf("top-down submitted %d quads, want the 16 cells alone: no faces from above", sink.Len())
	}

	// A point 10 up is drawn HeightUnit·10 above the ground under it.
	flatX, flatY := cam.Project(48, 48, 0)
	hillX, hillY := cam.Project(48, 48, 10)
	if hillX != flatX || hillY != flatY-10 {
		t.Errorf("a point 10 up is drawn at (%v, %v), the ground under it at (%v, %v); want 10 higher", hillX, hillY, flatX, flatY)
	}
}
