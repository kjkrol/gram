package topdown

import (
	"fmt"
	"slices"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	contract "github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
)

type basicCamera struct {
	world        geom.Vec
	viewportSize geom.Vec
	edges        aabbworld.Edges
	minZoomCfg   float32
	maxZoom      float32
	zoom         float32
	res          float32
	whole        bool

	effective plane.AABB
	fastening contract.Fastening
	form      form
	detail    contract.Detail
	layers    []contract.Layer
	layout    contract.Layout
	sharp     bool
	// the frame while a part of it is drawn, to come back to
	cropped *frame
	// its copy for the frame drawn, or the camera it is a copy of
	copy, origin *basicCamera
}

type frame struct {
	viewportSize geom.Vec
	res          float32
	effective    plane.AABB
	win          contract.Window
}

// form names how a camera from above is saved, so a save of another shape is refused at its
// first value
type form struct{}

const savedAs = "top 1"

func (form) GobEncode() ([]byte, error) { return []byte(savedAs), nil }

func (*form) GobDecode(b []byte) error {
	if string(b) != savedAs {
		return fmt.Errorf("saved as %q, not %q", b, savedAs)
	}
	return nil
}

var _ contract.Camera = (*basicCamera)(nil)
var _ contract.Fastenable = (*basicCamera)(nil)
var _ contract.Fitting = (*basicCamera)(nil)
var _ contract.Focused = (*basicCamera)(nil)
var _ contract.Detailed = (*basicCamera)(nil)
var _ contract.Resolved = (*basicCamera)(nil)
var _ contract.Layered = (*basicCamera)(nil)
var _ contract.Cropping = (*basicCamera)(nil)
var _ contract.Figured = (*basicCamera)(nil)
var _ contract.Snapped = (*basicCamera)(nil)

// Snap takes the camera's copy for the frame drawn next.
func (c *basicCamera) Snap() {
	if c.copy == nil {
		c.copy = &basicCamera{}
	}
	d := c.copy
	layers := d.layers
	*d = *c
	d.layers = append(layers[:0], c.layers...)
	d.cropped, d.copy, d.origin = nil, nil, c
}

func (c *basicCamera) Drawn() contract.Camera {
	if c.copy == nil {
		return c
	}
	return c.copy
}

func (c *basicCamera) Origin() contract.Camera {
	if c.origin == nil {
		return c
	}
	return c.origin
}

// Detail is how far the camera sees and how finely.
func (c *basicCamera) Detail() contract.Detail { return c.detail }

// SetDetail has the camera see as d says.
func (c *basicCamera) SetDetail(d contract.Detail) { c.detail = d }

// See has the camera see factor times as far.
func (c *basicCamera) See(factor float32) { c.detail = c.detail.Widened(factor) }

// Fasten fastens the camera to an entity, or to nothing.
func (c *basicCamera) Fasten(f contract.Fastening) { c.fastening = f }

func (c *basicCamera) Fastening() contract.Fastening { return c.fastening }

func newBasicCamera(world geom.Vec, viewport contract.AABB, edges aabbworld.Edges) *basicCamera {
	w := viewport.BottomRight.X - viewport.TopLeft.X
	h := viewport.BottomRight.Y - viewport.TopLeft.Y
	return &basicCamera{
		world:        world,
		viewportSize: geom.NewVec(w, h),
		edges:        edges,
		zoom:         1,
		res:          1,
		effective:    plane.NewAABB(viewport.TopLeft, w, h),
	}
}

// NewFromSpace is a top-down camera over a width x height world, viewing all of it by default.
func NewFromSpace(width, height uint32, edges aabbworld.Edges, viewport ...contract.AABB) contract.Camera {
	vp := contract.AABB{}
	if len(viewport) > 0 {
		vp = viewport[0]
	}
	if vp.Equals(contract.AABB{}) {
		vp = geom.NewAABBAt(geom.NewVec(0, 0), float32(width), float32(height))
	}
	return newBasicCamera(geom.NewVec(float32(width), float32(height)), vp, edges)
}

// NewFromSpaceWithConfig is a top-down camera over a width x height world as cfg says it starts,
// at cfg's zoom round the world's middle.
func NewFromSpaceWithConfig(width, height uint32, edges aabbworld.Edges, cfg contract.Config) contract.Camera {
	c := NewFromSpace(width, height, edges).(*basicCamera)
	c.whole = cfg.Whole
	if cfg.Zoom > 0 && !c.whole {
		c.setZoom(cfg.Zoom)
	}
	return limited(c, cfg)
}

func limited(cam contract.Camera, cfg contract.Config) contract.Camera {
	if cfg.MinZoom > 0 {
		cam.SetMinZoom(cfg.MinZoom)
	}
	if cfg.MaxZoom > 0 {
		cam.SetMaxZoom(cfg.MaxZoom)
	}
	return cam
}

func wrapRelative(v, ref, size float32) float32 {
	rel := f32.Mod(v-ref, size)
	if rel < 0 {
		rel += size
	}
	return ref + rel
}

func wrapMod(v, size float32) float32 {
	v = f32.Mod(v, size)
	if v < 0 {
		v += size
	}
	return v
}

func windowOffset(x, ref, ww, ws float32) float32 {
	fwd := wrapMod(x-ref, ww)
	distFwd := float32(0)
	if fwd > ws {
		distFwd = fwd - ws
	}
	back := fwd - ww
	distBack := float32(0)
	if back < 0 {
		distBack = -back
	}
	if distBack < distFwd {
		return back
	}
	return fwd
}

func (c *basicCamera) Projection() contract.Projection { return contract.TopDown{} }

// SetResolution has the camera drawn at s of the pixels it is shown at; its window stays.
func (c *basicCamera) SetResolution(s float32) { c.res = s }

func (c *basicCamera) Resolution() float32 { return c.res }

// SetLayers has the picture drawn in layers laid out as layout says, or whole for none or one.
func (c *basicCamera) SetLayers(layers []contract.Layer, layout contract.Layout) {
	c.layers, c.layout = slices.Clone(layers), layout
}

func (c *basicCamera) Layout() contract.Layout { return c.layout }

func (c *basicCamera) Layers() []contract.Layer { return slices.Clone(c.layers) }

// SetSharpFigures has the figures drawn at every shown pixel while the ground is drawn at the
// resolution, or at the resolution too.
func (c *basicCamera) SetSharpFigures(on bool) { c.sharp = on }

func (c *basicCamera) SharpFigures() bool { return c.sharp }

// Crop has the camera drawn the part win of its frame at s of its shown pixels, the zoom as it
// is; a camera keeping the whole world in view is drawn whole.
func (c *basicCamera) Crop(win contract.Window, s float32) {
	if c.whole || win.W <= 0 || win.H <= 0 {
		return
	}
	c.Uncrop()
	c.cropped = &frame{viewportSize: c.viewportSize, res: c.res, effective: c.effective, win: win}
	zoom := c.zoom
	at := geom.NewVec(c.effective.TopLeft.X+win.X/zoom,
		c.effective.TopLeft.Y+win.Y/zoom)
	c.viewportSize = geom.NewVec(win.W, win.H)
	c.res = s
	c.effective = plane.NewAABB(at, win.W/zoom, win.H/zoom)
}

// Uncrop puts the frame back as it was before Crop.
func (c *basicCamera) Uncrop() {
	if c.cropped == nil {
		return
	}
	c.viewportSize, c.res, c.effective = c.cropped.viewportSize, c.cropped.res, c.cropped.effective
	c.cropped = nil
}

func (c *basicCamera) Cropped() bool {
	return c.cropped != nil && c.viewportSize != c.cropped.viewportSize
}

func (c *basicCamera) Frame() (float32, float32) {
	size := c.viewportSize
	if c.cropped != nil {
		size = c.cropped.viewportSize
	}
	return size.X, size.Y
}

func (c *basicCamera) Window() contract.Window {
	if c.cropped != nil {
		return c.cropped.win
	}
	return contract.Window{W: c.viewportSize.X, H: c.viewportSize.Y}
}

// camera pixels a world unit
func (c *basicCamera) px() float32 { return c.zoom * c.res }

func (c *basicCamera) Viewport() (float32, float32) {
	return c.viewportSize.X * c.res, c.viewportSize.Y * c.res
}

func (c *basicCamera) SetViewport(w, h float32) {
	if w <= 0 || h <= 0 {
		return
	}
	if c.whole {
		c.viewportSize = geom.NewVec(w, h)
		c.fit()
		return
	}
	cx := c.effective.TopLeft.X + c.effective.Size.X/2
	cy := c.effective.TopLeft.Y + c.effective.Size.Y/2
	c.viewportSize = geom.NewVec(w, h)
	zoom := max(c.zoom, c.minZoom())
	if c.maxZoom > 0 {
		zoom = min(zoom, max(c.maxZoom, c.minZoom()))
	}
	c.zoom = zoom
	vw, vh := c.viewportSize.X/zoom, c.viewportSize.Y/zoom
	c.place(cx-vw/2, cy-vh/2, vw, vh)
}

func (c *basicCamera) Project(x, y, _ float32) (float32, float32) { return c.ToScreen(x, y) }

func (c *basicCamera) Unproject(sx, sy, _ float32) (float32, float32) { return c.FromScreen(sx, sy) }

func (c *basicCamera) Depth(_, y, _ float32) float32 { return y }

func (c *basicCamera) ToScreen(x, y float32) (float32, float32) {
	if c.edges.WrapsX() {
		x = wrapRelative(x, c.effective.TopLeft.X, c.world.X)
	}
	if c.edges.WrapsY() {
		y = wrapRelative(y, c.effective.TopLeft.Y, c.world.Y)
	}
	px := c.px()
	return (x - c.effective.TopLeft.X) * px, (y - c.effective.TopLeft.Y) * px
}

func (c *basicCamera) ToScreenQuads(x0, y0, x1, y1 float32, dst []contract.Quad) []contract.Quad {
	px := c.px()
	if c.edges&aabbworld.Torus == 0 {
		sx0, sy0 := c.ToScreen(x0, y0)
		return append(dst, contract.Quad{
			X0: sx0, Y0: sy0, X1: sx0 + (x1-x0)*px, Y1: sy0 + (y1-y0)*px,
			T0X: 0, T1X: 1, T0Y: 0, T1Y: 1,
		})
	}
	u0 := c.offsetX(x0)
	u1 := u0 + (x1 - x0)
	v0 := c.offsetY(y0)
	v1 := v0 + (y1 - y0)

	var xs, ys [2]rangePiece
	for _, xp := range axisPieces(u0, u1, c.world.X, c.edges.WrapsX(), &xs) {
		for _, yp := range axisPieces(v0, v1, c.world.Y, c.edges.WrapsY(), &ys) {
			sx0 := xp.screenLo * px
			sx1 := sx0 + (xp.hi-xp.lo)*px
			sy0 := yp.screenLo * px
			sy1 := sy0 + (yp.hi-yp.lo)*px
			dst = append(dst, contract.Quad{
				X0: sx0, Y0: sy0, X1: sx1, Y1: sy1,
				T0X: (xp.lo - u0) / (u1 - u0), T1X: (xp.hi - u0) / (u1 - u0),
				T0Y: (yp.lo - v0) / (v1 - v0), T1Y: (yp.hi - v0) / (v1 - v0),
			})
		}
	}
	return dst
}

type rangePiece struct{ lo, hi, screenLo float32 }

func (c *basicCamera) offsetX(x float32) float32 {
	ref := c.effective.TopLeft.X
	if !c.edges.WrapsX() {
		return x - ref
	}
	return windowOffset(x, ref, c.world.X, c.effective.Size.X)
}

func (c *basicCamera) offsetY(y float32) float32 {
	ref := c.effective.TopLeft.Y
	if !c.edges.WrapsY() {
		return y - ref
	}
	return windowOffset(y, ref, c.world.Y, c.effective.Size.Y)
}

func axisPieces(u0, u1, size float32, wraps bool, buf *[2]rangePiece) []rangePiece {
	if wraps && u1 > size {
		buf[0], buf[1] = rangePiece{u0, size, u0}, rangePiece{size, u1, 0}
		return buf[:2]
	}
	buf[0] = rangePiece{u0, u1, u0}
	return buf[:1]
}

func (c *basicCamera) FromScreen(sx, sy float32) (float32, float32) {
	px := c.px()
	x := sx/px + c.effective.TopLeft.X
	y := sy/px + c.effective.TopLeft.Y
	if c.edges.WrapsX() {
		x = wrapMod(x, c.world.X)
	}
	if c.edges.WrapsY() {
		y = wrapMod(y, c.world.Y)
	}
	return x, y
}

// Visible is false for a box only touching the window's edge.
func (c *basicCamera) Visible(box contract.AABB) bool {
	tlX, tlY := box.TopLeft.X, box.TopLeft.Y
	brX, brY := box.BottomRight.X, box.BottomRight.Y
	if c.edges.WrapsX() {
		width := brX - tlX
		tlX = c.effective.TopLeft.X + c.offsetX(tlX)
		brX = tlX + width
	}
	if c.edges.WrapsY() {
		height := brY - tlY
		tlY = c.effective.TopLeft.Y + c.offsetY(tlY)
		brY = tlY + height
	}
	far := c.Bounds()
	return brX > c.effective.TopLeft.X && tlX < far.BottomRight.X &&
		brY > c.effective.TopLeft.Y && tlY < far.BottomRight.Y
}

// Focus is the middle of the window, at the ground.
func (c *basicCamera) Focus() (float32, float32, float32) {
	return c.effective.TopLeft.X + c.effective.Size.X/2,
		c.effective.TopLeft.Y + c.effective.Size.Y/2, 0
}

// Bounds is the visible window, which on a torus may run past the world's own size.
func (c *basicCamera) Bounds() contract.AABB {
	return contract.AABB{
		TopLeft: c.effective.TopLeft,
		BottomRight: geom.NewVec(
			c.effective.TopLeft.X+c.effective.Size.X,
			c.effective.TopLeft.Y+c.effective.Size.Y,
		),
	}
}

// CenterOn puts (x, y) in the middle of the window.
func (c *basicCamera) CenterOn(x, y, _ float32) {
	w, h := c.effective.Size.X, c.effective.Size.Y
	c.place(x-w/2, y-h/2, w, h)
}

// MoveTo puts the visible window's top-left corner at (x, y), keeping its size.
func (c *basicCamera) MoveTo(x, y float32) {
	c.Translate(x-c.effective.TopLeft.X, y-c.effective.TopLeft.Y)
}

// Translate shifts the visible window by a delta in world units.
func (c *basicCamera) Translate(dx, dy float32) {
	c.place(c.effective.TopLeft.X+dx, c.effective.TopLeft.Y+dy, c.effective.Size.X, c.effective.Size.Y)
}

// Pan shifts the visible window by a delta in camera pixels.
func (c *basicCamera) Pan(dx, dy float32) {
	px := c.px()
	c.Translate(dx/px, dy/px)
}

func (c *basicCamera) place(x, y, w, h float32) {
	if c.whole {
		return
	}
	x = fitAxis(x, w, c.world.X, c.edges.WrapsX())
	y = fitAxis(y, h, c.world.Y, c.edges.WrapsY())
	c.effective = plane.NewAABB(geom.NewVec(x, y), w, h)
}

func fitAxis(lo, length, world float32, wraps bool) float32 {
	if !wraps {
		return min(max(lo, 0), max(world-length, 0))
	}
	lo = f32.Mod(lo, world)
	if lo < 0 {
		lo += world
	}
	return lo
}

// Zoom is the current zoom factor, camera pixels a world unit.
func (c *basicCamera) Zoom() float32 { return c.px() }

// ZoomIn multiplies the zoom by factor, keeping the world point (anchorX, anchorY) fixed on
// screen.
func (c *basicCamera) ZoomIn(factor float32, anchorX, anchorY float32) {
	if c.whole {
		return
	}
	beforeX, beforeY := c.ToScreen(anchorX, anchorY)

	newZoom := c.zoom * factor
	if newZoom < 0.01 {
		newZoom = 0.01
	}
	if min := c.minZoom(); newZoom < min {
		newZoom = min
	}
	if c.maxZoom > 0 && newZoom > c.maxZoom {
		newZoom = c.maxZoom
	}
	c.setZoom(newZoom)

	afterX, afterY := c.ToScreen(anchorX, anchorY)
	px := c.px()
	dx := (afterX - beforeX) / px
	dy := (afterY - beforeY) / px
	if dx != 0 || dy != 0 {
		c.Translate(dx, dy)
	}
}

func (c *basicCamera) setZoom(zoom float32) {
	cx := c.effective.TopLeft.X + c.effective.Size.X/2
	cy := c.effective.TopLeft.Y + c.effective.Size.Y/2

	w := c.viewportSize.X / zoom
	h := c.viewportSize.Y / zoom

	c.zoom = zoom
	c.place(cx-w/2, cy-h/2, w, h)
}

// Fits reports whether the camera keeps the whole world in view, and the world's size.
func (c *basicCamera) Fits() (float32, float32, bool) {
	return c.world.X, c.world.Y, c.whole
}

func (c *basicCamera) fit() {
	c.zoom = min(c.viewportSize.X/c.world.X, c.viewportSize.Y/c.world.Y)
	// the world whole, rounding notwithstanding
	w := max(c.viewportSize.X/c.zoom, c.world.X)
	h := max(c.viewportSize.Y/c.zoom, c.world.Y)
	c.effective = plane.NewAABB(geom.NewVec((c.world.X-w)/2, (c.world.Y-h)/2), w, h)
}

func (c *basicCamera) minZoom() float32 {
	byW := c.viewportSize.X / c.world.X
	byH := c.viewportSize.Y / c.world.Y
	floor := byW
	if byH > floor {
		floor = byH
	}
	if c.minZoomCfg > floor {
		floor = c.minZoomCfg
	}
	return floor
}

// SetMinZoom raises ZoomOut's floor above the world-fit one.
func (c *basicCamera) SetMinZoom(minZoom float32) { c.minZoomCfg = minZoom }

// SetMaxZoom caps ZoomIn; 0 leaves it unrestricted.
func (c *basicCamera) SetMaxZoom(maxZoom float32) { c.maxZoom = maxZoom }

// ZoomOut is ZoomIn(1/factor, anchorX, anchorY).
func (c *basicCamera) ZoomOut(factor float32, anchorX, anchorY float32) {
	c.ZoomIn(1/factor, anchorX, anchorY)
}

// State is the camera's current window and zoom.
func (c *basicCamera) State() contract.State {
	return contract.State{Viewport: c.effective.AABB, Zoom: c.zoom}
}

// Persisted is pointers to the live window, zoom, fastening and sight, for saves and loads, after
// the camera's form.
func (c *basicCamera) Persisted() []any {
	return []any{&c.form, &c.effective.AABB, &c.zoom, &c.fastening, &c.detail.Sight}
}

// Restore rebuilds derived state after a load has written through Persisted's pointers.
func (c *basicCamera) Restore() {
	w := c.effective.BottomRight.X - c.effective.TopLeft.X
	h := c.effective.BottomRight.Y - c.effective.TopLeft.Y
	c.effective = plane.NewAABB(c.effective.TopLeft, w, h)
	if c.whole && c.viewportSize.X > 0 && c.viewportSize.Y > 0 {
		c.fit()
	}
}

// Rays are the camera's lines of sight: straight down onto the ground, a camera pixel 1/zoom
// world units.
func (c *basicCamera) Rays() (contract.RayField, bool) {
	k := 1 / c.px()
	return contract.RayField{
		Origin: [3]float32{c.effective.TopLeft.X, c.effective.TopLeft.Y, basicEye},
		DX:     [3]float32{k, 0, 0}, DY: [3]float32{0, k, 0}, Dir: [3]float32{0, 0, -1},
	}, true
}

// over anything a flat world holds
const basicEye = 1 << 16
