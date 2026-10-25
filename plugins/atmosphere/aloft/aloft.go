package aloft

import (
	"embed"
	"math"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/render"
)

var _ render.Direct = (*Renderer)(nil)
var _ render.Overlying = (*Renderer)(nil)

//go:embed shaders/*.wgsl
var shaders embed.FS

var shader = render.NewMeshShaderWith("aloft", render.Files(shaders, "shaders/aloft.wgsl"),
	[]render.Uniform{
		{Name: "AloftView", Size: 2},
		{Name: "AloftFrom", Size: 3},
		{Name: "AloftDX", Size: 3},
		{Name: "AloftDY", Size: 3},
		{Name: "AloftWay", Size: 3},
		{Name: "AloftWayX", Size: 3},
		{Name: "AloftWayY", Size: 3},
		{Name: "AloftBase", Size: 1},
		{Name: "AloftLow", Size: 1},
		{Name: "AloftBody", Size: 1},
		{Name: "AloftBodiesTo", Size: 1},
		{Name: "AloftMiddle", Size: 2},
		{Name: "AloftShown", Size: 1},
		{Name: "AloftOrtho", Size: 1},
		{Name: "AloftSeeing", Size: 1},
		{Name: "AloftUnder", Size: 1},
		{Name: "AloftViewProj", Size: 16},
		{Name: "AloftEye", Size: 3},
		{Name: "AloftBend", Size: 1},
	})

var layShader = render.NewMeshShaderWith("aloft laid", render.Files(shaders, "shaders/lay.wgsl"),
	[]render.Uniform{
		{Name: "LayView", Size: 2},
		{Name: "LayCoarse", Size: 2},
	})

// Renderer is a layer of clouds seen from above, a render.Direct over the world at the layer's
// height, drawn while shown; the low heaps seen from under their top too, before the ground.
type Renderer struct {
	layer    air.Layer
	air      func() air.Weather
	shown    bool
	bodies   bool
	bodiesTo float32 // of the camera's sight the bodies reach
	coarse   int     // of the ground's pixels each way a pixel of the clouds spans

	opts     render.DrawMeshOptions
	layOpts  render.DrawMeshOptions
	uniforms map[string]any // boxed once in the options
	sizes    []size         // the screens drawn on of late, by size
}

// what the clouds are drawn with on a screen w by h: their image cw by ch, how deep they lie
// there, and a depth to lay them against on a screen without one
type size struct {
	w, h   int
	cw, ch int
	coarse *render.Image
	depth  *render.Depth
	alone  *render.Depth
}

// the sizes of screen kept, as render keeps its depths: a view in rings is drawn at up to four
// a frame, two such views at eight
const keptSizes = 8

// New is the layer of clouds of the weather air gives, seen from above.
func New(weather func() air.Weather, layer air.Layer) *Renderer {
	r := &Renderer{layer: layer, air: weather, uniforms: map[string]any{},
		coarse: DefaultCoarse}
	r.opts.Uniforms = map[string]any{}
	return r
}

// ground: of the screen's pixels each way a pixel of the ground under the clouds spans
func (r *Renderer) sizeFor(w, h int, ground float32) *size {
	gw, gh := w, h
	if ground > 1 {
		gw = max(int(math.Round(float64(float32(w)/ground))), 1)
		gh = max(int(math.Round(float64(float32(h)/ground))), 1)
	}
	c := r.coarse
	cw, ch := (gw+c-1)/c, (gh+c-1)/c
	for i := range r.sizes {
		if s := &r.sizes[i]; s.w == w && s.h == h && s.cw == cw && s.ch == ch {
			return s
		}
	}
	if len(r.sizes) == keptSizes {
		r.sizes[0].free()
		r.sizes = append(r.sizes[:0], r.sizes[1:]...)
	}
	r.sizes = append(r.sizes, size{w: w, h: h, cw: cw, ch: ch, coarse: render.NewImage(cw, ch),
		depth: render.NewDepth().Readable()})
	return &r.sizes[len(r.sizes)-1]
}

func (s *size) free() {
	s.coarse.Deallocate()
	s.depth.Release()
	if s.alone != nil {
		s.alone.Release()
	}
}

// how much of what is behind the thickest cloud it hides; how many pixels of the ground a pixel of
// the clouds spans each way to start: they are soft, worked out a sixteenth as often and laid over
// enlarged
const (
	shownAt       = 0.75
	DefaultCoarse = 4
)

// Coarse has a pixel of the clouds span n of the ground's each way, of the screen's over a ground
// drawn at every pixel, DefaultCoarse to start; under 1, 1.
func (r *Renderer) Coarse(n int) { r.coarse = max(n, 1) }

// Show has the clouds drawn from above, or none: only their shadows on the ground.
func (r *Renderer) Show(shown bool) { r.shown = shown }

// Shown reports whether the clouds are drawn from above.
func (r *Renderer) Shown() bool { return r.shown }

// Bodies has the clouds drawn as bodies, slices stacked as thick as they are, or flat, one sheet.
func (r *Renderer) Bodies(bodies bool) { r.bodies = bodies }

// BodiesTo has the bodies drawn out to share of the camera's sight from where it looks and flat
// beyond; 1 or more, everywhere; nothing where the camera has no sight.
func (r *Renderer) BodiesTo(share float32) { r.bodiesTo = share }

// Overlies: drawn over the figures' ground (render.Overlying).
func (*Renderer) Overlies() {}

// Compose has the clouds drawn among what is seen through, at the layer's height over the middle
// of the view.
func (r *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	if _, _, ok := r.field(cam); !ok {
		return
	}
	w, h := cam.Viewport()
	x, y := cam.Unproject(w/2, h/2, 0)
	f.DrawAt(f.At(x, y, r.base()))
}

func (r *Renderer) base() float32 { return r.air().Base(r.layer) }

// the lines of sight cam looks along, while there are clouds to see: from above, or for the low
// heaps from an eye under their top (under), before the ground; none of the high ones from under
// theirs, which sees them on the sky
func (r *Renderer) field(cam camera.Camera) (f camera.RayField, under, ok bool) {
	if w := r.air(); !r.shown || w.Share(w.Most(), r.layer).Cover <= 0 {
		return camera.RayField{}, false, false
	}
	rays, ok := cam.(camera.Rays)
	if !ok {
		return camera.RayField{}, false, false
	}
	f, ok = rays.Rays()
	if !ok {
		return camera.RayField{}, false, false
	}
	perspective := f.DX == ([3]float32{}) && f.DY == ([3]float32{})
	if !perspective || f.Origin[2] > r.air().Top(r.layer) {
		return f, false, true
	}
	// an eye among the clouds sees them on the sky, behind the hills; the low heaps before the
	// slopes too, as bodies
	if r.layer == air.High || !r.bodies {
		return camera.RayField{}, false, false
	}
	return f, true, true
}

// Draw draws the clouds into the target's screen through cam, hidden inside the ground as the
// frame's render.GroundHeights have it and behind what stands nearer, a pixel of theirs a Coarse
// of the ground's under them (render.Target.Coarse).
func (r *Renderer) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	f, under, ok := r.field(cam)
	if t.Screen == nil || !ok || under && !r.standsUp(u) {
		return
	}
	w, h := cam.Viewport()
	tr, ok := camera.SceneTransform(f, w, h)
	if !ok {
		return
	}
	u.Into(r.opts.Uniforms)
	r.set("AloftView", w, h)
	r.set("AloftFrom", f.Origin[:]...)
	r.set("AloftDX", f.DX[:]...)
	r.set("AloftDY", f.DY[:]...)
	r.set("AloftWay", f.Dir[:]...)
	r.set("AloftWayX", f.DDX[:]...)
	r.set("AloftWayY", f.DDY[:]...)
	r.set("AloftBase", r.base())
	r.set("AloftLow", flag(r.layer == air.Low))
	r.set("AloftBody", flag(r.bodies))
	r.set("AloftBodiesTo", r.reach(cam))
	mx, my, _ := camera.Viewpoint(cam)
	r.set("AloftMiddle", mx, my)
	r.set("AloftShown", shownAt)
	r.set("AloftOrtho", flag(f.DDX == ([3]float32{}) && f.DDY == ([3]float32{})))
	r.set("AloftSeeing", float32(r.air().Visibility(air.Under(cam))))
	r.set("AloftUnder", flag(under))
	r.set("AloftViewProj", tr.M[:]...)
	r.set("AloftEye", tr.Eye[:]...)
	r.set("AloftBend", tr.Bend)
	b := t.Screen.Bounds()
	s := r.sizeFor(b.Dx(), b.Dy(), t.Coarse)
	lw, lh := s.cw, s.ch
	r.opts.Vertices = 3
	s.coarse.Clear()
	r.opts.Depth, r.opts.ClearDepth, r.opts.WriteDepth = s.depth, true, true
	s.coarse.DrawMesh(nil, shader, &r.opts)
	if r.layer == air.High {
		// sheets over all the world stands up into, met once a pixel: no grain to blur
		var op render.DrawImageOptions
		op.GeoM.Scale(float64(b.Dx())/float64(lw), float64(b.Dy())/float64(lh))
		op.Filter = render.FilterLinear
		t.Screen.DrawImage(s.coarse, &op)
		return
	}
	r.set("LayView", float32(b.Dx()), float32(b.Dy()))
	r.set("LayCoarse", float32(b.Dx())/float32(lw), float32(b.Dy())/float32(lh))
	r.layOpts.Uniforms, r.layOpts.Vertices = r.opts.Uniforms, 3
	r.layOpts.Images[0] = s.coarse
	r.layOpts.Depth, r.layOpts.ReadDepth, r.layOpts.ClearDepth = t.Depth, s.depth, false
	if t.Depth == nil {
		if s.alone == nil {
			s.alone = render.NewDepth()
		}
		r.layOpts.Depth, r.layOpts.ClearDepth = s.alone, true
	}
	t.Screen.DrawMesh(nil, layShader, &r.layOpts)
}

// whether the frame's ground (render.GroundHeights) stands up into the low clouds anywhere
func (r *Renderer) standsUp(u render.Uniforms) bool {
	lattice, span := u.Get("GroundLattice"), u.Get("GroundSpan")
	if len(lattice) < 4 || lattice[3] < 0.5 || len(span) < 4 {
		return false
	}
	return span[2]+span[3] > r.air().Base(air.Low)
}

func (r *Renderer) set(name string, v ...float32) {
	u, ok := r.uniforms[name]
	if s, _ := u.([]float32); !ok || len(s) != len(v) {
		u = make([]float32, len(v))
		r.uniforms[name] = u
	}
	copy(u.([]float32), v)
	r.opts.Uniforms[name] = u
}

func flag(on bool) float32 {
	if on {
		return 1
	}
	return 0
}

// reach is how far from where cam looks the bodies are drawn, world units; 0 everywhere
func (r *Renderer) reach(cam camera.Camera) float32 {
	if r.bodiesTo <= 0 || r.bodiesTo >= 1 {
		return 0
	}
	if sight := camera.DetailOf(cam).Sight; sight > 0 {
		return float32(sight) * r.bodiesTo
	}
	return 0
}
