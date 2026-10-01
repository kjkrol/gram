package cameras

import (
	"math"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	contract "github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/topography/internal/vec"
)

// viewCamera is the topography's camera: one of its views at a time — from above, isometric, in
// perspective — every call going to the view it is in, and next switching them round with the
// ground point in the middle of the screen, the heading, the pitch and the scale there carried
// over. The perspective view is reached only when the game says so.
type viewCamera struct {
	iso     *isoCamera // from above, and isometric
	persp   *perspCamera
	cur     contract.Camera
	inPersp bool // saved with the game
	reaches bool // whether next reaches the perspective view
	// the view the eye was in before it went into a unit, to come back to: the free perspective
	// as it stood, or the isometric camera, left as it was
	wasPersp bool
	was      perspPose
}

var _ contract.Camera = (*viewCamera)(nil)
var _ contract.Rider = (*viewCamera)(nil)
var _ contract.Vanisher = (*viewCamera)(nil)
var _ contract.Scaler = (*viewCamera)(nil)
var _ contract.Eyed = (*viewCamera)(nil)
var _ contract.Picker = (*viewCamera)(nil)
var _ contract.Rayer = (*viewCamera)(nil)
var _ contract.Rays = (*viewCamera)(nil)

// Rays is every line of sight at once: in perspective from the eye, through the picture plane a
// focal length ahead; from above or isometrically all one way — the way everything projects
// along, down from Headroom over the highest ground.
func (c *viewCamera) Rays() (contract.RayField, bool) {
	if c.inPersp {
		p := c.persp.proj
		w, h := c.persp.Viewport()
		f := contract.RayField{Origin: p.eye, Bend: p.bend}
		f.DDX, f.DDY = vec.Scale(p.right, 1/p.focal), vec.Scale(p.up, -1/p.focal)
		f.Dir = vec.Add(p.forward, vec.Add(vec.Scale(f.DDX, -w/2), vec.Scale(f.DDY, -h/2)))
		return f, true
	}
	_, high := c.iso.layer()
	x0, y0 := c.iso.Unproject(0, 0, high)
	x1, y1 := c.iso.Unproject(1, 0, high)
	x2, y2 := c.iso.Unproject(0, 1, high)
	f := contract.RayField{Origin: [3]float32{x0, y0, high}, DX: [3]float32{x1 - x0, y1 - y0, 0}, DY: [3]float32{x2 - x0, y2 - y0, 0}}
	f.Dir = vec.Scale(c.iso.Projection().Toward(), -1)
	return f, true
}

// Ray is the way the screen point looks, in perspective; the other views look from infinitely far.
func (c *viewCamera) Ray(sx, sy float32) (float32, float32, float32, bool) {
	if !c.inPersp {
		return 0, 0, 0, false
	}
	return c.persp.Ray(sx, sy)
}

// Pick is the first ground the screen point sees, in the view the camera is in.
func (c *viewCamera) Pick(sx, sy float32) (float32, float32, bool) {
	if c.inPersp {
		return c.persp.Pick(sx, sy)
	}
	return c.iso.Pick(sx, sy)
}

// Eye is where the eye stands in perspective; the other views have none.
func (c *viewCamera) Eye() (float32, float32, float32, bool) {
	if !c.inPersp {
		return 0, 0, 0, false
	}
	return c.persp.Eye()
}

// Vanish is where the direction (dx, dy, dz) vanishes on the screen: in perspective; the other
// views have no vanishing points.
func (c *viewCamera) Vanish(dx, dy, dz float32) (float32, float32, bool) {
	if !c.inPersp {
		return 0, 0, false
	}
	return c.persp.Vanish(dx, dy, dz)
}

// worldBox is the world rectangle minX, minY to maxX, maxY clamped to a world of size, never
// turned inside out: a rectangle off the world is an empty one at its edge.
func worldBox(minX, minY, maxX, maxY float32, size geom.Vec) contract.AABB {
	w, h := float32(size.X), float32(size.Y)
	x0, y0 := min(max(minX, 0), w), min(max(minY, 0), h)
	x1, y1 := max(min(maxX, w), x0), max(min(maxY, h), y0)
	return contract.AABB{TopLeft: geom.NewVec(float64(x0), float64(y0)), BottomRight: geom.NewVec(float64(x1), float64(y1))}
}

// FirstPerson reports whether the camera rides in a unit: its eye the unit's, the keys that move
// a free camera steering the unit instead.
func (c *viewCamera) FirstPerson() bool { return c.insideUnit() }

// newCamera is a camera over a width x height world drawn through proj, configured by cfg, over
// ground — its top as it is drawn — between the heights extent gives (nil: level at sea level),
// which Pick walks over, its perspective view of fov
// (radians), the ground far off sinking bend per distance² under the eye's level, reached when
// reaches; it refuses a wrapping world.
func newCamera(proj projection, width, height uint32, edges aabbworld.Edges, cfg contract.Config, fov float32, reaches bool, ground func(x, y float32) float32, extent func() (low, high float32), bend float32) *viewCamera {
	vp := geom.NewAABBAt(geom.NewVec(0, 0), float64(width), float64(height))
	if cfg.ViewportWidth != 0 && cfg.ViewportHeight != 0 {
		vp = geom.NewAABBAt(geom.NewVec(0, 0), float64(cfg.ViewportWidth), float64(cfg.ViewportHeight))
	}
	world := geom.NewVec(float64(width), float64(height))
	iso := newIsoCamera(proj, world, vp, edges)
	iso.extent, iso.ground = extent, ground
	persp := newPerspCamera(iso.proj, fov, world, vp, ground, extent)
	persp.bend = bend
	persp.look()
	c := &viewCamera{iso: iso, persp: persp, reaches: reaches}
	c.cur = c.iso
	if cfg.MinZoom > 0 {
		c.SetMinZoom(cfg.MinZoom)
	}
	if cfg.MaxZoom > 0 {
		c.SetMaxZoom(cfg.MaxZoom)
	}
	return c
}

// relief reports whether the camera sees the world in relief — isometrically or in perspective —
// rather than from above.
func (c *viewCamera) relief() bool { return c.inPersp || !c.iso.proj.flat }

// insideUnit reports whether the camera's eye is a unit's.
func (c *viewCamera) insideUnit() bool { return c.inPersp && c.persp.inside }

// next looks the next way round: from above, isometrically, in perspective when reached, from
// above again; an eye inside a unit comes out instead.
func (c *viewCamera) next() {
	switch {
	case c.insideUnit():
		c.leaveInside()
	case c.inPersp:
		c.toIso(false)
	case c.iso.proj.flat:
		c.iso.SetIsometric(true)
	case c.reaches:
		c.toPersp()
	default:
		c.iso.SetIsometric(false)
	}
}

// toPersp switches to the perspective view over the ground point in the middle of the screen, as
// large there as the isometric view drew it: the eye as high as that takes, over the ceiling, the
// view narrowed where the ceiling keeps it too far.
func (c *viewCamera) toPersp() {
	w, h := c.iso.Viewport()
	x, y := c.iso.Unproject(w/2, h/2, 0)
	scale := c.iso.scale()
	p := c.persp
	p.inside, p.narrow = false, 1
	p.heading, p.pitch = c.iso.heading, max(c.iso.pitch, p.minPitch)
	var z float32
	if p.ground != nil {
		z = p.ground(x, y)
	}
	p.alt = max(p.ceiling(), z+p.focal()*float32(math.Sin(float64(p.pitch)))/scale)
	p.CenterOn(float64(x), float64(y), float64(z))
	if natural := p.Zoom(); natural < scale {
		p.narrow = min(scale/natural, narrowest)
		p.look()
	}
	c.inPersp, c.cur = true, p
}

// toIso switches from the perspective view to the isometric one over the ground point in the
// middle of the screen, as large there as the perspective drew it, and on to the view from above
// as the isometric view goes there, a cell as wide as it was.
func (c *viewCamera) toIso(isometric bool) {
	at := c.persp.target()
	x, y := float64(at[0]), float64(at[1])
	scale := c.persp.Zoom()
	c.inPersp, c.cur = false, c.iso
	c.iso.SetIsometric(true)
	c.iso.CenterOn(x, y, 0)
	c.iso.ZoomIn(scale/c.iso.scale(), float32(x), float32(y))
	if !isometric {
		c.iso.SetIsometric(false)
	}
}

// Turn turns the view by angle radians, the world clockwise on the screen, round the ground point
// in the middle of the screen; a view from above does not turn.
func (c *viewCamera) Turn(angle float32) {
	if c.inPersp {
		c.persp.Turn(angle)
		return
	}
	c.iso.Turn(angle)
}

// Tilt bows the head by angle radians, raises it for a negative one, between the flattest and
// straight down; a view from above looks straight down already.
func (c *viewCamera) Tilt(angle float32) {
	if c.inPersp {
		c.persp.Tilt(angle)
		return
	}
	c.iso.Tilt(angle)
}

func (c *viewCamera) Heading() float32 {
	if c.inPersp {
		return c.persp.Heading()
	}
	return c.iso.Heading()
}

func (c *viewCamera) Pitch() float32 {
	if c.inPersp {
		return c.persp.Pitch()
	}
	return c.iso.Pitch()
}

// enterPersp switches to the perspective view when it is reached and reports whether the camera
// is in it now.
func (c *viewCamera) enterPersp() bool {
	if !c.inPersp && c.reaches {
		c.toPersp()
	}
	return c.inPersp
}

// LookFrom puts the eye at the world point, over the ceiling, looking at the ground point in the
// middle of the screen, in perspective: from another view it switches to the perspective one
// first, when reached; false when it is not, and nothing moves.
func (c *viewCamera) LookFrom(x, y, z float32) bool {
	if !c.enterPersp() {
		return false
	}
	c.persp.LookFrom(x, y, z)
	return true
}

// LookAt has the eye look at the world point from where it is — see LookFrom.
func (c *viewCamera) LookAt(x, y, z float32) bool {
	if !c.enterPersp() {
		return false
	}
	c.persp.LookAt(x, y, z)
	return true
}

// enterInside makes the eye a unit's at eye, looking along the ground from heading and seeing
// across radians across the screen (0: the camera's own field), in perspective, minding the view
// it was in to come back to; false when the perspective is not reached.
func (c *viewCamera) enterInside(eye [3]float32, heading, across float32) bool {
	if !c.reaches {
		return false
	}
	if !c.insideUnit() {
		c.wasPersp, c.was = c.inPersp, c.persp.pose()
	}
	c.inPersp, c.cur = true, c.persp
	c.persp.enterInside(eye, heading, across)
	return true
}

// leaveInside takes an eye inside a unit out, back to the view it was in, over the unit.
func (c *viewCamera) leaveInside() {
	if !c.insideUnit() {
		return
	}
	at := c.persp.underEye()
	c.persp.setPose(c.was)
	if c.wasPersp {
		c.persp.CenterOn(float64(at[0]), float64(at[1]), float64(at[2]))
		return
	}
	c.inPersp, c.cur = false, c.iso
	c.iso.CenterOn(float64(at[0]), float64(at[1]), float64(at[2]))
}

// ScaleAt is how many screen units a world unit spans at the point: the zoom, but for the
// perspective, where what is further off is smaller and what is not in front of the eye 0.
func (c *viewCamera) ScaleAt(x, y, z float32) float32 {
	if c.inPersp {
		return c.persp.ScaleAt(x, y, z)
	}
	return c.iso.Zoom()
}

func (c *viewCamera) Projection() contract.Projection            { return c.cur.Projection() }
func (c *viewCamera) Project(x, y, z float32) (float32, float32) { return c.cur.Project(x, y, z) }
func (c *viewCamera) Unproject(sx, sy, z float32) (float32, float32) {
	return c.cur.Unproject(sx, sy, z)
}
func (c *viewCamera) Depth(x, y, z float32) float32            { return c.cur.Depth(x, y, z) }
func (c *viewCamera) Viewport() (float32, float32)             { return c.cur.Viewport() }
func (c *viewCamera) ToScreen(x, y float32) (float32, float32) { return c.cur.ToScreen(x, y) }
func (c *viewCamera) FromScreen(sx, sy float32) (float32, float32) {
	return c.cur.FromScreen(sx, sy)
}
func (c *viewCamera) ToScreenQuads(x0, y0, x1, y1 float32, dst []contract.Quad) []contract.Quad {
	return c.cur.ToScreenQuads(x0, y0, x1, y1, dst)
}
func (c *viewCamera) Visible(box contract.AABB) bool { return c.cur.Visible(box) }
func (c *viewCamera) Bounds() contract.AABB          { return c.cur.Bounds() }
func (c *viewCamera) MoveTo(x, y float64)            { c.cur.MoveTo(x, y) }
func (c *viewCamera) CenterOn(x, y, z float64)       { c.cur.CenterOn(x, y, z) }
func (c *viewCamera) Translate(dx, dy float64)       { c.cur.Translate(dx, dy) }
func (c *viewCamera) Pan(dx, dy float32)             { c.cur.Pan(dx, dy) }
func (c *viewCamera) Zoom() float32                  { return c.cur.Zoom() }
func (c *viewCamera) ZoomIn(factor, anchorX, anchorY float32) {
	c.cur.ZoomIn(factor, anchorX, anchorY)
}
func (c *viewCamera) ZoomOut(factor, anchorX, anchorY float32) {
	c.cur.ZoomOut(factor, anchorX, anchorY)
}
func (c *viewCamera) State() contract.State { return c.cur.State() }

// SetViewport resizes the screen of every view, so the one switched to next is right too.
func (c *viewCamera) SetViewport(w, h float32) {
	c.iso.SetViewport(w, h)
	c.persp.SetViewport(w, h)
}

func (c *viewCamera) SetMinZoom(minZoom float32) {
	c.iso.SetMinZoom(minZoom)
	c.persp.SetMinZoom(minZoom)
}

func (c *viewCamera) SetMaxZoom(maxZoom float32) {
	c.iso.SetMaxZoom(maxZoom)
	c.persp.SetMaxZoom(maxZoom)
}

// Persisted hands saves every view's state and which view the camera is in; Restore puts them
// back, the perspective view falling back to the isometric one where the game no longer reaches
// it.
func (c *viewCamera) Persisted() []any {
	return append(append(c.iso.Persisted(), c.persp.Persisted()...), &c.inPersp)
}

func (c *viewCamera) Restore() {
	c.iso.Restore()
	c.persp.Restore()
	if c.inPersp && !c.reaches {
		c.inPersp = false
	}
	c.cur = c.iso
	if c.inPersp {
		c.cur = c.persp
	}
}
