package heightfield

import (
	"image/color"
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
)

// The shader compiles.
func TestShader_Compiles(t *testing.T) {
	if _, err := ebiten.NewShader(kage); err != nil {
		t.Fatal(err)
	}
}

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

func (hill) At(p geom.Vec) float64 {
	if d := math.Abs(p.X/32 - 8); d < 1 {
		return 30 * (1 - d)
	}
	return 0
}

func (h hill) Version() uint64 { return h.version }

// flat colours every cell grey.
type flat struct{}

func (flat) Size() (int, int)           { return 16, 16 }
func (flat) Colour(int, int) color.RGBA { return color.RGBA{128, 128, 128, 255} }
func (flat) Version() uint64            { return 1 }

type stillSky struct{}

func (stillSky) Sun() sky.Sun     { return sky.DefaultSun }
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

// The ground hides from an eye west of the ridge what stands low east of it, not what stands on
// the near side nor a hawk over the ridge; nothing while the tiles are drawn.
func TestRenderer_HidesWhatTheGroundHidesFromTheEye(t *testing.T) {
	r := New(hill{}, flat{}, stillSky{}, Config{})
	cam := eye{Camera: icamera.NewFromSpace(512, 512, 0), at: [3]float32{100, 256, 4}}
	if !r.Hides(cam, 400, 256, 2) {
		t.Error("a walker 2 high beyond the ridge is not hidden")
	}
	if r.Hides(cam, 200, 256, 2) {
		t.Error("a walker on the near side of the ridge is hidden")
	}
	if r.Hides(cam, 400, 256, 60) {
		t.Error("a hawk 60 up beyond the ridge is hidden")
	}
	r.Hide(true)
	if r.Hides(cam, 400, 256, 2) {
		t.Error("the tiles drawn, the renderer still hides")
	}
}

// The heightmap and the colours are written once a version of the ground and of the board.
func TestRenderer_RefreshesTheImagesWhenTheGroundChanges(t *testing.T) {
	g := &hill{}
	r := New(g, flat{}, stillSky{}, Config{})
	if !r.refresh() || r.heights == nil || r.heights.Bounds().Dx() != 17 || r.albedo.Bounds().Dx() != 16 {
		t.Fatal("no images of the lattice and the cells after the first refresh")
	}
	if r.low != 0 || r.span != 30 {
		t.Errorf("low %v, span %v, want 0 and 30", r.low, r.span)
	}
	at := r.heightsAt
	r.refresh()
	if r.heightsAt != at {
		t.Error("the heightmap was written again with the ground as it was")
	}
	g.version++
	r.refresh()
	if r.heightsAt == at {
		t.Error("the heightmap was not written anew after the ground changed")
	}
}
