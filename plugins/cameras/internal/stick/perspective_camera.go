package stick

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	contract "github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/vec"
)

type perspCamera struct {
	world geom.Vec
	// camera pixels, the shown ones times res; the field of view fixed, the window is the same
	// at any resolution
	viewportSize geom.Vec
	res          float32
	// the stick, out of a unit: its foot in the middle of the screen, the eye length along it,
	// turned by heading and leaning by pitch, lifted lift over what the cage makes solid
	foot    [3]float32
	length  float32
	heading float32
	pitch   float32
	lift    float32
	// inside a unit: the eye, x and y and its height, and how many times the field of view is
	// narrowed, 1 up to narrowest
	origin geom.Vec
	alt    float32
	narrow float32

	fov        float32 // radians, the top of the screen to the bottom, unnarrowed
	minPitch   float32
	cell       float32
	headroom   float32
	minZoomCfg float32
	maxZoom    float32
	ground     func(x, y float32) float32 // nil, sea level
	extent     func() (low, high float32) // nil, sea level
	zooms      uint32                     // how many times the player has zoomed
	tilts      uint32                     // how many times the player has tilted
	inside     bool
	across     float32 // inside a unit, radians; 0, fov
	bend       float32 // the ground d off sinks bend·d² under the eye's level
	// behind a unit, the stick held on its arm: no spring lifts it
	held bool

	fine  func(x, y float32) (step, thin float32)
	solid func(x, y, z float32) (float32, bool)

	// the ground point in the middle of the screen; far along the way looked, at the sky
	at         [3]float32
	sky        bool // at is no ground point
	proj       perspective
	projection contract.Projection // proj boxed once, so asking for it allocates nothing
	// what part of the frame's width and height the viewport is while a part of it is drawn,
	// and the frame, to come back to
	crop    geom.Vec
	cropped *perspFrame
}

type perspFrame struct {
	viewportSize geom.Vec
	res          float32
	crop         geom.Vec
	proj         perspective
	projection   contract.Projection
}

var _ contract.Camera = (*perspCamera)(nil)
var _ contract.Cropping = (*perspCamera)(nil)

// 45° top to bottom; narrowest: how many times the eye inside a unit may narrow it
const (
	defaultFOV = math.Pi / 4
	narrowest  = 8
)

func newPerspCamera(
	o optics,
	heading, pitch, fov float32,
	world geom.Vec,
	viewport contract.AABB,
	ground func(x, y float32) float32,
	extent func() (low, high float32),
) *perspCamera {
	if fov <= 0 {
		fov = defaultFOV
	}
	c := &perspCamera{world: world, fov: min(fov, math.Pi-0.01), minPitch: o.MinPitch,
		cell: o.Cell, headroom: o.Headroom, ground: ground, extent: extent,
		viewportSize: geom.NewVec(viewport.BottomRight.X-viewport.TopLeft.X,
			viewport.BottomRight.Y-viewport.TopLeft.Y),
		heading: heading, pitch: pitch, narrow: 1, length: 4 * o.Cell, res: 1,
		crop: geom.NewVec(1, 1)}
	c.CenterOn((viewport.TopLeft.X+viewport.BottomRight.X)/2,
		(viewport.TopLeft.Y+viewport.BottomRight.Y)/2, 0)
	return c
}

// screen units: the frame's height fills the field of view, or inside a unit with an eye its
// width fills across
func (c *perspCamera) focal() float32 {
	if c.inside && c.across > 0 {
		return float32(c.viewportSize.X/c.crop.X) / 2 / float32(math.Tan(float64(c.across)/2))
	}
	return float32(c.viewportSize.Y/c.crop.Y) / 2 / float32(math.Tan(float64(c.fov)/2))
}

func (c *perspCamera) far() float32 { return 4 * float32(max(c.world.X, c.world.Y)) }

// riding, the eye may stand but a unit's height over the ground
func (c *perspCamera) near() float32 {
	if c.inside {
		return c.cell / 1024
	}
	return c.cell / 4
}

func (c *perspCamera) Eye() (float32, float32, float32, bool) {
	e := c.proj.eye
	return e[0], e[1], e[2], true
}

func (c *perspCamera) layer() (low, high float32) {
	if c.extent != nil {
		low, high = c.extent()
	}
	return low, high + c.headroom
}

func (c *perspCamera) Ray(sx, sy float32) (float32, float32, float32, bool) {
	d := c.proj.ray(sx-float32(c.viewportSize.X/2), sy-float32(c.viewportSize.Y/2))
	n := vec.Norm(d)
	if n == 0 {
		return 0, 0, 0, false
	}
	return d[0] / n, d[1] / n, d[2] / n, true
}

func (c *perspCamera) Vanish(dx, dy, dz float32) (float32, float32, bool) {
	sx, sy, ok := c.proj.Vanish(dx, dy, dz)
	return sx + float32(c.viewportSize.X/2), sy + float32(c.viewportSize.Y/2), ok
}

func (c *perspCamera) target() [3]float32 { return c.at }

func (c *perspCamera) eye() [3]float32 { return c.proj.eye }

// the way from the foot up the stick to the eye
func (c *perspCamera) out() [3]float32 {
	s, k := math.Sincos(float64(c.heading) + math.Pi/4)
	sp, cp := math.Sincos(float64(c.pitch))
	return [3]float32{float32(cp * s), float32(cp * k), float32(sp)}
}

// the stick's length within the zoom's bounds: a cell at the shortest, the world's diagonal at the
// longest
func (c *perspCamera) bounded(length float32) float32 {
	shortest, longest := c.cell, float32(math.Hypot(c.world.X, c.world.Y))
	if c.maxZoom > 0 {
		shortest = max(shortest, c.focal()/(c.maxZoom*c.res))
	}
	if c.minZoomCfg > 0 {
		longest = min(longest, c.focal()/(c.minZoomCfg*c.res))
	}
	return min(max(length, shortest), max(longest, shortest))
}

// the eye at the end of the stick, unlifted
func (c *perspCamera) rest() [3]float32 {
	return vec.Add(c.foot, vec.Scale(c.out(), c.length))
}

func (c *perspCamera) look() {
	c.heading = wrapAngle(c.heading)
	c.narrow = min(max(c.narrow, 1), narrowest)
	var e, at [3]float32
	if c.inside {
		c.pitch = min(max(c.pitch, -maxPitch), maxPitch)
		e = [3]float32{float32(c.origin.X), float32(c.origin.Y), c.alt}
		// far along the way looked, for the frame's precision
		at = vec.Add(e, vec.Scale(c.out(), -c.far()))
	} else {
		c.pitch = min(c.pitch, maxPitch)
		if !c.held {
			c.length = c.bounded(c.length)
		}
		e = c.rest()
		c.lift = max(c.lift, c.need(e))
		if c.held {
			c.lift = 0
		}
		e[2] += c.lift
		// the foot as the eye sees it, sunk by the ground's curve
		h := float32(math.Hypot(float64(e[0]-c.foot[0]), float64(e[1]-c.foot[1])))
		at = [3]float32{c.foot[0], c.foot[1], c.foot[2] + c.bend*h*h}
	}
	c.proj = newPerspective(e, at, c.heading, c.focal()*c.narrow, c.near(), c.far(), c.cell)
	c.proj.bend = c.bend
	c.projection = c.proj
	c.at, c.sky = c.middle()
}

// how far over e the eye has to go to be clear of what is solid there, and a quarter of a cell
// over it
func (c *perspCamera) need(e [3]float32) float32 {
	if c.solid == nil && c.ground == nil {
		return 0
	}
	solid := solidOf(c.solid, c.ground)
	free := func(x, y, z float32) bool {
		_, in := solid(x, y, z)
		return !in
	}
	under := e[2] - c.cell/4
	switch {
	case !free(e[0], e[1], under):
		return rise(free, e[0], e[1], under, c.cell) + c.cell/4 - e[2]
	case !free(e[0], e[1], e[2]):
		return rise(free, e[0], e[1], e[2], c.cell) + c.cell/4 - e[2]
	}
	return 0
}

// the spring easing the lifted eye back down as far as the cage lets it
func (c *perspCamera) settle(d time.Duration) {
	if c.inside || c.held || c.lift == 0 {
		return
	}
	need := c.need(c.rest())
	if need >= c.lift {
		return
	}
	c.lift = need + (c.lift-need)*(1-eased(d))
	if c.lift-need < c.cell/1024 {
		c.lift = need
	}
	c.look()
}

func (c *perspCamera) middle() ([3]float32, bool) {
	x, y, _, hit := c.Pick(float32(c.viewportSize.X/2), float32(c.viewportSize.Y/2))
	if !hit {
		return vec.Add(c.proj.eye, vec.Scale(c.proj.forward, c.far())), true
	}
	var z float32
	if c.ground != nil {
		z = c.ground(x, y)
	}
	return [3]float32{x, y, z}, false
}

// the foot slid along the stick onto what is seen in the middle of the screen, the eye where it
// is; not while the spring lifts the eye
func (c *perspCamera) plant() {
	o := c.out()
	if c.inside || c.held || c.lift > 0 || o[2] < 1e-3 {
		return
	}
	_, _, z, hit := c.Pick(float32(c.viewportSize.X/2), float32(c.viewportSize.Y/2))
	if !hit {
		return
	}
	k := (z - c.foot[2]) / o[2]
	if c.length-k < c.cell/4 {
		return
	}
	c.foot = vec.Add(c.foot, vec.Scale(o, k))
	c.length -= k
}

// the foot kept over the world
func (c *perspCamera) confine() {
	c.foot[0] = min(max(c.foot[0], 0), float32(c.world.X))
	c.foot[1] = min(max(c.foot[1], 0), float32(c.world.Y))
}

func (c *perspCamera) Pick(sx, sy float32) (float32, float32, float32, bool) {
	if c.ground == nil {
		x, y, ok := c.cast(sx, sy, 0)
		return x, y, 0, ok
	}
	_, high := c.layer()
	s := sight{o: c.proj.eye, d: c.proj.ray(sx-float32(c.viewportSize.X/2), sy-float32(c.viewportSize.Y/2)), bend: c.bend}
	return s.pick(solidOf(c.solid, c.ground), c.fine, c.far(), c.cell/2, high)
}

// heading stays as given for an eye straight over the target
func anglesOf(eye, target [3]float32, heading float32) (float32, float32) {
	hx, hy := float64(eye[0]-target[0]), float64(eye[1]-target[1])
	h, dz := math.Hypot(hx, hy), float64(eye[2]-target[2])
	if h > 1e-6 {
		heading = float32(math.Atan2(hx, hy) - math.Pi/4)
	}
	return wrapAngle(heading), float32(math.Atan2(dz, h))
}

// 0 up to 2π
func wrapAngle(a float32) float32 {
	h := math.Mod(float64(a), 2*math.Pi)
	if h < 0 {
		h += 2 * math.Pi
	}
	return float32(h)
}

// the eye put at (x, y, z), looking at the foot
func (c *perspCamera) LookFrom(x, y, z float32) {
	c.inside, c.narrow, c.held, c.lift = false, 1, false, 0
	e := [3]float32{x, y, z}
	if l := vec.Norm(vec.Sub(e, c.foot)); l > 1e-3 {
		c.length = l
		c.heading, c.pitch = anglesOf(e, c.foot, c.heading)
	}
	c.look()
}

// the eye where it is, the foot put at (x, y, z); below the pitch floor too, until the next Tilt
func (c *perspCamera) LookAt(x, y, z float32) {
	e := c.proj.eye
	c.inside, c.narrow, c.held, c.lift = false, 1, false, 0
	c.foot = [3]float32{x, y, z}
	if l := vec.Norm(vec.Sub(e, c.foot)); l > 1e-3 {
		c.length = l
		c.heading, c.pitch = anglesOf(e, c.foot, c.heading)
	}
	c.look()
}

// across is radians, 0 the camera's own field
func (c *perspCamera) enterInside(eye [3]float32, heading, across float32) {
	c.inside, c.narrow, c.across = true, 1, across
	c.origin, c.alt = geom.NewVec(float64(eye[0]), float64(eye[1])), eye[2]
	c.heading, c.pitch = heading, 0
	c.look()
}

// hold puts the foot at foot and the eye length up the stick from it, held there without the
// spring
func (c *perspCamera) hold(foot [3]float32, length float32) {
	c.held, c.lift = true, 0
	c.foot, c.length = foot, length
	c.look()
}

func (c *perspCamera) underEye() [3]float32 {
	e := c.proj.eye
	at := [3]float32{e[0], e[1], 0}
	if c.ground != nil {
		at[2] = c.ground(e[0], e[1])
	}
	return at
}

type perspPose struct {
	foot                   [3]float32
	length, heading, pitch float32
}

func (c *perspCamera) pose() perspPose {
	return perspPose{foot: c.foot, length: c.length, heading: c.heading, pitch: c.pitch}
}

func (c *perspCamera) setPose(p perspPose) {
	c.inside, c.narrow, c.held, c.lift = false, 1, false, 0
	c.foot, c.length, c.heading, c.pitch = p.foot, p.length, p.heading, max(p.pitch, c.minPitch)
	c.look()
}

func (c *perspCamera) Turn(angle float32) {
	c.plant()
	c.heading += angle
	c.look()
}

func (c *perspCamera) Tilt(angle float32) {
	floor := c.minPitch
	if c.inside {
		floor = -maxPitch
	}
	c.plant()
	c.pitch = min(max(c.pitch+angle, floor), maxPitch)
	c.tilts++
	c.look()
}

func (c *perspCamera) Heading() float32 { return c.heading }
func (c *perspCamera) Pitch() float32   { return c.pitch }

func (c *perspCamera) ScaleAt(x, y, z float32) float32 {
	_, _, ahead := c.proj.view(x, y, z)
	if ahead < c.proj.near {
		return 0
	}
	return c.focal() * c.narrow / ahead
}

func (c *perspCamera) Projection() contract.Projection { return c.projection }

func (c *perspCamera) Viewport() (float32, float32) {
	return float32(c.viewportSize.X), float32(c.viewportSize.Y)
}

func (c *perspCamera) SetViewport(w, h float32) {
	if w <= 0 || h <= 0 {
		return
	}
	c.viewportSize = geom.NewVec(float64(w*c.res), float64(h*c.res))
	c.look()
}

// SetResolution has the camera drawn at s of the pixels it is shown at; its window stays.
func (c *perspCamera) SetResolution(s float32) {
	k := float64(s / c.res)
	c.viewportSize = geom.NewVec(c.viewportSize.X*k, c.viewportSize.Y*k)
	c.res = s
	c.look()
}

func (c *perspCamera) Resolution() float32 { return c.res }

// Crop has the camera drawn the central w by h shown pixels of its frame at s of them, the
// stick and the field of view as they are.
func (c *perspCamera) Crop(w, h, s float32) {
	if w <= 0 || h <= 0 {
		return
	}
	c.Uncrop()
	c.cropped = &perspFrame{viewportSize: c.viewportSize, res: c.res, crop: c.crop,
		proj: c.proj, projection: c.projection}
	c.crop = geom.NewVec(float64(w*c.res)/c.viewportSize.X, float64(h*c.res)/c.viewportSize.Y)
	c.viewportSize = geom.NewVec(float64(w*s), float64(h*s))
	c.res = s
	c.proj.focal = c.focal() * c.narrow
	c.projection = c.proj
}

// Uncrop puts the frame back as it was before Crop.
func (c *perspCamera) Uncrop() {
	if c.cropped == nil {
		return
	}
	f := c.cropped
	c.viewportSize, c.res, c.crop = f.viewportSize, f.res, f.crop
	c.proj, c.projection = f.proj, f.projection
	c.cropped = nil
}

func (c *perspCamera) Cropped() bool { return c.crop.X < 1 || c.crop.Y < 1 }

func (c *perspCamera) Frame() (float32, float32) {
	size, res := c.viewportSize, c.res
	if c.cropped != nil {
		size, res = c.cropped.viewportSize, c.cropped.res
	}
	return float32(size.X) / res, float32(size.Y) / res
}

func (c *perspCamera) Project(x, y, z float32) (float32, float32) {
	sx, sy := c.proj.Project(x, y, z)
	return sx + float32(c.viewportSize.X/2), sy + float32(c.viewportSize.Y/2)
}

func (c *perspCamera) Unproject(sx, sy, z float32) (float32, float32) {
	x, y, _ := c.cast(sx, sy, z)
	return x, y
}

func (c *perspCamera) cast(sx, sy, z float32) (float32, float32, bool) {
	return c.proj.cast(sx-float32(c.viewportSize.X/2), sy-float32(c.viewportSize.Y/2), z)
}

func (c *perspCamera) Depth(x, y, z float32) float32 { return c.proj.Depth(x, y, z) }

func (c *perspCamera) ToScreen(x, y float32) (float32, float32)     { return c.Project(x, y, 0) }
func (c *perspCamera) FromScreen(sx, sy float32) (float32, float32) { return c.Unproject(sx, sy, 0) }

// A bound: renderers project corners instead.
func (c *perspCamera) ToScreenQuads(x0, y0, x1, y1 float32, dst []contract.Quad) []contract.Quad {
	minX, minY, maxX, maxY := c.rect(x0, y0, x1, y1, 0, 0)
	return append(dst, contract.Quad{X0: minX, Y0: minY, X1: maxX, Y1: maxY, T0X: 0, T1X: 1, T0Y: 0, T1Y: 1})
}

func (c *perspCamera) rect(x0, y0, x1, y1, z0, z1 float32) (minX, minY, maxX, maxY float32) {
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

func (c *perspCamera) Visible(box contract.AABB) bool {
	low, high := c.layer()
	minX, minY, maxX, maxY := c.rect(float32(box.TopLeft.X), float32(box.TopLeft.Y), float32(box.BottomRight.X), float32(box.BottomRight.Y), low, high)
	return maxX > 0 && minX < float32(c.viewportSize.X) && maxY > 0 && minY < float32(c.viewportSize.Y)
}

// Along an edge of the screen the lines of sight meet each height on a straight line, so the
// corners bound them, but where an edge crosses the horizon: there they run to far, and the line
// whose leaving the layer comes just at far bounds those.
func (c *perspCamera) Bounds() contract.AABB {
	low, high := c.layer()
	e, far := c.proj.eye, c.proj.far
	// the ground's curve sinks what lies far off: bound it straight, lower
	low -= c.bend * (2 * far) * (2 * far)
	minX, minY := float32(math.Inf(1)), float32(math.Inf(1))
	maxX, maxY := float32(math.Inf(-1)), float32(math.Inf(-1))
	along := func(d [3]float32) { // the stretch of the line of sight d within the layer
		in, out := float32(0), far
		if d[2] == 0 {
			if e[2] < low || e[2] > high {
				return
			}
		} else {
			t0, t1 := (low-e[2])/d[2], (high-e[2])/d[2]
			in, out = max(in, min(t0, t1)), min(out, max(t0, t1))
			if in > out {
				return
			}
		}
		for _, t := range [2]float32{in, out} {
			x, y := e[0]+d[0]*t, e[1]+d[1]*t
			minX, maxX = min(minX, x), max(maxX, x)
			minY, maxY = min(minY, y), max(maxY, y)
		}
	}
	w, h := float32(c.viewportSize.X), float32(c.viewportSize.Y)
	corners := [4][2]float32{{0, 0}, {w, 0}, {w, h}, {0, h}}
	for k, a := range corners {
		b := corners[(k+1)%4]
		da, db := c.proj.ray(a[0]-w/2, a[1]-h/2), c.proj.ray(b[0]-w/2, b[1]-h/2)
		along(da)
		for _, z := range [2]float32{low, high} { // the line leaving z just at far
			if dz := db[2] - da[2]; dz != 0 {
				if s := ((z-e[2])/far - da[2]) / dz; s > 0 && s < 1 {
					along(vec.Add(da, vec.Scale(vec.Sub(db, da), s)))
				}
			}
		}
	}
	if minX > maxX { // the whole screen looks over everything: nothing but where the eye is
		minX, maxX, minY, maxY = e[0], e[0], e[1], e[1]
	}
	return worldBox(minX, minY, maxX, maxY, c.world)
}

// the world point (x, y) in the screen's top-left corner, at the foot's height
func (c *perspCamera) MoveTo(x, y float64) {
	if c.inside {
		c.origin = geom.NewVec(x, y)
		c.look()
		return
	}
	if tx, ty, hit := c.cast(0, 0, c.foot[2]); hit {
		c.foot[0] += float32(x) - tx
		c.foot[1] += float32(y) - ty
	}
	c.confine()
	c.look()
}

func (c *perspCamera) CenterOn(x, y, z float64) {
	if c.inside {
		c.origin, c.alt = geom.NewVec(x, y), float32(z)
		c.look()
		return
	}
	c.foot = [3]float32{float32(x), float32(y), float32(z)}
	c.confine()
	c.look()
}

func (c *perspCamera) Translate(dx, dy float64) {
	c.foot[0] += float32(dx)
	c.foot[1] += float32(dy)
	c.confine()
	c.look()
}

// the foot slid at its height to what is drawn (dx, dy) off the middle of the screen
func (c *perspCamera) Pan(dx, dy float32) {
	if c.inside {
		return
	}
	mx, my := float32(c.viewportSize.X/2), float32(c.viewportSize.Y/2)
	x0, y0, hit0 := c.cast(mx, my, c.foot[2])
	x1, y1, hit1 := c.cast(mx+dx, my+dy, c.foot[2])
	if !hit0 || !hit1 {
		return
	}
	c.foot[0] += x1 - x0
	c.foot[1] += y1 - y0
	c.confine()
	c.look()
}

// screen units a world unit spans at the ground point in the middle of the screen
func (c *perspCamera) Zoom() float32 {
	return c.focal() * c.narrow / max(vec.Norm(vec.Sub(c.proj.eye, c.at)), c.proj.near)
}

// Inside a unit the view narrowed; out of it the eye and the foot drawn towards the ground under
// the anchor, the stick shortened so: the anchor stays where it is drawn, and coming in stops
// where the eye would no longer be free.
func (c *perspCamera) ZoomIn(factor float32, anchorX, anchorY float32) {
	if factor <= 0 {
		return
	}
	c.zooms++
	if c.inside {
		c.narrow *= factor
		c.look()
		return
	}
	a := [3]float32{anchorX, anchorY, 0}
	if c.ground != nil {
		a[2] = c.ground(anchorX, anchorY)
	}
	k := min(max(factor, c.length/c.bounded(c.length*1e6)), c.length/c.bounded(0))
	toward := func(k float32) ([3]float32, float32) {
		return vec.Add(a, vec.Scale(vec.Sub(c.foot, a), 1/k)), c.length / k
	}
	if k > 1 {
		free := func(k float32) bool {
			foot, length := toward(k)
			return c.need(vec.Add(foot, vec.Scale(c.out(), length))) == 0
		}
		if !free(k) {
			lo, hi := float32(1), k
			for range 24 {
				if mid := (lo + hi) / 2; free(mid) {
					lo = mid
				} else {
					hi = mid
				}
			}
			k = lo
		}
	}
	c.foot, c.length = toward(k)
	c.confine()
	c.look()
}

func (c *perspCamera) ZoomOut(factor float32, anchorX, anchorY float32) {
	c.ZoomIn(1/factor, anchorX, anchorY)
}

func (c *perspCamera) SetMinZoom(minZoom float32) {
	c.minZoomCfg = minZoom
	c.look()
}

func (c *perspCamera) SetMaxZoom(maxZoom float32) {
	c.maxZoom = maxZoom
	c.look()
}

func (c *perspCamera) State() contract.State {
	return contract.State{Viewport: c.Bounds(), Zoom: c.Zoom()}
}

func (c *perspCamera) Persisted() []any {
	return []any{&c.foot, &c.length, &c.heading, &c.pitch}
}

func (c *perspCamera) Restore() {
	c.inside, c.narrow, c.held, c.lift = false, 1, false, 0
	c.pitch = max(c.pitch, c.minPitch) // out of the unit until the camera system takes it back in
	c.look()
}
