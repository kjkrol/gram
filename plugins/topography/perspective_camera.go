package topography

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	contract "github.com/kjkrol/gram/camera"
)

// perspCamera is a camera.Camera in perspective: an eye flying over the world, never lower than
// eyeOver cells over its highest ground, turned and tilted like a head — what a save keeps is
// where it is, how high, the heading, the pitch and how far it narrows its field of view. Pan
// moves it along the ground, Turn goes round the ground point in the middle of the screen, Tilt
// raises or bows the head, Zoom comes in until the ceiling and narrows the view from there.
// Inside a unit the eye is the unit's: it goes with it, looking where it faces, and Pan does
// nothing — the keys that pan steer the unit.
type perspCamera struct {
	world        geom.Vec
	viewportSize geom.Vec
	origin       geom.Vec // the eye, x and y
	alt          float32  // the eye's height
	heading      float32
	pitch        float32
	narrow       float32 // how many times the field of view is narrowed, 1 up to narrowest

	fov        float32 // radians, the top of the screen to the bottom, unnarrowed
	minPitch   float32
	cell       float32
	headroom   float32
	minZoomCfg float32
	maxZoom    float32
	ground     func(x, y float32) float32 // the top of the ground as it is drawn; nil, sea level
	extent     func() (low, high float32) // the lowest and the highest ground; nil, sea level
	zooms      uint32                     // how many times the player has zoomed
	tilts      uint32                     // how many times the player has tilted
	inside     bool                       // the eye is a unit's, looking out from it
	bend       float32                    // how far the ground d off sinks under the eye's level, per d²

	at         [3]float32 // the ground point in the middle of the screen; far along the way looked, at the sky
	sky        bool       // the middle of the screen looks at the sky: at is no ground point
	proj       perspective
	projection contract.Projection // proj boxed once, so asking for it allocates nothing
}

var _ contract.Camera = (*perspCamera)(nil)

// defaultFOV is the field of view unless the game says otherwise: 45° from the top of the screen
// to the bottom; narrowest is how many times the eye may narrow it, looking hard; eyeOver is how
// many cells over the highest ground the eye flies at least.
const (
	defaultFOV = math.Pi / 4
	narrowest  = 8
	eyeOver    = 2
)

// newPerspCamera is a perspective camera over the world drawn to viewport, turned and tilted as
// proj is, over the middle of the viewport as high as the ceiling; fov 0 is defaultFOV.
func newPerspCamera(proj projection, fov float32, world geom.Vec, viewport contract.AABB, ground func(x, y float32) float32, extent func() (low, high float32)) *perspCamera {
	if fov <= 0 {
		fov = defaultFOV
	}
	c := &perspCamera{world: world, fov: min(fov, math.Pi-0.01), minPitch: proj.MinPitch, cell: proj.Cell, headroom: proj.Headroom, ground: ground, extent: extent,
		viewportSize: geom.NewVec(viewport.BottomRight.X-viewport.TopLeft.X, viewport.BottomRight.Y-viewport.TopLeft.Y),
		heading:      proj.Heading, pitch: proj.Pitch, narrow: 1}
	c.look()
	c.CenterOn((viewport.TopLeft.X+viewport.BottomRight.X)/2, (viewport.TopLeft.Y+viewport.BottomRight.Y)/2, 0)
	return c
}

// focal is the focal length in screen units: the screen's height fills the field of view.
func (c *perspCamera) focal() float32 {
	return float32(c.viewportSize.Y) / 2 / float32(math.Tan(float64(c.fov)/2))
}

// far is how far along its line a screen point looking past the ground is put.
func (c *perspCamera) far() float32 { return 4 * float32(max(c.world.X, c.world.Y)) }

// near is how near the eye a point may lie and be drawn where it lies: a quarter cell flying, a
// thousandth of one riding in a unit, whose eye may stand but a unit's height over the ground.
func (c *perspCamera) near() float32 {
	if c.inside {
		return c.cell / 1024
	}
	return c.cell / 4
}

// Eye is where the eye stands.
func (c *perspCamera) Eye() (float32, float32, float32, bool) {
	e := c.proj.eye
	return e[0], e[1], e[2], true
}

// ceiling is the lowest the eye flies: eyeOver cells over the highest ground.
func (c *perspCamera) ceiling() float32 {
	var high float32
	if c.extent != nil {
		_, high = c.extent()
	}
	return high + eyeOver*c.cell
}

// layer is the heights between which anything may be drawn: the lowest ground, sea level at most,
// up to headroom over the highest.
func (c *perspCamera) layer() (low, high float32) {
	if c.extent != nil {
		low, high = c.extent()
	}
	return low, high + c.headroom
}

// Vanish is where the direction (dx, dy, dz) vanishes on the screen, false for one not ahead of
// the eye.
func (c *perspCamera) Vanish(dx, dy, dz float32) (float32, float32, bool) {
	sx, sy, ok := c.proj.Vanish(dx, dy, dz)
	return sx + float32(c.viewportSize.X/2), sy + float32(c.viewportSize.Y/2), ok
}

// maxAlt is the highest the eye flies: where the middle of the screen, unnarrowed and at the
// flattest pitch, shows the world's diagonal across — one height whatever the heading and the
// pitch, so a turn or a tilt never moves the eye — or the world's diagonal, or where the bottom
// zoom puts it, whichever is lowest.
func (c *perspCamera) maxAlt() float32 {
	diagonal := float32(math.Hypot(c.world.X, c.world.Y))
	alt := diagonal
	if c.minZoomCfg > 0 {
		alt = c.focal() * float32(math.Sin(float64(max(c.pitch, c.minPitch)))) / c.minZoomCfg
	}
	across := diagonal * c.focal() / float32(c.viewportSize.X) * float32(math.Sin(float64(c.minPitch)))
	return min(alt, across)
}

// target is the ground point in the middle of the screen.
func (c *perspCamera) target() [3]float32 { return c.at }

// eye is where the eye stands.
func (c *perspCamera) eye() [3]float32 { return c.proj.eye }

// out is the way from what the eye looks at towards the eye, as heading and pitch have it.
func (c *perspCamera) out() [3]float32 {
	s, k := math.Sincos(float64(c.heading) + math.Pi/4)
	sp, cp := math.Sincos(float64(c.pitch))
	return [3]float32{float32(cp * s), float32(cp * k), float32(sp)}
}

// look draws from where the eye is, the way the heading and the pitch say — the eye over the
// ceiling, out of a unit — and finds the ground point in the middle of the screen.
func (c *perspCamera) look() {
	c.heading = wrapAngle(c.heading)
	c.pitch = min(max(c.pitch, -maxPitch), maxPitch)
	c.narrow = min(max(c.narrow, 1), narrowest)
	if !c.inside {
		c.alt = min(max(c.alt, c.ceiling()), max(c.maxAlt(), c.ceiling()))
	}
	e := [3]float32{float32(c.origin.X), float32(c.origin.Y), c.alt}
	ahead := add(e, scale(c.out(), -c.far())) // far along the way looked, for the frame's precision
	c.proj = newPerspective(e, ahead, c.heading, c.focal()*c.narrow, c.near(), c.far(), c.cell)
	c.proj.bend = c.bend
	c.projection = c.proj
	c.at, c.sky = c.middle()
	if c.maxZoom > 0 { // the top zoom caps the narrowing
		if z := c.Zoom(); z > c.maxZoom && c.narrow > 1 {
			c.narrow = max(1, c.narrow*c.maxZoom/z)
			c.proj = newPerspective(e, ahead, c.heading, c.focal()*c.narrow, c.near(), c.far(), c.cell)
			c.proj.bend = c.bend
			c.projection = c.proj
		}
	}
}

// middle is the ground point the middle of the screen looks at, read on the relief; far along the
// way looked, and true, when it looks at the sky.
func (c *perspCamera) middle() ([3]float32, bool) {
	x, y, hit := c.Pick(float32(c.viewportSize.X/2), float32(c.viewportSize.Y/2))
	if !hit {
		return add(c.proj.eye, scale(c.proj.forward, c.far())), true
	}
	var z float32
	if c.ground != nil {
		z = c.ground(x, y)
	}
	return [3]float32{x, y, z}, false
}

// confine keeps the ground point in the middle of the screen over the world, moving the eye along
// the ground by however far that point lies outside; not inside a unit, which goes where it goes,
// nor when the middle looks at the sky.
func (c *perspCamera) confine() {
	if c.inside || c.sky {
		return
	}
	x := min(max(c.at[0], 0), float32(c.world.X))
	y := min(max(c.at[1], 0), float32(c.world.Y))
	if x == c.at[0] && y == c.at[1] {
		return
	}
	c.origin = geom.NewVec(c.origin.X+float64(x-c.at[0]), c.origin.Y+float64(y-c.at[1]))
	c.look()
}

// aim turns the head — the heading, and the pitch no flatter than the floor — until the point a
// is drawn at (sx, sy) again, from where narrowing or widening the view about the middle of the
// screen has moved it: the view swings towards the point zoomed at. Where the floor holds the
// head, the eye flies on along the ground towards the point instead, as far as puts it back.
func (c *perspCamera) aim(a [3]float32, sx, sy float32) {
	for range 8 {
		ax, ay := c.Project(a[0], a[1], a[2])
		ex, ey := ax-sx, ay-sy
		if math.Abs(float64(ex)) < 1e-2 && math.Abs(float64(ey)) < 1e-2 {
			return
		}
		f := c.focal() * c.narrow
		c.heading -= ex / f
		pitch := c.pitch + ey/f
		c.pitch = min(max(pitch, c.minPitch), maxPitch)
		if pitch < c.pitch {
			c.advance(a, sy)
		}
		c.look()
	}
}

// advance moves the eye along the ground, the way it looks, to where the point a is drawn sy down
// the screen at the pitch as it stands: nearer draws it lower.
func (c *perspCamera) advance(a [3]float32, sy float32) {
	f := c.focal() * c.narrow
	below := float64(c.pitch) + math.Atan(float64(sy-float32(c.viewportSize.Y/2))/float64(f)) // how far under the horizontal the point should lie
	e := c.proj.eye
	drop := float64(e[2] - a[2])
	if below <= 1e-4 || drop <= 0 {
		return // no distance draws it there
	}
	s, k := math.Sincos(float64(c.heading) + math.Pi/4) // the eye looks along the ground the way -(s, k)
	ahead := -(float64(a[0]-e[0])*s + float64(a[1]-e[1])*k)
	on := ahead - drop/math.Tan(below)
	c.origin = geom.NewVec(c.origin.X-on*s, c.origin.Y-on*k)
}

// Pick is the first ground the screen point sees, along its line from the eye, the ground's curve
// and all: up a slope higher than the eye too.
func (c *perspCamera) Pick(sx, sy float32) (float32, float32, bool) {
	if c.ground == nil {
		return c.cast(sx, sy, 0)
	}
	_, high := c.layer()
	s := sight{o: c.proj.eye, d: c.proj.ray(sx-float32(c.viewportSize.X/2), sy-float32(c.viewportSize.Y/2)), bend: c.bend}
	return s.pick(c.ground, c.far(), c.cell/2, high)
}

// anglesOf is the heading and the pitch of an eye at eye looking at target; heading stays as
// given for an eye straight over the target.
func anglesOf(eye, target [3]float32, heading float32) (float32, float32) {
	hx, hy := float64(eye[0]-target[0]), float64(eye[1]-target[1])
	h, dz := math.Hypot(hx, hy), float64(eye[2]-target[2])
	if h > 1e-6 {
		heading = float32(math.Atan2(hx, hy) - math.Pi/4)
	}
	return wrapAngle(heading), float32(math.Atan2(dz, h))
}

// wrapAngle is a, 0 up to 2π.
func wrapAngle(a float32) float32 {
	h := math.Mod(float64(a), 2*math.Pi)
	if h < 0 {
		h += 2 * math.Pi
	}
	return float32(h)
}

// LookFrom puts the eye at the world point (x, y, z) — over the ceiling — looking at the ground
// point it looks at now; an eye inside a unit comes out.
func (c *perspCamera) LookFrom(x, y, z float32) {
	at := c.at
	c.inside, c.narrow = false, 1
	c.origin, c.alt = geom.NewVec(float64(x), float64(y)), max(z, c.ceiling())
	c.heading, c.pitch = anglesOf([3]float32{x, y, c.alt}, at, c.heading)
	c.look()
}

// LookAt has the eye look at the world point (x, y, z) from where it is: below the pitch floor
// too, until the next Tilt; an eye inside a unit comes out.
func (c *perspCamera) LookAt(x, y, z float32) {
	c.inside, c.narrow = false, 1
	c.alt = max(c.alt, c.ceiling())
	c.heading, c.pitch = anglesOf([3]float32{float32(c.origin.X), float32(c.origin.Y), c.alt}, [3]float32{x, y, z}, c.heading)
	c.look()
}

// enterInside makes the eye a unit's at eye, looking along the ground from heading: where
// CenterOn moves it and Turn turns it.
func (c *perspCamera) enterInside(eye [3]float32, heading float32) {
	c.inside, c.narrow = true, 1
	c.origin, c.alt = geom.NewVec(float64(eye[0]), float64(eye[1])), eye[2]
	c.heading, c.pitch = heading, 0
	c.look()
}

// underEye is the ground point under the eye.
func (c *perspCamera) underEye() [3]float32 {
	e := c.proj.eye
	at := [3]float32{e[0], e[1], 0}
	if c.ground != nil {
		at[2] = c.ground(e[0], e[1])
	}
	return at
}

// perspPose is a free eye as it stands: where, how high, the heading, the pitch and how far the
// view is narrowed.
type perspPose struct {
	origin                      geom.Vec
	alt, heading, pitch, narrow float32
}

// pose is the eye as it stands.
func (c *perspCamera) pose() perspPose {
	return perspPose{origin: c.origin, alt: c.alt, heading: c.heading, pitch: c.pitch, narrow: c.narrow}
}

// setPose has the eye stand as p says, free: out of any unit, no flatter than the pitch floor.
func (c *perspCamera) setPose(p perspPose) {
	c.inside = false
	c.origin, c.alt, c.heading, c.pitch, c.narrow = p.origin, p.alt, p.heading, max(p.pitch, c.minPitch), p.narrow
	c.look()
}

// Turn turns the view by angle radians, the world clockwise on the screen: round the ground point
// in the middle of the screen, the eye as high as it is; inside a unit, on the spot.
func (c *perspCamera) Turn(angle float32) {
	if c.inside {
		c.heading += angle
		c.look()
		return
	}
	at := c.at
	r := math.Hypot(c.origin.X-float64(at[0]), c.origin.Y-float64(at[1]))
	c.heading = wrapAngle(c.heading + angle)
	s, k := math.Sincos(float64(c.heading) + math.Pi/4)
	c.origin = geom.NewVec(float64(at[0])+s*r, float64(at[1])+k*r)
	c.look()
}

// Tilt bows the head by angle radians, raises it for a negative one: between the pitch floor and
// straight down, the eye where it is; inside a unit, straight up to straight down.
func (c *perspCamera) Tilt(angle float32) {
	floor := c.minPitch
	if c.inside {
		floor = -maxPitch
	}
	c.pitch = min(max(c.pitch+angle, floor), maxPitch)
	c.tilts++
	c.look()
}

func (c *perspCamera) Heading() float32 { return c.heading }
func (c *perspCamera) Pitch() float32   { return c.pitch }

// ScaleAt is how many screen units a world unit spans at the point (x, y, z), across the way the
// eye looks; 0 for a point not in front of the eye.
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

// SetViewport resizes the screen, the field of view filling its height still; the eye stays.
func (c *perspCamera) SetViewport(w, h float32) {
	if w <= 0 || h <= 0 {
		return
	}
	c.viewportSize = geom.NewVec(float64(w), float64(h))
	c.look()
}

func (c *perspCamera) Project(x, y, z float32) (float32, float32) {
	sx, sy := c.proj.Project(x, y, z)
	return sx + float32(c.viewportSize.X/2), sy + float32(c.viewportSize.Y/2)
}

func (c *perspCamera) Unproject(sx, sy, z float32) (float32, float32) {
	x, y, _ := c.cast(sx, sy, z)
	return x, y
}

// cast is the ground point at height z under the screen point, and whether the screen point looks
// at that height at all.
func (c *perspCamera) cast(sx, sy, z float32) (float32, float32, bool) {
	return c.proj.cast(sx-float32(c.viewportSize.X/2), sy-float32(c.viewportSize.Y/2), z)
}

func (c *perspCamera) Depth(x, y, z float32) float32 { return c.proj.Depth(x, y, z) }

func (c *perspCamera) ToScreen(x, y float32) (float32, float32)     { return c.Project(x, y, 0) }
func (c *perspCamera) FromScreen(sx, sy float32) (float32, float32) { return c.Unproject(sx, sy, 0) }

// ToScreenQuads is the screen rectangle round the projected world rectangle — a bound; renderers
// drawing through this camera project corners instead.
func (c *perspCamera) ToScreenQuads(x0, y0, x1, y1 float32, dst []contract.Quad) []contract.Quad {
	minX, minY, maxX, maxY := c.rect(x0, y0, x1, y1, 0, 0)
	return append(dst, contract.Quad{X0: minX, Y0: minY, X1: maxX, Y1: maxY, T0X: 0, T1X: 1, T0Y: 0, T1Y: 1})
}

// rect is the screen rectangle round a world box drawn between heights z0 and z1.
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

// Visible reports whether the box, drawn anywhere from the lowest ground up to headroom over the
// highest, meets the screen.
func (c *perspCamera) Visible(box contract.AABB) bool {
	low, high := c.layer()
	minX, minY, maxX, maxY := c.rect(float32(box.TopLeft.X), float32(box.TopLeft.Y), float32(box.BottomRight.X), float32(box.BottomRight.Y), low, high)
	return maxX > 0 && minX < float32(c.viewportSize.X) && maxY > 0 && minY < float32(c.viewportSize.Y)
}

// Bounds is the world rectangle round everything the screen may show: where the lines of sight
// through its edge run between the lowest ground and headroom over the highest — from the eye
// itself when it is among them, out to far where they never leave — clamped to the world. Along an
// edge of the screen the lines meet each height on a straight line, so the corners bound them, but
// for where an edge crosses the horizon: there they run to far, and the line whose leaving the
// layer comes just at far bounds those.
func (c *perspCamera) Bounds() contract.AABB {
	low, high := c.layer()
	e, far := c.proj.eye, c.proj.far
	low -= c.bend * (2 * far) * (2 * far) // the ground's curve sinks what lies far off: bound it straight, lower
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
				return // this line of sight never runs where anything is
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
					along(add(da, scale(sub(db, da), s)))
				}
			}
		}
	}
	if minX > maxX { // the whole screen looks over everything: nothing but where the eye is
		minX, maxX, minY, maxY = e[0], e[0], e[1], e[1]
	}
	return worldBox(minX, minY, maxX, maxY, c.world)
}

// MoveTo puts the ground point (x, y) under the screen's top-left corner, as far as that corner
// looks at the ground; else in the middle. Inside a unit, it moves the eye there.
func (c *perspCamera) MoveTo(x, y float64) {
	if c.inside {
		c.origin = geom.NewVec(x, y)
		c.look()
		return
	}
	at := c.at
	if tx, ty, hit := c.cast(0, 0, at[2]); hit {
		x, y = x+float64(at[0]-tx), y+float64(at[1]-ty)
	}
	c.CenterOn(x, y, float64(at[2]))
}

// CenterOn looks at the point (x, y) at height z: the eye, as high as it flies, stands back from
// it along the way it looks so that it is drawn in the middle of the screen, as far as the middle
// stays over the world. Inside a unit, it puts the eye there.
func (c *perspCamera) CenterOn(x, y, z float64) {
	if c.inside {
		c.origin, c.alt = geom.NewVec(x, y), float32(z)
		c.look()
		return
	}
	c.alt = max(c.alt, c.ceiling())
	r := 0.0
	if t := math.Tan(float64(c.pitch)); t > 1e-4 {
		r = float64(c.alt-float32(z)) / t
	}
	s, k := math.Sincos(float64(c.heading) + math.Pi/4)
	c.origin = geom.NewVec(x+s*r, y+k*r)
	c.look()
	c.confine()
}

// Translate moves the eye by a world delta along the ground, as far as the middle of the screen
// stays over the world.
func (c *perspCamera) Translate(dx, dy float64) {
	c.origin = geom.NewVec(c.origin.X+dx, c.origin.Y+dy)
	c.look()
	c.confine()
}

// Pan moves the eye along the ground as far as the ground point in the middle of the screen would
// move a screen delta, the same pixels at any height, as far as that point is on the ground and
// stays over the world. Inside a unit it does nothing: the eye goes where the unit goes.
func (c *perspCamera) Pan(dx, dy float32) {
	if c.inside {
		return
	}
	mx, my := float32(c.viewportSize.X/2), float32(c.viewportSize.Y/2)
	x0, y0, hit0 := c.cast(mx, my, c.at[2])
	x1, y1, hit1 := c.cast(mx+dx, my+dy, c.at[2])
	if !hit0 || !hit1 {
		return
	}
	c.origin = geom.NewVec(c.origin.X+float64(x1-x0), c.origin.Y+float64(y1-y0))
	c.look()
	c.confine()
}

// Zoom is how many screen units a world unit spans at the ground point in the middle of the
// screen.
func (c *perspCamera) Zoom() float32 {
	return c.focal() * c.narrow / max(norm(sub(c.proj.eye, c.at)), c.proj.near)
}

// ZoomIn brings the eye factor times nearer the ground under (anchorX, anchorY) along its line of
// sight to it — the point stays where it is drawn — down to the ceiling, and narrows the field of
// view from there, turning the head so the point still stays: less of the ground, larger, the
// view swinging towards the point. Zooming out widens the view back first, then lifts the eye, no
// higher than maxAlt. Inside a unit it narrows the view alone.
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
	bx, by := c.Project(a[0], a[1], a[2])
	narrow := c.narrow
	e := c.proj.eye
	if factor >= 1 {
		to := add(a, scale(sub(e, a), 1/factor))
		if floor := c.ceiling(); to[2] < floor {
			k := float32(1)
			if drop := e[2] - to[2]; drop > 0 {
				k = max(min((e[2]-floor)/drop, 1), 0)
			}
			to = add(e, scale(sub(to, e), k))
			achieved := norm(sub(e, a)) / max(norm(sub(to, a)), c.proj.near)
			c.narrow *= factor / max(achieved, 1e-3)
		}
		e = to
	} else {
		rest := factor
		if c.narrow > 1 {
			n := max(c.narrow*factor, 1)
			rest = factor * c.narrow / n
			c.narrow = n
		}
		if rest < 1 {
			to := add(a, scale(sub(e, a), 1/rest))
			if top := max(c.maxAlt(), c.ceiling()); to[2] > top { // the step shortened along the same line
				k := float32(1)
				if rise := to[2] - e[2]; rise > 0 {
					k = max(min((top-e[2])/rise, 1), 0)
				}
				to = add(e, scale(sub(to, e), k))
			}
			e = to
		}
	}
	c.origin, c.alt = geom.NewVec(float64(e[0]), float64(e[1])), e[2]
	c.look()
	if c.narrow != narrow {
		c.aim(a, bx, by)
	}
	c.confine()
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

// Persisted hands saves where the eye is and how high, the heading, the pitch and how far the
// view is narrowed; Restore draws from there, out of any unit: a save keeps the view, not the eye
// in a unit.
func (c *perspCamera) Persisted() []any {
	return []any{&c.origin, &c.alt, &c.heading, &c.pitch, &c.narrow}
}

func (c *perspCamera) Restore() {
	c.inside = false
	c.pitch = max(c.pitch, c.minPitch) // a game saved riding in a unit comes back free
	c.look()
	c.confine()
}
