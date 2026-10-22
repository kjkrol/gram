package look_test

import (
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
)

// the left and the right half of a board 128 wide
type halves struct{}

func (halves) Len() int  { return 2 }
func (halves) Cols() int { return 2 }
func (halves) Box(i int) geom.AABB {
	return geom.NewAABB(geom.NewVec(float64(i)*64, 0), geom.NewVec(float64(i+1)*64, 128))
}
func (halves) Of(x, _ float64) int { return min(max(int(x/64), 0), 1) }
func (halves) Off(int) bool        { return false }

// seeing the left half a unit a pixel, the right a tenth of that, from the left edge's middle
type sighted struct {
	camera.Camera
	d camera.Detail
}

func (s sighted) Detail() camera.Detail  { return s.d }
func (sighted) Visible(camera.AABB) bool { return true }
func (sighted) Focus() (x, y, z float32) { return 0, 64, 0 }
func (sighted) ScaleAt(x, _, _ float32) float32 {
	return map[bool]float32{true: 1, false: 0.1}[x < 64]
}

// the tiles a 4 by 4 board of cells 32 wide hands its look through a camera seeing as d says
func tilesSeen(t *testing.T, d camera.Detail) []look.Tile {
	t.Helper()
	brd, brdW := board.NewBoard(grid.DefaultGrids{}.Square(4, 4, 32))
	brdW.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	cam := sighted{Camera: cameratest.NewFromSpace(128, 128, 0), d: d}
	v := view.NewSeen(cam, halves{})
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 128, Height: 128, BucketSize: 32})
	if err != nil {
		t.Fatal(err)
	}
	views := []*view.View{v}
	view.NewSystem(space, &views, 128, 128).Update(nil, 0)
	var seen []look.Tile
	lk := lookFn(func(_ *render.Frame, _ camera.Camera, tile *look.Tile) {
		seen = append(seen, *tile)
	})
	r := newRenderer(brd, flatAtlas{}, look.RenderState{}, mapOf(lk)).
		WithSeen(func(camera.Camera) *view.View { return v })
	compose(r, cam)
	return seen
}

func TestRenderer_Compose_HandsTheLookTheCellsOfTheSectorsSeenAtTheirLevels(t *testing.T) {
	tiles := tilesSeen(t, camera.Detail{Coarse: 0.5})
	coarse := 0
	for _, tile := range tiles {
		if tile.Coarse != (tile.X0 >= 64) {
			t.Errorf("the tile at %v coarse %v, want it coarse in the right half alone", tile.X0,
				tile.Coarse)
		}
		if tile.Coarse {
			coarse++
		}
	}
	if len(tiles) != 16 || coarse != 8 {
		t.Errorf("%d tiles, %d coarse; want all 16, the right 8 coarse", len(tiles), coarse)
	}
	near := tilesSeen(t, camera.Detail{Sight: 60, Fog: 40})
	if len(near) != 8 {
		t.Errorf("seeing 60 far from the left edge, %d tiles, want the left half's 8", len(near))
	}
	fog := map[[2]float32]float32{}
	for _, tile := range near {
		fog[[2]float32{tile.X0, tile.Y0}] = tile.Fog
	}
	// the middles 23 and 51 off the left edge's middle, into the fog from 20 to 60
	if a, b := fog[[2]float32{0, 32}], fog[[2]float32{0, 0}]; a <= 0 || b <= a || b >= 1 {
		t.Errorf("fogged by %v near and %v far, want more the farther, short of all", a, b)
	}
}
