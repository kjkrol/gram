package camera

import (
	"math"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	contract "github.com/kjkrol/gram/camera"
)

// basicCamera is the top-down Camera — construct via NewFromSpace.
type basicCamera struct {
	world        geom.Vec
	viewportSize geom.Vec // fixed size at zoom 1 (e.g. screen size)
	edges        aabbworld.Edges
	minZoomCfg   float32 // 0 = only the automatic world-fit floor applies
	maxZoom      float32 // 0 = unrestricted
	zoom         float32

	effective plane.AABB // the current visible window — always valid and clamped to the world
}

var _ contract.Camera = (*basicCamera)(nil)

func newBasicCamera(world geom.Vec, viewport contract.AABB, edges aabbworld.Edges) *basicCamera {
	w := viewport.BottomRight.X - viewport.TopLeft.X
	h := viewport.BottomRight.Y - viewport.TopLeft.Y
	return &basicCamera{
		world:        world,
		viewportSize: geom.NewVec(w, h),
		edges:        edges,
		zoom:         1,
		effective:    plane.NewAABB(viewport.TopLeft, w, h),
	}
}

// NewFromSpace builds a camera.Camera over a width x height world, viewing all of it by default.
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

// NewFromSpaceWithConfig is NewFromSpace with cfg's viewport size and zoom limits.
func NewFromSpaceWithConfig(width, height uint32, edges aabbworld.Edges, cfg contract.Config) contract.Camera {
	var viewport []contract.AABB
	if cfg.ViewportWidth != 0 && cfg.ViewportHeight != 0 {
		viewport = []contract.AABB{geom.NewAABBAt(geom.NewVec(0, 0), float64(cfg.ViewportWidth), float64(cfg.ViewportHeight))}
	}
	return limited(NewFromSpace(width, height, edges, viewport...), cfg)
}

// limited applies cfg's zoom limits to cam.
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

func (c *basicCamera) Viewport() (float32, float32) {
	return float32(c.viewportSize.X), float32(c.viewportSize.Y)
}

func (c *basicCamera) SetViewport(w, h float32) {
	if w <= 0 || h <= 0 {
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

// Project is ToScreen: a top-down view draws no height.
func (c *basicCamera) Project(x, y, _ float32) (float32, float32) { return c.ToScreen(x, y) }

// Unproject is FromScreen at any height.
func (c *basicCamera) Unproject(sx, sy, _ float32) (float32, float32) { return c.FromScreen(sx, sy) }

// Depth is the world y: further up the screen is drawn first.
func (c *basicCamera) Depth(_, y, _ float32) float32 { return y }

func (c *basicCamera) ToScreen(x, y float32) (float32, float32) {
	if c.edges.WrapsX() {
		x = wrapRelative(x, float32(c.effective.TopLeft.X), float32(c.world.X))
	}
	if c.edges.WrapsY() {
		y = wrapRelative(y, float32(c.effective.TopLeft.Y), float32(c.world.Y))
	}
	return (x - float32(c.effective.TopLeft.X)) * c.zoom, (y - float32(c.effective.TopLeft.Y)) * c.zoom
}

func (c *basicCamera) ToScreenQuads(x0, y0, x1, y1 float32, dst []contract.Quad) []contract.Quad {
	if c.edges&aabbworld.Torus == 0 {
		sx0, sy0 := c.ToScreen(x0, y0)
		return append(dst, contract.Quad{X0: sx0, Y0: sy0, X1: sx0 + (x1-x0)*c.zoom, Y1: sy0 + (y1-y0)*c.zoom, T0X: 0, T1X: 1, T0Y: 0, T1Y: 1})
	}
	u0 := c.offsetX(x0)
	u1 := u0 + (x1 - x0)
	v0 := c.offsetY(y0)
	v1 := v0 + (y1 - y0)

	var xs, ys [2]rangePiece
	for _, xp := range axisPieces(u0, u1, float32(c.world.X), c.edges.WrapsX(), &xs) {
		for _, yp := range axisPieces(v0, v1, float32(c.world.Y), c.edges.WrapsY(), &ys) {
			sx0 := xp.screenLo * c.zoom
			sx1 := sx0 + (xp.hi-xp.lo)*c.zoom
			sy0 := yp.screenLo * c.zoom
			sy1 := sy0 + (yp.hi-yp.lo)*c.zoom
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

// offsetX is how far right of the window's left edge x lies, the short way round if X wraps.
func (c *basicCamera) offsetX(x float32) float32 {
	ref := float32(c.effective.TopLeft.X)
	if !c.edges.WrapsX() {
		return x - ref
	}
	return windowOffset(x, ref, float32(c.world.X), float32(c.effective.Size.X))
}

// offsetY is how far below the window's top edge y lies, the short way round if Y wraps.
func (c *basicCamera) offsetY(y float32) float32 {
	ref := float32(c.effective.TopLeft.Y)
	if !c.edges.WrapsY() {
		return y - ref
	}
	return windowOffset(y, ref, float32(c.world.Y), float32(c.effective.Size.Y))
}

// axisPieces fills buf with the one or two pieces [u0, u1] falls into along an axis.
func axisPieces(u0, u1, size float32, wraps bool, buf *[2]rangePiece) []rangePiece {
	if wraps && u1 > size {
		buf[0], buf[1] = rangePiece{u0, size, u0}, rangePiece{size, u1, 0}
		return buf[:2]
	}
	buf[0] = rangePiece{u0, u1, u0}
	return buf[:1]
}

func (c *basicCamera) FromScreen(sx, sy float32) (float32, float32) {
	x := sx/c.zoom + float32(c.effective.TopLeft.X)
	y := sy/c.zoom + float32(c.effective.TopLeft.Y)
	if c.edges.WrapsX() {
		x = wrapMod(x, float32(c.world.X))
	}
	if c.edges.WrapsY() {
		y = wrapMod(y, float32(c.world.Y))
	}
	return x, y
}

// Visible uses strict >/< — touching edges are not visible.
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

// Bounds returns the visible extent, which on a torus may run past the world's own size.
func (c *basicCamera) Bounds() contract.AABB {
	return contract.AABB{
		TopLeft: c.effective.TopLeft,
		BottomRight: geom.NewVec(
			c.effective.TopLeft.X+c.effective.Size.X,
			c.effective.TopLeft.Y+c.effective.Size.Y,
		),
	}
}

// CenterOn puts (x, y) in the middle of the window, clamped or wrapped like any move.
func (c *basicCamera) CenterOn(x, y, _ float64) {
	w, h := c.effective.Size.X, c.effective.Size.Y
	c.place(x-w/2, y-h/2, w, h)
}

// MoveTo repositions the visible window's top-left corner, keeping size.
func (c *basicCamera) MoveTo(x, y float64) {
	c.Translate(x-c.effective.TopLeft.X, y-c.effective.TopLeft.Y)
}

// Translate shifts the visible window by a signed delta, clamped or wrapped against the world.
func (c *basicCamera) Translate(dx, dy float64) {
	c.place(c.effective.TopLeft.X+dx, c.effective.TopLeft.Y+dy, c.effective.Size.X, c.effective.Size.Y)
}

// Pan is Translate by a screen-space delta, so a drag follows the cursor at any zoom.
func (c *basicCamera) Pan(dx, dy float32) {
	c.Translate(float64(dx)/float64(c.zoom), float64(dy)/float64(c.zoom))
}

// place puts a w x h window at (x, y): wrapped on a wrapping axis, held inside the world otherwise.
func (c *basicCamera) place(x, y, w, h float64) {
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

// Zoom returns the current zoom factor (1 = default).
func (c *basicCamera) Zoom() float32 { return c.zoom }

// ZoomIn multiplies the zoom by factor, keeping world point (anchorX, anchorY) fixed on screen.
func (c *basicCamera) ZoomIn(factor float32, anchorX, anchorY float32) {
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
	dx := float64((afterX - beforeX) / c.zoom)
	dy := float64((afterY - beforeY) / c.zoom)
	if dx != 0 || dy != 0 {
		c.Translate(dx, dy)
	}
}

// setZoom resizes the visible window for zoom around its centre and fits it back into the world.
func (c *basicCamera) setZoom(zoom float32) {
	cx := c.effective.TopLeft.X + c.effective.Size.X/2
	cy := c.effective.TopLeft.Y + c.effective.Size.Y/2

	w := c.viewportSize.X / float64(zoom)
	h := c.viewportSize.Y / float64(zoom)

	c.zoom = zoom
	c.place(cx-w/2, cy-h/2, w, h)
}

// minZoom is the larger of the automatic world-fit floor and any SetMinZoom override.
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

// SetMinZoom raises ZoomOut's floor above the automatic world-fit one.
func (c *basicCamera) SetMinZoom(minZoom float32) { c.minZoomCfg = minZoom }

// SetMaxZoom caps ZoomIn — 0 (the default) leaves zoom-in unrestricted.
func (c *basicCamera) SetMaxZoom(maxZoom float32) { c.maxZoom = maxZoom }

// ZoomOut is ZoomIn(1/factor, anchorX, anchorY).
func (c *basicCamera) ZoomOut(factor float32, anchorX, anchorY float32) {
	c.ZoomIn(1/factor, anchorX, anchorY)
}

// camera.State returns the camera's current Viewport/Zoom.
func (c *basicCamera) State() contract.State {
	return contract.State{Viewport: c.effective.AABB, Zoom: c.zoom}
}

// Persisted returns pointers to the live Viewport and Zoom for Persistence to save and load.
func (c *basicCamera) Persisted() []any {
	return []any{&c.effective.AABB, &c.zoom}
}

// Restore rebuilds derived state after a Load has written through Persisted's pointers.
func (c *basicCamera) Restore() {
	w := c.effective.BottomRight.X - c.effective.TopLeft.X
	h := c.effective.BottomRight.Y - c.effective.TopLeft.Y
	c.effective = plane.NewAABB(c.effective.TopLeft, w, h)
}

// Rays are the camera's lines of sight: straight down onto the ground from over the window's
// top-left corner, a screen pixel 1/zoom world units — for drawing on the GPU (camera.TransformOf).
func (c *basicCamera) Rays() (contract.RayField, bool) {
	k := 1 / c.zoom
	return contract.RayField{
		Origin: [3]float32{float32(c.effective.TopLeft.X), float32(c.effective.TopLeft.Y), basicEye},
		DX:     [3]float32{k, 0, 0}, DY: [3]float32{0, k, 0}, Dir: [3]float32{0, 0, -1},
	}, true
}

// basicEye is how high over the ground a top-down camera's rays start: over anything a flat world
// holds.
const basicEye = 1 << 16
