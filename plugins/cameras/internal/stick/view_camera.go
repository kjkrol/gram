package stick

import (
	"math"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	contract "github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/vec"
)

type viewCamera struct {
	iso     *isoCamera // from above, and isometric
	persp   *perspCamera
	cur     contract.Camera
	inPersp bool
	reaches bool
	// the view the eye was in before it went into a unit, to come back to
	wasPersp  bool
	was       perspPose
	fastening contract.Fastening
	// radians a pixel of a pan the world's edge stopped
}

var _ contract.Camera = (*viewCamera)(nil)
var _ contract.Fastenable = (*viewCamera)(nil)
var _ contract.Vanisher = (*viewCamera)(nil)
var _ contract.Scaler = (*viewCamera)(nil)
var _ contract.Eyed = (*viewCamera)(nil)
var _ contract.Picker = (*viewCamera)(nil)
var _ contract.Rayer = (*viewCamera)(nil)
var _ contract.Rays = (*viewCamera)(nil)

// From above or isometrically all one way, from Headroom over the highest ground.
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
	// a pixel's step taken by the origin: between two far points it loses its last digits
	p, k := c.iso.proj, 1/c.iso.zoom
	ox, oy := p.Unproject(0, 0, high)
	x1, y1 := p.Unproject(k, 0, high)
	x2, y2 := p.Unproject(0, k, high)
	f := contract.RayField{
		Origin: [3]float32{x0, y0, high},
		DX:     [3]float32{x1 - ox, y1 - oy, 0},
		DY:     [3]float32{x2 - ox, y2 - oy, 0},
	}
	f.Dir = vec.Scale(c.iso.Projection().Toward(), -1)
	return f, true
}

func (c *viewCamera) Ray(sx, sy float32) (float32, float32, float32, bool) {
	if !c.inPersp {
		return 0, 0, 0, false
	}
	return c.persp.Ray(sx, sy)
}

func (c *viewCamera) Pick(sx, sy float32) (float32, float32, float32, bool) {
	if c.inPersp {
		return c.persp.Pick(sx, sy)
	}
	return c.iso.Pick(sx, sy)
}

func (c *viewCamera) Eye() (float32, float32, float32, bool) {
	if !c.inPersp {
		return 0, 0, 0, false
	}
	return c.persp.Eye()
}

func (c *viewCamera) Vanish(dx, dy, dz float32) (float32, float32, bool) {
	if !c.inPersp {
		return 0, 0, false
	}
	return c.persp.Vanish(dx, dy, dz)
}

// never turned inside out: a rectangle off the world is an empty one at its edge
func worldBox(minX, minY, maxX, maxY float32, size geom.Vec) contract.AABB {
	w, h := float32(size.X), float32(size.Y)
	x0, y0 := min(max(minX, 0), w), min(max(minY, 0), h)
	x1, y1 := max(min(maxX, w), x0), max(min(maxY, h), y0)
	return contract.AABB{TopLeft: geom.NewVec(float64(x0), float64(y0)), BottomRight: geom.NewVec(float64(x1), float64(y1))}
}

func (c *viewCamera) Fasten(f contract.Fastening) { c.fastening = f }

func (c *viewCamera) Fastening() contract.Fastening { return c.fastening }

// fov is radians; the camera starts a pixel wide until whoever shows it gives it its size
func newCamera(
	proj projection, width, height uint32, edges aabbworld.Edges, cfg contract.Config, fov float32,
	reaches bool, ground func(x, y float32) float32, extent func() (low, high float32),
	bend float32,
) *viewCamera {
	if cfg.Whole {
		panic("topography: a camera in relief cannot keep the whole world in view (camera.Config.Whole): that is a top-down camera's")
	}
	vp := geom.NewAABBAt(geom.NewVec(float64(width)/2-0.5, float64(height)/2-0.5), 1, 1)
	world := geom.NewVec(float64(width), float64(height))
	iso := newIsoCamera(proj, world, vp, edges)
	if cfg.Zoom > 0 {
		iso.zoom = cfg.Zoom
		iso.CenterOn(world.X/2, world.Y/2, 0)
	}
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

func (c *viewCamera) relief() bool { return c.inPersp || !c.iso.proj.flat }

func (c *viewCamera) insideUnit() bool { return c.inPersp && c.persp.inside }

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

func (c *viewCamera) Turn(angle float32) {
	if c.inPersp {
		c.persp.Turn(angle)
		return
	}
	c.iso.Turn(angle)
}

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

func (c *viewCamera) enterPersp() bool {
	if !c.inPersp && c.reaches {
		c.toPersp()
	}
	return c.inPersp
}

func (c *viewCamera) LookFrom(x, y, z float32) bool {
	if !c.enterPersp() {
		return false
	}
	c.persp.LookFrom(x, y, z)
	return true
}

func (c *viewCamera) LookAt(x, y, z float32) bool {
	if !c.enterPersp() {
		return false
	}
	c.persp.LookAt(x, y, z)
	return true
}

// across is radians, 0 the camera's own field
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

func (c *viewCamera) Pan(dx, dy float32) { c.shift(dx, dy) }

// bx, by: the part of the delta the world's edge stopped, screen pixels
func (c *viewCamera) shift(dx, dy float32) (bx, by float32) {
	if c.inPersp {
		return c.persp.shift(dx, dy)
	}
	return c.iso.shift(dx, dy)
}

func (c *viewCamera) Zoom() float32 { return c.cur.Zoom() }
func (c *viewCamera) ZoomIn(factor, anchorX, anchorY float32) {
	c.cur.ZoomIn(factor, anchorX, anchorY)
}
func (c *viewCamera) ZoomOut(factor, anchorX, anchorY float32) {
	c.cur.ZoomOut(factor, anchorX, anchorY)
}
func (c *viewCamera) State() contract.State { return c.cur.State() }

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

func (c *viewCamera) Persisted() []any {
	return append(append(c.iso.Persisted(), c.persp.Persisted()...), &c.inPersp, &c.fastening)
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
