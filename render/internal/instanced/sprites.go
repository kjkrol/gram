package instanced

import (
	_ "embed"
	"slices"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render/internal/gfx"
)

//go:embed sprites.wgsl
var spritesWGSL string

//go:embed points.wgsl
var pointsWGSL string

//go:embed uncovered.wgsl
var uncoveredWGSL string

var spritesShader = gfx.NewMeshShaderWith("sprites", []byte(spritesWGSL+"\n"),
	[]gfx.Uniform{
		{Name: "ViewSize", Size: 2},
		{Name: "ViewProj", Size: 16},
		{Name: "Wrap", Size: 4},
		{Name: "Window", Size: 4},
		{Name: "Deep", Size: 1},
		{Name: "Cut", Size: 1},
		{Name: "Uncovering", Size: 1},
		{Name: "UncoveredIn", Size: 4},
	}).Instanced(4)

var uncoveredShader = gfx.NewMeshShaderWith("sprites uncovered", []byte(uncoveredWGSL+"\n"),
	[]gfx.Uniform{
		{Name: "ViewSize", Size: 2},
		{Name: "ViewProj", Size: 16},
		{Name: "Wrap", Size: 4},
		{Name: "Window", Size: 4},
	}).Instanced(4)

var pointsShader = gfx.NewMeshShaderWith("points", []byte(pointsWGSL+"\n"),
	[]gfx.Uniform{
		{Name: "ViewSize", Size: 2},
		{Name: "ViewProj", Size: 16},
		{Name: "Wrap", Size: 4},
		{Name: "Window", Size: 4},
		{Name: "Deep", Size: 1},
		{Name: "Eye", Size: 3},
		{Name: "Bend", Size: 1},
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
	cutout  bool
	fog     float32
	takes   Taking
	sun     float32
	shelter uint16
	// whether the pieces added next are uncovered, and give way; the uncovered, a piece's four
	// vec4s each, and whether any piece gives way
	uncover   bool
	gives     bool
	uncovered []float32
	anyGives  bool
	// by piece
	layers  []layer
	atlases []AtlasSource
	cutouts []bool
	anyCut  bool
	// the pieces as drawn by layer, kept from frame to frame
	order     []int32
	drawn     []float32
	drawnRuns []spriteRun
	// four vec4s a point: its box's middle and size, its part of the atlas, its light and the
	// height it is seen at, its fog, the shade it takes, the sun's share and its depth's height
	points    []float32
	pointRuns []spriteRun
	// the last point's sprite and its part of the atlas
	pointID SpriteID
	pointUV [4]float32
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

// Cutout has the pieces added next drawn in the cutout pass: whole, cut at half their alpha,
// writing the depth, front to back; off, among what is seen through.
func (s *Sprites) Cutout(on bool) { s.cutout = on }

// Uncover has the pieces added next uncovered: whatever gives way (GiveWay) is left out over
// them, thinning outwards, where it lies nearer than they do, in a frame with a depth.
func (s *Sprites) Uncover(on bool) { s.uncover = on }

// Uncovers reports whether any piece gathered is uncovered, and GivesWay whether any gives way:
// a source holding both asks for the pass (Frame.DrawUncovered).
func (s *Sprites) Uncovers() bool { return len(s.uncovered) > 0 }

// GivesWay reports whether any piece gathered gives way.
func (s *Sprites) GivesWay() bool { return s.anyGives }

// GiveWay has the pieces added next give way to those uncovered (Uncover), their own or another
// source's.
func (s *Sprites) GiveWay(on bool) { s.gives = on }

// what a piece takes of the shade, with whether it gives way and the shelter it is under: past
// a Taking's bits
func (s *Sprites) taking() float32 {
	t := float32(s.takes) + float32(s.shelter)*shelterBit
	if s.gives {
		s.anyGives = true
		return t + givesWay
	}
	return t
}

// givesWay marks a piece giving way in the float its Taking rides in, and shelterBit counts the
// shelter it is under past it (the library's givesWay, shelterOf)
const (
	givesWay   = 256
	shelterBit = 512
)

// bare keeps the last piece added among the uncovered
func (s *Sprites) bare() {
	if s.uncover {
		s.uncovered = append(s.uncovered, s.instances[len(s.instances)-spriteFloats:]...)
	}
}

func (s *Sprites) piece(atlas AtlasSource) {
	if n := len(s.runs); n == 0 || s.runs[n-1].atlas != atlas {
		s.runs = append(s.runs, spriteRun{atlas: atlas, first: len(s.instances) / spriteFloats})
	}
	s.runs[len(s.runs)-1].count++
	s.layers, s.atlases = append(s.layers, s.layer), append(s.atlases, atlas)
	s.cutouts = append(s.cutouts, s.cutout)
	s.anyCut = s.anyCut || s.cutout
}

// Rect adds the part u0..u1, v0..v1 of sprite id of atlas over the world rectangle (x0, y0)-(x1,
// y1), in light.
func (s *Sprites) Rect(
	atlas AtlasSource, id SpriteID, x0, y0, x1, y1, u0, v0, u1, v1 float32, light Light) {
	sx0, sy0, sx1, sy1 := inset(atlas.UV(id))
	w, h := sx1-sx0, sy1-sy0
	s.piece(atlas)
	s.instances = append(s.instances, x0, y0, x1, y1, sx0+u0*w, sy0+v0*h, sx0+u1*w, sy0+v1*h,
		light[0], light[1], light[2], 0, s.layer.at.Z(), s.fog, s.taking(), s.sun)
	s.bare()
}

// Take has the pieces added next take the shade takes says.
func (s *Sprites) Take(takes Taking) { s.takes = takes }

// Direct has the pieces added next lit share of their light by the distant light, 0 to 1: what a
// shadow over them takes away.
func (s *Sprites) Direct(share float32) { s.sun = share }

// Shelter has the pieces added next drawn under the frame's shelter at (Shadowing.Shelter), 0
// in the open: their light the lamps' alone, what of the sky's gets in worked out pixel by pixel.
func (s *Sprites) Shelter(at uint16) { s.shelter = at }

// Turned adds the whole of sprite id of atlas over the world rectangle (x0, y0)-(x1, y1), turned
// by angle radians about its middle, anticlockwise as the world is seen from above, in light.
func (s *Sprites) Turned(
	atlas AtlasSource, id SpriteID, x0, y0, x1, y1, angle float32, light Light) {
	sx0, sy0, sx1, sy1 := atlas.UV(id)
	s.piece(atlas)
	s.instances = append(s.instances, x0, y0, x1, y1, sx0, sy0, sx1, sy1,
		light[0], light[1], light[2], angle, s.layer.at.Z(), s.fog, s.taking(), s.sun)
	s.bare()
}

// Point adds sprite id of atlas as a point at the middle (x, y) of a box w by h, seen at height z
// and lying at height deep, in light: the box as large as it shows, a pixel at least each way, as
// much less opaque as it shows less. Drawn after the pieces, by no Layer.
func (s *Sprites) Point(atlas AtlasSource, id SpriteID, x, y, w, h, z, deep float32, light Light) {
	n := len(s.pointRuns)
	if fresh := n == 0 || s.pointRuns[n-1].atlas != atlas; fresh || id != s.pointID {
		if fresh {
			s.pointRuns = append(s.pointRuns, spriteRun{atlas: atlas,
				first: len(s.points) / spriteFloats})
			n++
		}
		s.pointID = id
		s.pointUV[0], s.pointUV[1], s.pointUV[2], s.pointUV[3] = inset(atlas.UV(id))
	}
	s.pointRuns[n-1].count++
	uv := &s.pointUV
	s.points = append(s.points, x, y, w, h, uv[0], uv[1], uv[2], uv[3],
		light[0], light[1], light[2], z, s.fog, float32(s.takes), s.sun, deep)
}

const spriteFloats = 16

// Reset drops what was gathered.
func (s *Sprites) Reset() {
	s.runs, s.instances = s.runs[:0], s.instances[:0]
	s.layers, s.atlases, s.cutouts, s.anyCut = s.layers[:0], s.atlases[:0], s.cutouts[:0], false
	s.points, s.pointRuns = s.points[:0], s.pointRuns[:0]
	s.layer, s.layered, s.cutout, s.fog, s.takes, s.sun = layer{}, false, false, 0, TakesAll, 0
	s.shelter = 0
	s.uncover, s.gives, s.uncovered, s.anyGives = false, false, s.uncovered[:0], false
}

// Draw draws the gathered sprites into the target's screen, a viewport cam's size, as cam lays
// them under the frame's uniforms u, keeping them for another draw till Reset; into the target's
// depth, where it has one, each at its height, what shows of it written. In the cutout pass the
// pieces added under Cutout alone, cut at half their alpha, front to back; in the pass seen
// through the rest and the points; in the pass of what is uncovered, where those added under
// Uncover lie. A camera that cannot say its lines of sight draws none.
func (s *Sprites) Draw(t Target, cam camera.Camera, u Uniforms) {
	cutting, uncovering := t.Pass == Cutout, t.Pass == Uncovered
	if t.Screen == nil || len(s.runs) == 0 && len(s.pointRuns) == 0 || cutting && !s.anyCut ||
		uncovering && (len(s.uncovered) == 0 || t.Depth == nil) {
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
	// six vertices a copy of the world a piece may show in: one, or one past each seam it wraps at
	s.opts.Vertices = 6 * (1 + int(s.wrap[2])) * (1 + int(s.wrap[3]))
	if uncovering {
		s.opts.ReadDepth, s.opts.Images[0] = nil, nil
		s.opts.Instances = s.uncovered
		t.Screen.DrawMesh(nil, uncoveredShader, &s.opts)
		if t.UncoveredIn != nil {
			s.within(t.UncoveredIn, &tr.M, w, h)
		}
		return
	}
	s.set("Deep", flag(t.Depth != nil))
	s.set("Cut", flag(cutting && t.Depth != nil))
	// what gives way reads where the uncovered lie, against its own depth, within the
	// rectangle they lie in: the whole viewport where none is told
	s.opts.ReadDepth = nil
	if t.Depth != nil {
		s.opts.ReadDepth = t.Uncovered
	}
	s.set("Uncovering", flag(s.opts.ReadDepth != nil))
	if in := t.UncoveredIn; in != nil {
		s.set("UncoveredIn", in[:]...)
	} else {
		s.set("UncoveredIn", 0, 0, w, h)
	}
	instances, runs := s.instances, s.runs
	if s.layered || s.anyCut {
		instances, runs = s.byLayer(cutting)
	}
	for _, r := range runs {
		s.opts.Images[0] = r.atlas.Atlas()
		s.opts.Instances = instances[spriteFloats*r.first : spriteFloats*(r.first+r.count)]
		t.Screen.DrawMesh(nil, spritesShader, &s.opts)
	}
	if len(s.pointRuns) == 0 || cutting {
		return
	}
	s.set("Eye", tr.Eye[:]...)
	s.set("Bend", tr.Bend)
	for _, r := range s.pointRuns {
		s.opts.Images[0] = r.atlas.Atlas()
		s.opts.Instances = s.points[spriteFloats*r.first : spriteFloats*(r.first+r.count)]
		t.Screen.DrawMesh(nil, pointsShader, &s.opts)
	}
}

// within grows in, a rectangle of a viewport w by h, to hold the discs of the uncovered as m
// lays them (uncovered.wgsl); the whole viewport over a world that wraps, where a piece shows
// in copies
func (s *Sprites) within(in *[4]float32, m *[16]float32, w, h float32) {
	if s.wrap[2] > 0 || s.wrap[3] > 0 {
		*in = [4]float32{0, 0, w, h}
		return
	}
	for k := 0; k < len(s.uncovered); k += spriteFloats {
		x0, y0, x1, y1 := s.uncovered[k], s.uncovered[k+1], s.uncovered[k+2], s.uncovered[k+3]
		cx, cy := (x0+x1)/2, (y0+y1)/2
		reach := max(x1-x0, x0-x1, y1-y0, y0-y1)
		for _, c := range [4][2]float32{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
			x, y := cx+c[0]*reach, cy+c[1]*reach
			cw := m[3]*x + m[7]*y + m[15]
			px := ((m[0]*x+m[4]*y+m[12])/cw + 1) / 2 * w
			py := (1 - (m[1]*x+m[5]*y+m[13])/cw) / 2 * h
			in[0], in[1] = min(in[0], px), min(in[1], py)
			in[2], in[3] = max(in[2], px), max(in[3], py)
		}
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

// the pieces of the pass, the cutouts or the rest, in the order of their layers, the cutouts front
// to back, and the runs of one atlas they make; as they came where they are all the pass's and
// lie in order already
func (s *Sprites) byLayer(cutouts bool) ([]float32, []spriteRun) {
	s.order = s.order[:0]
	for i := range s.layers {
		if !s.anyCut || s.cutouts[i] == cutouts {
			s.order = append(s.order, int32(i))
		}
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
	if cutouts {
		order := before
		before = func(a, b int32) int { return -order(a, b) }
	}
	if len(s.order) == len(s.layers) && slices.IsSortedFunc(s.order, before) {
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
