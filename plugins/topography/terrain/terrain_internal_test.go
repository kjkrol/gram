package terrain

import (
	"image/color"
	"math"
	"testing"

	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/render"
)

// Heights encoded in 16 bits come back within a sixty-thousandth of their span.
func TestEncode_HoldsTheHeightsWithinTheSpan(t *testing.T) {
	heights := []float32{-12.5, 0, 3.25, 180, 99.99}
	low, span, buf := encode(heights, nil)
	if low != -12.5 || span != 192.5 || len(buf) != 4*len(heights) {
		t.Fatalf("low %v, span %v, %d bytes; want -12.5, 192.5 and four a corner", low, span, len(buf))
	}
	for i, h := range heights {
		if got := decode(buf[4*i], buf[4*i+1], low, span); math.Abs(float64(got-h)) > float64(span)/65535 {
			t.Errorf("corner %d encoded as %v, want %v", i, got, h)
		}
		if buf[4*i+3] != 255 {
			t.Errorf("corner %d is not opaque", i)
		}
	}
	if low, span, _ := encode([]float32{5, 5}, nil); low != 5 || span != 1 {
		t.Errorf("level ground encodes with low %v and span %v, want 5 and a span of 1", low, span)
	}
}

// hill is a ground 16 cells across with a ridge 30 high along x = 8 cells.
type hill struct{ version uint64 }

func (hill) Lattice() (cols, rows int, cell float32, heights []float32, ok bool) {
	heights = make([]float32, 17*17)
	for j := range 17 {
		for i := range 17 {
			if i == 8 {
				heights[j*17+i] = 30
			}
		}
	}
	return 17, 17, 32, heights, true
}

func (h hill) Version() uint64 { return h.version }

// flat is a surface of 16 by 16 cells painted grey, a pixel each, with its coast; its version is
// bumped by hand.
type flat struct {
	img    *render.Image
	shores []Shore
	coast  uint64
	grid   float32
}

func (f *flat) Surface() Painted {
	if f.img == nil {
		f.img = render.NewImage(16, 16)
		f.img.Fill(color.RGBA{128, 128, 128, 255})
	}
	return Painted{Albedo: f.img, Px: 1, Shores: f.shores, Reach: 96, Coast: f.coast, Grid: f.grid}
}

// none is a surface of nothing: plain grey, no water, open water everywhere.
type none struct{}

func (none) Surface() Painted { return Painted{} }

// Every cell is split in two along the diagonal whose corners stand nearer, as the shader's drawn
// splits it, and the triangles cover the lattice's cells once each.
func TestTriangles_SplitEachCellAlongItsNearerDiagonal(t *testing.T) {
	// a 3 by 2 lattice: the left cell's top-left and bottom-right corners stand level, the right
	// cell's top-right and bottom-left stand nearer than its others
	heights := []float32{0, 0, 10, 5, 0, 20}
	_, _, enc := encode(heights, nil)
	got := triangles(3, 2, enc, nil)
	if len(got) != 12 {
		t.Fatalf("%d indices for two cells, want 12", len(got))
	}
	cells := [][4]uint32{{0, 1, 3, 4}, {1, 2, 4, 5}}
	for c, k := range cells {
		tri := got[6*c : 6*c+6]
		h := func(i uint32) float32 { return heights[i] }
		nearer03 := math.Abs(float64(h(k[0])-h(k[3]))) < math.Abs(float64(h(k[1])-h(k[2])))
		shared := map[uint32]int{}
		for _, i := range tri {
			shared[i]++
		}
		if len(shared) != 4 {
			t.Fatalf("cell %d: triangles %v do not span its four corners %v", c, tri, k)
		}
		if nearer03 && (shared[k[0]] != 2 || shared[k[3]] != 2) {
			t.Errorf("cell %d: triangles %v, want them split along %d-%d", c, tri, k[0], k[3])
		}
		if !nearer03 && (shared[k[1]] != 2 || shared[k[2]] != 2) {
			t.Errorf("cell %d: triangles %v, want them split along %d-%d", c, tri, k[1], k[2])
		}
	}
}

// Every corner's normal faces up on the level, leans away from the ridge on its sides, and is
// worked out one-sided at the lattice's edge; every pixel of it is opaque.
func TestNormals_LeanAwayFromTheRidge(t *testing.T) {
	cols, rows, cell, heights, _ := hill{}.Lattice()
	buf := normals(cols, rows, cell, heights, nil)
	if len(buf) != 4*cols*rows {
		t.Fatalf("%d bytes for %d corners, want four each", len(buf), cols*rows)
	}
	at := func(x, y int) [3]float32 { i := 4 * (y*cols + x); return decodeNormal(buf[i], buf[i+1], buf[i+2]) }
	near := func(a, b float32) bool { return math.Abs(float64(a-b)) < 0.02 }
	if n := at(8, 5); !near(n[0], 0) || !near(n[1], 0) || !near(n[2], 1) {
		t.Errorf("the ridge's crest faces %v, want straight up", n)
	}
	if w, e := at(7, 5), at(9, 5); w[0] >= -0.3 || e[0] <= 0.3 || !near(w[0], -e[0]) || !near(w[1], 0) {
		t.Errorf("the ridge's west side faces %v and its east %v, want leaning apart, alike", w, e)
	}
	if n := at(0, 5); !near(n[2], 1) {
		t.Errorf("the lattice's edge faces %v, want up, from the one cell beside it", n)
	}
	for i := 3; i < len(buf); i += 4 {
		if buf[i] != 255 {
			t.Fatalf("corner %d is not opaque", i/4)
		}
	}
}

type stillSky struct{}

func (stillSky) Air() air.Weather { return air.Weather{} }

// eye is a camera looking from a point of the world: a perspective for the rays, the top-down
// camera for everything else.
type eye struct {
	camera.Camera
	at [3]float32
}

func (e eye) Rays() (camera.RayField, bool) {
	// looking along +x, the screen's x across y, its y down: the ray through (sx, sy) runs along
	// (1, (sx-200)/400, -(sy-150)/400)
	return camera.RayField{Origin: e.at, Dir: [3]float32{1, -0.5, 0.375}, DDX: [3]float32{0, 1.0 / 400, 0}, DDY: [3]float32{0, 0, -1.0 / 400}}, true
}

func (e eye) Project(x, y, z float32) (float32, float32) {
	dx, dy, dz := x-e.at[0], y-e.at[1], z-e.at[2]
	return 200 + 400*dy/dx, 150 - 400*dz/dx
}

// The lattice's quadrants are written once a version of the ground, the shores once a version of
// the coast; without an albedo the ground is drawn plain.
func TestRenderer_RefreshesTheLatticeWhenTheGroundOrTheCoastChanges(t *testing.T) {
	g := &hill{}
	f := &flat{}
	r := New(g, f, stillSky{}, Config{})
	if !r.refresh() || r.lattice == nil || r.lattice.Bounds().Dx() != 34 || r.lattice.Bounds().Dy() != 34 {
		t.Fatal("no lattice of four 17 by 17 quadrants after the first refresh")
	}
	if r.low != 0 || r.span != 30 {
		t.Errorf("low %v, span %v, want 0 and 30", r.low, r.span)
	}
	heights, coast := r.heightsAt, r.coastAt
	r.refresh()
	if r.heightsAt != heights || r.coastAt != coast {
		t.Error("the lattice was written again with the ground and the coast as they were")
	}
	g.version++
	f.coast++
	r.refresh()
	if r.heightsAt == heights || r.coastAt == coast {
		t.Error("the lattice was not written anew after the ground and the coast changed")
	}
}

// The way to the shore is held within a 255th: its way, how far it is, and open water where a
// corner has none — which decodes to no way at all.
func TestShores_HoldTheWayAndHowFar(t *testing.T) {
	reach := float32(96)
	from := []Shore{{X: 0.6, Y: -0.8, Dist: 30, Near: 1 - 30/reach}, {}, {X: -1, Y: 0, Dist: 0, Near: 1}}
	buf := shores(2, 2, from, reach, nil)
	decode := func(i int) (x, y, d float32) {
		return float32(buf[4*i])/255*2 - 1, float32(buf[4*i+1])/255*2 - 1, float32(buf[4*i+2]) / 255 * reach
	}
	near := func(a, b, tol float32) bool { return math.Abs(float64(a-b)) <= float64(tol) }
	if x, y, d := decode(0); !near(x, 0.6, 1.0/127) || !near(y, -0.8, 1.0/127) || !near(d, 30, reach/255) {
		t.Errorf("a shore 30 off to the south-east decodes to (%v, %v) %v off", x, y, d)
	}
	if x, y, d := decode(3); math.Hypot(float64(x), float64(y)) > 0.01 || !near(d, reach, reach/255) {
		t.Errorf("a corner with no shore given decodes to (%v, %v) %v off, want no way, open water", x, y, d)
	}
	if x, _, d := decode(2); !near(x, -1, 1.0/127) || d != 0 {
		t.Errorf("a corner on the shore decodes to x %v, %v off; want west, on it", x, d)
	}
	for i := 3; i < len(buf); i += 4 {
		if buf[i] != 255 {
			t.Fatalf("corner %d is not opaque", i/4)
		}
	}
}

// A draw's uniforms are the frame's with the renderer's own over them, the grid from the
// surface; the frame's own are left as they were.
func TestRenderer_PreparesTheFramesUniformsWithItsOwnOver(t *testing.T) {
	f := &flat{grid: 6}
	r := New(hill{}, f, stillSky{}, Config{})
	cam := eye{Camera: icamera.NewFromSpace(512, 512, 0), at: [3]float32{100, 256, 4}}
	frame := map[string]any{"Pixel": []float32{0.5}, "Sun": []float32{1, 2, 3}, "Cell": []float32{9, 9}}
	u := render.UniformsOf(frame)
	if !r.prepare(cam, u) {
		t.Fatal("nothing to draw")
	}
	get := func(name string) []float32 { v, _ := r.opts.Uniforms[name].([]float32); return v }
	if v := get("Pixel"); len(v) != 1 || v[0] != 0.5 {
		t.Errorf("Pixel %v, want the frame's 0.5: the waves as fine as on the tiles", v)
	}
	if v := get("Eye"); len(v) != 3 || v[0] != 100 || v[2] != 4 {
		t.Errorf("Eye %v, want the camera's (100, 256, 4)", v)
	}
	if v := get("ViewProj"); len(v) != 16 {
		t.Errorf("ViewProj %v, want the camera's transform", v)
	}
	if v := get("Sun"); len(v) != 3 || v[2] != 3 {
		t.Errorf("Sun %v, want the frame's", v)
	}
	if v := get("Cell"); len(v) != 1 || v[0] != 32 {
		t.Errorf("Cell %v, want the renderer's own 32 over the frame's", v)
	}
	if v := get("GridFrom"); len(v) != 1 || v[0] != 6 {
		t.Errorf("GridFrom %v, want the surface's 6", v)
	}
	if v := frame["Pixel"].([]float32); v[0] != 0.5 {
		t.Errorf("the frame's own Pixel became %v: the renderer wrote into the composer's", v[0])
	}
}
