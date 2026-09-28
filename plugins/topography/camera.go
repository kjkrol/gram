package topography

import (
	contract "github.com/kjkrol/gram/camera"
	"math"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
)

// isoCamera is a camera.Camera through a projection — isometric, or flat from above — switched at
// play: the world never wraps, the window is a screen rectangle over the projected world, and the
// visible world region is what lies under it.
type isoCamera struct {
	proj         projection
	projection   contract.Projection // proj boxed once, so asking for it allocates nothing
	world        geom.Vec
	viewportSize geom.Vec
	minZoomCfg   float32
	maxZoom      float32
	zoom         float32
	// pan is where the projected world origin lands on the screen, in pixels.
	pan geom.Vec
	// origin is the ground point under the screen's top-left corner, heading the view's turn and
	// pitch how steeply it looks down — what a save keeps.
	origin  geom.Vec
	heading float32
	pitch   float32
	flat    bool

	// the projected world's extent at zoom 1 and height 0, for fitting and clamping
	minSX, maxSX, minSY, maxSY float32
	// extent is the lowest and the highest ground, for what the screen may show; nil, level at 0
	extent func() (low, high float32)
	// ground is the top of the ground as it is drawn, for Pick; nil, level at 0
	ground func(x, y float32) float32
}

var _ contract.Camera = (*isoCamera)(nil)

// newIsoCamera is a camera over world drawn through proj to viewport; it refuses a wrapping world.
func newIsoCamera(proj projection, world geom.Vec, viewport contract.AABB, edges aabbworld.Edges) *isoCamera {
	if edges.WrapsX() || edges.WrapsY() {
		panic("topography: a world that wraps cannot be seen in relief")
	}
	proj = proj.withDefaults()
	c := &isoCamera{world: world, zoom: 1,
		viewportSize: geom.NewVec(viewport.BottomRight.X-viewport.TopLeft.X, viewport.BottomRight.Y-viewport.TopLeft.Y)}
	c.look(proj)
	c.MoveTo(viewport.TopLeft.X, viewport.TopLeft.Y)
	return c
}

// Isometric reports whether the camera looks isometrically rather than from above.
func (c *isoCamera) Isometric() bool { return !c.proj.flat }

// SetIsometric has the camera look isometrically, or from above, keeping the ground point in the
// middle of the screen where it is and a cell as wide on the screen as it was.
func (c *isoCamera) SetIsometric(iso bool) {
	if iso == c.Isometric() {
		return
	}
	x, y := c.Unproject(float32(c.viewportSize.X/2), float32(c.viewportSize.Y/2), 0)
	if iso {
		c.zoom *= c.proj.Cell / c.proj.TileW
	} else {
		c.zoom *= c.proj.TileW / c.proj.Cell
	}
	c.look(c.proj.viewed(!iso))
	c.zoom = max(c.zoom, c.minZoom())
	if c.maxZoom > 0 {
		c.zoom = min(c.zoom, max(c.maxZoom, c.minZoom()))
	}
	c.CenterOn(float64(x), float64(y), 0)
}

// look draws through proj from now on, and measures the projected world anew.
func (c *isoCamera) look(proj projection) {
	c.proj, c.projection, c.heading, c.pitch, c.flat = proj, proj, proj.Heading, proj.Pitch, proj.flat
	c.minSX, c.maxSX, c.minSY, c.maxSY = float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.Inf(1)), float32(math.Inf(-1))
	w, h := float32(c.world.X), float32(c.world.Y)
	for _, corner := range [4][2]float32{{0, 0}, {w, 0}, {0, h}, {w, h}} {
		sx, sy := proj.Project(corner[0], corner[1], 0)
		c.minSX, c.maxSX = min(c.minSX, sx), max(c.maxSX, sx)
		c.minSY, c.maxSY = min(c.minSY, sy), max(c.maxSY, sy)
	}
}

// Turn turns the view by angle radians, the world clockwise on the screen, keeping the ground point
// in the middle of the screen where it is; a view from above does not turn.
func (c *isoCamera) Turn(angle float32) {
	if c.proj.flat {
		return
	}
	x, y := c.Unproject(float32(c.viewportSize.X/2), float32(c.viewportSize.Y/2), 0)
	c.look(c.proj.turned(c.proj.Heading + angle))
	c.zoom = max(c.zoom, c.minZoom())
	c.CenterOn(float64(x), float64(y), 0)
}

func (c *isoCamera) Heading() float32 { return c.proj.Heading }

// Tilt looks down by angle radians more steeply, less for a negative one, between the flattest and
// straight down, keeping the ground point in the middle of the screen where it is; a view from
// above looks straight down already.
func (c *isoCamera) Tilt(angle float32) {
	if c.proj.flat {
		return
	}
	x, y := c.Unproject(float32(c.viewportSize.X/2), float32(c.viewportSize.Y/2), 0)
	c.look(c.proj.tilted(c.proj.Pitch + angle))
	c.zoom = max(c.zoom, c.minZoom())
	c.CenterOn(float64(x), float64(y), 0)
}

func (c *isoCamera) Pitch() float32 { return c.proj.Pitch }

func (c *isoCamera) Projection() contract.Projection { return c.projection }

func (c *isoCamera) Viewport() (float32, float32) {
	return float32(c.viewportSize.X), float32(c.viewportSize.Y)
}

func (c *isoCamera) SetViewport(w, h float32) {
	if w <= 0 || h <= 0 {
		return
	}
	x, y := c.Unproject(float32(c.viewportSize.X/2), float32(c.viewportSize.Y/2), 0)
	c.viewportSize = geom.NewVec(float64(w), float64(h))
	c.zoom = max(c.zoom, c.minZoom())
	c.CenterOn(float64(x), float64(y), 0)
}

func (c *isoCamera) Project(x, y, z float32) (float32, float32) {
	sx, sy := c.proj.Project(x, y, z)
	return sx*c.zoom + float32(c.pan.X), sy*c.zoom + float32(c.pan.Y)
}

func (c *isoCamera) Unproject(sx, sy, z float32) (float32, float32) {
	return c.proj.Unproject((sx-float32(c.pan.X))/c.zoom, (sy-float32(c.pan.Y))/c.zoom, z)
}

// Pick is the first ground the screen point sees, down its line from over the highest top.
func (c *isoCamera) Pick(sx, sy float32) (float32, float32, bool) {
	if c.ground == nil {
		x, y := c.Unproject(sx, sy, 0)
		return x, y, true
	}
	low, high := c.layer()
	x0, y0 := c.Unproject(sx, sy, high)
	x1, y1 := c.Unproject(sx, sy, high-1)
	s := sight{o: [3]float32{x0, y0, high}, d: [3]float32{x1 - x0, y1 - y0, -1}}
	return s.pick(c.ground, high-low+1, c.proj.Cell/2, high)
}

func (c *isoCamera) Depth(x, y, z float32) float32 { return c.proj.Depth(x, y, z) }

func (c *isoCamera) ToScreen(x, y float32) (float32, float32)     { return c.Project(x, y, 0) }
func (c *isoCamera) FromScreen(sx, sy float32) (float32, float32) { return c.Unproject(sx, sy, 0) }

// ToScreenQuads is the screen rectangle round the projected world rectangle — a bound, since a
// rectangle projects to a diamond; renderers drawing through this camera project corners instead.
func (c *isoCamera) ToScreenQuads(x0, y0, x1, y1 float32, dst []contract.Quad) []contract.Quad {
	minX, minY, maxX, maxY := c.rect(x0, y0, x1, y1, 0, 0)
	return append(dst, contract.Quad{X0: minX, Y0: minY, X1: maxX, Y1: maxY, T0X: 0, T1X: 1, T0Y: 0, T1Y: 1})
}

// rect is the screen rectangle round a world box drawn between heights z0 and z1.
func (c *isoCamera) rect(x0, y0, x1, y1, z0, z1 float32) (minX, minY, maxX, maxY float32) {
	minX, minY = float32(math.Inf(1)), float32(math.Inf(1))
	maxX, maxY = float32(math.Inf(-1)), float32(math.Inf(-1))
	for _, z := range [2]float32{z0, z1} {
		for _, corner := range [4][2]float32{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} {
			sx, sy := c.Project(corner[0], corner[1], z)
			minX, maxX = min(minX, sx), max(maxX, sx)
			minY, maxY = min(minY, sy), max(maxY, sy)
		}
	}
	return
}

// layer is the heights between which anything may be drawn: the lowest ground, sea level at most,
// up to the projection's Headroom over the highest.
func (c *isoCamera) layer() (low, high float32) {
	if c.extent != nil {
		low, high = c.extent()
	}
	return low, high + c.proj.Headroom
}

// Visible reports whether the box, drawn anywhere from the lowest ground up to Headroom over the
// highest, meets the screen.
func (c *isoCamera) Visible(box contract.AABB) bool {
	low, high := c.layer()
	minX, minY, maxX, maxY := c.rect(float32(box.TopLeft.X), float32(box.TopLeft.Y), float32(box.BottomRight.X), float32(box.BottomRight.Y), low, high)
	return maxX > 0 && minX < float32(c.viewportSize.X) && maxY > 0 && minY < float32(c.viewportSize.Y)
}

// Bounds is the world rectangle round the ground the screen shows, anywhere from the lowest
// ground up to Headroom over the highest — high ground from beyond the screen's lower edge at sea
// level is drawn on it — clamped to the world.
func (c *isoCamera) Bounds() contract.AABB {
	low, high := c.layer()
	minX, minY := float32(math.Inf(1)), float32(math.Inf(1))
	maxX, maxY := float32(math.Inf(-1)), float32(math.Inf(-1))
	w, h := float32(c.viewportSize.X), float32(c.viewportSize.Y)
	for _, z := range [2]float32{low, high} {
		for _, corner := range [4][2]float32{{0, 0}, {w, 0}, {0, h}, {w, h}} {
			x, y := c.Unproject(corner[0], corner[1], z)
			minX, maxX = min(minX, x), max(maxX, x)
			minY, maxY = min(minY, y), max(maxY, y)
		}
	}
	return worldBox(minX, minY, maxX, maxY, c.world)
}

// MoveTo puts the ground point (x, y) at the screen's top-left corner.
func (c *isoCamera) MoveTo(x, y float64) {
	sx, sy := c.proj.Project(float32(x), float32(y), 0)
	c.place(geom.NewVec(-float64(sx)*float64(c.zoom), -float64(sy)*float64(c.zoom)))
}

// CenterOn pans so the point (x, y) at height z is drawn in the middle of the screen, as far as
// the window may go over the projected world.
func (c *isoCamera) CenterOn(x, y, z float64) {
	sx, sy := c.proj.Project(float32(x), float32(y), float32(z))
	z0 := float64(c.zoom)
	c.place(geom.NewVec(c.viewportSize.X/2-float64(sx)*z0, c.viewportSize.Y/2-float64(sy)*z0))
}

// Translate shifts the window by a world delta along the ground.
func (c *isoCamera) Translate(dx, dy float64) {
	sx, sy := c.proj.Project(float32(dx), float32(dy), 0)
	c.place(geom.NewVec(c.pan.X-float64(sx)*float64(c.zoom), c.pan.Y-float64(sy)*float64(c.zoom)))
}

// Pan shifts the window by a screen delta: the same pixels at any zoom.
func (c *isoCamera) Pan(dx, dy float32) {
	c.place(geom.NewVec(c.pan.X-float64(dx), c.pan.Y-float64(dy)))
}

// place sets the pan, keeping the screen's centre over the projected world.
func (c *isoCamera) place(pan geom.Vec) {
	z := float64(c.zoom)
	cx, cy := c.viewportSize.X/2, c.viewportSize.Y/2
	pan.X = min(max(pan.X, cx-float64(c.maxSX)*z), cx-float64(c.minSX)*z)
	pan.Y = min(max(pan.Y, cy-float64(c.maxSY)*z), cy-float64(c.minSY)*z)
	c.pan = pan
	x, y := c.Unproject(0, 0, 0)
	c.origin = geom.NewVec(float64(x), float64(y))
}

func (c *isoCamera) Zoom() float32 { return c.zoom }

// scale is how many screen units a world unit spans across the screen: the zoom from above, a
// cell's diamond's width over its side isometrically.
func (c *isoCamera) scale() float32 {
	if c.proj.flat {
		return c.zoom
	}
	return c.zoom * c.proj.TileW / (math.Sqrt2 * c.proj.Cell)
}

// ZoomIn multiplies the zoom by factor, keeping the ground point (anchorX, anchorY) where it is.
func (c *isoCamera) ZoomIn(factor float32, anchorX, anchorY float32) {
	beforeX, beforeY := c.Project(anchorX, anchorY, 0)
	zoom := max(c.zoom*factor, 0.01)
	if floor := c.minZoom(); zoom < floor {
		zoom = floor
	}
	if c.maxZoom > 0 && zoom > c.maxZoom {
		zoom = c.maxZoom
	}
	c.zoom = zoom
	afterX, afterY := c.Project(anchorX, anchorY, 0)
	c.place(geom.NewVec(c.pan.X+float64(beforeX-afterX), c.pan.Y+float64(beforeY-afterY)))
}

func (c *isoCamera) ZoomOut(factor float32, anchorX, anchorY float32) {
	c.ZoomIn(1/factor, anchorX, anchorY)
}

// minZoom fits the projected world's diamond into the viewport, or the configured floor if higher.
func (c *isoCamera) minZoom() float32 {
	floor := min(float32(c.viewportSize.X)/(c.maxSX-c.minSX), float32(c.viewportSize.Y)/(c.maxSY-c.minSY))
	return max(floor, c.minZoomCfg)
}

func (c *isoCamera) SetMinZoom(minZoom float32) { c.minZoomCfg = minZoom }
func (c *isoCamera) SetMaxZoom(maxZoom float32) { c.maxZoom = maxZoom }

func (c *isoCamera) State() contract.State { return contract.State{Viewport: c.Bounds(), Zoom: c.zoom} }

// Persisted hands saves the ground point under the screen's corner, the zoom, the heading, the
// pitch and the view; Restore puts the window back over them.
func (c *isoCamera) Persisted() []any {
	return []any{&c.origin, &c.zoom, &c.heading, &c.pitch, &c.flat}
}

func (c *isoCamera) Restore() {
	c.look(c.proj.turned(c.heading).tilted(c.pitch).viewed(c.flat))
	c.zoom = max(c.zoom, 0.01)
	c.MoveTo(c.origin.X, c.origin.Y)
}
