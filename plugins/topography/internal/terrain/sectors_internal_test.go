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

func (pair) Len() int { return 2 }
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
	// the left sector whole, then the right one coarse and the skirt, which follow one another
	want := []span{r.spans[0].whole, {r.spans[1].coarse.first,
		r.spans[1].coarse.count + r.skirted.count}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("runs %v, want %v", got, want)
	}
}
