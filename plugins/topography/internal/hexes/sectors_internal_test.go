package hexes

import (
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world/view"
)

// four sectors of 512 in a row
type row struct{}

func (row) Len() int  { return 4 }
func (row) Cols() int { return 4 }
func (row) Box(i int) geom.AABB {
	return geom.NewAABB(geom.NewVec(float64(i)*512, 0), geom.NewVec(float64(i+1)*512, 512))
}
func (row) Of(x, _ float64) int { return min(max(int(x/512), 0), 3) }
func (row) Off(i int) bool      { return i == 3 }

type seeing struct{ camera.Camera }

func (seeing) Detail() camera.Detail    { return camera.Detail{Coarse: 0.5} }
func (seeing) Visible(camera.AABB) bool { return true }
func (seeing) ScaleAt(x, _, _ float32) float32 {
	return map[bool]float32{true: 1, false: 0.1}[x < 1024]
}

func TestEach_DrawsTheSectorsSeenWholeOrTheirTopsAloneNeighboursAtOneGo(t *testing.T) {
	cam := seeing{cameratest.NewFromSpace(2048, 512, 0)}
	v := view.NewSeen(cam, row{})
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 2048, Height: 512, BucketSize: 128})
	if err != nil {
		t.Fatal(err)
	}
	views := []*view.View{v}
	view.NewSystem(space, &views, 2048, 512).Update(nil, 0)
	// two cells in the first sector, one in the second, three in the third, one in the fourth,
	// beyond the bands
	h := &Ground{sectors: row{}, runs: []run{{0, 2}, {2, 1}, {3, 3}, {6, 1}},
		cells: make([]float32, 7*cellFloats)}
	type drawn struct{ cells, vertices int }
	var got []drawn
	h.each(v, func(cells []float32, vertices int) {
		got = append(got, drawn{len(cells) / cellFloats, vertices})
	})
	want := []drawn{{3, prismVertices}, {3, topVertices}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("drawn %v, want the near sectors' prisms whole at one go, the far one's tops, "+
			"nothing beyond", got)
	}
}
