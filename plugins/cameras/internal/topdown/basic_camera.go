package topdown

import (
	"fmt"
	"math"
	"slices"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	contract "github.com/kjkrol/gram/camera"
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
}

type frame struct {
	viewportSize geom.Vec
	res          float32
	effective    plane.AABB
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

// Detail is how far the camera sees and how finely.
func (c *basicCamera) Detail() contract.Detail { return c.detail }

// SetDetail has the camera see as d says.
func (c *basicCamera) SetDetail(d contract.Detail) { c.detail = d }

// See has the camera see factor times as far.
func (c *basicCamera) See(factor float64) { c.detail = c.detail.Widened(factor) }

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
		vp = geom.NewAABBAt(geom.NewVec(0, 0), float64(width), float64(height))
	}
	return newBasicCamera(geom.NewVec(float64(width), float64(height)), vp, edges)
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
	rel := float32(math.Mod(float64(v-ref), float64(size)))
	if rel < 0 {
		rel += size
	}
	return ref + rel
}

func wrapMod(v, size float32) float32 {
	v = float32(math.Mod(float64(v), float64(size)))
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

// Crop has the camera drawn the central w by h shown pixels of its frame at s of them, the
// zoom as it is; a camera keeping the whole world in view is drawn whole.
func (c *basicCamera) Crop(w, h, s float32) {
	if c.whole || w <= 0 || h <= 0 {
		return
	}
	c.Uncrop()
	c.cropped = &frame{viewportSize: c.viewportSize, res: c.res, effective: c.effective}
	cx := c.effective.TopLeft.X + c.effective.Size.X/2
	cy := c.effective.TopLeft.Y + c.effective.Size.Y/2
	vw, vh := float64(w)/float64(c.zoom), float64(h)/float64(c.zoom)
	c.viewportSize = geom.NewVec(float64(w), float64(h))
	c.res = s
	c.effective = plane.NewAABB(geom.NewVec(cx-vw/2, cy-vh/2), vw, vh)
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
	return float32(size.X), float32(size.Y)
}

// camera pixels a world unit
func (c *basicCamera) px() float32 { return c.zoom * c.res }

func (c *basicCamera) Viewport() (float32, float32) {
	return float32(c.viewportSize.X) * c.res, float32(c.viewportSize.Y) * c.res
}

func (c *basicCamera) SetViewport(w, h float32) {
	if w <= 0 || h <= 0 {
		return
	}
	if c.whole {
		c.viewportSize = geom.NewVec(float64(w), float64(h))
		c.fit()
		return
	}
	cx := c.effective.TopLeft.X + c.effective.Size.X/2
	cy := c.effective.TopLeft.Y + c.effective.Size.Y/2
	c.viewportSize = geom.NewVec(float64(w), float64(h))
	zoom := max(c.zoom, c.minZoom())
	if c.maxZoom > 0 {
		zoom = min(zoom, max(c.maxZoom, c.minZoom()))
	}
	c.zoom = zoom
	vw, vh := c.viewportSize.X/float64(zoom), c.viewportSize.Y/float64(zoom)
	c.place(cx-vw/2, cy-vh/2, vw, vh)
}

func (c *basicCamera) Project(x, y, _ float32) (float32, float32) { return c.ToScreen(x, y) }

func (c *basicCamera) Unproject(sx, sy, _ float32) (float32, float32) { return c.FromScreen(sx, sy) }

func (c *basicCamera) Depth(_, y, _ float32) float32 { return y }

func (c *basicCamera) ToScreen(x, y float32) (float32, float32) {
	if c.edges.WrapsX() {
		x = wrapRelative(x, float32(c.effective.TopLeft.X), float32(c.world.X))
	}
	if c.edges.WrapsY() {
		y = wrapRelative(y, float32(c.effective.TopLeft.Y), float32(c.world.Y))
	}
	px := c.px()
	return (x - float32(c.effective.TopLeft.X)) * px, (y - float32(c.effective.TopLeft.Y)) * px
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
	for _, xp := range axisPieces(u0, u1, float32(c.world.X), c.edges.WrapsX(), &xs) {
		for _, yp := range axisPieces(v0, v1, float32(c.world.Y), c.edges.WrapsY(), &ys) {
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
	ref := float32(c.effective.TopLeft.X)
	if !c.edges.WrapsX() {
		return x - ref
	}
	return windowOffset(x, ref, float32(c.world.X), float32(c.effective.Size.X))
}

func (c *basicCamera) offsetY(y float32) float32 {
	ref := float32(c.effective.TopLeft.Y)
	if !c.edges.WrapsY() {
		return y - ref
	}
	return windowOffset(y, ref, float32(c.world.Y), float32(c.effective.Size.Y))
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
	x := sx/px + float32(c.effective.TopLeft.X)
	y := sy/px + float32(c.effective.TopLeft.Y)
	if c.edges.WrapsX() {
		x = wrapMod(x, float32(c.world.X))
	}
	if c.edges.WrapsY() {
		y = wrapMod(y, float32(c.world.Y))
	}
	return x, y
}

// Visible is false for a box only touching the window's edge.
func (c *basicCamera) Visible(box contract.AABB) bool {
	tlX, tlY := float32(box.TopLeft.X), float32(box.TopLeft.Y)
	brX, brY := float32(box.BottomRight.X), float32(box.BottomRight.Y)
	if c.edges.WrapsX() {
		width := brX - tlX
		tlX = float32(c.effective.TopLeft.X) + c.offsetX(tlX)
		brX = tlX + width
	}
	if c.edges.WrapsY() {
		height := brY - tlY
		tlY = float32(c.effective.TopLeft.Y) + c.offsetY(tlY)
		brY = tlY + height
	}
	far := c.Bounds()
	return brX > float32(c.effective.TopLeft.X) && tlX < float32(far.BottomRight.X) &&
		brY > float32(c.effective.TopLeft.Y) && tlY < float32(far.BottomRight.Y)
}

// Focus is the middle of the window, at the ground.
func (c *basicCamera) Focus() (float32, float32, float32) {
	return float32(c.effective.TopLeft.X + c.effective.Size.X/2),
		float32(c.effective.TopLeft.Y + c.effective.Size.Y/2), 0
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
func (c *basicCamera) CenterOn(x, y, _ float64) {
	w, h := c.effective.Size.X, c.effective.Size.Y
	c.place(x-w/2, y-h/2, w, h)
}

// MoveTo puts the visible window's top-left corner at (x, y), keeping its size.
func (c *basicCamera) MoveTo(x, y float64) {
	c.Translate(x-c.effective.TopLeft.X, y-c.effective.TopLeft.Y)
}

// Translate shifts the visible window by a delta in world units.
func (c *basicCamera) Translate(dx, dy float64) {
	c.place(c.effective.TopLeft.X+dx, c.effective.TopLeft.Y+dy, c.effective.Size.X, c.effective.Size.Y)
}

// Pan shifts the visible window by a delta in camera pixels.
func (c *basicCamera) Pan(dx, dy float32) {
	px := float64(c.px())
	c.Translate(float64(dx)/px, float64(dy)/px)
}

func (c *basicCamera) place(x, y, w, h float64) {
	if c.whole {
		return
	}
	x = fitAxis(x, w, c.world.X, c.edges.WrapsX())
	y = fitAxis(y, h, c.world.Y, c.edges.WrapsY())
	c.effective = plane.NewAABB(geom.NewVec(x, y), w, h)
}

func fitAxis(lo, length, world float64, wraps bool) float64 {
	if !wraps {
		return min(max(lo, 0), max(world-length, 0))
	}
	lo = math.Mod(lo, world)
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
	dx := float64((afterX - beforeX) / px)
	dy := float64((afterY - beforeY) / px)
	if dx != 0 || dy != 0 {
		c.Translate(dx, dy)
	}
}

func (c *basicCamera) setZoom(zoom float32) {
	cx := c.effective.TopLeft.X + c.effective.Size.X/2
	cy := c.effective.TopLeft.Y + c.effective.Size.Y/2

	w := c.viewportSize.X / float64(zoom)
	h := c.viewportSize.Y / float64(zoom)

	c.zoom = zoom
	c.place(cx-w/2, cy-h/2, w, h)
}

// Fits reports whether the camera keeps the whole world in view, and the world's size.
func (c *basicCamera) Fits() (float32, float32, bool) {
	return float32(c.world.X), float32(c.world.Y), c.whole
}

func (c *basicCamera) fit() {
	c.zoom = float32(min(c.viewportSize.X/c.world.X, c.viewportSize.Y/c.world.Y))
	// the world whole, rounding notwithstanding
	w := max(c.viewportSize.X/float64(c.zoom), c.world.X)
	h := max(c.viewportSize.Y/float64(c.zoom), c.world.Y)
	c.effective = plane.NewAABB(geom.NewVec((c.world.X-w)/2, (c.world.Y-h)/2), w, h)
}

func (c *basicCamera) minZoom() float32 {
	byW := float32(c.viewportSize.X) / float32(c.world.X)
	byH := float32(c.viewportSize.Y) / float32(c.world.Y)
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
		Origin: [3]float32{float32(c.effective.TopLeft.X), float32(c.effective.TopLeft.Y), basicEye},
		DX:     [3]float32{k, 0, 0}, DY: [3]float32{0, k, 0}, Dir: [3]float32{0, 0, -1},
	}, true
}

// over anything a flat world holds
const basicEye = 1 << 16
