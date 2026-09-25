package navigation

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugins/board"
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
	cam := icamera.NewFromSpaceWithConfig(128, 128, 0, camera.Config{Projection: camera.Isometric{Cell: 32}})
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
