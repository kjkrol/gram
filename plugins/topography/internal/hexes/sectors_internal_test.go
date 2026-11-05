package hexes

import (
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world/view"
)

// four sectors of 512 in a row, the last asleep, the rest at the level a camera sees each at its
// left edge from half a pixel a world unit
type row struct{}

func (row) Len() int  { return 4 }
func (row) Cols() int { return 4 }
func (row) Box(i int) geom.AABB {
	return geom.NewAABB(geom.NewVec(float32(i)*512, 0), geom.NewVec(float32(i+1)*512, 512))
}
func (row) Of(x, _ float32) int { return min(max(int(x/512), 0), 3) }
func (row) Ladder() lod.Ladder  { return lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 0.5}} }

func (r row) Level(cam camera.Camera, i int) lod.Level {
	if i == 3 {
		return lod.L6
	}
	return r.Ladder().At(camera.ScaleShown(cam, float32(i)*512, 0, 0), 0)
}

type seeing struct{ camera.Camera }

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
	// asleep
	h := &Ground{sectors: row{}, runs: []run{{0, 2}, {2, 1}, {3, 3}, {6, 1}},
		cells: make([]float32, 7*cellFloats)}
	type drawn struct {
		cells, vertices int
		level           lod.Level
	}
	var got []drawn
	h.each(v, func(cells []float32, vertices int, level lod.Level) {
		got = append(got, drawn{len(cells) / cellFloats, vertices, level})
	})
	want := []drawn{{3, wholeVertices, lod.L2}, {3, topVertices, lod.L3}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("drawn %v, want the near sectors' cells whole at one go, the far one's tops, "+
			"nothing beyond", got)
	}
}

// five sectors of 512 in a row, each at its own level from first on
type ladder struct{ first lod.Level }

func (ladder) Len() int  { return 5 }
func (ladder) Cols() int { return 5 }
func (ladder) Box(i int) geom.AABB {
	return geom.NewAABB(geom.NewVec(float32(i)*512, 0), geom.NewVec(float32(i+1)*512, 512))
}
func (ladder) Of(x, _ float32) int                      { return min(max(int(x/512), 0), 4) }
func (ladder) Ladder() lod.Ladder                       { return lod.Ladder{} }
func (l ladder) Level(_ camera.Camera, i int) lod.Level { return l.first + lod.Level(i) }

func TestEach_DrawsTheTopsAtL3AndL4AndASheetAtL5(t *testing.T) {
	cam := seeing{cameratest.NewFromSpace(2560, 512, 0)}
	v := view.NewSeen(cam, ladder{lod.Standard})
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 2560, Height: 512, BucketSize: 128})
	if err != nil {
		t.Fatal(err)
	}
	views := []*view.View{v}
	view.NewSystem(space, &views, 2560, 512).Update(nil, 0)
	// a cell in each sector, two in the one at L5, the mean of whose corners stands 6 high
	h := &Ground{sectors: ladder{lod.Standard}, runs: []run{{0, 1}, {1, 1}, {2, 1}, {3, 2}, {5, 1}},
		heights: []float32{0, 0, 0, 6, 0}, cells: make([]float32, 6*cellFloats)}
	type drawn struct {
		cells, vertices int
		level           lod.Level
	}
	var got []drawn
	h.each(v, func(cells []float32, vertices int, level lod.Level) {
		got = append(got, drawn{len(cells) / cellFloats, vertices, level})
	})
	want := []drawn{{1, wholeVertices, lod.L2}, {1, topVertices, lod.L3},
		{1, topVertices, lod.L4}}
	if !slices.Equal(got, want) {
		t.Errorf("drawn %v, want the sector at L2 whole and those at L3 and L4 their tops, a "+
			"level at a go", got)
	}
	sheet := []float32{1536, 0, 2048, 512, 6, 0, 0, 0}
	if !slices.Equal(h.sheet, sheet) {
		t.Errorf("sheets %v, want the sector at L5's box at its cells' mean height %v", h.sheet,
			sheet)
	}
	if want := [lod.Levels]int{0, 0, 1, 1, 1, 1, 0}; h.Levels() != want {
		t.Errorf("sectors drawn by level %v, want %v", h.Levels(), want)
	}
	if n := testing.AllocsPerRun(10, func() {
		h.each(v, func([]float32, int, lod.Level) {})
	}); n != 0 {
		t.Errorf("%v allocations a frame, want none", n)
	}
}

func TestEach_DrawsTheCloseLevelsWholeAsStandard(t *testing.T) {
	cam := seeing{cameratest.NewFromSpace(2560, 512, 0)}
	v := view.NewSeen(cam, ladder{lod.Close})
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 2560, Height: 512, BucketSize: 128})
	if err != nil {
		t.Fatal(err)
	}
	views := []*view.View{v}
	view.NewSystem(space, &views, 2560, 512).Update(nil, 0)
	h := &Ground{sectors: ladder{lod.Close}, runs: []run{{0, 1}, {1, 1}, {2, 1}, {3, 1}, {4, 1}},
		heights: make([]float32, 5), cells: make([]float32, 5*cellFloats)}
	type drawn struct {
		cells, vertices int
		level           lod.Level
	}
	var got []drawn
	h.each(v, func(cells []float32, vertices int, level lod.Level) {
		got = append(got, drawn{len(cells) / cellFloats, vertices, level})
	})
	want := []drawn{{3, wholeVertices, lod.L2}, {1, topVertices, lod.L3},
		{1, topVertices, lod.L4}}
	if !slices.Equal(got, want) || len(h.sheet) != 0 {
		t.Errorf("drawn %v and sheets %v, want the sectors at L0 to L2 whole at one go, those at "+
			"L3 and L4 their tops, no sheet", got, h.sheet)
	}
}
