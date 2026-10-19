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
// camera's resolution and shown at its size, its pixels the shown ones; a camera drawn in rings
// (camera.Ringed) is drawn once a ring, the frame cropped to each, the widest first and the inner
// ones laid over with feathered edges; one whose figures are sharp (camera.Figured) has a
// Grounded picture's ground drawn at its resolution and what lies over it at every shown pixel.
// Several feeds may share one picture.
type Feed struct {
	cam     camera.Camera
	picture Picture
	img     *Image
	ground  *Image // the ground drawn apart, at the camera's resolution
	w, h    int
	layers  []*Image // an image a ring
}

// RingFeather is the shown pixels an inner ring's edge fades over.
const RingFeather = 20

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

// Draw draws the world through the camera into the feed's image: at the camera's resolution, or
// at the shown size in rings or with the figures sharp.
func (f *Feed) Draw() *Image {
	if f.w <= 0 || f.h <= 0 {
		return nil
	}
	rings, cropping := f.rings()
	grounded, crop := f.sharp()
	switch {
	case rings != nil:
		f.freeGround()
		f.img = sized(f.img, f.w, f.h)
		f.img.Clear()
		f.drawRings(f.img, rings, cropping)
	case grounded != nil:
		f.freeLayers()
		f.img = sized(f.img, f.w, f.h)
		f.img.Clear()
		f.drawSharp(f.img, grounded, crop)
	default:
		f.freeLayers()
		f.freeGround()
		w, h := f.drawn()
		f.img = sized(f.img, w, h)
		f.img.Clear()
		f.picture.DrawWorld(f.img, f.cam)
	}
	return f.img
}

// the rings the camera is drawn in and the camera to crop to each: none for a camera drawn whole
// or one that cannot be cropped
func (f *Feed) rings() ([]camera.Ring, camera.Cropping) {
	c, ok := f.cam.(camera.Cropping)
	if !ok {
		return nil, nil
	}
	if rings := camera.RingsOf(f.cam); len(rings) > 1 {
		return rings, c
	}
	return nil, nil
}

// the picture and the camera to crop for a feed drawing the figures at every pixel: a camera
// that says so, drawn under every pixel and whole, before a picture drawing its ground apart
func (f *Feed) sharp() (Grounded, camera.Cropping) {
	if !camera.SharpFiguresOf(f.cam) || camera.ResolutionOf(f.cam) >= 1 {
		return nil, nil
	}
	if rings, _ := f.rings(); rings != nil {
		return nil, nil
	}
	g, grounded := f.picture.(Grounded)
	c, crops := f.cam.(camera.Cropping)
	if !grounded || !crops {
		return nil, nil
	}
	return g, c
}

// the ground drawn at the camera's resolution and laid over dst scaled, then what lies over the
// figures drawn straight onto dst, the camera cropped to the whole frame at every pixel
func (f *Feed) drawSharp(dst *Image, g Grounded, c camera.Cropping) {
	gw, gh := f.drawn()
	f.ground = sized(f.ground, gw, gh)
	f.ground.Clear()
	g.DrawGround(f.ground, f.cam)
	b := dst.Bounds()
	op := DrawImageOptions{Filter: FilterLinear}
	op.GeoM.Scale(float64(b.Dx())/float64(gw), float64(b.Dy())/float64(gh))
	op.GeoM.Translate(float64(b.Min.X), float64(b.Min.Y))
	dst.DrawImage(f.ground, &op)
	c.Crop(float32(f.w), float32(f.h), 1)
	g.DrawOver(dst, f.ground, f.cam)
	c.Uncrop()
}

// the picture drawn a ring at a time, the camera cropped to each, laid over dst at the ring's
// place in the frame; a ring of the whole frame at every pixel is drawn straight onto dst
func (f *Feed) drawRings(dst *Image, rings []camera.Ring, c camera.Cropping) {
	if len(f.layers) != len(rings) {
		f.freeLayers()
		f.layers = make([]*Image, len(rings))
	}
	b := dst.Bounds()
	for i, r := range rings {
		cw, ch := cropSize(f.w, f.h, r.Share)
		c.Crop(float32(cw), float32(ch), r.Resolution)
		if cw == f.w && ch == f.h && r.Resolution == 1 {
			f.picture.DrawWorld(dst, f.cam)
			c.Uncrop()
			continue
		}
		lw, lh := f.drawn()
		f.layers[i] = sized(f.layers[i], lw, lh)
		f.layers[i].Clear()
		f.picture.DrawWorld(f.layers[i], f.cam)
		c.Uncrop()
		op := DrawImageOptions{Filter: FilterLinear}
		op.GeoM.Scale(float64(cw)/float64(lw), float64(ch)/float64(lh))
		op.GeoM.Translate(float64(b.Min.X+(f.w-cw)/2), float64(b.Min.Y+(f.h-ch)/2))
		if i > 0 {
			op.Fade = Fade{RingFeather, RingFeather, RingFeather, RingFeather}
		}
		dst.DrawImage(f.layers[i], &op)
	}
}

// the central part of a w by h frame share of it wide and high, in whole pixels, the frame an
// even number of pixels wider and higher so the part lies a whole pixel in
func cropSize(w, h int, share float32) (int, int) {
	side := func(n int) int {
		c := int(math.Round(float64(n) * float64(share)))
		if (n-c)%2 != 0 {
			c++
		}
		return min(max(c, 2), n)
	}
	return side(w), side(h)
}

// img at w by h: itself at that size, else a new image in its place
func sized(img *Image, w, h int) *Image {
	if img != nil && img.Bounds().Dx() == w && img.Bounds().Dy() == h {
		return img
	}
	if img != nil {
		img.Deallocate()
	}
	return NewImage(w, h)
}

func (f *Feed) freeLayers() {
	for _, l := range f.layers {
		if l != nil {
			l.Deallocate()
		}
	}
	f.layers = nil
}

func (f *Feed) freeGround() {
	if f.ground != nil {
		f.ground.Deallocate()
		f.ground = nil
	}
}

// the size the picture is drawn at: the camera's pixels
func (f *Feed) drawn() (w, h int) {
	vw, vh := f.cam.Viewport()
	return max(int(math.Round(float64(vw))), 1), max(int(math.Round(float64(vh))), 1)
}

// DrawOn draws the world through the camera onto dst, shown at dst's size: straight onto it at
// the resolution of every pixel, else drawn at the camera's and scaled up; in rings, each over
// the last; with the figures sharp, the ground scaled up and what lies over it straight onto dst.
func (f *Feed) DrawOn(dst *Image) {
	b := dst.Bounds()
	f.Resize(b.Dx(), b.Dy())
	if rings, cropping := f.rings(); rings != nil {
		f.freeGround()
		f.drawRings(dst, rings, cropping)
		return
	}
	f.freeLayers()
	if g, c := f.sharp(); g != nil {
		f.drawSharp(dst, g, c)
		return
	}
	f.freeGround()
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
