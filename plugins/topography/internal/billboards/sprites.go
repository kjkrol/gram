package billboards

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/topography/internal/relief"
	"github.com/kjkrol/gram/plugins/topography/internal/terrain"
	"github.com/kjkrol/gram/plugins/topography/internal/vec"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

var sight = []render.Uniform{
	{Name: "ViewProj", Size: 16}, {Name: "Eye", Size: 3}, {Name: "Bend", Size: 1}, {Name: "Right", Size: 3}, {Name: "Up", Size: 3},
	{Name: "Look", Size: 3}, {Name: "Perspective", Size: 1},
}

var boards = render.NewMeshShaderWith("billboards",
	render.Files(shaders, "shaders/sight.wgsl", "shaders/billboard.wgsl"), sight).Instanced(4)

var lyings = render.NewMeshShaderWith("lying",
	render.Files(shaders, "shaders/sight.wgsl", "shaders/lying.wgsl"), sight).Instanced(6)

type sprites struct {
	sky     Sky
	relief  *relief.Relief
	ground  *terrain.Renderer // nil over hex prisms, where shades lays the shadows
	shades  shades
	on      bool
	shading bool

	batches []spriteBatch
	lain    []spriteBatch
	shadows []sky.Patch
	opts    render.DrawMeshOptions
	own     map[string][]float32
}

type spriteBatch struct {
	atlas render.AtlasSource
	inst  []float32
}

func newSprites(sky Sky, r *relief.Relief, ground *terrain.Renderer) *sprites {
	s := &sprites{sky: sky, relief: r, ground: ground, own: map[string][]float32{}}
	s.opts = render.DrawMeshOptions{WriteDepth: true, Vertices: 6, Uniforms: map[string]any{}}
	return s
}

func (s *sprites) begin(f *render.Frame, cam camera.Camera) {
	for i := range s.batches {
		s.batches[i].inst = s.batches[i].inst[:0]
	}
	for i := range s.lain {
		s.lain[i].inst = s.lain[i].inst[:0]
	}
	s.shadows = s.shadows[:0]
	_, rays := cam.(camera.Rays)
	s.on, s.shading = rays && inRelief(cam), rays
	if s.shading && s.ground == nil {
		f.ReadGround() // the shades find the hex prisms under them
	}
}

func (s *sprites) add(cam camera.Camera, box plane.AABB, z world.Z, atlas render.AtlasSource, id render.SpriteID, light render.Light, sway, t float32) {
	x0, y0 := float32(box.TopLeft.X), float32(box.TopLeft.Y)
	x1, y1 := float32(box.BottomRight.X), float32(box.BottomRight.Y)
	if ridden(cam, x0, y0, x1, y1) {
		return // an eye does not see what it rides in
	}
	alt, h := float32(z.Altitude), tall(z, y0, y1)
	cx, cy := (x0+x1)/2, (y0+y1)/2
	sun, weather := s.sky.Sun(), s.sky.Air()
	var lx, ly float32
	if sway > 0 {
		lx, ly = weather.Sway(t, cx, cy, sway)
	}
	haze := weather.Haze(cam, cx, cy, alt)
	u0, v0, u1, v1 := atlas.UV(id)
	l := lit(sun, light)
	b := batchOf(&s.batches, atlas)
	b.inst = append(b.inst, cx, cy, alt, h, x1-x0, lx*h, ly*h, haze, u0, v0, u1, v1, l[0], l[1], l[2], 0)
	s.shadow(box, z)
}

// angle in degrees; wherever the ground stands higher, the sprite lies on the ground
func (s *sprites) lay(cam camera.Camera, box plane.AABB, z world.Z, atlas render.AtlasSource,
	id render.SpriteID, angle float32, light render.Light) {
	hw, hh := float32(box.Size.X)/2, float32(box.Size.Y)/2
	cx, cy := float32(box.TopLeft.X)+hw, float32(box.TopLeft.Y)+hh
	sin, cos := math.Sincos(float64(angle) * math.Pi / 180)
	s32, c32 := float32(sin), float32(cos)
	alt := float32(z.Altitude)
	b := batchOf(&s.lain, atlas)
	for _, k := range [4][2]float32{{-hw, -hh}, {hw, -hh}, {-hw, hh}, {hw, hh}} {
		x, y := cx+k[0]*c32+k[1]*s32, cy+k[1]*c32-k[0]*s32
		b.inst = append(b.inst, x, y, max(alt, s.groundAt(x, y)), 0)
	}
	u0, v0, u1, v1 := atlas.UV(id)
	l := lit(s.sky.Sun(), light)
	b.inst = append(b.inst, u0, v0, u1, v1, l[0], l[1], l[2], s.sky.Air().Haze(cam, cx, cy, alt))
}

// c is top-left, top-right, bottom-left, bottom-right
func (s *sprites) level(cam camera.Camera, c [4]geom.Vec, alt float32, atlas render.AtlasSource,
	id render.SpriteID, light render.Light) {
	b := batchOf(&s.lain, atlas)
	for _, p := range c {
		b.inst = append(b.inst, float32(p.X), float32(p.Y), alt, 0)
	}
	mx, my := float32(c[0].X+c[3].X)/2, float32(c[0].Y+c[3].Y)/2
	u0, v0, u1, v1 := atlas.UV(id)
	l := lit(s.sky.Sun(), light)
	b.inst = append(b.inst, u0, v0, u1, v1, l[0], l[1], l[2], s.sky.Air().Haze(cam, mx, my, alt))
}

// seen from the right of a to b, the way out of what it edges
func (s *sprites) upright(cam camera.Camera, a, b geom.Vec, alt, high float32,
	atlas render.AtlasSource, id render.SpriteID, light render.Light) {
	if high <= 0 {
		return
	}
	bt := batchOf(&s.lain, atlas)
	ax, ay, bx, by := float32(a.X), float32(a.Y), float32(b.X), float32(b.Y)
	top := alt + high
	bt.inst = append(bt.inst, ax, ay, top, 0, bx, by, top, 0, ax, ay, alt, 0, bx, by, alt, 0)
	dx, dy := bx-ax, by-ay
	n := float32(math.Hypot(float64(dx), float64(dy)))
	sun := s.sky.Sun().Light(-dy/n, dx/n, 0)
	u0, v0, u1, v1 := atlas.UV(id)
	haze := s.sky.Air().Haze(cam, (ax+bx)/2, (ay+by)/2, alt)
	bt.inst = append(bt.inst, u0, v0, u1, v1,
		sun[0]*light[0], sun[1]*light[1], sun[2]*light[2], haze)
}

func (s *sprites) shadow(box plane.AABB, z world.Z) {
	if p, ok := s.sky.Sun().ShadowOf(box.AABB, z, s.groundAt); ok {
		s.shadows = append(s.shadows, p)
	}
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
	if !s.shading || t.Screen == nil || t.Depth == nil {
		return
	}
	if s.ground != nil {
		s.ground.DrawShadows(t, cam, u, s.shadows)
	} else {
		s.shades.draw(t, cam, s.relief, s.shadows)
	}
	if !s.on {
		return
	}
	f, _ := cam.(camera.Rays).Rays()
	w, h := cam.Viewport()
	tr, ok := camera.SceneTransform(f, w, h)
	if !ok {
		return
	}
	right, up, persp := f.DX, [3]float32{-f.DY[0], -f.DY[1], -f.DY[2]}, float32(0)
	if f.DDX != ([3]float32{}) || f.DDY != ([3]float32{}) {
		right, up, persp = f.DDX, [3]float32{-f.DDY[0], -f.DDY[1], -f.DDY[2]}, 1
	} else { // the screen's up, square to the way looked
		d := vec.Unit(f.Dir)
		up = vec.Sub(up, vec.Scale(d, vec.Dot(up, d)))
	}
	right, up = vec.Unit(right), vec.Unit(up)
	u.Into(s.opts.Uniforms)
	s.set("ViewProj", tr.M[:]...)
	s.set("Eye", tr.Eye[:]...)
	s.set("Bend", tr.Bend)
	s.set("Right", right[:]...)
	s.set("Up", up[:]...)
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
	o, ok := s.own[name]
	if !ok || len(o) != len(v) {
		o = make([]float32, len(v))
		s.own[name] = o
	}
	copy(o, v)
	s.opts.Uniforms[name] = o
}
