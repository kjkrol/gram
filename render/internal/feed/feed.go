package feed

import (
	"math"
	"slices"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render/internal/gfx"
)

// Picture is what is in the world before a camera, drawn through the camera it is handed.
type Picture interface {
	DrawWorld(screen *gfx.Image, cam camera.Camera)
}

// Laid is a Picture drawn in layers from one composition: Lay composes every source (the
// ground's alone when apart) through cam once, in cam's pixels; DrawLayer draws what was
// composed into layer, cam cropped to the frame's window at, the pieces placed by the window's
// scale and offset, the Directs drawing through cam, the ground leaving out the frame's window
// hole a layer drawn after covers; DrawOver composes the Overlying sources and draws them onto
// screen over the layers laid since Lay, tested against their depths placed at screen's size,
// the last laid over the first. A nil layer or screen only composes.
type Laid interface {
	Lay(cam camera.Camera, apart bool)
	DrawLayer(layer *gfx.Image, cam camera.Camera, at, hole camera.Window)
	DrawOver(screen *gfx.Image, cam camera.Camera)
}

// Feed is the world seen through a camera: picture drawn through cam every frame, drawn at its
// camera's resolution and shown at its size, its pixels the shown ones; a camera drawn in layers
// (camera.Layered) is drawn once a layer, the frame cropped to each, the widest first and the inner
// ones laid over with feathered edges, a Laid picture composed once for them all; one whose
// figures are sharp (camera.Figured) has a Laid picture's ground drawn so, or whole at its
// resolution, and what lies over it at every shown pixel. Several feeds may share one picture.
type Feed struct {
	cam     camera.Camera
	picture Picture
	img     *gfx.Image
	ground  *gfx.Image // the ground drawn apart, at the camera's resolution
	w, h    int
	imgs    []*gfx.Image // an image a layer
}

// LayerFeather is the shown pixels an inner layer's edge fades over.
const LayerFeather = 20

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
func (f *Feed) Draw() *gfx.Image {
	if f.w <= 0 || f.h <= 0 {
		return nil
	}
	layers, cropping := f.layers()
	sharp, crop := f.sharp(layers)
	switch {
	case layers != nil:
		f.freeGround()
		f.img = sized(f.img, f.w, f.h)
		f.img.Clear()
		f.drawLayers(f.img, layers, cropping, sharp != nil)
	case sharp != nil:
		f.freeLayers()
		f.img = sized(f.img, f.w, f.h)
		f.img.Clear()
		f.drawSharp(f.img, sharp, crop)
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
// that says so, drawn under every pixel in one of its layers or whole, before a Laid picture; in
// layers the camera's resolution is the innermost one's, often every pixel
func (f *Feed) sharp(layers []camera.Layer) (Laid, camera.Cropping) {
	coarse := camera.ResolutionOf(f.cam) < 1
	if layers != nil {
		coarse = slices.ContainsFunc(layers, func(l camera.Layer) bool { return l.Resolution < 1 })
	}
	if !camera.SharpFiguresOf(f.cam) || !coarse {
		return nil, nil
	}
	l, laid := f.picture.(Laid)
	c, crops := f.cam.(camera.Cropping)
	if !laid || !crops {
		return nil, nil
	}
	return l, c
}

// the ground composed once and drawn at the camera's resolution, laid over dst scaled, then what
// lies over the figures drawn straight onto dst, both composed through the camera cropped to the
// whole frame at every pixel
func (f *Feed) drawSharp(dst *gfx.Image, l Laid, c camera.Cropping) {
	f.cropWhole(c)
	l.Lay(f.cam, true)
	c.Uncrop()
	gw, gh := f.drawn()
	f.ground = sized(f.ground, gw, gh)
	f.ground.Clear()
	l.DrawLayer(f.ground, f.cam, f.whole(), camera.Window{})
	b := dst.Bounds()
	op := gfx.DrawImageOptions{Filter: gfx.FilterLinear}
	op.GeoM.Scale(float64(b.Dx())/float64(gw), float64(b.Dy())/float64(gh))
	op.GeoM.Translate(float64(b.Min.X), float64(b.Min.Y))
	dst.DrawImage(f.ground, &op)
	f.cropWhole(c)
	l.DrawOver(dst, f.cam)
	c.Uncrop()
}

// the camera cropped to the whole frame at every shown pixel, the pixels a Laid picture is
// composed in
func (f *Feed) cropWhole(c camera.Cropping) { c.Crop(f.whole(), 1) }

// the picture drawn a layer at a time, the camera cropped to each layer's window as the camera's
// layout lays it, laid over dst there, a Laid picture composed once for them all and, with the
// figures sharp, what lies over them drawn straight onto dst; a layer of the whole frame at every
// pixel is drawn straight onto dst
func (f *Feed) drawLayers(dst *gfx.Image, layers []camera.Layer, c camera.Cropping, sharp bool) {
	if len(f.imgs) != len(layers) {
		f.freeLayers()
		f.imgs = make([]*gfx.Image, len(layers))
	}
	laid, _ := f.picture.(Laid)
	if laid != nil {
		f.cropWhole(c)
		laid.Lay(f.cam, sharp)
		c.Uncrop()
	}
	b := dst.Bounds()
	wins := camera.Arrange(camera.LayoutOf(f.cam), layers, f.w, f.h)
	for i, r := range layers {
		win := wins[i]
		var hole camera.Window
		if i+1 < len(wins) {
			hole = f.holeOf(wins[i+1], r.Resolution)
		}
		c.Crop(win, r.Resolution)
		if win.W == float32(f.w) && win.H == float32(f.h) && r.Resolution == 1 {
			f.drawLayer(dst, laid, win, hole)
			c.Uncrop()
			continue
		}
		lw, lh := f.drawn()
		f.imgs[i] = sized(f.imgs[i], lw, lh)
		f.imgs[i].Clear()
		f.drawLayer(f.imgs[i], laid, win, hole)
		c.Uncrop()
		op := gfx.DrawImageOptions{Filter: gfx.FilterLinear, Fade: fadeOf(win, f.w, f.h)}
		op.GeoM.Scale(float64(win.W)/float64(lw), float64(win.H)/float64(lh))
		op.GeoM.Translate(float64(b.Min.X)+float64(win.X), float64(b.Min.Y)+float64(win.Y))
		dst.DrawImage(f.imgs[i], &op)
	}
	if sharp {
		f.cropWhole(c)
		laid.DrawOver(dst, f.cam)
		c.Uncrop()
	}
}

// the picture drawn into img through the camera cropped to the frame's window win: a layer of
// what Lay composed where the picture is Laid, its ground leaving out the window hole
func (f *Feed) drawLayer(img *gfx.Image, laid Laid, win, hole camera.Window) {
	if laid == nil {
		f.picture.DrawWorld(img, f.cam)
		return
	}
	laid.DrawLayer(img, f.cam, win, hole)
}

// the part of the frame's window inner that its layer covers whole when laid over a layer at
// res: inside the feather of each side that fades, and two of that layer's pixels more, which
// its filter blends with the next ones when scaled up
func (f *Feed) holeOf(inner camera.Window, res float32) camera.Window {
	fade := fadeOf(inner, f.w, f.h)
	side := func(feather float32) float32 {
		if feather == 0 {
			return 0
		}
		return feather + 2/res
	}
	l, r, t, b := side(fade.Left), side(fade.Right), side(fade.Top), side(fade.Bottom)
	hole := camera.Window{X: inner.X + l, Y: inner.Y + t, W: inner.W - l - r, H: inner.H - t - b}
	if hole.W <= 0 || hole.H <= 0 {
		return camera.Window{}
	}
	return hole
}

// the window of the whole frame
func (f *Feed) whole() camera.Window { return camera.Window{W: float32(f.w), H: float32(f.h)} }

// how a layer's window in a frame fw by fh fades: over the feather on each side inside the frame,
// hard on the frame's edges
func fadeOf(win camera.Window, fw, fh int) gfx.Fade {
	var fade gfx.Fade
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
func sized(img *gfx.Image, w, h int) *gfx.Image {
	if img != nil && img.Bounds().Dx() == w && img.Bounds().Dy() == h {
		return img
	}
	if img != nil {
		img.Deallocate()
	}
	return gfx.NewImage(w, h)
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

// DrawGlass has the picture's Glass sources draw on the glass over the feed, dst, as it is
// shown, through the camera (ui draws it after everything laid over the feed).
func (f *Feed) DrawGlass(dst *gfx.Image) {
	if g, ok := f.picture.(interface {
		DrawGlass(dst *gfx.Image, cam camera.Camera)
	}); ok {
		g.DrawGlass(dst, f.cam)
	}
}

// DrawOn draws the world through the camera onto dst, shown at dst's size: straight onto it at
// the resolution of every pixel, else drawn at the camera's and scaled up; in layers, each over
// the last; with the figures sharp, the ground scaled up and what lies over it straight onto dst.
func (f *Feed) DrawOn(dst *gfx.Image) {
	b := dst.Bounds()
	f.Resize(b.Dx(), b.Dy())
	layers, cropping := f.layers()
	sharp, crop := f.sharp(layers)
	if layers != nil {
		f.freeGround()
		f.drawLayers(dst, layers, cropping, sharp != nil)
		return
	}
	f.freeLayers()
	if sharp != nil {
		f.drawSharp(dst, sharp, crop)
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
	op := &gfx.DrawImageOptions{Filter: gfx.FilterLinear}
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
