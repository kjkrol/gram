package stick

import (
	"fmt"
	"slices"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	contract "github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/vec"
)

type viewCamera struct {
	form    form
	ortho   *orthoCamera
	persp   *perspCamera
	cur     contract.Camera
	inPersp bool
	reaches bool
	// the view the eye was in before it went into a unit, to come back to
	wasPersp bool
	was      perspPose
	// behind an entity, the tactical camera's own scale and stick, to come back to
	tactical  struct{ scale, length float32 }
	fastening contract.Fastening
	detail    contract.Detail
	layers    []contract.Layer
	layout    contract.Layout
	sharp     bool
	// its copy for the frame drawn, or the camera it is a copy of
	copy, origin *viewCamera
}

// form names how a stick camera is saved, so a save of another shape is refused at its first
// value
type form struct{}

const savedAs = "stick 3"

func (form) GobEncode() ([]byte, error) { return []byte(savedAs), nil }

func (*form) GobDecode(b []byte) error {
	if string(b) != savedAs {
		return fmt.Errorf("saved as %q, not %q", b, savedAs)
	}
	return nil
}

var _ contract.Camera = (*viewCamera)(nil)
var _ contract.Fastenable = (*viewCamera)(nil)
var _ contract.Vanisher = (*viewCamera)(nil)
var _ contract.Scaler = (*viewCamera)(nil)
var _ contract.Eyed = (*viewCamera)(nil)
var _ contract.Picker = (*viewCamera)(nil)
var _ contract.Rayer = (*viewCamera)(nil)
var _ contract.Rays = (*viewCamera)(nil)
var _ contract.Focused = (*viewCamera)(nil)
var _ contract.Detailed = (*viewCamera)(nil)
var _ contract.Headed = (*viewCamera)(nil)
var _ contract.Resolved = (*viewCamera)(nil)
var _ contract.Layered = (*viewCamera)(nil)
var _ contract.Cropping = (*viewCamera)(nil)
var _ contract.Figured = (*viewCamera)(nil)
var _ contract.Snapped = (*viewCamera)(nil)

// Snap takes the camera's copy for the frame drawn next: its own sticks, the one looked through
// its own.
func (c *viewCamera) Snap() {
	if c.copy == nil {
		c.copy = &viewCamera{ortho: &orthoCamera{}, persp: &perspCamera{}}
	}
	d := c.copy
	ortho, persp, layers := d.ortho, d.persp, d.layers
	*d = *c
	d.ortho, d.persp, d.layers = ortho, persp, append(layers[:0], c.layers...)
	c.ortho.copyInto(d.ortho)
	c.persp.copyInto(d.persp)
	d.cur = d.ortho
	if c.cur == contract.Camera(c.persp) {
		d.cur = d.persp
	}
	d.copy, d.origin = nil, c
}

func (c *viewCamera) Drawn() contract.Camera {
	if c.copy == nil {
		return c
	}
	return c.copy
}

func (c *viewCamera) Origin() contract.Camera {
	if c.origin == nil {
		return c
	}
	return c.origin
}

// Detail is how far the camera sees and how finely.
func (c *viewCamera) Detail() contract.Detail { return c.detail }

// SetDetail has the camera see as d says.
func (c *viewCamera) SetDetail(d contract.Detail) { c.detail = d }

// See has the camera see factor times as far.
func (c *viewCamera) See(factor float64) { c.detail = c.detail.Widened(factor) }

// In the parallel view all one way, from Headroom over the highest ground.
func (c *viewCamera) Rays() (contract.RayField, bool) {
	if c.inPersp {
		p := c.persp.proj
		cx, cy := c.persp.centre()
		f := contract.RayField{Origin: p.eye, Bend: p.bend}
		f.DDX, f.DDY = vec.Scale(p.right, 1/p.focal), vec.Scale(p.up, -1/p.focal)
		f.Dir = vec.Add(p.forward, vec.Add(vec.Scale(f.DDX, -cx), vec.Scale(f.DDY, -cy)))
		return f, true
	}
	_, high := c.ortho.layer()
	x0, y0 := c.ortho.Unproject(0, 0, high)
	// a pixel's step taken by the origin: between two far points it loses its last digits
	p, k := c.ortho.proj, 1/(c.ortho.scale*c.ortho.res)
	ox, oy := p.Unproject(0, 0, high)
	x1, y1 := p.Unproject(k, 0, high)
	x2, y2 := p.Unproject(0, k, high)
	f := contract.RayField{
		Origin: [3]float32{x0, y0, high},
		DX:     [3]float32{x1 - ox, y1 - oy, 0},
		DY:     [3]float32{x2 - ox, y2 - oy, 0},
	}
	f.Dir = vec.Scale(p.Toward(), -1)
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
	return c.ortho.Pick(sx, sy)
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

// Focus is the foot of the stick, or the eye riding in a unit.
func (c *viewCamera) Focus() (float32, float32, float32) {
	switch {
	case c.insideUnit():
		e := c.persp.proj.eye
		return e[0], e[1], e[2]
	case c.inPersp:
		return c.persp.foot[0], c.persp.foot[1], c.persp.foot[2]
	}
	f := c.ortho.foot
	return float32(f[0]), float32(f[1]), float32(f[2])
}

func (c *viewCamera) Fastening() contract.Fastening { return c.fastening }

// fov is radians; the camera starts a pixel wide until whoever shows it gives it its size
func newCamera(
	o optics,
	start Start,
	width, height uint32,
	edges aabbworld.Edges,
	cfg contract.Config,
	fov float32,
	reaches bool,
	ground func(x, y float32) float32,
	extent func() (low, high float32),
	bend float32,
) *viewCamera {
	if cfg.Whole {
		panic("cameras: a camera in relief cannot keep the whole world in view " +
			"(camera.Config.Whole): that is a top-down camera's")
	}
	vp := geom.NewAABBAt(geom.NewVec(float64(width)/2-0.5, float64(height)/2-0.5), 1, 1)
	world := geom.NewVec(float64(width), float64(height))
	ortho := newOrthoCamera(o, start, world, vp, edges)
	if cfg.Zoom > 0 {
		ortho.scale = cfg.Zoom
		ortho.CenterOn(world.X/2, world.Y/2, 0)
	}
	ortho.extent, ortho.ground = extent, ground
	persp := newPerspCamera(ortho.optics, ortho.Heading(), ortho.Pitch(), fov, world, vp, ground,
		extent)
	persp.bend = bend
	persp.look()
	c := &viewCamera{ortho: ortho, persp: persp, reaches: reaches}
	c.cur = c.ortho
	if cfg.MinZoom > 0 {
		c.SetMinZoom(cfg.MinZoom)
	}
	if cfg.MaxZoom > 0 {
		c.SetMaxZoom(cfg.MaxZoom)
	}
	return c
}

func (c *viewCamera) insideUnit() bool { return c.inPersp && c.persp.inside }

// out of the unit, or into the other projection keeping the foot, the turn, the pitch and the
// scale there
func (c *viewCamera) next() {
	switch {
	case c.insideUnit():
		c.leaveInside()
	case !c.reaches:
	case c.inPersp:
		c.toOrtho()
	default:
		c.toPersp()
	}
}

// the stick stood on what is seen in the middle, as long as the focal length over the scale
func (c *viewCamera) toPersp() {
	o, p := c.ortho, c.persp
	o.plant()
	p.inside, p.narrow, p.held, p.lift = false, 1, false, 0
	p.heading, p.pitch = o.Heading(), max(o.Pitch(), p.minPitch)
	p.foot = [3]float32{float32(o.foot[0]), float32(o.foot[1]), float32(o.foot[2])}
	p.length = p.focal() / o.Zoom()
	p.look()
	c.inPersp, c.cur = true, p
}

// the scale the focal length over the stick's length, in shown pixels
func (c *viewCamera) toOrtho() {
	p, o := c.persp, c.ortho
	o.turn, o.pitch = float64(p.Heading())+eighth, float64(p.Pitch())
	o.look()
	o.scale = o.limited(p.focal() / p.length / o.res)
	c.inPersp, c.cur = false, o
	o.CenterOn(float64(p.foot[0]), float64(p.foot[1]), float64(p.foot[2]))
}

func (c *viewCamera) Turn(angle float32) {
	if c.inPersp {
		c.persp.Turn(angle)
		return
	}
	c.ortho.Turn(angle)
}

func (c *viewCamera) Tilt(angle float32) {
	if c.inPersp {
		c.persp.Tilt(angle)
		return
	}
	c.ortho.Tilt(angle)
}

func (c *viewCamera) Heading() float32 {
	if c.inPersp {
		return c.persp.Heading()
	}
	return c.ortho.Heading()
}

func (c *viewCamera) Ahead() (x, y float32) {
	f := facing(c.Heading())
	return float32(f.X), float32(f.Y)
}

func (c *viewCamera) Pitch() float32 {
	if c.inPersp {
		return c.persp.Pitch()
	}
	return c.ortho.Pitch()
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
	c.inPersp, c.cur = false, c.ortho
	c.ortho.CenterOn(float64(at[0]), float64(at[1]), float64(at[2]))
}

func (c *viewCamera) ScaleAt(x, y, z float32) float32 {
	if c.inPersp {
		return c.persp.ScaleAt(x, y, z)
	}
	return c.ortho.Zoom()
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

func (c *viewCamera) Pan(dx, dy float32) { c.cur.Pan(dx, dy) }

func (c *viewCamera) Zoom() float32 { return c.cur.Zoom() }

// behind an entity the camera is no tactical one: nothing zooms it
func (c *viewCamera) ZoomIn(factor, anchorX, anchorY float32) {
	if c.fastening.How != contract.Behind {
		c.cur.ZoomIn(factor, anchorX, anchorY)
	}
}

func (c *viewCamera) ZoomOut(factor, anchorX, anchorY float32) {
	if c.fastening.How != contract.Behind {
		c.cur.ZoomOut(factor, anchorX, anchorY)
	}
}
func (c *viewCamera) State() contract.State { return c.cur.State() }

func (c *viewCamera) SetViewport(w, h float32) {
	c.ortho.SetViewport(w, h)
	c.persp.SetViewport(w, h)
}

func (c *viewCamera) SetResolution(s float32) {
	c.ortho.SetResolution(s)
	c.persp.SetResolution(s)
}

func (c *viewCamera) Resolution() float32 { return c.ortho.res }

func (c *viewCamera) Crop(win contract.Window, s float32) {
	c.ortho.Crop(win, s)
	c.persp.Crop(win, s)
}

func (c *viewCamera) Uncrop() {
	c.ortho.Uncrop()
	c.persp.Uncrop()
}

func (c *viewCamera) Cropped() bool { return c.ortho.Cropped() }

func (c *viewCamera) Frame() (float32, float32) { return c.ortho.Frame() }

func (c *viewCamera) Window() contract.Window { return c.ortho.Window() }

// SetLayers has the picture drawn in layers laid out as layout says, or whole for none or one.
func (c *viewCamera) SetLayers(layers []contract.Layer, layout contract.Layout) {
	c.layers, c.layout = slices.Clone(layers), layout
}

func (c *viewCamera) Layout() contract.Layout { return c.layout }

func (c *viewCamera) Layers() []contract.Layer { return slices.Clone(c.layers) }

// SetSharpFigures has the figures drawn at every shown pixel while the ground is drawn at the
// resolution, or at the resolution too.
func (c *viewCamera) SetSharpFigures(on bool) { c.sharp = on }

func (c *viewCamera) SharpFigures() bool { return c.sharp }

func (c *viewCamera) SetMinZoom(minZoom float32) {
	c.ortho.SetMinZoom(minZoom)
	c.persp.SetMinZoom(minZoom)
}

func (c *viewCamera) SetMaxZoom(maxZoom float32) {
	c.ortho.SetMaxZoom(maxZoom)
	c.persp.SetMaxZoom(maxZoom)
}

func (c *viewCamera) Persisted() []any {
	saved := append([]any{&c.form}, c.ortho.Persisted()...)
	return append(append(saved, c.persp.Persisted()...), &c.inPersp, &c.fastening,
		&c.detail.Sight)
}

func (c *viewCamera) Restore() {
	c.ortho.Restore()
	c.persp.Restore()
	if c.inPersp && !c.reaches {
		c.inPersp = false
	}
	c.cur = c.ortho
	if c.inPersp {
		c.cur = c.persp
	}
}
