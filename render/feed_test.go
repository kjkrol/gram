package render_test

import (
	"math"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/render"
)

func TestFeed_ResizeGivesTheCameraItsViewport(t *testing.T) {
	cam := icamera.NewFromSpace(1000, 1000, 0)
	feed := render.NewFeed(cam, nil)
	feed.Resize(320, 200)
	if w, h := cam.Viewport(); w != 320 || h != 200 {
		t.Fatalf("viewport %vx%v, want 320x200", w, h)
	}
}

func TestFeed_DrawsNothingWithoutASize(t *testing.T) {
	feed := render.NewFeed(icamera.NewFromSpace(1000, 1000, 0), nil)
	if img := feed.Draw(); img != nil {
		t.Fatalf("a feed of no size drew %v", img.Bounds())
	}
}

func TestFeed_ToWorldAndToPixelsAreEachOthersInverse(t *testing.T) {
	cam := icamera.NewFromSpace(1000, 1000, 0)
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
	cam := icamera.NewFromSpace(1000, 1000, 0)
	feed := render.NewFeed(cam, nil)
	feed.Resize(100, 100)
	cam.MoveTo(0, 0)
	if _, _, _, visible := feed.ToPixels(900, 900, 0); visible {
		t.Fatal("a point far off the feed is visible")
	}
}

// picking is a camera that picks the ground itself.
type picking struct{ camera.Camera }

func (picking) Pick(sx, sy float32) (float32, float32, bool) { return sx + 1000, sy + 1000, false }

func TestFeed_ToWorldAsksAPickerFirst(t *testing.T) {
	feed := render.NewFeed(picking{icamera.NewFromSpace(1000, 1000, 0)}, nil)
	x, y, ok := feed.ToWorld(5, 6)
	if x != 1005 || y != 1006 || ok {
		t.Fatalf("got (%v, %v, %v), want the picker's (1005, 1006, false)", x, y, ok)
	}
}

// recording is a picture that notes what it was drawn into and through.
type recording struct {
	w, h int
	cam  camera.Camera
}

func (*recording) Init(*goke.SysInit) {}

func (r *recording) DrawWorld(screen *render.Image, cam camera.Camera) {
	r.w, r.h, r.cam = screen.Bounds().Dx(), screen.Bounds().Dy(), cam
}

func TestFeed_DrawsItsPictureThroughItsCameraAtItsSize(t *testing.T) {
	cam := icamera.NewFromSpace(1000, 1000, 0)
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
