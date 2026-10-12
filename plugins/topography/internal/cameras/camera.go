package cameras

import (
	"math"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	contract "github.com/kjkrol/gram/camera"
)

type isoCamera struct {
	proj         projection
	projection   contract.Projection // proj boxed once, so asking for it allocates nothing
	world        geom.Vec
	viewportSize geom.Vec
	minZoomCfg   float32
	maxZoom      float32
	zoom         float32
	// where the projected world origin lands on the screen, pixels
	pan geom.Vec
	// the ground point under the screen's top-left corner; what a save keeps, with the rest
	origin  geom.Vec
	heading float32
	pitch   float32
	flat    bool

	// the ground the screen covers at zoom 1 and sea level, for the zoom floor
	spanX, spanY float32
	// nil, level at 0
	extent func() (low, high float32)
	// nil, level at 0
	ground func(x, y float32) float32
	// nil or 0, as ever
	fine func(x, y float32) (step, thin float32)
	// nil, under ground alone
	solid func(x, y, z float32) (float32, bool)
}

var _ contract.Camera = (*isoCamera)(nil)

func newIsoCamera(proj projection, world geom.Vec, viewport contract.AABB, edges aabbworld.Edges) *isoCamera {
	if edges.WrapsX() || edges.WrapsY() {
		panic("topography: a world that wraps cannot be seen in relief")
	}
	proj = proj.withDefaults()
	c := &isoCamera{world: world, zoom: 1,
		viewportSize: geom.NewVec(viewport.BottomRight.X-viewport.TopLeft.X, viewport.BottomRight.Y-viewport.TopLeft.Y)}
	c.look(proj)
	c.zoom = max(c.zoom, c.minZoom())
	c.MoveTo(viewport.TopLeft.X, viewport.TopLeft.Y)
	return c
}

func (c *isoCamera) Isometric() bool { return !c.proj.flat }

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

func (c *isoCamera) look(proj projection) {
	c.proj, c.projection, c.heading, c.pitch, c.flat = proj, proj, proj.Heading, proj.Pitch, proj.flat
	c.measure()
}

// at zoom 1 the projection is affine, so the footprint scales as one over the zoom
func (c *isoCamera) measure() {
	minX, maxX, minY, maxY := c.under(c.proj.Unproject)
	c.spanX, c.spanY = maxX-minX, maxY-minY
}

func (c *isoCamera) under(unproject func(sx, sy, z float32) (float32, float32)) (minX, maxX, minY, maxY float32) {
	minX, maxX, minY, maxY = float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.Inf(1)), float32(math.Inf(-1))
	w, h := float32(c.viewportSize.X), float32(c.viewportSize.Y)
	for _, corner := range [4][2]float32{{0, 0}, {w, 0}, {0, h}, {w, h}} {
		x, y := unproject(corner[0], corner[1], 0)
		minX, maxX = min(minX, x), max(maxX, x)
		minY, maxY = min(minY, y), max(maxY, y)
	}
	return
}

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
	c.measure()
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

func (c *isoCamera) Pick(sx, sy float32) (float32, float32, float32, bool) {
	if c.ground == nil {
		x, y := c.Unproject(sx, sy, 0)
		return x, y, 0, true
	}
	low, high := c.layer()
	x0, y0 := c.Unproject(sx, sy, high)
	x1, y1 := c.Unproject(sx, sy, high-1)
	s := sight{o: [3]float32{x0, y0, high}, d: [3]float32{x1 - x0, y1 - y0, -1}}
	return s.pick(solidOf(c.solid, c.ground), c.fine, high-low+1, c.proj.Cell/2, high)
}

func solidOf(solid func(x, y, z float32) (float32, bool),
	ground func(x, y float32) float32) func(x, y, z float32) (float32, bool) {
	if solid != nil {
		return solid
	}
	return grounded(ground)
}

func (c *isoCamera) Depth(x, y, z float32) float32 { return c.proj.Depth(x, y, z) }

func (c *isoCamera) ToScreen(x, y float32) (float32, float32)     { return c.Project(x, y, 0) }
func (c *isoCamera) FromScreen(sx, sy float32) (float32, float32) { return c.Unproject(sx, sy, 0) }

// A bound, since a rectangle projects to a diamond: renderers project corners instead.
func (c *isoCamera) ToScreenQuads(x0, y0, x1, y1 float32, dst []contract.Quad) []contract.Quad {
	minX, minY, maxX, maxY := c.rect(x0, y0, x1, y1, 0, 0)
	return append(dst, contract.Quad{X0: minX, Y0: minY, X1: maxX, Y1: maxY, T0X: 0, T1X: 1, T0Y: 0, T1Y: 1})
}

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

func (c *isoCamera) layer() (low, high float32) {
	if c.extent != nil {
		low, high = c.extent()
	}
	return low, high + c.proj.Headroom
}

func (c *isoCamera) Visible(box contract.AABB) bool {
	low, high := c.layer()
	minX, minY, maxX, maxY := c.rect(float32(box.TopLeft.X), float32(box.TopLeft.Y), float32(box.BottomRight.X), float32(box.BottomRight.Y), low, high)
	return maxX > 0 && minX < float32(c.viewportSize.X) && maxY > 0 && minY < float32(c.viewportSize.Y)
}

// At any height of the layer: high ground from beyond the screen's lower edge at sea level is
// drawn on it.
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

func (c *isoCamera) MoveTo(x, y float64) {
	sx, sy := c.proj.Project(float32(x), float32(y), 0)
	c.place(geom.NewVec(-float64(sx)*float64(c.zoom), -float64(sy)*float64(c.zoom)))
}

func (c *isoCamera) CenterOn(x, y, z float64) {
	sx, sy := c.proj.Project(float32(x), float32(y), float32(z))
	z0 := float64(c.zoom)
	c.place(geom.NewVec(c.viewportSize.X/2-float64(sx)*z0, c.viewportSize.Y/2-float64(sy)*z0))
}

func (c *isoCamera) Translate(dx, dy float64) {
	sx, sy := c.proj.Project(float32(dx), float32(dy), 0)
	c.place(geom.NewVec(c.pan.X-float64(sx)*float64(c.zoom), c.pan.Y-float64(sy)*float64(c.zoom)))
}

func (c *isoCamera) Pan(dx, dy float32) { c.shift(dx, dy) }

// bx, by: the part of the delta the world's edge stopped, screen pixels
func (c *isoCamera) shift(dx, dy float32) (bx, by float32) {
	want := geom.NewVec(c.pan.X-float64(dx), c.pan.Y-float64(dy))
	c.place(want)
	return float32(c.pan.X - want.X), float32(c.pan.Y - want.Y)
}

// the whole screen stays over the world at sea level, as the top-down camera keeps its window
func (c *isoCamera) place(pan geom.Vec) {
	c.pan = pan
	minX, maxX, minY, maxY := c.under(c.Unproject)
	dx, dy := fitSpan(minX, maxX, float32(c.world.X)), fitSpan(minY, maxY, float32(c.world.Y))
	if dx != 0 || dy != 0 {
		sx, sy := c.proj.Project(dx, dy, 0)
		c.pan = geom.NewVec(c.pan.X-float64(sx)*float64(c.zoom), c.pan.Y-float64(sy)*float64(c.zoom))
	}
	x, y := c.Unproject(0, 0, 0)
	c.origin = geom.NewVec(float64(x), float64(y))
}

func fitSpan(lo, hi, size float32) float32 {
	switch {
	case hi-lo > size:
		return (size - lo - hi) / 2
	case lo < 0:
		return -lo
	case hi > size:
		return size - hi
	}
	return 0
}

func (c *isoCamera) Zoom() float32 { return c.zoom }

// screen units a world unit spans across the screen, which isometrically is not the zoom
func (c *isoCamera) scale() float32 {
	if c.proj.flat {
		return c.zoom
	}
	return c.zoom * c.proj.TileW / (math.Sqrt2 * c.proj.Cell)
}

func (c *isoCamera) ZoomIn(factor float32, anchorX, anchorY float32) {
	var z float32
	if c.ground != nil {
		z = c.ground(anchorX, anchorY)
	}
	beforeX, beforeY := c.Project(anchorX, anchorY, z)
	zoom := max(c.zoom*factor, 0.01)
	if floor := c.minZoom(); zoom < floor {
		zoom = floor
	}
	if c.maxZoom > 0 && zoom > c.maxZoom {
		zoom = c.maxZoom
	}
	c.zoom = zoom
	afterX, afterY := c.Project(anchorX, anchorY, z)
	c.place(geom.NewVec(c.pan.X+float64(beforeX-afterX), c.pan.Y+float64(beforeY-afterY)))
}

func (c *isoCamera) ZoomOut(factor float32, anchorX, anchorY float32) {
	c.ZoomIn(1/factor, anchorX, anchorY)
}

// the top-down camera's world-fit floor, for the ground the screen covers
func (c *isoCamera) minZoom() float32 {
	floor := max(c.spanX/float32(c.world.X), c.spanY/float32(c.world.Y))
	return max(floor, c.minZoomCfg)
}

func (c *isoCamera) SetMinZoom(minZoom float32) { c.minZoomCfg = minZoom }
func (c *isoCamera) SetMaxZoom(maxZoom float32) { c.maxZoom = maxZoom }

func (c *isoCamera) State() contract.State { return contract.State{Viewport: c.Bounds(), Zoom: c.zoom} }

func (c *isoCamera) Persisted() []any {
	return []any{&c.origin, &c.zoom, &c.heading, &c.pitch, &c.flat}
}

func (c *isoCamera) Restore() {
	c.look(c.proj.turned(c.heading).tilted(c.pitch).viewed(c.flat))
	c.zoom = max(c.zoom, 0.01, c.minZoom())
	c.MoveTo(c.origin.X, c.origin.Y)
}
