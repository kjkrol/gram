package render

import "github.com/kjkrol/gram/camera"

// Surface is a picture shown at a size whoever shows it gives it — a Feed of the world, say.
type Surface interface {
	// Resize gives the surface its size in pixels.
	Resize(w, h int)
	// Draw is the picture for this frame, drawn anew; nil while it has no size.
	Draw() *Image
}

// Feed is the world seen through a camera: picture drawn through cam every frame into an image of
// the size it is shown at. Several feeds may share one picture; whoever lists the picture
// initialises it once.
type Feed struct {
	cam     camera.Camera
	picture Picture
	img     *Image
	w, h    int
}

var _ Surface = (*Feed)(nil)

// NewFeed is the world picture draws seen through cam.
func NewFeed(cam camera.Camera, picture Picture) *Feed {
	return &Feed{cam: cam, picture: picture}
}

// Resize gives the feed its size, the camera's viewport with it.
func (f *Feed) Resize(w, h int) {
	if w == f.w && h == f.h {
		return
	}
	f.w, f.h = w, h
	if w > 0 && h > 0 {
		f.cam.SetViewport(float32(w), float32(h))
	}
}

// Draw draws the world through the camera into the feed's image.
func (f *Feed) Draw() *Image {
	if f.w <= 0 || f.h <= 0 {
		return nil
	}
	if f.img == nil || f.img.Bounds().Dx() != f.w || f.img.Bounds().Dy() != f.h {
		if f.img != nil {
			f.img.Deallocate()
		}
		f.img = NewImage(f.w, f.h)
	}
	f.img.Clear()
	f.picture.DrawWorld(f.img, f.cam)
	return f.img
}

// DrawOn draws the world through the camera straight onto dst, the camera's viewport dst's size:
// a feed filling the whole screen, drawn without an image of its own between.
func (f *Feed) DrawOn(dst *Image) {
	b := dst.Bounds()
	f.Resize(b.Dx(), b.Dy())
	f.picture.DrawWorld(dst, f.cam)
}

// ToWorld is the point of the world under the feed's pixel (px, py): the ground the camera picks
// there where it can (camera.Picker), else the point at sea level; false where it sees no ground.
func (f *Feed) ToWorld(px, py float32) (x, y float32, ok bool) {
	if p, is := f.cam.(camera.Picker); is {
		return p.Pick(px, py)
	}
	x, y = f.cam.FromScreen(px, py)
	return x, y, true
}

// ToPixels is where in the feed the world point (x, y) at height z lies, and its depth; visible
// is false for a point outside the feed or behind the eye.
func (f *Feed) ToPixels(x, y, z float32) (px, py, depth float32, visible bool) {
	px, py = f.cam.Project(x, y, z)
	depth = f.cam.Depth(x, y, z)
	w, h := f.cam.Viewport()
	visible = px >= 0 && py >= 0 && px < w && py < h
	if s, is := f.cam.(camera.Scaler); is && s.ScaleAt(x, y, z) <= 0 {
		visible = false
	}
	return px, py, depth, visible
}
