package topography

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Through a perspective each tile is drawn with the detail of where it is: one near the eye with
// all of it and dressed in full, one far off with none and from the ground sheet; drawn from above
// or isometrically every tile alike, as the zoom says.
func TestDresser_DetailGoesWithEachTilesDistanceThroughAPerspective(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(100, 100, 32)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	d := newDresser(brd, reliefFor(brd), func() world.Sun { return world.DefaultSun }, true, st)
	type seen struct {
		px, detail float32
		baked      bool
	}
	tiles := map[board.CellID]seen{}
	look := lookFn(func(_ *render.Frame, _ camera.Camera, tl *tile) {
		d.sheeted = true // as with an atlas to paint on
		tiles[tl.ID] = seen{tl.px(), tl.Detail(), d.baked(tl)}
	})
	r := dressed(brd, d, func() world.Sun { return world.DefaultSun }, look)
	cam := newCamera(testProjection, 3200, 3200, 0, camera.Config{ViewportWidth: 400, ViewportHeight: 300}, 0, true, nil, nil)
	cam.enterInside([3]float32{3000, 3000, 40}, 0)
	cam.Tilt(0.25)
	compose(r, cam)
	ahead := facing(0)
	at := func(dist float64) seen {
		c, ok := brd.CellAt(geom.NewVec(3000+ahead.X*dist, 3000+ahead.Y*dist))
		if !ok {
			t.Fatalf("no cell %v ahead", dist)
		}
		s, drawn := tiles[c]
		if !drawn {
			t.Fatalf("the cell %v ahead was not drawn", dist)
		}
		return s
	}
	near, far := at(96), at(2500)
	if near.px < nearCell || near.detail != 1 || near.baked {
		t.Errorf("a tile 96 ahead spans %v px with detail %v, baked %v; want over %v px, all its detail, dressed in full", near.px, near.detail, near.baked, nearCell)
	}
	if far.px >= farCell || far.detail != 0 || !far.baked {
		t.Errorf("a tile 2500 ahead spans %v px with detail %v, baked %v; want under %v px, none of it, from the sheet", far.px, far.detail, far.baked, farCell)
	}
	clear(tiles)
	iso := newCamera(testProjection, 3200, 3200, 0, camera.Config{ViewportWidth: 400, ViewportHeight: 300}, 0, true, nil, nil)
	compose(r, iso)
	want := float32(32) * iso.Zoom()
	for c, s := range tiles {
		if math.Abs(float64(s.px-want)) > 1e-3 {
			t.Fatalf("isometrically the tile %v spans %v px, want every one %v", c, s.px, want)
		}
	}
}
