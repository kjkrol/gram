package camera

import (
	contract "github.com/kjkrol/gram/camera"
	"math"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
)

// isoCamera is a camera.Camera over an camera.Isometric projection: the world never wraps, the window is a
// screen rectangle over the projected world, and the visible world region is the diamond under it.
type isoCamera struct {
	proj         contract.Isometric
	world        geom.Vec
	viewportSize geom.Vec
	minZoomCfg   float32
	maxZoom      float32
	zoom         float32
	// pan is where the projected world origin lands on the screen, in pixels.
	pan geom.Vec
	// origin is the ground point under the screen's top-left corner — what a save keeps.
	origin geom.Vec

	// the projected world's extent at zoom 1 and height 0, for fitting and clamping
	minSX, maxSX, minSY, maxSY float32
}

var _ contract.Camera = (*isoCamera)(nil)

func newIsoCamera(proj contract.Isometric, world geom.Vec, viewport contract.AABB, edges aabbworld.Edges) *isoCamera {
	if edges.WrapsX() || edges.WrapsY() {
		panic("camera: an isometric projection cannot draw a wrapping world")
	}
	proj = proj.WithDefaults()
	c := &isoCamera{proj: proj, world: world, zoom: 1,
		viewportSize: geom.NewVec(viewport.BottomRight.X-viewport.TopLeft.X, viewport.BottomRight.Y-viewport.TopLeft.Y)}
	c.minSX, c.maxSX, c.minSY, c.maxSY = float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.Inf(1)), float32(math.Inf(-1))
	for _, corner := range [4][2]float32{{0, 0}, {float32(world.X), 0}, {0, float32(world.Y)}, {float32(world.X), float32(world.Y)}} {
		sx, sy := proj.Project(corner[0], corner[1], 0)
		c.minSX, c.maxSX = min(c.minSX, sx), max(c.maxSX, sx)
		c.minSY, c.maxSY = min(c.minSY, sy), max(c.maxSY, sy)
	}
	c.MoveTo(viewport.TopLeft.X, viewport.TopLeft.Y)
	return c
}

func (c *isoCamera) Projection() contract.Projection { return c.proj }

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

// Visible reports whether the box, drawn from the ground up to the projection's Headroom, meets
// the screen.
func (c *isoCamera) Visible(box contract.AABB) bool {
	minX, minY, maxX, maxY := c.rect(float32(box.TopLeft.X), float32(box.TopLeft.Y), float32(box.BottomRight.X), float32(box.BottomRight.Y), 0, c.proj.Headroom)
	return maxX > 0 && minX < float32(c.viewportSize.X) && maxY > 0 && minY < float32(c.viewportSize.Y)
}

// Bounds is the world rectangle round the ground the screen shows, clamped to the world.
func (c *isoCamera) Bounds() contract.AABB {
	minX, minY := float32(math.Inf(1)), float32(math.Inf(1))
	maxX, maxY := float32(math.Inf(-1)), float32(math.Inf(-1))
	w, h := float32(c.viewportSize.X), float32(c.viewportSize.Y)
	for _, corner := range [4][2]float32{{0, 0}, {w, 0}, {0, h}, {w, h}} {
		x, y := c.Unproject(corner[0], corner[1], 0)
		minX, maxX = min(minX, x), max(maxX, x)
		minY, maxY = min(minY, y), max(maxY, y)
	}
	return contract.AABB{
		TopLeft:     geom.NewVec(float64(max(minX, 0)), float64(max(minY, 0))),
		BottomRight: geom.NewVec(float64(min(maxX, float32(c.world.X))), float64(min(maxY, float32(c.world.Y)))),
	}
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

// Persisted hands saves the ground point under the screen's corner and the zoom; Restore puts the
// window back over them.
func (c *isoCamera) Persisted() []any { return []any{&c.origin, &c.zoom} }

func (c *isoCamera) Restore() {
	c.zoom = max(c.zoom, 0.01)
	c.MoveTo(c.origin.X, c.origin.Y)
}
