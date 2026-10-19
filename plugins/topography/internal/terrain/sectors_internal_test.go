package terrain

import (
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world/view"
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
			t.Errorf("%v: triangles cover %v cells, want %d", c, float64(area)/2, w*h)
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

// two sectors of 512 side by side, as halves of a world 1024 wide
type pair struct{}

func (pair) Len() int  { return 2 }
func (pair) Cols() int { return 2 }
func (pair) Box(i int) geom.AABB {
	return geom.NewAABB(geom.NewVec(float64(i)*512, 0), geom.NewVec(float64(i+1)*512, 512))
}
func (pair) Of(x, _ float64) int { return min(max(int(x/512), 0), 1) }
func (pair) Off(int) bool        { return false }

type seeing struct {
	camera.Camera
	d camera.Detail
}

func (s seeing) Detail() camera.Detail  { return s.d }
func (seeing) Visible(camera.AABB) bool { return true }
func (seeing) ScaleAt(x, _, _ float32) float32 {
	return map[bool]float32{true: 1, false: 0.1}[x < 512]
}

func TestRuns_DrawEachSectorSeenAtItsLevelAndTheSkirt(t *testing.T) {
	cam := seeing{Camera: cameratest.NewFromSpace(1024, 512, 0), d: camera.Detail{Coarse: 0.5}}
	v := view.NewSeen(cam, pair{})
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 1024, Height: 512, BucketSize: 128})
	if err != nil {
		t.Fatal(err)
	}
	views := []*view.View{v}
	view.NewSystem(space, &views, 1024, 512).Update(nil, 0)
	r := &Renderer{cfg: Config{Seen: func(camera.Camera) *view.View { return v }}, grid: pair{}}
	_, _, enc := encode(make([]float32, 33*17), nil)
	r.lay(33, 17, 32, enc)
	got := r.runs(cam, nil)
	// the left sector whole, then the right one coarse and the skirt, which follow one another,
	// all with full care as no care is set
	whole := r.spans[0].whole
	whole.care = fullCare
	want := []span{whole, {r.spans[1].coarse.first,
		r.spans[1].coarse.count + r.skirted.count, fullCare}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("runs %v, want %v", got, want)
	}
}

// five sectors of 512 by three, the world 2560 by 1536
type sheet struct{}

func (sheet) Len() int  { return 15 }
func (sheet) Cols() int { return 5 }
func (sheet) Box(i int) geom.AABB {
	c, r := float64(i%5), float64(i/5)
	return geom.NewAABB(geom.NewVec(c*512, r*512), geom.NewVec((c+1)*512, (r+1)*512))
}
func (sheet) Of(x, y float64) int {
	return min(max(int(y/512), 0), 2)*5 + min(max(int(x/512), 0), 4)
}
func (sheet) Off(int) bool { return false }

// seeing everything exactly, the middle of the screen over the second sector of the middle row
// and, in a perspective, the eye over the sector given
type centred struct {
	seeing
	eye int
}

func (centred) Focus() (x, y, z float32) { return 768, 768, 0 }
func (c centred) Eye() (x, y, z float32, ok bool) {
	return float32(c.eye%5)*512 + 256, float32(c.eye/5)*512 + 256, 100, c.eye >= 0
}

func TestRuns_ShadeTheSectorsWithLessCareRingByRingFromTheScreensMiddle(t *testing.T) {
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 2560, Height: 1536, BucketSize: 128})
	if err != nil {
		t.Fatal(err)
	}
	_, _, enc := encode(make([]float32, 81*49), nil)
	type ring struct{ sectors, care int }
	for _, tc := range []struct {
		name              string
		eye               int
		want              []ring
		full, less, least int
	}{
		// the eye's sector, the screen's middle's, counts as a middle too: the eye over the
		// row's last sector leaves no sector two or more rings from either
		{"the middle alone", -1, []ring{{3, fullCare}, {1, lessCare}, {1, leastCare},
			{3, fullCare}, {1, lessCare}, {1, leastCare},
			{3, fullCare}, {1, lessCare}, {1, leastCare}}, 9, 3, 3},
		{"the middle and the eye", 9, []ring{{15, fullCare}}, 15, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cam := centred{seeing{Camera: cameratest.NewFromSpace(2560, 1536, 0)}, tc.eye}
			v := view.NewSeen(cam, sheet{})
			views := []*view.View{v}
			view.NewSystem(space, &views, 2560, 1536).Update(nil, 0)
			r := &Renderer{cfg: Config{Seen: func(camera.Camera) *view.View { return v }},
				grid: sheet{}, care: 1}
			r.lay(81, 49, 32, enc)
			got := r.runs(cam, nil)
			// a run the sectors following one another with one care, and the skirt with the
			// least care apart, laid after the coarse runs
			var want []span
			from := 0
			for _, w := range tc.want {
				count := 0
				for i := range w.sectors {
					count += r.spans[from+i].whole.count
				}
				want = append(want, span{r.spans[from].whole.first, count, w.care})
				from += w.sectors
			}
			want = append(want, span{r.skirted.first, r.skirted.count, leastCare})
			if len(got) != len(want) {
				t.Fatalf("%d runs %v, want %d %v", len(got), got, len(want), want)
			}
			for k := range want {
				if got[k] != want[k] {
					t.Errorf("run %d is %v, want %v", k, got[k], want[k])
				}
			}
			if full, less, least := r.counts[fullCare], r.counts[lessCare],
				r.counts[leastCare]; full != tc.full || less != tc.less || least != tc.least {
				t.Errorf("%d sectors with full care, %d less, %d least; want %d, %d, %d",
					full, less, least, tc.full, tc.less, tc.least)
			}
		})
	}
}
