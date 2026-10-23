package instanced

import (
	_ "embed"
	"slices"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render/internal/gfx"
)

//go:embed sprites.wgsl
var spritesWGSL string

var spritesShader = gfx.NewMeshShaderWith("sprites", []byte(spritesWGSL+"\n"),
	[]gfx.Uniform{
		{Name: "ViewSize", Size: 2},
		{Name: "ViewProj", Size: 16},
		{Name: "Wrap", Size: 4},
		{Name: "Window", Size: 4},
		{Name: "Deep", Size: 1},
	}).Instanced(4)

// Sprites is sprites a Direct source draws on the GPU as instances, by Layer or else in the order
// they came, one call a run sharing an atlas: each a rectangle of the world the GPU lays on the
// screen as the camera does, what Frame.SpriteRectUV would lay, without the frame.
type Sprites struct {
	runs []spriteRun
	// four vec4s a piece: its rectangle in the world, on the atlas, its light and its angle, its
	// height, its fog, the shade it takes and the sun's share of its light
	instances []float32
	opts      DrawMeshOptions
	own       map[string][]float32
	wrap      [4]float32 // the world's width and height, and whether it wraps across and down

	layer   layer
	layered bool
	fog     float32
	takes   Taking
	sun     float32
	// by piece
	layers  []layer
	atlases []AtlasSource
	// the pieces as drawn by layer, kept from frame to frame
	order     []int32
	drawn     []float32
	drawnRuns []spriteRun
}

type spriteRun struct {
	atlas        AtlasSource
	first, count int
}

type layer struct {
	at    At
	under bool
}

// Fog has the pieces added next turn into the frame's Fog colour by amount, 0 clear to 1 all fog.
func (s *Sprites) Fog(amount float32) { s.fog = amount }

// Wrap has the sprites laid over a world w by h that wraps across where x says and down where y
// does, each where a camera's window sees it.
func (s *Sprites) Wrap(w, h float32, x, y bool) {
	s.wrap = [4]float32{w, h, flag(x), flag(y)}
}

func flag(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// Layer has the pieces added next lie at at, drawn back to front and at one place those under
// before the rest; pieces added before any Layer lie at the zero At.
func (s *Sprites) Layer(at At, under bool) {
	s.layer, s.layered = layer{at: at, under: under}, true
}

func (s *Sprites) piece(atlas AtlasSource) {
	if n := len(s.runs); n == 0 || s.runs[n-1].atlas != atlas {
		s.runs = append(s.runs, spriteRun{atlas: atlas, first: len(s.instances) / spriteFloats})
	}
	s.runs[len(s.runs)-1].count++
	s.layers, s.atlases = append(s.layers, s.layer), append(s.atlases, atlas)
}

// Rect adds the part u0..u1, v0..v1 of sprite id of atlas over the world rectangle (x0, y0)-(x1,
// y1), in light.
func (s *Sprites) Rect(
	atlas AtlasSource, id SpriteID, x0, y0, x1, y1, u0, v0, u1, v1 float32, light Light) {
	sx0, sy0, sx1, sy1 := inset(atlas.UV(id))
	w, h := sx1-sx0, sy1-sy0
	s.piece(atlas)
	s.instances = append(s.instances, x0, y0, x1, y1, sx0+u0*w, sy0+v0*h, sx0+u1*w, sy0+v1*h,
		light[0], light[1], light[2], 0, s.layer.at.Z(), s.fog, float32(s.takes), s.sun)
}

// Take has the pieces added next take the shade takes says.
func (s *Sprites) Take(takes Taking) { s.takes = takes }

// Direct has the pieces added next lit share of their light by the distant light, 0 to 1: what a
// shadow over them takes away.
func (s *Sprites) Direct(share float32) { s.sun = share }

// Turned adds the whole of sprite id of atlas over the world rectangle (x0, y0)-(x1, y1), turned
// by angle radians about its middle, anticlockwise as the world is seen from above, in light.
func (s *Sprites) Turned(
	atlas AtlasSource, id SpriteID, x0, y0, x1, y1, angle float32, light Light) {
	sx0, sy0, sx1, sy1 := atlas.UV(id)
	s.piece(atlas)
	s.instances = append(s.instances, x0, y0, x1, y1, sx0, sy0, sx1, sy1,
		light[0], light[1], light[2], angle, s.layer.at.Z(), s.fog, float32(s.takes), s.sun)
}

const spriteFloats = 16

// Reset drops what was gathered.
func (s *Sprites) Reset() {
	s.runs, s.instances = s.runs[:0], s.instances[:0]
	s.layers, s.atlases = s.layers[:0], s.atlases[:0]
	s.layer, s.layered, s.fog, s.takes, s.sun = layer{}, false, 0, TakesAll, 0
}

// Draw draws the gathered sprites into the target's screen, a viewport cam's size, as cam lays
// them under the frame's uniforms u, keeping them for another draw till Reset; into the target's
// depth, where it has one, each at its height, what shows of it written. A camera that cannot
// say its lines of sight draws none.
func (s *Sprites) Draw(t Target, cam camera.Camera, u Uniforms) {
	if t.Screen == nil || len(s.runs) == 0 {
		return
	}
	rays, ok := cam.(camera.Rays)
	if !ok {
		return
	}
	f, ok := rays.Rays()
	if !ok {
		return
	}
	w, h := cam.Viewport()
	tr, ok := camera.SceneTransform(f, w, h)
	if !ok {
		return
	}
	if s.own == nil {
		s.own, s.opts.Uniforms = map[string][]float32{}, map[string]any{}
	}
	u.Into(s.opts.Uniforms)
	s.set("ViewSize", w, h)
	s.set("ViewProj", tr.M[:]...)
	s.set("Wrap", s.wrap[:]...)
	// the camera's window on the world: where it begins and how much of it it spans
	s.set("Window", f.Origin[0], f.Origin[1], w*f.DX[0], h*f.DY[1])
	s.opts.Depth, s.opts.WriteDepth = t.Depth, t.Depth != nil
	s.set("Deep", flag(t.Depth != nil))
	// six vertices a copy of the world a piece may show in: one, or one past each seam it wraps at
	s.opts.Vertices = 6 * (1 + int(s.wrap[2])) * (1 + int(s.wrap[3]))
	instances, runs := s.instances, s.runs
	if s.layered {
		instances, runs = s.byLayer()
	}
	for _, r := range runs {
		s.opts.Images[0] = r.atlas.Atlas()
		s.opts.Instances = instances[spriteFloats*r.first : spriteFloats*(r.first+r.count)]
		t.Screen.DrawMesh(nil, spritesShader, &s.opts)
	}
}

// the uniform's slice is kept between frames, boxed once
func (s *Sprites) set(name string, v ...float32) {
	u, ok := s.own[name]
	if !ok || len(u) != len(v) {
		u = make([]float32, len(v))
		s.own[name] = u
		s.opts.Uniforms[name] = u
	}
	copy(u, v)
}

// the pieces in the order of their layers and the runs of one atlas they make, as they came where
// they lie in order already
func (s *Sprites) byLayer() ([]float32, []spriteRun) {
	s.order = s.order[:0]
	for i := range s.layers {
		s.order = append(s.order, int32(i))
	}
	before := func(a, b int32) int {
		x, y := s.layers[a], s.layers[b]
		switch {
		case x.at.Before(y.at), !y.at.Before(x.at) && x.under && !y.under:
			return -1
		case y.at.Before(x.at), !x.at.Before(y.at) && !x.under && y.under:
			return 1
		}
		return 0
	}
	if slices.IsSortedFunc(s.order, before) {
		return s.instances, s.runs
	}
	slices.SortStableFunc(s.order, before)
	s.drawn, s.drawnRuns = s.drawn[:0], s.drawnRuns[:0]
	for _, k := range s.order {
		if n := len(s.drawnRuns); n == 0 || s.drawnRuns[n-1].atlas != s.atlases[k] {
			run := spriteRun{atlas: s.atlases[k], first: len(s.drawn) / spriteFloats}
			s.drawnRuns = append(s.drawnRuns, run)
		}
		s.drawnRuns[len(s.drawnRuns)-1].count++
		s.drawn = append(s.drawn, s.instances[spriteFloats*k:spriteFloats*k+spriteFloats]...)
	}
	return s.drawn, s.drawnRuns
}

// half a texel in, so a quad's edge never samples the neighbouring sprite of the sheet
func inset(sx0, sy0, sx1, sy1 float32) (float32, float32, float32, float32) {
	return sx0 + 0.5, sy0 + 0.5, sx1 - 0.5, sy1 - 0.5
}
