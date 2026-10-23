package look

import (
	"image/color"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

type lookSource struct {
	look   *Flat
	direct bool
	boxes  []plane.AABB
	atlas  render.AtlasSource
}

func (s *lookSource) Compose(f *render.Frame, cam camera.Camera) {
	if s.direct {
		s.look.Begin(f, cam)
		f.DrawAt(f.At(0, 0, 0))
	}
	for _, b := range s.boxes {
		s.look.Sprite(f, cam, b, entity.Z{}, s.atlas, render.Appearance{SpriteID: 1}, render.Illumination{Light: render.Light{0.8, 0.6, 1}})
	}
}
func (s *lookSource) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	s.look.DrawSprites(t, cam, u)
}

func TestFlatLook_DrawsOnTheGPUAsOnTheFrame(t *testing.T) {
	gputest.Need(t)
	atlas := render.NewAtlas()
	atlas.Add(render.SpriteID(1), 8, func(dst *render.Canvas, size int) {
		dst.FillRect(0, 0, float32(size), float32(size), color.RGBA{R: 200, G: 40, B: 40, A: 255})
		dst.FillRect(0, 0, float32(size)/2, float32(size)/2, color.RGBA{R: 40, G: 200, B: 240, A: 255})
	})
	atlas.Close()
	boxes := []plane.AABB{plane.NewAABB(geom.NewVec(20, 30), 16, 16), plane.NewAABB(geom.NewVec(250, 60), 12, 20), plane.NewAABB(geom.NewVec(90, 250), 30, 10)}
	cam := cameratest.NewFromSpace(256, 256, aabbworld.Torus)
	cam.SetViewport(160, 120)
	cam.ZoomIn(2, 0, 0)
	cam.MoveTo(230, 20) // the seam in view
	pix := func(direct bool) []byte {
		screen := render.NewImage(160, 120)
		src := &lookSource{
			look:   NewFlat(256, 256, true, true),
			direct: direct,
			boxes:  boxes,
			atlas:  atlas,
		}
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

func TestFlatLook_TurnedFromAboveDrawsOnTheGPUAsOnTheFrame(t *testing.T) {
	gputest.Need(t)
	atlas := render.NewAtlas()
	atlas.Add(render.SpriteID(1), 8, func(dst *render.Canvas, size int) {
		dst.FillRect(0, 0, float32(size), float32(size), color.RGBA{R: 200, G: 40, B: 40, A: 255})
	})
	atlas.Close()
	box := plane.NewAABB(geom.NewVec(110, 120), 40, 16)
	cam := cameratest.NewFromAbove(256, 256)
	cam.SetViewport(160, 120)
	cam.CenterOn(130, 128, 0)
	cameratest.Turn(cam, 0.6)
	pix := func(direct bool) []byte {
		screen := render.NewImage(160, 120)
		src := &lookSource{
			look:   NewFlat(256, 256, false, false),
			direct: direct,
			boxes:  []plane.AABB{box},
			atlas:  atlas,
		}
		render.NewComposer(src).DrawWorld(screen, cam)
		out := make([]byte, 4*160*120)
		screen.ReadPixels(out)
		return out
	}
	frame, direct := pix(false), pix(true)
	// the box's border, within which a pixel is partly the sprite's
	border := func(x, y float32) bool {
		d := min(x-110, 150-x, y-120, 136-y)
		return d > -1.5 && d < 1.5
	}
	differ, bare, over, partial := 0, 0, 0, 0
	for i := 0; i < len(frame); i += 4 {
		x, y := cam.Unproject(float32((i/4)%160)+0.5, float32((i/4)/160)+0.5, 0)
		if border(x, y) {
			if a := direct[i+3]; a != 0 && a != 255 {
				partial++
			}
			continue
		}
		for k := range 4 {
			if d := int(frame[i+k]) - int(direct[i+k]); d > 1 || d < -1 {
				differ++
				break
			}
		}
		in := x > 110 && x < 150 && y > 120 && y < 136
		switch {
		case !in && direct[i+3] != 0:
			over++
		case in && direct[i+3] != 255:
			bare++
		}
	}
	if differ != 0 {
		t.Errorf("%d pixels off the border differ between the GPU and the frame, want the same "+
			"turned box", differ)
	}
	if bare != 0 || over != 0 {
		t.Errorf("turned, %d pixels within the box are not the sprite's and %d off it are: want "+
			"the sprite turned with the world, not the screen's box round it", bare, over)
	}
	if partial == 0 {
		t.Error("turned, no pixel on the box's border is partly the sprite's: want its edge smooth")
	}
	fp := (&Flat{}).Footprint(cam, box.AABB, 0, nil)
	if len(fp) != 1 || fp[0][0][1] == fp[0][1][1] {
		t.Errorf("turned, the footprint is %v, want one turned with the world", fp)
	}
}
