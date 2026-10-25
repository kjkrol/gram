package feed

import (
	"math"
	"slices"
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/render/internal/gfx"
)

func TestFeed_ResizeGivesTheCameraItsViewport(t *testing.T) {
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	feed := NewFeed(cam, nil)
	feed.Resize(320, 200)
	if w, h := cam.Viewport(); w != 320 || h != 200 {
		t.Fatalf("viewport %vx%v, want 320x200", w, h)
	}
}

func TestFeed_DrawsNothingWithoutASize(t *testing.T) {
	feed := NewFeed(cameratest.NewFromSpace(1000, 1000, 0), nil)
	if img := feed.Draw(); img != nil {
		t.Fatalf("a feed of no size drew %v", img.Bounds())
	}
}

func TestFeed_ToWorldAndToPixelsAreEachOthersInverse(t *testing.T) {
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	feed := NewFeed(cam, nil)
	feed.Resize(400, 300)
	cam.ZoomIn(2, 0, 0)
	cam.CenterOn(500, 500, 0)
	feed.Resize(400, 300) // laid out again, as every frame: its camera copied

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
	feed := NewFeed(cam, nil)
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
	feed := NewFeed(picking{cameratest.NewFromSpace(1000, 1000, 0)}, nil)
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
	screens []*gfx.Image
	layers  []*gfx.Image    // the images DrawLayer drew into
	wins    []camera.Window // the windows DrawWorld was cropped to
	holes   []camera.Window // the windows DrawLayer left out
	// whether the camera a copy drawn through was taken of was cropped as it was drawn
	originCropped bool
}

// one DrawWorld: the screen's size and the camera's viewport
type call struct {
	w, h   int
	vw, vh float32
}

// one Lay, DrawLayer or DrawOver: which, the target's size (none for Lay), the camera's viewport
// and a layer's window
type pass struct {
	name string
	call
	at camera.Window
}

func (r *recording) DrawWorld(screen *gfx.Image, cam camera.Camera) {
	r.w, r.h, r.cam = screen.Bounds().Dx(), screen.Bounds().Dy(), cam
	r.originCropped = r.originCropped || camera.IsCropped(camera.OriginOf(cam))
	r.calls = append(r.calls, callOf(screen, cam))
	r.screens = append(r.screens, screen)
	if c, ok := cam.(camera.Cropping); ok {
		r.wins = append(r.wins, c.Window())
	}
}

func (r *recording) Lay(cam camera.Camera, apart bool) {
	name := "lay"
	if apart {
		name = "lay apart"
	}
	vw, vh := cam.Viewport()
	r.passes = append(r.passes, pass{name: name, call: call{vw: vw, vh: vh}})
}

func (r *recording) DrawLayer(layer *gfx.Image, cam camera.Camera, at, hole camera.Window) {
	r.layers = append(r.layers, layer)
	r.passes = append(r.passes, pass{"layer", callOf(layer, cam), at})
	r.holes = append(r.holes, hole)
}

func (r *recording) DrawOver(screen *gfx.Image, cam camera.Camera) {
	r.passes = append(r.passes, pass{name: "over", call: callOf(screen, cam)})
	r.screens = append(r.screens, screen)
}

// plain is a picture drawn whole only, never Laid
type plain struct{ Picture }

func callOf(screen *gfx.Image, cam camera.Camera) call {
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
	camera.Layered
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
	feed := NewFeed(cam, picture)
	feed.Resize(64, 48)
	img := feed.Draw()
	if img == nil || picture.w != 64 || picture.h != 48 || picture.cam == cam ||
		camera.OriginOf(picture.cam) != cam {
		t.Fatalf("picture drawn %dx%d through %p, want 64x48 through a copy of the feed's camera",
			picture.w, picture.h, picture.cam)
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
	feed := NewFeed(cam, picture)
	feed.Resize(400, 300)
	img := feed.Draw()
	if img == nil || img.Bounds().Dx() != 200 || img.Bounds().Dy() != 150 || picture.w != 200 ||
		picture.h != 150 {
		t.Fatalf("at 0.5 a feed shown at 400x300 drew %v through a picture %dx%d, want 200x150",
			img.Bounds(), picture.w, picture.h)
	}
	cam.ZoomIn(2, 0, 0)
	cam.CenterOn(500, 500, 0)
	feed.Resize(400, 300)
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

func TestFeed_DrawsALayeredCameraALayerARingCoarseToFine(t *testing.T) {
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	cam.(camera.Layered).SetLayers([]camera.Layer{
		{Share: 1, Resolution: 0.5},
		{Share: 0.6, Resolution: 0.75},
		{Share: 0.35, Resolution: 1},
	}, camera.Inset)
	picture := &recording{}
	feed := NewFeed(cam, plain{picture})
	feed.Resize(400, 300)
	img := feed.Draw()
	want := []call{{200, 150, 200, 150}, {180, 135, 180, 135}, {140, 106, 140, 106}}
	if !slices.Equal(picture.calls, want) {
		t.Errorf("the picture was drawn %v, want %v: the whole frame at 0.5, the central 240x180 "+
			"at 0.75, the central 140x106 at every pixel, a layer each, composed anew for each "+
			"by a picture not Laid", picture.calls, want)
	}
	if img == nil || img.Bounds().Dx() != 400 || img.Bounds().Dy() != 300 {
		t.Fatalf("the feed's image is %v, want 400x300: the shown size", img.Bounds())
	}
	if w, h := cam.Viewport(); w != 400 || h != 300 || camera.IsCropped(cam) ||
		picture.originCropped {
		t.Errorf("after the frame the viewport is %vx%v, cropped %v, as drawn %v; want 400x300, "+
			"never cropped: its copy is", w, h, camera.IsCropped(cam), picture.originCropped)
	}
	cam.ZoomIn(2, 0, 0)
	cam.CenterOn(500, 500, 0)
	feed.Resize(400, 300)
	x, y, ok := feed.ToWorld(120, 80)
	px, py, _, visible := feed.ToPixels(x, y, 0)
	if !ok || !visible || math.Abs(float64(px-120)) > 1e-3 || math.Abs(float64(py-80)) > 1e-3 {
		t.Errorf("under (120, 80) lies (%v, %v) ok %v, back to (%v, %v) visible %v; want (120, 80)",
			x, y, ok, px, py, visible)
	}
}

func TestFeed_DrawsAFootedCameraALayerOnTheBottomEdge(t *testing.T) {
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	cam.(camera.Layered).SetLayers([]camera.Layer{
		{Share: 1, Resolution: 0.5},
		{Share: 0.6, Resolution: 0.75},
		{Share: 0.35, Resolution: 1},
	}, camera.Footed)
	picture := &recording{}
	feed := NewFeed(cam, plain{picture})
	feed.Resize(400, 300)
	feed.Draw()
	wins := []camera.Window{
		{X: 0, Y: 0, W: 400, H: 300},
		{X: 80, Y: 120, W: 240, H: 180},
		{X: 130, Y: 195, W: 140, H: 105},
	}
	if !slices.Equal(picture.wins, wins) {
		t.Errorf("the camera was cropped to %v, want %v: each layer centred across, standing on "+
			"the bottom edge", picture.wins, wins)
	}
	want := []call{{200, 150, 200, 150}, {180, 135, 180, 135}, {140, 105, 140, 105}}
	if !slices.Equal(picture.calls, want) {
		t.Errorf("the picture was drawn %v, want %v: each window at its layer's resolution",
			picture.calls, want)
	}
	if w, h := cam.Viewport(); w != 400 || h != 300 || camera.IsCropped(cam) ||
		picture.originCropped {
		t.Errorf("after the frame the viewport is %vx%v, cropped %v, as drawn %v; want 400x300, "+
			"never cropped: its copy is", w, h, camera.IsCropped(cam), picture.originCropped)
	}
}

func TestFeed_LeavesOutOfEachLayerTheNextOnesWindowInsideItsFeather(t *testing.T) {
	const f = LayerFeather
	layers := []camera.Layer{
		{Share: 1, Resolution: 0.5},
		{Share: 0.6, Resolution: 0.75},
		{Share: 0.35, Resolution: 1},
	}
	// two of the outer layer's pixels past the feather, which its filter blends scaled up
	outer, middle := f+2/float32(0.5), f+2/float32(0.75)
	for _, tc := range []struct {
		layout camera.Layout
		want   []camera.Window
	}{
		{camera.Inset, []camera.Window{
			{X: 80 + outer, Y: 60 + outer, W: 240 - outer - outer, H: 180 - outer - outer},
			{X: 130 + middle, Y: 97 + middle, W: 140 - middle - middle, H: 106 - middle - middle},
			{},
		}},
		{camera.Footed, []camera.Window{
			{X: 80 + outer, Y: 120 + outer, W: 240 - outer - outer, H: 180 - outer},
			{X: 130 + middle, Y: 195 + middle, W: 140 - middle - middle, H: 105 - middle},
			{},
		}},
	} {
		cam := cameratest.NewFromSpace(1000, 1000, 0)
		cam.(camera.Layered).SetLayers(layers, tc.layout)
		picture := &recording{}
		feed := NewFeed(cam, picture)
		feed.Resize(400, 300)
		feed.Draw()
		if !slices.Equal(picture.holes, tc.want) {
			t.Errorf("%v: the layers left out %v, want %v: each the next one's window inside its "+
				"feather, hard on the frame's edge, the innermost nothing", tc.layout,
				picture.holes, tc.want)
		}
	}
}

func TestFeed_FadesOnlyTheSidesInsideTheFrame(t *testing.T) {
	const f = LayerFeather
	all := gfx.Fade{Left: f, Right: f, Top: f, Bottom: f}
	for _, c := range []struct {
		win  camera.Window
		want gfx.Fade
		why  string
	}{
		{camera.Window{W: 400, H: 300}, gfx.Fade{}, "the whole frame"},
		{camera.Window{X: 80, Y: 60, W: 240, H: 180}, all, "inset"},
		{camera.Window{X: 80, Y: 120, W: 240, H: 180}, gfx.Fade{Left: f, Right: f, Top: f},
			"footed, its bottom on the frame's"},
		{camera.Window{Y: 50, W: 400, H: 250}, gfx.Fade{Top: f},
			"the frame's width, on its bottom edge"},
	} {
		if got := fadeOf(c.win, 400, 300); got != c.want {
			t.Errorf("%s: %v fades %v, want %v", c.why, c.win, got, c.want)
		}
	}
}

func TestFeed_DrawsACameraOfOneRingAsAResolvedOne(t *testing.T) {
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	cam.(camera.Layered).SetLayers([]camera.Layer{{Share: 1, Resolution: 0.5}}, camera.Inset)
	cam.(camera.Resolved).SetResolution(0.5)
	picture := &recording{}
	feed := NewFeed(cam, picture)
	feed.Resize(400, 300)
	feed.Draw()
	if want := []call{{200, 150, 200, 150}}; !slices.Equal(picture.calls, want) {
		t.Errorf("the picture was drawn %v, want %v: once, at the camera's resolution",
			picture.calls, want)
	}
}

func TestFeed_DrawOnDrawsTheWholeFrameAtEveryPixelStraightOntoTheTarget(t *testing.T) {
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	cam.(camera.Layered).SetLayers([]camera.Layer{
		{Share: 1, Resolution: 1},
		{Share: 0.5, Resolution: 0.5},
	}, camera.Inset)
	picture := &recording{}
	feed := NewFeed(cam, picture)
	dst := gfx.NewImage(400, 300)
	feed.DrawOn(dst)
	want := []pass{
		{"lay", call{0, 0, 400, 300}, camera.Window{}},
		{"layer", call{400, 300, 400, 300}, camera.Window{W: 400, H: 300}},
		{"layer", call{100, 75, 100, 75}, camera.Window{X: 100, Y: 75, W: 200, H: 150}},
	}
	if !slices.Equal(picture.passes, want) || picture.layers[0] != dst {
		t.Errorf("the picture was drawn %v, the first layer onto the target %v; want %v, the "+
			"first straight onto it", picture.passes, picture.layers[0] == dst, want)
	}
	if w, h := cam.Viewport(); w != 400 || h != 300 || camera.IsCropped(cam) ||
		picture.originCropped {
		t.Errorf("after the frame the viewport is %vx%v, cropped %v, as drawn %v; want 400x300, "+
			"never cropped: its copy is", w, h, camera.IsCropped(cam), picture.originCropped)
	}
}

func TestFeed_DrawsTheGroundAtItsResolutionAndTheFiguresAtEveryPixel(t *testing.T) {
	cam := newFigured(true)
	cam.SetResolution(0.5)
	picture := &recording{}
	feed := NewFeed(cam, picture)
	feed.Resize(64, 48)
	img := feed.Draw()
	want := []pass{
		{"lay apart", call{0, 0, 64, 48}, camera.Window{}},
		{"layer", call{32, 24, 32, 24}, camera.Window{W: 64, H: 48}},
		{"over", call{64, 48, 64, 48}, camera.Window{}},
	}
	if !slices.Equal(picture.passes, want) || len(picture.calls) != 0 {
		t.Errorf("the picture was drawn %v and whole %v, want %v and never whole: the ground "+
			"composed at every pixel and drawn at a half, what lies over it at every pixel",
			picture.passes, picture.calls, want)
	}
	if img == nil || img.Bounds().Dx() != 64 || img.Bounds().Dy() != 48 {
		t.Fatalf("the feed's image is %v, want 64x48: the shown size", img.Bounds())
	}
	if len(picture.screens) != 1 || picture.screens[0] != img {
		t.Error("what lies over the figures was drawn onto another image than the feed's")
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
	feed := NewFeed(cam, picture)
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

// a feed of 64 by 48 shown pixels through cam in two layers, the whole frame at a half and the
// central half at 0.75, and what its picture was asked to draw
func layered(cam *figured) *recording {
	cam.SetLayers([]camera.Layer{{Share: 1, Resolution: 0.5}, {Share: 0.5, Resolution: 0.75}},
		camera.Inset)
	cam.SetResolution(0.75)
	picture := &recording{}
	feed := NewFeed(cam, picture)
	feed.Resize(64, 48)
	feed.Draw()
	return picture
}

func TestFeed_DrawsALayeredCameraSharpFromOneComposition(t *testing.T) {
	cam := newFigured(true)
	picture := layered(cam)
	want := []pass{
		{"lay apart", call{0, 0, 64, 48}, camera.Window{}},
		{"layer", call{32, 24, 32, 24}, camera.Window{W: 64, H: 48}},
		{"layer", call{24, 18, 24, 18}, camera.Window{X: 16, Y: 12, W: 32, H: 24}},
		{"over", call{64, 48, 64, 48}, camera.Window{}},
	}
	if !slices.Equal(picture.passes, want) || len(picture.calls) != 0 {
		t.Errorf("the picture was drawn %v and whole %v, want %v and never whole: the ground "+
			"composed once at every pixel, drawn a layer each, what lies over it at every pixel",
			picture.passes, picture.calls, want)
	}
	if w, h := cam.Viewport(); w != 48 || h != 36 || camera.IsCropped(cam) {
		t.Errorf("after the frame the viewport is %vx%v, cropped %v; want 48x36, not cropped",
			w, h, camera.IsCropped(cam))
	}
}

func TestFeed_DrawsALayeredCameraSoftFromOneComposition(t *testing.T) {
	picture := layered(newFigured(false))
	want := []pass{
		{"lay", call{0, 0, 64, 48}, camera.Window{}},
		{"layer", call{32, 24, 32, 24}, camera.Window{W: 64, H: 48}},
		{"layer", call{24, 18, 24, 18}, camera.Window{X: 16, Y: 12, W: 32, H: 24}},
	}
	if !slices.Equal(picture.passes, want) || len(picture.calls) != 0 {
		t.Errorf("the picture was drawn %v and whole %v, want %v: every source composed once at "+
			"every pixel, drawn a layer each, nothing over them", picture.passes, picture.calls,
			want)
	}
	cam := newFigured(true)
	cam.SetLayers([]camera.Layer{{Share: 1, Resolution: 1}, {Share: 0.5, Resolution: 1}},
		camera.Inset)
	picture = &recording{}
	feed := NewFeed(cam, picture)
	feed.Resize(64, 48)
	feed.Draw()
	if n := len(picture.passes); n != 3 || picture.passes[0].name != "lay" {
		t.Errorf("with the figures sharp in layers at every pixel the picture was drawn %v, want "+
			"laid whole and drawn a layer each, nothing over them", picture.passes)
	}
}

func TestFeed_DrawOnDrawsWhatLiesOverTheFiguresStraightOntoTheTarget(t *testing.T) {
	cam := newFigured(true)
	cam.SetResolution(0.5)
	picture := &recording{}
	feed := NewFeed(cam, picture)
	dst := gfx.NewImage(64, 48)
	feed.DrawOn(dst)
	want := []pass{
		{"lay apart", call{0, 0, 64, 48}, camera.Window{}},
		{"layer", call{32, 24, 32, 24}, camera.Window{W: 64, H: 48}},
		{"over", call{64, 48, 64, 48}, camera.Window{}},
	}
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
