package terrain

import (
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
)

func TestCoarse_CoversItsCellsWithoutACrackAgainstWholeNeighbours(t *testing.T) {
	const cols = 16
	_, _, enc := encode(make([]float32, cols*cols), nil)
	l := lattice{cols: cols, enc: enc}
	for _, c := range []cells{{0, 0, 10, 8}, {3, 2, 12, 11}, {1, 1, 4, 4}, {0, 0, 15, 15}} {
		tris := l.coarse(c, nil)
		at := func(i uint32) (int, int) { return int(i) % cols, int(i) / cols }
		area, edges := 0, map[[2]uint32]int{}
		for k := 0; k < len(tris); k += 3 {
			ax, ay := at(tris[k])
			bx, by := at(tris[k+1])
			cx, cy := at(tris[k+2])
			// twice the area, signed as a whole cell's triangles go
			twice := (bx-ax)*(cy-ay) - (by-ay)*(cx-ax)
			if twice <= 0 {
				t.Fatalf("%v: triangle %v turns the other way", c, tris[k:k+3])
			}
			area += twice
			for _, e := range [3][2]uint32{{tris[k], tris[k+1]}, {tris[k+1], tris[k+2]},
				{tris[k+2], tris[k]}} {
				edges[[2]uint32{min(e[0], e[1]), max(e[0], e[1])}]++
			}
		}
		w, h := c.x1-c.x0, c.y1-c.y0
		if area != 2*w*h {
			t.Errorf("%v: triangles cover %v cells, want %d", c, float32(area)/2, w*h)
		}
		border := 0
		for e, n := range edges {
			ax, ay := at(e[0])
			bx, by := at(e[1])
			onEdge := ax == bx && (ax == c.x0 || ax == c.x1) ||
				ay == by && (ay == c.y0 || ay == c.y1)
			unit := (ax-bx)*(ax-bx)+(ay-by)*(ay-by) == 1
			switch {
			case n > 2:
				t.Errorf("%v: the edge %v is shared by %d triangles", c, e, n)
			case n == 1 && (!onEdge || !unit):
				t.Errorf("%v: the edge %v is open inside, or long on the border: a crack", c, e)
			case n == 1:
				border++
			}
		}
		if border != 2*(w+h) {
			t.Errorf("%v: %d border edges, want every one of the %d whole", c, border, 2*(w+h))
		}
		if w >= 6 && h >= 6 && len(tris)/3 >= 2*w*h {
			t.Errorf("%v: %d triangles coarse, want fewer than the %d whole", c, len(tris)/3,
				2*w*h)
		}
	}
}

// two sectors of 512 side by side, as halves of a world 1024 wide, each at the level ladder says
// a camera sees it at its left edge
type pair struct{ ladder lod.Ladder }

func (pair) Len() int  { return 2 }
func (pair) Cols() int { return 2 }
func (pair) Box(i int) geom.AABB {
	return geom.NewAABB(geom.NewVec(float32(i)*512, 0), geom.NewVec(float32(i+1)*512, 512))
}
func (pair) Of(x, _ float32) int  { return min(max(int(x/512), 0), 1) }
func (p pair) Ladder() lod.Ladder { return p.ladder }

func (p pair) Level(cam camera.Camera, i int) lod.Level {
	return p.ladder.At(camera.ScaleShown(cam, float32(i)*512, 0, 0), 0)
}

type seeing struct{ camera.Camera }

func (seeing) Visible(camera.AABB) bool { return true }
func (seeing) ScaleAt(x, _, _ float32) float32 {
	return map[bool]float32{true: 1, false: 0.1}[x < 512]
}

func TestRuns_DrawEachSectorSeenAtItsLevelAndTheSkirt(t *testing.T) {
	cam := seeing{Camera: cameratest.NewFromSpace(1024, 512, 0)}
	sectors := pair{ladder: lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 0.5}}}
	v := view.NewSeen(cam, sectors)
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 1024, Height: 512, BucketSize: 128})
	if err != nil {
		t.Fatal(err)
	}
	views := []*view.View{v}
	view.NewSystem(space, &views, 1024, 512).Update(nil, 0)
	r := &Renderer{cfg: Config{Seen: func(camera.Camera) *view.View { return v }}, grid: sectors}
	_, _, enc := encode(make([]float32, 33*17), nil)
	r.lay(33, 17, 32, enc)
	got := r.runs(cam, nil)
	// the left sector at L2 and the right at L3, both whole, then the skirt as a sheet
	want := []span{r.spans[0].whole, r.spans[1].whole, r.skirted}
	want[0].level, want[1].level, want[2].level = lod.L2, lod.L3, lod.L5
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("runs %v, want %v", got, want)
	}
}

// five sectors of 512 by three, the world 2560 by 1536, each seen at the level levels gives it
type sheet struct{ levels [15]lod.Level }

func (sheet) Len() int  { return 15 }
func (sheet) Cols() int { return 5 }
func (sheet) Box(i int) geom.AABB {
	c, r := float32(i%5), float32(i/5)
	return geom.NewAABB(geom.NewVec(c*512, r*512), geom.NewVec((c+1)*512, (r+1)*512))
}
func (sheet) Of(x, y float32) int {
	return min(max(int(y/512), 0), 2)*5 + min(max(int(x/512), 0), 4)
}
func (sheet) Ladder() lod.Ladder                       { return lod.Ladder{} }
func (s sheet) Level(_ camera.Camera, i int) lod.Level { return s.levels[i] }

func TestRuns_LayEachLevelsSectorsWholeOrCoarseTheSkirtAsASheetAndDrawTheFinestFirst(
	t *testing.T) {
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 2560, Height: 1536, BucketSize: 128})
	if err != nil {
		t.Fatal(err)
	}
	_, _, enc := encode(make([]float32, 81*49), nil)
	// the middle row: L3 and L2 side by side, then L4, L5 and one at L6; the rows round it L6
	var grid sheet
	for i := range grid.levels {
		grid.levels[i] = lod.L6
	}
	copy(grid.levels[5:], []lod.Level{lod.L3, lod.L2, lod.L4, lod.L5, lod.L6})
	cam := seeing{Camera: cameratest.NewFromSpace(2560, 1536, 0)}
	v := view.NewSeen(cam, grid)
	views := []*view.View{v}
	view.NewSystem(space, &views, 2560, 1536).Update(nil, 0)
	r := &Renderer{cfg: Config{Seen: func(camera.Camera) *view.View { return v }}, grid: grid}
	r.lay(81, 49, 32, enc)
	got := r.runs(cam, nil)
	at := func(s span, level lod.Level) span {
		s.level = level
		return s
	}
	// the whole runs laid before the coarse ones, and the skirt after those of every sector
	want := []span{at(r.spans[5].whole, lod.L3), at(r.spans[6].whole, lod.L2),
		at(r.spans[7].coarse, lod.L4), at(r.spans[8].coarse, lod.L5), at(r.skirted, lod.L5)}
	if len(got) != len(want) {
		t.Fatalf("%d runs %v, want %d %v", len(got), got, len(want), want)
	}
	for k := range want {
		if got[k] != want[k] {
			t.Errorf("run %d is %v, want %v", k, got[k], want[k])
		}
	}
	if want := [lod.Levels]int{0, 0, 1, 1, 1, 1, 0}; r.counts != want {
		t.Errorf("sectors drawn by level %v, want %v", r.counts, want)
	}
	var levels []lod.Level
	var ranges [][]render.IndexRange
	r.eachLevel(got, func(level lod.Level, rs []render.IndexRange) {
		levels = append(levels, level)
		ranges = append(ranges, append([]render.IndexRange(nil), rs...))
	})
	if !slices.Equal(levels, []lod.Level{lod.L2, lod.L3, lod.L4, lod.L5}) {
		t.Fatalf("drawn by level %v, want the finest first", levels)
	}
	of := func(s span) render.IndexRange {
		return render.IndexRange{First: s.first, Count: s.count}
	}
	for k, want := range [][]render.IndexRange{{of(want[1])}, {of(want[0])}, {of(want[2])},
		{of(want[3]), of(want[4])}} {
		if !slices.Equal(ranges[k], want) {
			t.Errorf("L%d drawn as %v, want %v", levels[k], ranges[k], want)
		}
	}
	if n := testing.AllocsPerRun(10, func() {
		r.eachLevel(got, func(lod.Level, []render.IndexRange) {})
	}); n != 0 {
		t.Errorf("%v allocations a frame grouping the runs by level, want none", n)
	}
}

func TestRuns_LayTheCloseLevelsWholeAndDrawThemAsStandard(t *testing.T) {
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 2560, Height: 1536, BucketSize: 128})
	if err != nil {
		t.Fatal(err)
	}
	_, _, enc := encode(make([]float32, 81*49), nil)
	var grid sheet
	for i := range grid.levels {
		grid.levels[i] = lod.Omitted
	}
	copy(grid.levels[5:], []lod.Level{lod.L0, lod.L1})
	cam := seeing{Camera: cameratest.NewFromSpace(2560, 1536, 0)}
	v := view.NewSeen(cam, grid)
	views := []*view.View{v}
	view.NewSystem(space, &views, 2560, 1536).Update(nil, 0)
	r := &Renderer{cfg: Config{Seen: func(camera.Camera) *view.View { return v }}, grid: grid}
	r.lay(81, 49, 32, enc)
	got := r.runs(cam, nil)
	if len(got) < 2 || got[0].first != r.spans[5].whole.first || got[0].level != lod.L0 ||
		got[1].first != r.spans[6].whole.first || got[1].level != lod.L1 {
		t.Errorf("runs %v, want the sectors at L0 and L1 whole at their levels", got)
	}
	for _, l := range []lod.Level{lod.L0, lod.L1} {
		if byLevel[l] != byLevel[lod.Standard] {
			t.Errorf("L%d drawn by a shader of its own, want Standard's", l)
		}
	}
}
