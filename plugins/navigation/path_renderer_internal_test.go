package navigation

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/isometry"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

func TestPathRenderer_SpriteHeightsFollowTheTilesCorners(t *testing.T) {
	grid := board.DefaultGrids{}.Square(4, 4, 32)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	brd.SetHeights(board.MeanOfCells(grid, func(c board.CellID) float64 { // a ridge down the right half
		if x, _, _ := grid.Coords(c); x >= 2 {
			return 10
		}
		return 0
	}))
	cam := isoCamera(128, 128, camera.Config{})
	r := NewPathRenderer(brd, nil, PathSprites{}, 0)
	r.camera = cam

	slope, _ := grid.CellIndex(1, 1) // its right corners meet the ridge
	if z := r.spriteHeights(slope); z[0] >= z[1] || z[2] >= z[3] || z[1] != 5 {
		t.Errorf("sprite heights on the slope = %v, want the tile's corners rising to 5 on the right", z)
	}
	flat := board.DefaultGrids{}.Hex(4, 4, 16)
	hexBoard := board.NewBoard(flat, board.NewTerrainMap())
	hexBoard.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	hexBoard.SetHeights(func(geom.Vec) float64 { return 7 })
	hr := NewPathRenderer(hexBoard, nil, PathSprites{}, 0)
	hr.camera = cam
	c, _ := flat.CellIndex(1, 1)
	if z := hr.spriteHeights(c); z != [4]float32{7, 7, 7, 7} {
		t.Errorf("sprite heights on a hex cell = %v, want the cell's altitude on every corner", z)
	}
}

// sheetOf is an AtlasSource of one sprite with no image behind it.
type sheetOf struct{}

func (sheetOf) Atlas() *ebiten.Image                            { return nil }
func (sheetOf) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 8, 8 }
func (sheetOf) White() (u, v float32)                           { return 9, 9 }

func TestPathRenderer_LaysARouteSpriteOnTheOverlaysTierAtItsCellsDepth(t *testing.T) {
	grid := board.DefaultGrids{}.Square(4, 4, 32)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	cam := isoCamera(128, 128, camera.Config{})
	r := NewPathRenderer(brd, sheetOf{}, PathSprites{}, 0)
	var f render.Frame
	f.Reset(cam)
	r.frame, r.camera = &f, cam

	c, _ := grid.CellIndex(2, 1)
	r.appendCellSprite(c, 0)
	centre := grid.CellCenter(c)
	f.Each(func(tier render.Tier, depth float32, _ []ebiten.Vertex) {
		if want := cam.Depth(float32(centre.X), float32(centre.Y), 0); tier != render.Overlays || depth != want {
			t.Errorf("route sprite on tier %d at depth %v, want Overlays at the cell's %v", tier, depth, want)
		}
	})
	if f.Len() != 1 {
		t.Errorf("%d pieces for one route sprite", f.Len())
	}
}

// isoCamera is a camera of a width x height world put in the isometric view.
func isoCamera(width, height uint32, cfg camera.Config) camera.Camera {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: width, Height: height},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 100},
		Camera:   cfg,
	})
	isometry.NewPlugin(w, isometry.Config{Cell: 32, HeightUnit: 1})
	return w.Camera()
}
