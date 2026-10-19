package render

import (
	"math"

	"github.com/kjkrol/gram/camera"
)

// Surface is a picture shown at a size whoever shows it gives it.
type Surface interface {
	// Resize gives the surface its size in pixels.
	Resize(w, h int)
	// Draw is the picture for this frame, drawn anew; nil while it has no size.
	Draw() *Image
}

// Feed is the world seen through a camera: picture drawn through cam every frame, drawn at its
// camera's resolution and shown at its size, its pixels the shown ones. Several feeds may share one
// picture.
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

// Camera is the camera the feed draws its picture through.
func (f *Feed) Camera() camera.Camera { return f.cam }

// Picture is what the feed draws.
func (f *Feed) Picture() Picture { return f.picture }

// Proportions are the width and height of the world the feed shows whole, when its camera keeps
// the whole world in view.
func (f *Feed) Proportions() (w, h float64, ok bool) {
	if c, is := f.cam.(camera.Fitting); is {
		if cw, ch, whole := c.Fits(); whole {
			return float64(cw), float64(ch), true
		}
	}
	return 0, 0, false
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
	w, h := f.drawn()
	if f.img == nil || f.img.Bounds().Dx() != w || f.img.Bounds().Dy() != h {
		if f.img != nil {
			f.img.Deallocate()
		}
		f.img = NewImage(w, h)
	}
	f.img.Clear()
	f.picture.DrawWorld(f.img, f.cam)
	return f.img
}

// the size the picture is drawn at: the camera's pixels
func (f *Feed) drawn() (w, h int) {
	vw, vh := f.cam.Viewport()
	return max(int(math.Round(float64(vw))), 1), max(int(math.Round(float64(vh))), 1)
}

// DrawOn draws the world through the camera onto dst, shown at dst's size: straight onto it at
// the resolution of every pixel, else drawn at the camera's and scaled up.
func (f *Feed) DrawOn(dst *Image) {
	b := dst.Bounds()
	f.Resize(b.Dx(), b.Dy())
	if camera.ResolutionOf(f.cam) == 1 {
		f.picture.DrawWorld(dst, f.cam)
		return
	}
	img := f.Draw()
	if img == nil {
		return
	}
	op := &DrawImageOptions{Filter: FilterLinear}
	op.GeoM.Scale(float64(b.Dx())/float64(img.Bounds().Dx()),
		float64(b.Dy())/float64(img.Bounds().Dy()))
	dst.DrawImage(img, op)
}

// ToWorld is the point of the world under the feed's pixel (px, py), false where the camera sees
// no ground there.
func (f *Feed) ToWorld(px, py float32) (x, y float32, ok bool) {
	k := camera.ResolutionOf(f.cam)
	px, py = px*k, py*k
	if p, is := f.cam.(camera.Picker); is {
		x, y, _, ok = p.Pick(px, py)
		return x, y, ok
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
	k := camera.ResolutionOf(f.cam)
	return px / k, py / k, depth, visible
}
