package topography

import (
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
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
	cam := newCamera(testProjection, 3200, 3200, 0, camera.Config{ViewportWidth: 400, ViewportHeight: 300}, 0, true, nil, nil, 0)
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
	iso := newCamera(testProjection, 3200, 3200, 0, camera.Config{ViewportWidth: 400, ViewportHeight: 300}, 0, true, nil, nil, 0)
	compose(r, iso)
	want := float32(32) * iso.Zoom()
	for c, s := range tiles {
		if math.Abs(float64(s.px-want)) > 1e-3 {
			t.Fatalf("isometrically the tile %v spans %v px, want every one %v", c, s.px, want)
		}
	}
}

// Riding low on a steep slope and looking down it, the tile the eye stands in has corners behind
// the eye: drawn as it is, each thrown millions of pixels off, it would cover the sky. It is drawn
// in pieces instead, only those in front, every one on or about the screen, with all its detail.
func TestBlocks_TheTileTheEyeStandsInIsDrawnOnlyInFront(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(20, 20, 32)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	relief := reliefFor(brd)
	relief.SetHeights(func(p geom.Vec) float64 { return (640 - p.X) + (640 - p.Y) }) // rising to the north-west
	d := newDresser(brd, relief, func() world.Sun { return world.DefaultSun }, true, st)
	r := board.NewRenderer(brd, flatAtlas{}, testMap{look: blocks{d: d}, d: d}, func() world.Sun { return world.DefaultSun })
	ground := func(x, y float32) float32 { return float32(relief.GroundAt(geom.NewVec(float64(x), float64(y)))) }
	extent := func() (float32, float32) { low, high := relief.Extent(); return float32(low), float32(high) }
	cam := newCamera(testProjection, 640, 640, 0, camera.Config{ViewportWidth: 400, ViewportHeight: 300}, 0, true, ground, extent, 0)
	cam.enterInside([3]float32{320, 320, ground(320, 320) + 0.64}, math.Pi) // looking down the slope, south-east
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	pieces := 0
	f.Each(func(tier render.Tier, _ float32, v []ebiten.Vertex) {
		if tier != render.Ground {
			return
		}
		pieces++
		for _, x := range v {
			if x.DstX < -4*400 || x.DstX > 5*400 || x.DstY < -4*300 || x.DstY > 5*300 {
				t.Fatalf("a piece of ground has a corner at (%v, %v), thrown far off the 400 x 300 screen", x.DstX, x.DstY)
			}
		}
	})
	if pieces == 0 {
		t.Fatal("no ground was drawn")
	}
	c, _ := brd.CellAt(geom.NewVec(320, 320))
	var bt board.Tile
	center := brd.CellCenter(c)
	bt.ID, bt.X0, bt.Y0, bt.X1, bt.Y1 = c, float32(center.X-16), float32(center.Y-16), float32(center.X+16), float32(center.Y+16)
	d.Begin(&f, cam)
	if px := d.tileOf(&bt).px(); px < nearCell {
		t.Errorf("the tile the eye stands in spans %v px by its nearest corner, want all its detail, over %v", px, nearCell)
	}
}
