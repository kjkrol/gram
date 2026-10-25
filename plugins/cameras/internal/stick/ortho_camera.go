package stick

import (
	"math"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	contract "github.com/kjkrol/gram/camera"
)

type orthoCamera struct {
	optics
	proj ortho
	// proj boxed once, so asking for it allocates nothing
	projection   contract.Projection
	world        geom.Vec
	viewportSize geom.Vec
	minZoomCfg   float32
	maxZoom      float32
	// the stick, what a save keeps: its foot in the middle of the screen, the view turned and
	// tilted about it, scale screen units a world unit
	foot  [3]float64
	turn  float64
	pitch float64
	scale float32
	// camera pixels a shown one: the stick and its scale in shown pixels, the screen in these
	res float32
	// where the world's origin at sea level lands on the screen, from the stick, in shown pixels
	pan [2]float64

	// the ground the screen covers at a screen unit a world unit and sea level, for the zoom floor
	spanX, spanY float32
	// nil, level at 0
	extent func() (low, high float32)
	// nil, level at 0
	ground func(x, y float32) float32
	// nil or 0, as ever
	fine func(x, y float32) (step, thin float32)
	// nil, under ground alone
	solid func(x, y, z float32) (float32, bool)
	// the frame while a part of it is drawn, to come back to
	cropped *orthoFrame
	// a copy's extent, as it stood when the copy was taken
	still     bool
	low, high float32
}

type orthoFrame struct {
	viewportSize geom.Vec
	res          float32
	pan          [2]float64
	win          contract.Window
}

var _ contract.Camera = (*orthoCamera)(nil)
var _ contract.Cropping = (*orthoCamera)(nil)

func newOrthoCamera(
	o optics,
	start Start,
	world geom.Vec,
	viewport contract.AABB,
	edges aabbworld.Edges,
) *orthoCamera {
	if edges.WrapsX() || edges.WrapsY() {
		panic("cameras: a world that wraps cannot be seen in relief")
	}
	c := &orthoCamera{optics: o.withDefaults(), world: world, pitch: maxPitch, scale: 1, res: 1,
		viewportSize: geom.NewVec(viewport.BottomRight.X-viewport.TopLeft.X,
			viewport.BottomRight.Y-viewport.TopLeft.Y)}
	if start == Isometrically {
		c.turn, c.pitch, c.scale = math.Pi/4, c.isoPitch(), c.tile()
	}
	c.look()
	c.scale = max(c.scale, c.minZoom())
	c.aim()
	c.MoveTo(viewport.TopLeft.X, viewport.TopLeft.Y)
	return c
}

// steps of a turn land on an eighth of it exactly, where a row's cells tie in depth
const eighth = math.Pi / 4

func (c *orthoCamera) look() {
	if r := math.Round(c.turn/eighth) * eighth; math.Abs(c.turn-r) < 1e-6 {
		c.turn = r
	}
	c.turn = wrapTurn(c.turn)
	c.pitch = min(max(c.pitch, float64(c.MinPitch)), maxPitch)
	c.proj = newOrtho(c.optics, c.turn, c.pitch)
	c.projection = c.proj
	c.measure()
}

// 0 up to 2π
func wrapTurn(a float64) float64 {
	a = math.Mod(a, 2*math.Pi)
	if a < 0 {
		a += 2 * math.Pi
	}
	return a
}

// at a screen unit a world unit the projection is affine, so the footprint scales as one over the
// scale
func (c *orthoCamera) measure() {
	w, h := c.viewportSize.X, c.viewportSize.Y
	minX, maxX, minY, maxY := c.under(w, h, c.proj.Unproject)
	c.spanX, c.spanY = maxX-minX, maxY-minY
}

// the ground under a screen w by h pixels of unproject's
func (c *orthoCamera) under(
	w, h float32,
	unproject func(sx, sy, z float32) (float32, float32),
) (minX, maxX, minY, maxY float32) {
	minX, maxX = float32(math.Inf(1)), float32(math.Inf(-1))
	minY, maxY = float32(math.Inf(1)), float32(math.Inf(-1))
	for _, corner := range [4][2]float32{{0, 0}, {w, 0}, {0, h}, {w, h}} {
		x, y := unproject(corner[0], corner[1], 0)
		minX, maxX = min(minX, x), max(maxX, x)
		minY, maxY = min(minY, y), max(maxY, y)
	}
	return
}

func (c *orthoCamera) Turn(angle float32) {
	c.turn += float64(angle)
	c.lean()
}

func (c *orthoCamera) Tilt(angle float32) {
	c.pitch += float64(angle)
	c.lean()
}

// the stick turned or tilted about its foot, stood first on what is seen in the middle
func (c *orthoCamera) lean() {
	c.plant()
	c.look()
	c.scale = max(c.scale, c.minZoom())
	c.place()
}

// the foot slid along the stick onto what is seen in the middle of the screen, the picture as it
// was: over no ground it stays where it is
func (c *orthoCamera) plant() {
	if c.ground != nil {
		w, h := c.Viewport()
		if _, _, z, hit := c.Pick(w/2, h/2); hit {
			p := c.proj
			k := (float64(z) - c.foot[2]) / float64(p.sp)
			c.foot[0] += k * float64(p.sin*p.lift)
			c.foot[1] += k * float64(p.cos*p.lift)
			c.foot[2] = float64(z)
		}
	}
}

func (c *orthoCamera) Heading() float32 { return float32(wrapTurn(c.turn - eighth)) }
func (c *orthoCamera) Pitch() float32   { return float32(c.pitch) }

func (c *orthoCamera) Projection() contract.Projection { return c.projection }

func (c *orthoCamera) Viewport() (float32, float32) {
	return c.viewportSize.X * c.res, c.viewportSize.Y * c.res
}

// SetResolution has the camera drawn at s of the pixels it is shown at; its window stays.
func (c *orthoCamera) SetResolution(s float32) { c.res = s }

func (c *orthoCamera) Resolution() float32 { return c.res }

// Crop has the camera drawn the part win of its frame at s of its shown pixels, the stick and its
// scale as they are.
func (c *orthoCamera) Crop(win contract.Window, s float32) {
	if win.W <= 0 || win.H <= 0 {
		return
	}
	c.Uncrop()
	c.cropped = &orthoFrame{viewportSize: c.viewportSize, res: c.res, pan: c.pan, win: win}
	c.viewportSize = geom.NewVec(win.W, win.H)
	c.res = s
	c.pan = [2]float64{c.pan[0] - float64(win.X), c.pan[1] - float64(win.Y)}
}

// Uncrop puts the frame back as it was before Crop.
func (c *orthoCamera) Uncrop() {
	if c.cropped == nil {
		return
	}
	c.viewportSize, c.res, c.pan = c.cropped.viewportSize, c.cropped.res, c.cropped.pan
	c.cropped = nil
}

func (c *orthoCamera) Cropped() bool {
	return c.cropped != nil && c.viewportSize != c.cropped.viewportSize
}

func (c *orthoCamera) Frame() (float32, float32) {
	size := c.viewportSize
	if c.cropped != nil {
		size = c.cropped.viewportSize
	}
	return size.X, size.Y
}

func (c *orthoCamera) Window() contract.Window {
	if c.cropped != nil {
		return c.cropped.win
	}
	return contract.Window{W: c.viewportSize.X, H: c.viewportSize.Y}
}

func (c *orthoCamera) SetViewport(w, h float32) {
	if w <= 0 || h <= 0 {
		return
	}
	c.viewportSize = geom.NewVec(w, h)
	c.measure()
	c.scale = max(c.scale, c.minZoom())
	c.place()
}

func (c *orthoCamera) Project(x, y, z float32) (float32, float32) {
	sx, sy := c.proj.Project(x, y, z)
	return (sx*c.scale + float32(c.pan[0])) * c.res, (sy*c.scale + float32(c.pan[1])) * c.res
}

func (c *orthoCamera) Unproject(sx, sy, z float32) (float32, float32) {
	sx, sy = sx/c.res, sy/c.res
	return c.proj.Unproject((sx-float32(c.pan[0]))/c.scale, (sy-float32(c.pan[1]))/c.scale, z)
}

// where (x, y, z) lands on the screen before the pan, in float64: the stick's own arithmetic
func (c *orthoCamera) screen(x, y, z float64) (float64, float64) {
	s, k := float64(c.proj.sin), float64(c.proj.cos)
	sx := x*k - y*s
	sy := (x*s+y*k)*float64(c.proj.sp) - z*float64(c.proj.lift)
	return sx * float64(c.scale), sy * float64(c.scale)
}

func (c *orthoCamera) Pick(sx, sy float32) (float32, float32, float32, bool) {
	if c.ground == nil {
		x, y := c.Unproject(sx, sy, 0)
		return x, y, 0, true
	}
	low, high := c.layer()
	x0, y0 := c.Unproject(sx, sy, high)
	x1, y1 := c.Unproject(sx, sy, high-1)
	s := sight{o: [3]float32{x0, y0, high}, d: [3]float32{x1 - x0, y1 - y0, -1}}
	return s.pick(solidOf(c.solid, c.ground), c.fine, high-low+1, c.Cell/2, high)
}

func solidOf(solid func(x, y, z float32) (float32, bool),
	ground func(x, y float32) float32) func(x, y, z float32) (float32, bool) {
	if solid != nil {
		return solid
	}
	return grounded(ground)
}

func (c *orthoCamera) Depth(x, y, z float32) float32 { return c.proj.Depth(x, y, z) }

func (c *orthoCamera) ToScreen(x, y float32) (float32, float32) { return c.Project(x, y, 0) }

func (c *orthoCamera) FromScreen(sx, sy float32) (float32, float32) {
	return c.Unproject(sx, sy, 0)
}

// A bound, since a rectangle leaning projects to a diamond: renderers project corners instead.
func (c *orthoCamera) ToScreenQuads(x0, y0, x1, y1 float32, dst []contract.Quad) []contract.Quad {
	minX, minY, maxX, maxY := c.rect(x0, y0, x1, y1, 0, 0)
	return append(dst, contract.Quad{
		X0:  minX,
		Y0:  minY,
		X1:  maxX,
		Y1:  maxY,
		T0X: 0,
		T1X: 1,
		T0Y: 0,
		T1Y: 1,
	})
}

func (c *orthoCamera) rect(x0, y0, x1, y1, z0, z1 float32) (minX, minY, maxX, maxY float32) {
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

func (c *orthoCamera) layer() (low, high float32) {
	switch {
	case c.still:
		low, high = c.low, c.high
	case c.extent != nil:
		low, high = c.extent()
	}
	return low, high + c.Headroom
}

// copyInto has dst draw as c does now, its extent still and the cage read no more
func (c *orthoCamera) copyInto(dst *orthoCamera) {
	*dst = *c
	low, high := c.layer()
	dst.still, dst.low, dst.high = true, low, high-c.Headroom
	dst.extent, dst.ground, dst.fine, dst.solid, dst.cropped = nil, nil, nil, nil, nil
}

func (c *orthoCamera) Visible(box contract.AABB) bool {
	low, high := c.layer()
	minX, minY, maxX, maxY := c.rect(box.TopLeft.X, box.TopLeft.Y,
		box.BottomRight.X, box.BottomRight.Y, low, high)
	w, h := c.Viewport()
	return maxX > 0 && minX < w && maxY > 0 && minY < h
}

// At any height of the layer: high ground from beyond the screen's lower edge at sea level is
// drawn on it.
func (c *orthoCamera) Bounds() contract.AABB {
	low, high := c.layer()
	minX, minY := float32(math.Inf(1)), float32(math.Inf(1))
	maxX, maxY := float32(math.Inf(-1)), float32(math.Inf(-1))
	w, h := c.Viewport()
	for _, z := range [2]float32{low, high} {
		for _, corner := range [4][2]float32{{0, 0}, {w, 0}, {0, h}, {w, h}} {
			x, y := c.Unproject(corner[0], corner[1], z)
			minX, maxX = min(minX, x), max(maxX, x)
			minY, maxY = min(minY, y), max(maxY, y)
		}
	}
	return worldBox(minX, minY, maxX, maxY, c.world)
}

// the world point (x, y) at sea level in the screen's top-left corner
func (c *orthoCamera) MoveTo(x, y float32) {
	sx, sy := c.screen(float64(x), float64(y), 0)
	c.slide(sx+c.pan[0], sy+c.pan[1])
}

func (c *orthoCamera) CenterOn(x, y, z float32) {
	c.foot = [3]float64{float64(x), float64(y), float64(z)}
	c.place()
}

func (c *orthoCamera) Translate(dx, dy float32) {
	c.foot[0] += float64(dx)
	c.foot[1] += float64(dy)
	c.place()
}

func (c *orthoCamera) Pan(dx, dy float32) {
	c.slide(float64(dx/c.res), float64(dy/c.res))
}

// the foot moved, at its height, to what is drawn (dsx, dsy) shown pixels off the middle of the
// screen
func (c *orthoCamera) slide(dsx, dsy float64) {
	dx, dy := c.proj.step(dsx, dsy, float64(c.scale))
	c.foot[0] += dx
	c.foot[1] += dy
	c.place()
}

// the whole screen stays over the world at sea level, as the top-down camera keeps its window
func (c *orthoCamera) place() {
	c.aim()
	w, h := c.Viewport()
	minX, maxX, minY, maxY := c.under(w, h, c.Unproject)
	dx, dy := fitSpan(minX, maxX, c.world.X), fitSpan(minY, maxY, c.world.Y)
	if dx != 0 || dy != 0 {
		c.foot[0] += float64(dx)
		c.foot[1] += float64(dy)
		c.aim()
	}
}

func (c *orthoCamera) aim() {
	sx, sy := c.screen(c.foot[0], c.foot[1], c.foot[2])
	c.pan = [2]float64{float64(c.viewportSize.X)/2 - sx, float64(c.viewportSize.Y)/2 - sy}
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

func (c *orthoCamera) Zoom() float32 { return c.scale * c.res }

func (c *orthoCamera) ZoomIn(factor float32, anchorX, anchorY float32) {
	var z float32
	if c.ground != nil {
		z = c.ground(anchorX, anchorY)
	}
	bx, by := c.Project(anchorX, anchorY, z)
	c.scale = c.limited(c.scale * factor)
	c.aim()
	ax, ay := c.Project(anchorX, anchorY, z)
	c.Pan(ax-bx, ay-by)
}

func (c *orthoCamera) ZoomOut(factor float32, anchorX, anchorY float32) {
	c.ZoomIn(1/factor, anchorX, anchorY)
}

// over the floor and under the top zoom, the top winning
func (c *orthoCamera) limited(scale float32) float32 {
	scale = max(scale, 0.01, c.minZoom())
	if c.maxZoom > 0 && scale > c.maxZoom {
		scale = c.maxZoom
	}
	return scale
}

// the top-down camera's world-fit floor, for the ground the screen covers
func (c *orthoCamera) minZoom() float32 {
	floor := max(c.spanX/c.world.X, c.spanY/c.world.Y)
	return max(floor, c.minZoomCfg)
}

func (c *orthoCamera) SetMinZoom(minZoom float32) { c.minZoomCfg = minZoom }
func (c *orthoCamera) SetMaxZoom(maxZoom float32) { c.maxZoom = maxZoom }

func (c *orthoCamera) State() contract.State {
	return contract.State{Viewport: c.Bounds(), Zoom: c.scale}
}

func (c *orthoCamera) Persisted() []any {
	return []any{&c.foot, &c.turn, &c.pitch, &c.scale}
}

func (c *orthoCamera) Restore() {
	c.look()
	c.scale = max(c.scale, 0.01, c.minZoom())
	c.place()
}
