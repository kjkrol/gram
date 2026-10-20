package render_test

import (
	"math"
	"slices"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/render"
)

func TestFeed_ResizeGivesTheCameraItsViewport(t *testing.T) {
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	feed := render.NewFeed(cam, nil)
	feed.Resize(320, 200)
	if w, h := cam.Viewport(); w != 320 || h != 200 {
		t.Fatalf("viewport %vx%v, want 320x200", w, h)
	}
}

func TestFeed_DrawsNothingWithoutASize(t *testing.T) {
	feed := render.NewFeed(cameratest.NewFromSpace(1000, 1000, 0), nil)
	if img := feed.Draw(); img != nil {
		t.Fatalf("a feed of no size drew %v", img.Bounds())
	}
}

func TestFeed_ToWorldAndToPixelsAreEachOthersInverse(t *testing.T) {
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	feed := render.NewFeed(cam, nil)
	feed.Resize(400, 300)
	cam.ZoomIn(2, 0, 0)
	cam.CenterOn(500, 500, 0)

	x, y, ok := feed.ToWorld(120, 80)
	if !ok {
		t.Fatal("a top-down feed sees the ground under every pixel")
	}
	px, py, _, visible := feed.ToPixels(x, y, 0)
	if !visible || math.Abs(float64(px-120)) > 1e-3 || math.Abs(float64(py-80)) > 1e-3 {
		t.Fatalf("back to (%v, %v) visible %v, want (120, 80) visible", px, py, visible)
	}
}

func TestFeed_APointOutsideIsNotVisible(t *testing.T) {
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	feed := render.NewFeed(cam, nil)
	feed.Resize(100, 100)
	cam.MoveTo(0, 0)
	if _, _, _, visible := feed.ToPixels(900, 900, 0); visible {
		t.Fatal("a point far off the feed is visible")
	}
}

type picking struct{ camera.Camera }

func (picking) Pick(sx, sy float32) (float32, float32, float32, bool) {
	return sx + 1000, sy + 1000, 0, false
}

func TestFeed_ToWorldAsksAPickerFirst(t *testing.T) {
	feed := render.NewFeed(picking{cameratest.NewFromSpace(1000, 1000, 0)}, nil)
	x, y, ok := feed.ToWorld(5, 6)
	if x != 1005 || y != 1006 || ok {
		t.Fatalf("got (%v, %v, %v), want the picker's (1005, 1006, false)", x, y, ok)
	}
}

type recording struct {
	w, h    int
	cam     camera.Camera
	calls   []call
	passes  []pass
	screens []*render.Image
	ground  *render.Image // the image DrawGround drew into
	over    *render.Image // the ground DrawOver was handed
}

// one DrawWorld: the screen's size and the camera's viewport
type call struct {
	w, h   int
	vw, vh float32
}

// one DrawGround or DrawOver: which, the target's size and the camera's viewport
type pass struct {
	name string
	call
}

func (*recording) Init(*goke.SysInit) {}

func (r *recording) DrawWorld(screen *render.Image, cam camera.Camera) {
	r.w, r.h, r.cam = screen.Bounds().Dx(), screen.Bounds().Dy(), cam
	r.calls = append(r.calls, callOf(screen, cam))
	r.screens = append(r.screens, screen)
}

func (r *recording) DrawGround(ground *render.Image, cam camera.Camera) {
	r.ground = ground
	r.passes = append(r.passes, pass{"ground", callOf(ground, cam)})
}

func (r *recording) DrawOver(screen, ground *render.Image, cam camera.Camera) {
	r.over = ground
	r.passes = append(r.passes, pass{"over", callOf(screen, cam)})
	r.screens = append(r.screens, screen)
}

func callOf(screen *render.Image, cam camera.Camera) call {
	vw, vh := cam.Viewport()
	return call{screen.Bounds().Dx(), screen.Bounds().Dy(), vw, vh}
}

// a camera from above with the figures at every pixel at its word
type figured struct {
	sharpened
	sharp bool
}

type sharpened interface {
	camera.Camera
	camera.Resolved
	camera.Ringed
	camera.Cropping
}

func newFigured(sharp bool) *figured {
	return &figured{cameratest.NewFromSpace(1000, 1000, 0).(sharpened), sharp}
}

func (f *figured) SetSharpFigures(on bool) { f.sharp = on }
func (f *figured) SharpFigures() bool      { return f.sharp }

func TestFeed_DrawsItsPictureThroughItsCameraAtItsSize(t *testing.T) {
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	picture := &recording{}
	feed := render.NewFeed(cam, picture)
	feed.Resize(64, 48)
	img := feed.Draw()
	if img == nil || picture.w != 64 || picture.h != 48 || picture.cam != cam {
		t.Fatalf("picture drawn %dx%d through %v, want 64x48 through the feed's camera", picture.w, picture.h, picture.cam)
	}
	feed.Resize(32, 24)
	if img := feed.Draw(); img.Bounds().Dx() != 32 || img.Bounds().Dy() != 24 {
		t.Fatalf("resized feed drew %v, want 32x24", img.Bounds())
	}
}

func TestFeed_DrawsAtItsCamerasResolutionAndSpeaksShownPixels(t *testing.T) {
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	cam.(camera.Resolved).SetResolution(0.5)
	picture := &recording{}
	feed := render.NewFeed(cam, picture)
	feed.Resize(400, 300)
	img := feed.Draw()
	if img == nil || img.Bounds().Dx() != 200 || img.Bounds().Dy() != 150 || picture.w != 200 ||
		picture.h != 150 {
		t.Fatalf("at 0.5 a feed shown at 400x300 drew %v through a picture %dx%d, want 200x150",
			img.Bounds(), picture.w, picture.h)
	}
	cam.ZoomIn(2, 0, 0)
	cam.CenterOn(500, 500, 0)
	x, y, ok := feed.ToWorld(120, 80)
	if cx, cy := cam.FromScreen(60, 40); !ok || cx != x || cy != y {
		t.Fatalf("under the shown pixel (120, 80) lies (%v, %v), want (%v, %v): the camera's (60, 40)",
			x, y, cx, cy)
	}
	px, py, _, visible := feed.ToPixels(x, y, 0)
	if !visible || math.Abs(float64(px-120)) > 1e-3 || math.Abs(float64(py-80)) > 1e-3 {
		t.Fatalf("back to (%v, %v) visible %v, want (120, 80) visible", px, py, visible)
	}
}

func TestFeed_DrawsARingedCameraALayerARingCoarseToFine(t *testing.T) {
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	cam.(camera.Ringed).SetRings([]camera.Ring{
		{Share: 1, Resolution: 0.5},
		{Share: 0.6, Resolution: 0.75},
		{Share: 0.35, Resolution: 1},
	})
	picture := &recording{}
	feed := render.NewFeed(cam, picture)
	feed.Resize(400, 300)
	img := feed.Draw()
	want := []call{{200, 150, 200, 150}, {180, 135, 180, 135}, {140, 106, 140, 106}}
	if !slices.Equal(picture.calls, want) {
		t.Errorf("the picture was drawn %v, want %v: the whole frame at 0.5, the central 240x180 "+
			"at 0.75, the central 140x106 at every pixel, a layer each", picture.calls, want)
	}
	if img == nil || img.Bounds().Dx() != 400 || img.Bounds().Dy() != 300 {
		t.Fatalf("the feed's image is %v, want 400x300: the shown size", img.Bounds())
	}
	if w, h := cam.Viewport(); w != 400 || h != 300 || camera.IsCropped(cam) {
		t.Errorf("after the frame the viewport is %vx%v, cropped %v; want 400x300, not cropped",
			w, h, camera.IsCropped(cam))
	}
	cam.ZoomIn(2, 0, 0)
	cam.CenterOn(500, 500, 0)
	x, y, ok := feed.ToWorld(120, 80)
	px, py, _, visible := feed.ToPixels(x, y, 0)
	if !ok || !visible || math.Abs(float64(px-120)) > 1e-3 || math.Abs(float64(py-80)) > 1e-3 {
		t.Errorf("under (120, 80) lies (%v, %v) ok %v, back to (%v, %v) visible %v; want (120, 80)",
			x, y, ok, px, py, visible)
	}
}

func TestFeed_DrawsACameraOfOneRingAsAResolvedOne(t *testing.T) {
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	cam.(camera.Ringed).SetRings([]camera.Ring{{Share: 1, Resolution: 0.5}})
	cam.(camera.Resolved).SetResolution(0.5)
	picture := &recording{}
	feed := render.NewFeed(cam, picture)
	feed.Resize(400, 300)
	feed.Draw()
	if want := []call{{200, 150, 200, 150}}; !slices.Equal(picture.calls, want) {
		t.Errorf("the picture was drawn %v, want %v: once, at the camera's resolution",
			picture.calls, want)
	}
}

func TestFeed_DrawOnDrawsTheWholeFrameAtEveryPixelStraightOntoTheTarget(t *testing.T) {
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	cam.(camera.Ringed).SetRings([]camera.Ring{
		{Share: 1, Resolution: 1},
		{Share: 0.5, Resolution: 0.5},
	})
	picture := &recording{}
	feed := render.NewFeed(cam, picture)
	dst := render.NewImage(400, 300)
	feed.DrawOn(dst)
	want := []call{{400, 300, 400, 300}, {100, 75, 100, 75}}
	if !slices.Equal(picture.calls, want) || picture.screens[0] != dst {
		t.Errorf("the picture was drawn %v, the first onto the target %v; want %v, the first "+
			"straight onto it", picture.calls, picture.screens[0] == dst, want)
	}
	if w, h := cam.Viewport(); w != 400 || h != 300 || camera.IsCropped(cam) {
		t.Errorf("after the frame the viewport is %vx%v, cropped %v; want 400x300, not cropped",
			w, h, camera.IsCropped(cam))
	}
}

func TestFeed_DrawsTheGroundAtItsResolutionAndTheFiguresAtEveryPixel(t *testing.T) {
	cam := newFigured(true)
	cam.SetResolution(0.5)
	picture := &recording{}
	feed := render.NewFeed(cam, picture)
	feed.Resize(64, 48)
	img := feed.Draw()
	want := []pass{{"ground", call{32, 24, 32, 24}}, {"over", call{64, 48, 64, 48}}}
	if !slices.Equal(picture.passes, want) || len(picture.calls) != 0 {
		t.Errorf("the picture was drawn %v and whole %v, want %v and never whole: the ground at a "+
			"half, what lies over it at every pixel", picture.passes, picture.calls, want)
	}
	if picture.over == nil || picture.over != picture.ground {
		t.Error("what lies over the figures was drawn over another ground than the one drawn")
	}
	if img == nil || img.Bounds().Dx() != 64 || img.Bounds().Dy() != 48 {
		t.Fatalf("the feed's image is %v, want 64x48: the shown size", img.Bounds())
	}
	if w, h := cam.Viewport(); w != 32 || h != 24 || camera.IsCropped(cam) {
		t.Errorf("after the frame the viewport is %vx%v, cropped %v; want 32x24, not cropped",
			w, h, camera.IsCropped(cam))
	}
}

func TestFeed_DrawsWholeWithTheFiguresSoftOrEveryPixelDrawn(t *testing.T) {
	cam := newFigured(false)
	cam.SetResolution(0.5)
	picture := &recording{}
	feed := render.NewFeed(cam, picture)
	feed.Resize(64, 48)
	feed.Draw()
	if want := []call{{32, 24, 32, 24}}; !slices.Equal(picture.calls, want) ||
		len(picture.passes) != 0 {
		t.Errorf("with the figures soft the picture was drawn whole %v and apart %v, want whole "+
			"%v once", picture.calls, picture.passes, want)
	}
	cam.SetSharpFigures(true)
	cam.SetResolution(1)
	picture.calls = nil
	feed.Draw()
	if want := []call{{64, 48, 64, 48}}; !slices.Equal(picture.calls, want) ||
		len(picture.passes) != 0 {
		t.Errorf("at every pixel the picture was drawn whole %v and apart %v, want whole %v once",
			picture.calls, picture.passes, want)
	}
}

func TestFeed_DrawsARingedCameraInRingsWhateverItsFigures(t *testing.T) {
	cam := newFigured(true)
	cam.SetRings([]camera.Ring{{Share: 1, Resolution: 0.5}, {Share: 0.5, Resolution: 0.75}})
	cam.SetResolution(0.75)
	picture := &recording{}
	feed := render.NewFeed(cam, picture)
	feed.Resize(64, 48)
	feed.Draw()
	want := []call{{32, 24, 32, 24}, {24, 18, 24, 18}}
	if !slices.Equal(picture.calls, want) || len(picture.passes) != 0 {
		t.Errorf("the picture was drawn %v and apart %v, want a ring each %v and never apart",
			picture.calls, picture.passes, want)
	}
}

func TestFeed_DrawOnDrawsWhatLiesOverTheFiguresStraightOntoTheTarget(t *testing.T) {
	cam := newFigured(true)
	cam.SetResolution(0.5)
	picture := &recording{}
	feed := render.NewFeed(cam, picture)
	dst := render.NewImage(64, 48)
	feed.DrawOn(dst)
	want := []pass{{"ground", call{32, 24, 32, 24}}, {"over", call{64, 48, 64, 48}}}
	if !slices.Equal(picture.passes, want) || len(picture.screens) != 1 ||
		picture.screens[0] != dst {
		t.Errorf("the picture was drawn %v, over the figures onto the target %v; want %v, onto "+
			"it", picture.passes, len(picture.screens) == 1 && picture.screens[0] == dst, want)
	}
	if w, h := cam.Viewport(); w != 32 || h != 24 || camera.IsCropped(cam) {
		t.Errorf("after the frame the viewport is %vx%v, cropped %v; want 32x24, not cropped",
			w, h, camera.IsCropped(cam))
	}
}
