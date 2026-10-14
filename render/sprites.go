package render

import (
	"slices"

	"github.com/kjkrol/gram/camera"
)

var spritesShader = NewMeshShaderWith("sprites", Files(shaderFiles, "shaders/sprites.wgsl"), []Uniform{{Name: "ViewSize", Size: 2}}).Instanced(3)

// Sprites is sprites a Direct source draws on the GPU as instances, by Layer or else in the order
// they came, one call a run sharing an atlas: what Frame.SpriteRectUV would lay, without the frame.
type Sprites struct {
	runs []spriteRun
	// three vec4s a piece: its rectangle on screen, on the atlas, its light and its angle
	instances []float32
	quads     []camera.Quad
	opts      DrawMeshOptions
	view      []float32

	layer   layer
	layered bool
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
	depth float32
	under bool
}

// Layer has the pieces added next drawn at depth, the lower first, and at one depth those under
// before the rest; pieces added before any Layer are at depth 0.
func (s *Sprites) Layer(depth float32, under bool) {
	s.layer, s.layered = layer{depth: depth, under: under}, true
}

func (s *Sprites) piece(atlas AtlasSource) {
	if n := len(s.runs); n == 0 || s.runs[n-1].atlas != atlas {
		s.runs = append(s.runs, spriteRun{atlas: atlas, first: len(s.instances) / 12})
	}
	s.runs[len(s.runs)-1].count++
	s.layers, s.atlases = append(s.layers, s.layer), append(s.atlases, atlas)
}

// Rect adds sprite id of atlas over the world rectangle (x0, y0)-(x1, y1) through cam, the part
// u0..u1, v0..v1 of it, split where it crosses a wrap seam, in light.
func (s *Sprites) Rect(cam camera.Camera, atlas AtlasSource, id SpriteID, x0, y0, x1, y1, u0, v0, u1, v1 float32, light Light) {
	s.quads = spritePieces(cam, atlas, id, x0, y0, x1, y1, u0, v0, u1, v1, s.quads[:0], func(q camera.Quad, a0, b0, a1, b1 float32) {
		s.piece(atlas)
		s.instances = append(s.instances, q.X0, q.Y0, q.X1, q.Y1, a0, b0, a1, b1, light[0], light[1], light[2], 0)
	})
}

// Turned adds the whole of sprite id of atlas over the world rectangle (x0, y0)-(x1, y1) through
// cam, turned by angle radians about its middle, in light; never split across a seam.
func (s *Sprites) Turned(cam camera.Camera, atlas AtlasSource, id SpriteID, x0, y0, x1, y1, angle float32, light Light) {
	sx0, sy0, sx1, sy1 := atlas.UV(id)
	s.quads = cam.ToScreenQuads(x0, y0, x1, y1, s.quads[:0])
	for _, q := range s.quads {
		s.piece(atlas)
		s.instances = append(s.instances, q.X0, q.Y0, q.X1, q.Y1, sx0, sy0, sx1, sy1, light[0], light[1], light[2], angle)
	}
}

// Len is how many pieces are gathered.
func (s *Sprites) Len() int { return len(s.instances) / 12 }

// Reset drops what was gathered.
func (s *Sprites) Reset() {
	s.runs, s.instances = s.runs[:0], s.instances[:0]
	s.layers, s.atlases = s.layers[:0], s.atlases[:0]
	s.layer, s.layered = layer{}, false
}

// Draw draws the gathered sprites into the target's screen, a viewport cam's size, and starts
// gathering anew.
func (s *Sprites) Draw(t Target, cam camera.Camera) {
	defer s.Reset()
	if t.Screen == nil || len(s.runs) == 0 {
		return
	}
	w, h := cam.Viewport()
	if s.view == nil {
		s.view = make([]float32, 2)
		s.opts.Uniforms = map[string]any{"ViewSize": s.view}
	}
	s.view[0], s.view[1] = w, h
	s.opts.Vertices = 6
	instances, runs := s.instances, s.runs
	if s.layered {
		instances, runs = s.byLayer()
	}
	for _, r := range runs {
		s.opts.Images[0] = r.atlas.Atlas()
		s.opts.Instances = instances[12*r.first : 12*(r.first+r.count)]
		t.Screen.DrawMesh(nil, spritesShader, &s.opts)
	}
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
		case x.depth < y.depth, x.depth == y.depth && x.under && !y.under:
			return -1
		case x.depth > y.depth, x.depth == y.depth && !x.under && y.under:
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
			run := spriteRun{atlas: s.atlases[k], first: len(s.drawn) / 12}
			s.drawnRuns = append(s.drawnRuns, run)
		}
		s.drawnRuns[len(s.drawnRuns)-1].count++
		s.drawn = append(s.drawn, s.instances[12*k:12*k+12]...)
	}
	return s.drawn, s.drawnRuns
}

func spritePieces(cam camera.Camera, atlas AtlasSource, id SpriteID, x0, y0, x1, y1, u0, v0, u1, v1 float32, quads []camera.Quad, piece func(q camera.Quad, a0, b0, a1, b1 float32)) []camera.Quad {
	sx0, sy0, sx1, sy1 := inset(atlas.UV(id))
	w, h := sx1-sx0, sy1-sy0
	quads = cam.ToScreenQuads(x0, y0, x1, y1, quads)
	for _, q := range quads {
		pu0, pu1 := u0+q.T0X*(u1-u0), u0+q.T1X*(u1-u0)
		pv0, pv1 := v0+q.T0Y*(v1-v0), v0+q.T1Y*(v1-v0)
		piece(q, sx0+pu0*w, sy0+pv0*h, sx0+pu1*w, sy0+pv1*h)
	}
	return quads
}
