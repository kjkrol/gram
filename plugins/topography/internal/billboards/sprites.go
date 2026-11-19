package billboards

import (
	"math"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/internal/vec"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/light"
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
	{Name: "SunShare", Size: 1},
	{Name: "Uncovering", Size: 1},
	{Name: "UncoveredIn", Size: 4},
}

var boards = render.NewMeshShaderWith("billboards",
	render.Files(shaders, "shaders/sight.wgsl", "shaders/billboard.wgsl"), sight).Instanced(5)

var lyings = render.NewMeshShaderWith("lying",
	render.Files(shaders, "shaders/sight.wgsl", "shaders/lying.wgsl"), sight).Instanced(7)

var wholes = render.NewMeshShaderWith("bodies",
	render.Files(shaders, "shaders/sight.wgsl", "shaders/body.wgsl"), sight).Instanced(4)

var uncovereds = render.NewMeshShaderWith("billboards uncovered",
	render.Files(shaders, "shaders/sight.wgsl", "shaders/uncovered.wgsl"), sight).Instanced(1)

// floats an instance of a billboard, of what lies or stands upright, of a body whole and of a
// disc round what is uncovered, and into how many slices of depth the bodies are counted to be
// drawn from the nearest
const (
	boardFloats = 20
	lainFloats  = 28
	bodyFloats  = 16
	discFloats  = 4
	bodySlices  = 256
)

// givesWay marks what gives way in the float its Taking rides in, past a Taking's bits, and
// shelterBit counts the shelter it is under past it (the library's givesWay, shelterOf)
const (
	givesWay   = 256
	shelterBit = 512
)

type sprites struct {
	sky    Sky
	drawn  func() (ground *relief.Relief, hazed bool)
	relief *relief.Relief // the frame's
	on     bool

	// the frame's sky: its sun, the sun's share of the light on what faces up, and its weather
	beam    light.Beam
	up      float32
	weather air.Weather

	batches []spriteBatch
	lain    []spriteBatch
	// the bodies standing whole, bodyFloats each, their kinds' table, and how deep each lies
	// along the way the camera looks; ordered, once they are laid from the nearest
	bodies  []float32
	table   *render.Image
	depths  []float32
	ordered bool
	scratch []float32
	// the way the camera looks where its lines of sight all run one way, else its eye
	way    [3]float32
	eye    [3]float32
	persp  bool
	haze   air.Hazer
	hazed  bool
	fog    float32 // how far into the fog before the camera's sight what is laid next lies
	cutout bool    // the billboards added next are drawn in the cutout pass
	// whether what is added next is uncovered, and gives way; the uncovered, a disc each: its
	// middle and how far it reaches; whether anything added gives way
	uncover  bool
	gives    bool
	discs    []float32
	anyGives bool
	opts     render.DrawMeshOptions
	own      map[string]any
}

type spriteBatch struct {
	atlas  render.AtlasSource
	cutout bool
	inst   []float32
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
	s.bodies, s.ordered = s.bodies[:0], false
	s.uncover, s.gives, s.discs, s.anyGives = false, false, s.discs[:0], false
	r, rays := cam.(camera.Rays)
	s.on = rays && inRelief(cam)
	if s.on {
		if f, ok := r.Rays(); ok {
			s.way, s.eye = f.Dir, f.Origin
			s.persp = f.DDX != ([3]float32{}) || f.DDY != ([3]float32{})
		}
	}
	s.relief, s.hazed = s.drawn()
	s.beam, s.weather = s.sky.Distant().Beam(), s.sky.Air()
	s.up = directShare(s.beam, 0, 0, 1)
	s.haze = air.Hazer{}
	if s.hazed {
		s.haze = s.weather.Hazer(cam)
	}
}

// taking is the shade taken as an instance carries it, with whether it gives way and the shelter
// it is under
func (s *sprites) taking(lit render.Illumination) float32 {
	t := float32(lit.Takes) + float32(lit.Shelter)*shelterBit
	if s.gives {
		s.anyGives = true
		return t + givesWay
	}
	return t
}

// bare has what stands over box from alt up high uncovered, where what is added next is: a
// disc round its middle, as far as its longer side
func (s *sprites) bare(box plane.AABB, alt, high float32) {
	if !s.uncover {
		return
	}
	s.discs = append(s.discs, box.TopLeft.X+box.Size.X/2, box.TopLeft.Y+box.Size.Y/2,
		alt+high/2, max(box.Size.X, box.Size.Y, high))
}

// sway is how much it bends in the wind: the shader leans its top by the frame's gusts
func (s *sprites) add(cam camera.Camera, box plane.AABB, z world.Z, atlas render.AtlasSource,
	id render.SpriteID, lit render.Illumination, sway float32) {
	x0, y0 := box.TopLeft.X, box.TopLeft.Y
	x1, y1 := box.BottomRight.X, box.BottomRight.Y
	alt, h := z.Altitude, tall(z, y0, y1)
	cx, cy := (x0+x1)/2, (y0+y1)/2
	haze := s.hazeAt(cx, cy, alt)
	u0, v0, u1, v1 := atlas.UV(id)
	l := lit.Light
	b := batchOf(&s.batches, atlas, s.cutout)
	b.inst = append(b.inst, cx, cy, alt, h, x1-x0, sway, 0, haze, u0, v0, u1, v1,
		l[0], l[1], l[2], 0, s.groundAt(cx, cy), s.taking(lit), 0, 0)
	s.bare(box, alt, h)
}

// card adds a kind's body as a card, the ways out of it beside: a billboard of the view of it
// nearest the side the camera sees it from, or, seen from steeply above, the view from above
// lying over its foot; turn is how the body is turned about its foot
func (s *sprites) card(box plane.AABB, z world.Z, atlas render.AtlasSource, b *world.Bodied,
	turn float32, lit render.Illumination, sway float32) {
	x0, y0 := box.TopLeft.X, box.TopLeft.Y
	cx, cy := (x0+box.BottomRight.X)/2, (y0+box.BottomRight.Y)/2
	tall, top := b.Tall, b.TopAt
	if b.Stretched {
		tall, top = z.Height, b.TopAt*z.Height/b.Tall
	}
	way := s.way
	if s.persp {
		way = [3]float32{cx - s.eye[0], cy - s.eye[1], z.Altitude + top - s.eye[2]}
	}
	view, ways, lies := b.Card(way, turn)
	u0, v0, u1, v1 := atlas.UV(view)
	wu, wv, _, _ := atlas.UV(ways)
	var lying float32
	if lies {
		// a whole turn on, so no turn says it stands
		const whole = 2 * math.Pi
		lying = whole + turn - whole*f32.Floor(turn/whole)
	}
	l := lit.Light
	bt := batchOf(&s.batches, atlas, s.cutout)
	bt.inst = append(bt.inst, cx, cy, z.Altitude, tall, b.Wide, sway, lying,
		s.hazeAt(cx, cy, z.Altitude), u0, v0, u1, v1, l[0], l[1], l[2], top,
		s.groundAt(cx, cy), s.taking(lit), wu+1, wv+1)
	s.bare(box, z.Altitude, tall)
}

// body adds a kind's body standing whole, turned about its foot by turn
func (s *sprites) body(box plane.AABB, z world.Z, b *world.Bodied, turn float32,
	lit render.Illumination, sway float32) {
	cx := (box.TopLeft.X + box.BottomRight.X) / 2
	cy := (box.TopLeft.Y + box.BottomRight.Y) / 2
	tall := b.Tall
	if b.Stretched {
		tall = z.Height
	}
	l := lit.Light
	s.table = b.Table
	s.bodies = append(s.bodies, cx, cy, z.Altitude, tall, b.Wide, float32(b.Row), turn,
		float32(b.Parts), l[0], l[1], l[2], s.hazeAt(cx, cy, z.Altitude), s.up,
		s.groundAt(cx, cy), s.taking(lit), sway)
	s.bare(box, z.Altitude, tall)
}

// nearestFirst lays the bodies from the nearest along the way the camera looks, counted into
// slices of depth, one pass and no sorting: only what is drawn before a body hides it from the
// pixels' work, and a slice is no deeper than a body or two
func (s *sprites) nearestFirst(way [3]float32) {
	n := len(s.bodies) / bodyFloats
	if s.ordered || n < 2 {
		return
	}
	s.ordered = true
	s.depths = s.depths[:0]
	low, high := float32(math.Inf(1)), float32(math.Inf(-1))
	for i := 0; i < len(s.bodies); i += bodyFloats {
		b := s.bodies[i : i+bodyFloats]
		d := b[0]*way[0] + b[1]*way[1] + (b[2]+b[3]/2)*way[2]
		s.depths = append(s.depths, d)
		low, high = min(low, d), max(high, d)
	}
	var slice float32
	if high > low {
		slice = (bodySlices - 1) / (high - low)
	}
	var at [bodySlices + 1]int
	for _, d := range s.depths {
		at[int((d-low)*slice)+1]++
	}
	for i := 1; i <= bodySlices; i++ {
		at[i] += at[i-1]
	}
	s.scratch = slices.Grow(s.scratch[:0], len(s.bodies))[:len(s.bodies)]
	for k, d := range s.depths {
		in := int((d - low) * slice)
		copy(s.scratch[at[in]*bodyFloats:], s.bodies[k*bodyFloats:(k+1)*bodyFloats])
		at[in]++
	}
	s.bodies, s.scratch = s.scratch, s.bodies
}

// angle in degrees; wherever the ground stands higher, the sprite lies on the ground
func (s *sprites) lay(cam camera.Camera, box plane.AABB, z world.Z, atlas render.AtlasSource,
	id render.SpriteID, angle float32, lit render.Illumination) {
	hw, hh := box.Size.X/2, box.Size.Y/2
	cx, cy := box.TopLeft.X+hw, box.TopLeft.Y+hh
	sin, cos := f32.Sincos(angle * math.Pi / 180)
	s32, c32 := sin, cos
	alt := z.Altitude
	b := batchOf(&s.lain, atlas, false)
	for _, k := range [4][2]float32{{-hw, -hh}, {hw, -hh}, {-hw, hh}, {hw, hh}} {
		x, y := cx+k[0]*c32+k[1]*s32, cy+k[1]*c32-k[0]*s32
		ground := s.groundAt(x, y)
		b.inst = append(b.inst, x, y, max(alt, ground), ground)
	}
	u0, v0, u1, v1 := atlas.UV(id)
	l := lit.Light
	b.inst = append(b.inst, u0, v0, u1, v1, l[0], l[1], l[2], s.hazeAt(cx, cy, alt),
		s.up, lyingSoft*min(hw, hh), s.taking(lit), 0)
	s.bare(box, alt, 0)
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
	b := batchOf(&s.lain, atlas, false)
	for _, p := range c {
		x, y := p.X, p.Y
		b.inst = append(b.inst, x, y, alt, s.groundAt(x, y))
	}
	mx, my := (c[0].X+c[3].X)/2, (c[0].Y+c[3].Y)/2
	u0, v0, u1, v1 := atlas.UV(id)
	l := lit.Light
	short := min(f32.Hypot(c[1].X-c[0].X, c[1].Y-c[0].Y),
		f32.Hypot(c[2].X-c[0].X, c[2].Y-c[0].Y))
	b.inst = append(b.inst, u0, v0, u1, v1, l[0], l[1], l[2], s.hazeAt(mx, my, alt),
		s.up, lyingSoft*short/2, s.taking(lit), 0)
}

// seen from the right of a to b, the way out of what it edges
func (s *sprites) upright(cam camera.Camera, a, b geom.Vec, alt, high float32,
	atlas render.AtlasSource, id render.SpriteID, lit render.Illumination) {
	if high <= 0 {
		return
	}
	bt := batchOf(&s.lain, atlas, false)
	ax, ay, bx, by := a.X, a.Y, b.X, b.Y
	top := alt + high
	ga, gb := s.groundAt(ax, ay), s.groundAt(bx, by)
	bt.inst = append(bt.inst, ax, ay, top, ga, bx, by, top, gb, ax, ay, alt, ga, bx, by, alt, gb)
	dx, dy := bx-ax, by-ay
	n := f32.Hypot(dx, dy)
	sun := faceLit(s.beam, -dy/n, dx/n, 0, lit)
	u0, v0, u1, v1 := atlas.UV(id)
	haze := s.hazeAt((ax+bx)/2, (ay+by)/2, alt)
	bt.inst = append(bt.inst, u0, v0, u1, v1, sun[0], sun[1], sun[2], haze,
		directShare(s.beam, -dy/n, dx/n, 0), uprightSoft*high, s.taking(lit), 0)
}

func batchOf(batches *[]spriteBatch, atlas render.AtlasSource, cutout bool) *spriteBatch {
	for i := range *batches {
		if (*batches)[i].atlas == atlas && (*batches)[i].cutout == cutout {
			return &(*batches)[i]
		}
	}
	*batches = append(*batches, spriteBatch{atlas: atlas, cutout: cutout})
	return &(*batches)[len(*batches)-1]
}

func (s *sprites) groundAt(x, y float32) float32 {
	return s.relief.GroundAt(geom.NewVec(x, y))
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
	s.set("SunShare", s.up)
	s.opts.Depth = t.Depth
	if t.Pass == render.Uncovered {
		s.drawDiscs(t, &tr, right, w, h)
		return
	}
	// what gives way reads where the uncovered lie, within the rectangle of the target's
	// pixels they lie in
	s.opts.ReadDepth = t.Uncovered
	s.set("Uncovering", flag(t.Uncovered != nil))
	x0, y0, x1, y1 := within(t, w, h)
	s.set("UncoveredIn", x0, y0, x1, y1)
	// the cutouts in their pass, before what is seen through; a billboard is cut at half its
	// alpha either way, so the pass decides its order alone
	cutting := t.Pass == render.Cutout
	if !cutting {
		s.drawAll(t, lyings, s.lain, false)
	}
	// the bodies before the billboards: they stand nearest
	if cutting && len(s.bodies) > 0 {
		s.nearestFirst(vec.Unit(f.Dir))
		s.opts.Images[0], s.opts.Instances = s.table, s.bodies
		s.opts.Vertices = 6 * world.MaxParts
		t.Screen.DrawMesh(nil, wholes, &s.opts)
		s.opts.Vertices = 6
	}
	s.drawAll(t, boards, s.batches, cutting)
}

func flag(on bool) float32 {
	if on {
		return 1
	}
	return 0
}

// within is the rectangle the uncovered lie in (render.Target.UncoveredIn, the pixels of a
// viewport w by h), in the pixels of the target's texture; the whole of it where none is told
func within(t render.Target, w, h float32) (x0, y0, x1, y1 float32) {
	b := t.Screen.Bounds()
	x, y := float32(b.Min.X), float32(b.Min.Y)
	sx, sy := float32(b.Dx())/w, float32(b.Dy())/h
	in := [4]float32{0, 0, w, h}
	if t.UncoveredIn != nil {
		in = *t.UncoveredIn
	}
	return x + in[0]*sx, y + in[1]*sy, x + in[2]*sx, y + in[3]*sy
}

// drawDiscs draws the discs of the uncovered into the target's depth, theirs, and grows the
// rectangle of the viewport w by h they lie in by each, as tr lays it
func (s *sprites) drawDiscs(t render.Target, tr *camera.Transform, right [3]float32,
	w, h float32) {
	if len(s.discs) == 0 {
		return
	}
	s.opts.ReadDepth, s.opts.Images[0] = nil, nil
	s.opts.Instances = s.discs
	t.Screen.DrawMesh(nil, uncovereds, &s.opts)
	in := t.UncoveredIn
	if in == nil {
		return
	}
	m := &tr.M
	// where a point goes on the screen, sunk as far off as it lies (sight.wgsl's project)
	at := func(x, y, z float32) (float32, float32, bool) {
		dx, dy := x-tr.Eye[0], y-tr.Eye[1]
		z -= tr.Bend * (dx*dx + dy*dy)
		cw := m[3]*x + m[7]*y + m[11]*z + m[15]
		if cw <= 0 {
			return 0, 0, false
		}
		return ((m[0]*x+m[4]*y+m[8]*z+m[12])/cw + 1) / 2 * w,
			(1 - (m[1]*x+m[5]*y+m[9]*z+m[13])/cw) / 2 * h, true
	}
	for k := 0; k < len(s.discs); k += discFloats {
		d := s.discs[k : k+discFloats]
		px, py, ok := at(d[0], d[1], d[2])
		qx, qy, ok2 := at(d[0]+right[0]*d[3], d[1]+right[1]*d[3], d[2])
		if !ok || !ok2 {
			continue
		}
		// as far up and down the screen as across it: the disc faces the eye
		r := f32.Hypot(qx-px, qy-py)
		in[0], in[1] = min(in[0], px-r), min(in[1], py-r)
		in[2], in[3] = max(in[2], px+r), max(in[3], py+r)
	}
}

func (s *sprites) drawAll(t render.Target, shader *render.Shader, batches []spriteBatch,
	cutouts bool) {
	for _, b := range batches {
		if len(b.inst) == 0 || b.cutout != cutouts {
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
