package navigation

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// A route sprite lies on the tile as the board's Map draws it: its corners at the tops' heights,
// its depth at the cell's level; without tops, on the ground at 0.
func TestPathRenderer_LaysTheSpriteOnTheTilesCorners(t *testing.T) {
	grid := board.DefaultGrids{}.Square(4, 4, 32)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	relief := topography.NewRelief(brd)
	relief.SetHeights(topography.MeanOfCells(grid, func(c board.CellID) float64 { // a ridge down the right half
		if x, _, _ := grid.Coords(c); x >= 2 {
			return 10
		}
		return 0
	}))
	cam := isoCamera(128, 128, camera.Config{})
	slope, _ := grid.CellIndex(1, 1) // its right corners meet the ridge
	drawn := func(r *PathRenderer) (v []ebiten.Vertex, depth float32) {
		var f render.Frame
		f.Reset(cam)
		r.frame, r.camera = &f, cam
		r.appendCellSprite(slope, 0)
		f.Each(func(_ render.Tier, d float32, verts []ebiten.Vertex) { v, depth = verts, d })
		return
	}
	r := NewPathRenderer(brd, sheetOf{}, PathSprites{}, 0).WithTops(func(c board.CellID) ([4]float32, float32) { return relief.Top(c, 0) })
	v, depth := drawn(r)
	_, left := cam.Project(32, 32, 0)
	_, right := cam.Project(64, 32, 5)
	if len(v) != 4 || v[0].DstY != left || v[1].DstY != right {
		t.Errorf("the sprite's top corners are drawn at y %v and %v, want %v and %v: the tile's corners, the right one 5 up", v[0].DstY, v[1].DstY, left, right)
	}
	if depth != cam.Depth(48, 48, float32(relief.Altitude(slope))) {
		t.Errorf("the sprite lies at depth %v, want the cell's at its level", depth)
	}
	flat := NewPathRenderer(brd, sheetOf{}, PathSprites{}, 0)
	_, level := cam.Project(64, 32, 0)
	if v, _ := drawn(flat); len(v) != 4 || v[1].DstY != level {
		t.Errorf("without tops the sprite's top-right corner is drawn at y %v, want %v: on the ground", v[1].DstY, level)
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
		Heights:  true,
	})
	b := board.NewPlugin(board.DefaultGrids{}.Square(width/32, height/32, 32), &board.MultipleOccupancy{}, w)
	topography.NewPlugin(w, b, topography.Config{Cell: 32, HeightUnit: 1, Isometric: true})
	return w.Camera()
}
