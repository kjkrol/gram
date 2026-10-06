package look

import (
	"image/color"
	"os"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
)

// lookSource lays boxes through a look, readied for the GPU or not, drawn where render.Objects
// comes.
type lookSource struct {
	look   *Flat
	direct bool
	boxes  []plane.AABB
	atlas  render.AtlasSource
}

func (*lookSource) Init(*goke.SysInit) {}
func (*lookSource) Tier() render.Tier  { return render.Objects }
func (s *lookSource) Compose(f *render.Frame, cam camera.Camera) {
	if s.direct {
		s.look.Begin(cam)
	}
	for _, b := range s.boxes {
		s.look.Sprite(f, cam, b, entity.Z{}, s.atlas, render.Appearance{SpriteID: 1}, render.Light{0.8, 0.6, 1})
	}
}
func (s *lookSource) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	s.look.DrawSprites(t, cam, u)
}

// Seen from above the flat look's sprites drawn on the GPU are the ones it lays on the frame, pixel
// for pixel: zoomed in, in their light, their halves either side of a wrapping world's seam.
func TestFlatLook_DrawsOnTheGPUAsOnTheFrame(t *testing.T) {
	needGPU(t)
	atlas := render.NewAtlas()
	atlas.Add(render.SpriteID(1), 8, func(dst *render.Canvas, size int) {
		dst.FillRect(0, 0, float32(size), float32(size), color.RGBA{R: 200, G: 40, B: 40, A: 255})
		dst.FillRect(0, 0, float32(size)/2, float32(size)/2, color.RGBA{R: 40, G: 200, B: 240, A: 255})
	})
	atlas.Close()
	boxes := []plane.AABB{plane.NewAABB(geom.NewVec(20, 30), 16, 16), plane.NewAABB(geom.NewVec(250, 60), 12, 20), plane.NewAABB(geom.NewVec(90, 250), 30, 10)}
	cam := icamera.NewFromSpace(256, 256, aabbworld.Torus)
	cam.SetViewport(160, 120)
	cam.ZoomIn(2, 0, 0)
	cam.MoveTo(230, 20) // the seam in view
	pix := func(direct bool) []byte {
		screen := render.NewImage(160, 120)
		src := &lookSource{look: NewFlat(256, 256), direct: direct, boxes: boxes, atlas: atlas}
		render.NewComposer(src).DrawWorld(screen, cam)
		out := make([]byte, 4*160*120)
		screen.ReadPixels(out)
		return out
	}
	frame, direct := pix(false), pix(true)
	drawn := 0
	for i := 0; i < len(frame); i += 4 {
		if frame[i+3] != 0 {
			drawn++
		}
		for k := range 4 {
			if d := int(frame[i+k]) - int(direct[i+k]); d > 1 || d < -1 {
				t.Fatalf("pixel %d is %v on the GPU, %v on the frame", i/4, direct[i:i+4], frame[i:i+4])
			}
		}
	}
	if drawn == 0 {
		t.Fatal("no sprite in view")
	}
}

// needGPU readies a device without a window; a machine without one skips.
func needGPU(t *testing.T) {
	t.Helper()
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
}
