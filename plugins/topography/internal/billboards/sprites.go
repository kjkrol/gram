package billboards

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/vec"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/topography/internal/relief"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

var sight = []render.Uniform{
	{Name: "ViewProj", Size: 16},
	{Name: "Eye", Size: 3},
	{Name: "Bend", Size: 1},
	{Name: "Right", Size: 3},
	{Name: "Look", Size: 3},
	{Name: "Perspective", Size: 1},
}

var boards = render.NewMeshShaderWith("billboards",
	render.Files(shaders, "shaders/sight.wgsl", "shaders/billboard.wgsl"), sight).Instanced(5)

var lyings = render.NewMeshShaderWith("lying",
	render.Files(shaders, "shaders/sight.wgsl", "shaders/lying.wgsl"), sight).Instanced(7)

// floats an instance of a billboard and of what lies or stands upright
const (
	boardFloats = 20
	lainFloats  = 28
)

type sprites struct {
	sky    Sky
	drawn  func() (ground *relief.Relief, hazed bool)
	relief *relief.Relief // the frame's
	on     bool

	batches []spriteBatch
	lain    []spriteBatch
	haze    air.Hazer
	hazed   bool
	fog     float32 // how far into the fog before the camera's sight what is laid next lies
	opts    render.DrawMeshOptions
	own     map[string]any
}

type spriteBatch struct {
	atlas render.AtlasSource
	inst  []float32
}

func newSprites(sky Sky, drawn func() (*relief.Relief, bool)) *sprites {
	s := &sprites{sky: sky, drawn: drawn, own: map[string]any{}}
	s.opts = render.DrawMeshOptions{WriteDepth: true, Vertices: 6, Uniforms: map[string]any{}}
	return s
}

func (s *sprites) begin(cam camera.Camera) {
	for i := range s.batches {
		s.batches[i].inst = s.batches[i].inst[:0]
	}
	for i := range s.lain {
		s.lain[i].inst = s.lain[i].inst[:0]
	}
	_, rays := cam.(camera.Rays)
	s.on = rays && inRelief(cam)
	s.relief, s.hazed = s.drawn()
	s.haze = air.Hazer{}
	if s.hazed {
		s.haze = s.sky.Air().Hazer(cam)
	}
}

func (s *sprites) add(cam camera.Camera, box plane.AABB, z world.Z, atlas render.AtlasSource,
	id render.SpriteID, lit render.Illumination, sway, t float32) {
	x0, y0 := float32(box.TopLeft.X), float32(box.TopLeft.Y)
	x1, y1 := float32(box.BottomRight.X), float32(box.BottomRight.Y)
	if ridden(cam, x0, y0, x1, y1) {
		return // an eye does not see what it rides in
	}
	alt, h := float32(z.Altitude), tall(z, y0, y1)
	cx, cy := (x0+x1)/2, (y0+y1)/2
	sun, weather := s.sky.Distant(), s.sky.Air()
	var lx, ly float32
	if sway > 0 {
		lx, ly = weather.Sway(t, cx, cy, sway)
	}
	haze := s.hazeAt(cx, cy, alt)
	u0, v0, u1, v1 := atlas.UV(id)
	l := lit.Light
	b := batchOf(&s.batches, atlas)
	b.inst = append(b.inst, cx, cy, alt, h, x1-x0, lx*h, ly*h, haze, u0, v0, u1, v1,
		l[0], l[1], l[2], directShare(sun, 0, 0, 1), s.groundAt(cx, cy), float32(lit.Takes), 0, 0)
}

// angle in degrees; wherever the ground stands higher, the sprite lies on the ground
func (s *sprites) lay(cam camera.Camera, box plane.AABB, z world.Z, atlas render.AtlasSource,
	id render.SpriteID, angle float32, lit render.Illumination) {
	hw, hh := float32(box.Size.X)/2, float32(box.Size.Y)/2
	cx, cy := float32(box.TopLeft.X)+hw, float32(box.TopLeft.Y)+hh
	sin, cos := math.Sincos(float64(angle) * math.Pi / 180)
	s32, c32 := float32(sin), float32(cos)
	alt := float32(z.Altitude)
	b := batchOf(&s.lain, atlas)
	for _, k := range [4][2]float32{{-hw, -hh}, {hw, -hh}, {-hw, hh}, {hw, hh}} {
		x, y := cx+k[0]*c32+k[1]*s32, cy+k[1]*c32-k[0]*s32
		ground := s.groundAt(x, y)
		b.inst = append(b.inst, x, y, max(alt, ground), ground)
	}
	u0, v0, u1, v1 := atlas.UV(id)
	sun := s.sky.Distant()
	l := lit.Light
	b.inst = append(b.inst, u0, v0, u1, v1, l[0], l[1], l[2], s.hazeAt(cx, cy, alt),
		directShare(sun, 0, 0, 1), lyingSoft*min(hw, hh), float32(lit.Takes), 0)
}

// how far into the relief's shadow what lies over the ground fades into it, of half its shorter
// side, and what stands upright of its height
const (
	lyingSoft   = 0.5
	uprightSoft = 0.25
)

// c is top-left, top-right, bottom-left, bottom-right
func (s *sprites) level(cam camera.Camera, c [4]geom.Vec, alt float32, atlas render.AtlasSource,
	id render.SpriteID, lit render.Illumination) {
	b := batchOf(&s.lain, atlas)
	for _, p := range c {
		x, y := float32(p.X), float32(p.Y)
		b.inst = append(b.inst, x, y, alt, s.groundAt(x, y))
	}
	mx, my := float32(c[0].X+c[3].X)/2, float32(c[0].Y+c[3].Y)/2
	u0, v0, u1, v1 := atlas.UV(id)
	sun := s.sky.Distant()
	l := lit.Light
	short := float32(min(math.Hypot(c[1].X-c[0].X, c[1].Y-c[0].Y),
		math.Hypot(c[2].X-c[0].X, c[2].Y-c[0].Y)))
	b.inst = append(b.inst, u0, v0, u1, v1, l[0], l[1], l[2], s.hazeAt(mx, my, alt),
		directShare(sun, 0, 0, 1), lyingSoft*short/2, float32(lit.Takes), 0)
}

// seen from the right of a to b, the way out of what it edges
func (s *sprites) upright(cam camera.Camera, a, b geom.Vec, alt, high float32,
	atlas render.AtlasSource, id render.SpriteID, lit render.Illumination) {
	if high <= 0 {
		return
	}
	bt := batchOf(&s.lain, atlas)
	ax, ay, bx, by := float32(a.X), float32(a.Y), float32(b.X), float32(b.Y)
	top := alt + high
	ga, gb := s.groundAt(ax, ay), s.groundAt(bx, by)
	bt.inst = append(bt.inst, ax, ay, top, ga, bx, by, top, gb, ax, ay, alt, ga, bx, by, alt, gb)
	dx, dy := bx-ax, by-ay
	n := float32(math.Hypot(float64(dx), float64(dy)))
	sky := s.sky.Distant()
	sun := faceLit(sky, -dy/n, dx/n, 0, lit)
	u0, v0, u1, v1 := atlas.UV(id)
	haze := s.hazeAt((ax+bx)/2, (ay+by)/2, alt)
	bt.inst = append(bt.inst, u0, v0, u1, v1, sun[0], sun[1], sun[2], haze,
		directShare(sky, -dy/n, dx/n, 0), uprightSoft*high, float32(lit.Takes), 0)
}

func batchOf(batches *[]spriteBatch, atlas render.AtlasSource) *spriteBatch {
	for i := range *batches {
		if (*batches)[i].atlas == atlas {
			return &(*batches)[i]
		}
	}
	*batches = append(*batches, spriteBatch{atlas: atlas})
	return &(*batches)[len(*batches)-1]
}

func (s *sprites) groundAt(x, y float32) float32 {
	return float32(s.relief.GroundAt(geom.NewVec(float64(x), float64(y))))
}

func (s *sprites) draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	if !s.on || t.Screen == nil || t.Depth == nil {
		return
	}
	f, _ := cam.(camera.Rays).Rays()
	w, h := cam.Viewport()
	tr, ok := camera.SceneTransform(f, w, h)
	if !ok {
		return
	}
	right, persp := f.DX, float32(0)
	if f.DDX != ([3]float32{}) || f.DDY != ([3]float32{}) {
		right, persp = f.DDX, 1
	}
	// level, so a billboard across it stands upright
	right = vec.Unit([3]float32{right[0], right[1], 0})
	u.Into(s.opts.Uniforms)
	s.set("ViewProj", tr.M[:]...)
	s.set("Eye", tr.Eye[:]...)
	s.set("Bend", tr.Bend)
	s.set("Right", right[:]...)
	s.set("Look", f.Dir[:]...)
	s.set("Perspective", persp)
	s.opts.Depth = t.Depth
	s.drawAll(t, lyings, s.lain)
	s.drawAll(t, boards, s.batches)
}

func (s *sprites) drawAll(t render.Target, shader *render.Shader, batches []spriteBatch) {
	for _, b := range batches {
		if len(b.inst) == 0 {
			continue
		}
		s.opts.Images[0], s.opts.Instances = b.atlas.Atlas(), b.inst
		t.Screen.DrawMesh(nil, shader, &s.opts)
	}
}

// kept between frames and boxed once
func (s *sprites) set(name string, v ...float32) {
	u, ok := s.own[name]
	if o, _ := u.([]float32); !ok || len(o) != len(v) {
		u = make([]float32, len(v))
		s.own[name] = u
	}
	copy(u.([]float32), v)
	s.opts.Uniforms[name] = u
}

// the air's haze at the point, at least the fog before the camera's sight
func (s *sprites) hazeAt(x, y, z float32) float32 { return max(s.haze.At(x, y, z), s.fog) }
