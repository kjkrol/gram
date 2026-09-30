package billboards

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/topography/internal/vec"
	"github.com/kjkrol/gram/plugins/topography/relief"
	"github.com/kjkrol/gram/plugins/topography/terrain"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// boards draws the world's entities in relief (shaders/billboard.wgsl), an instance an entity.
var boards = render.NewMeshShaderWith("billboards", render.Files(shaders, "shaders/billboard.wgsl"), []render.Uniform{
	{Name: "ViewProj", Size: 16}, {Name: "Eye", Size: 3}, {Name: "Bend", Size: 1}, {Name: "Right", Size: 3}, {Name: "Up", Size: 3},
	{Name: "Look", Size: 3}, {Name: "Perspective", Size: 1},
}).Instanced(4)

// sprites are the world's entities drawn on the GPU through a view in relief: billboards tested
// against the depth the ground left, so the hills hide what stands behind them, and their shadows
// laid on the ground before them — draped over the terrain's mesh, over hex prisms read from the
// frame's depth. Sprite takes them a frame at a time, Draw draws them.
type sprites struct {
	sky     Sky
	relief  *relief.Relief
	ground  *terrain.Renderer // nil over hex prisms, where shades lays the shadows
	shades  shades
	on      bool // the frame's camera draws the billboards here
	shading bool // and the shadows

	batches []spriteBatch
	shadows []sky.Patch
	opts    render.DrawMeshOptions
	own     map[string][]float32 // the billboards' own uniforms, boxed once in the options
}

// spriteBatch is the instances of one atlas's sprites.
type spriteBatch struct {
	atlas render.AtlasSource
	inst  []float32
}

func newSprites(sky Sky, r *relief.Relief, ground *terrain.Renderer) *sprites {
	s := &sprites{sky: sky, relief: r, ground: ground, own: map[string][]float32{}}
	s.opts = render.DrawMeshOptions{WriteDepth: true, Vertices: 6, Uniforms: map[string]any{}}
	return s
}

// begin readies a frame through cam: the billboards are drawn here through a view in relief with
// Rays, the shadows through any view with them.
func (s *sprites) begin(cam camera.Camera) {
	for i := range s.batches {
		s.batches[i].inst = s.batches[i].inst[:0]
	}
	s.shadows = s.shadows[:0]
	_, rays := cam.(camera.Rays)
	s.on, s.shading = rays && inRelief(cam), rays
}

// add takes the sprite id of atlas for an entity standing in box as z says, in light, swaying as
// much as sway says at the frame's time t, and its shadow.
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
	b := s.batch(atlas)
	b.inst = append(b.inst, cx, cy, alt, h, x1-x0, lx*h, ly*h, haze, u0, v0, u1, v1, l[0], l[1], l[2], 0)
	s.shadow(box, z)
}

// shadow takes the shadow of an entity standing in box as z says, laid on the ground away from the
// sun.
func (s *sprites) shadow(box plane.AABB, z world.Z) {
	if p, ok := s.sky.Sun().ShadowOf(box.AABB, z, s.groundAt); ok {
		s.shadows = append(s.shadows, p)
	}
}

// batch is the batch of atlas's sprites, made at its first.
func (s *sprites) batch(atlas render.AtlasSource) *spriteBatch {
	for i := range s.batches {
		if s.batches[i].atlas == atlas {
			return &s.batches[i]
		}
	}
	s.batches = append(s.batches, spriteBatch{atlas: atlas})
	return &s.batches[len(s.batches)-1]
}

// groundAt is the relief's height at a point, for the shadows.
func (s *sprites) groundAt(x, y float32) float32 {
	return float32(s.relief.GroundAt(geom.NewVec(float64(x), float64(y))))
}

// draw lays the frame's shadows on the ground and draws its billboards into the target through cam.
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
	for _, b := range s.batches {
		if len(b.inst) == 0 {
			continue
		}
		s.opts.Images[0], s.opts.Instances = b.atlas.Atlas(), b.inst
		t.Screen.DrawMesh(nil, boards, &s.opts)
	}
}

// set hands the billboards' shader the uniform name as v, kept between frames and boxed once.
func (s *sprites) set(name string, v ...float32) {
	o, ok := s.own[name]
	if !ok || len(o) != len(v) {
		o = make([]float32, len(v))
		s.own[name] = o
	}
	copy(o, v)
	s.opts.Uniforms[name] = o
}
