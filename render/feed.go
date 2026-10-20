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
// camera's resolution and shown at its size, its pixels the shown ones; a camera drawn in layers
// (camera.Layered) is drawn once a layer, the frame cropped to each, the widest first and the inner
// ones laid over with feathered edges; one whose figures are sharp (camera.Figured) has a
// Grounded picture's ground drawn at its resolution and what lies over it at every shown pixel.
// Several feeds may share one picture.
type Feed struct {
	cam     camera.Camera
	picture Picture
	img     *Image
	ground  *Image // the ground drawn apart, at the camera's resolution
	w, h    int
	imgs    []*Image // an image a layer
}

// LayerFeather is the shown pixels an inner layer's edge fades over.
const LayerFeather = 20

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
// at the shown size in layers or with the figures sharp.
func (f *Feed) Draw() *Image {
	if f.w <= 0 || f.h <= 0 {
		return nil
	}
	layers, cropping := f.layers()
	grounded, crop := f.sharp()
	switch {
	case layers != nil:
		f.freeGround()
		f.img = sized(f.img, f.w, f.h)
		f.img.Clear()
		f.drawLayers(f.img, layers, cropping)
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

// the layers the camera is drawn in and the camera to crop to each: none for a camera drawn whole
// or one that cannot be cropped
func (f *Feed) layers() ([]camera.Layer, camera.Cropping) {
	c, ok := f.cam.(camera.Cropping)
	if !ok {
		return nil, nil
	}
	if layers := camera.LayersOf(f.cam); len(layers) > 1 {
		return layers, c
	}
	return nil, nil
}

// the picture and the camera to crop for a feed drawing the figures at every pixel: a camera
// that says so, drawn under every pixel and whole, before a picture drawing its ground apart
func (f *Feed) sharp() (Grounded, camera.Cropping) {
	if !camera.SharpFiguresOf(f.cam) || camera.ResolutionOf(f.cam) >= 1 {
		return nil, nil
	}
	if layers, _ := f.layers(); layers != nil {
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
	c.Crop(camera.Window{W: float32(f.w), H: float32(f.h)}, 1)
	g.DrawOver(dst, f.ground, f.cam)
	c.Uncrop()
}

// the picture drawn a layer at a time, the camera cropped to each layer's window as the camera's
// layout lays it, laid over dst there; a layer of the whole frame at every pixel is drawn
// straight onto dst
func (f *Feed) drawLayers(dst *Image, layers []camera.Layer, c camera.Cropping) {
	if len(f.imgs) != len(layers) {
		f.freeLayers()
		f.imgs = make([]*Image, len(layers))
	}
	b := dst.Bounds()
	wins := camera.Arrange(camera.LayoutOf(f.cam), layers, f.w, f.h)
	for i, r := range layers {
		win := wins[i]
		c.Crop(win, r.Resolution)
		if win.W == float32(f.w) && win.H == float32(f.h) && r.Resolution == 1 {
			f.picture.DrawWorld(dst, f.cam)
			c.Uncrop()
			continue
		}
		lw, lh := f.drawn()
		f.imgs[i] = sized(f.imgs[i], lw, lh)
		f.imgs[i].Clear()
		f.picture.DrawWorld(f.imgs[i], f.cam)
		c.Uncrop()
		op := DrawImageOptions{Filter: FilterLinear, Fade: fadeOf(win, f.w, f.h)}
		op.GeoM.Scale(float64(win.W)/float64(lw), float64(win.H)/float64(lh))
		op.GeoM.Translate(float64(b.Min.X)+float64(win.X), float64(b.Min.Y)+float64(win.Y))
		dst.DrawImage(f.imgs[i], &op)
	}
}

// how a layer's window in a frame fw by fh fades: over the feather on each side inside the frame,
// hard on the frame's edges
func fadeOf(win camera.Window, fw, fh int) Fade {
	var fade Fade
	if win.X > 0 {
		fade.Left = LayerFeather
	}
	if win.X+win.W < float32(fw) {
		fade.Right = LayerFeather
	}
	if win.Y > 0 {
		fade.Top = LayerFeather
	}
	if win.Y+win.H < float32(fh) {
		fade.Bottom = LayerFeather
	}
	return fade
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
	for _, l := range f.imgs {
		if l != nil {
			l.Deallocate()
		}
	}
	f.imgs = nil
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
// the resolution of every pixel, else drawn at the camera's and scaled up; in layers, each over
// the last; with the figures sharp, the ground scaled up and what lies over it straight onto dst.
func (f *Feed) DrawOn(dst *Image) {
	b := dst.Bounds()
	f.Resize(b.Dx(), b.Dy())
	if layers, cropping := f.layers(); layers != nil {
		f.freeGround()
		f.drawLayers(dst, layers, cropping)
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
