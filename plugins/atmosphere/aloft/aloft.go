package aloft

import (
	"embed"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/render"
)

var _ render.Direct = (*Renderer)(nil)

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
		{Name: "AloftShown", Size: 1},
		{Name: "AloftOrtho", Size: 1},
		{Name: "AloftSeeing", Size: 1},
	})

var layShader = render.NewMeshShaderWith("aloft laid", render.Files(shaders, "shaders/lay.wgsl"),
	[]render.Uniform{
		{Name: "LayView", Size: 2},
		{Name: "LayCoarse", Size: 1},
	})

// Renderer is a layer of clouds seen from above, a render.Direct over the world at the layer's
// height, drawn while shown.
type Renderer struct {
	layer  air.Layer
	air    func() air.Weather
	shown  bool
	bodies bool

	opts     render.DrawMeshOptions
	layOpts  render.DrawMeshOptions
	uniforms map[string][]float32 // boxed once in the options
	coarse   *render.Image        // the clouds, coarser than the screen
}

// New is the layer of clouds of the weather air gives, seen from above.
func New(weather func() air.Weather, layer air.Layer) *Renderer {
	r := &Renderer{layer: layer, air: weather, uniforms: map[string][]float32{}}
	r.opts.Uniforms = map[string]any{}
	return r
}

// how much of what is behind the thickest cloud it hides; how many pixels of the screen a pixel of
// the clouds spans each way: they are soft, worked out a sixteenth as often and laid over enlarged
const (
	shownAt = 0.75
	coarse  = 4
)

// Show has the clouds drawn from above, or none: only their shadows on the ground.
func (r *Renderer) Show(shown bool) { r.shown = shown }

// Shown reports whether the clouds are drawn from above.
func (r *Renderer) Shown() bool { return r.shown }

// Bodies has the clouds drawn as bodies, slices stacked as thick as they are, or flat, one sheet.
func (r *Renderer) Bodies(bodies bool) { r.bodies = bodies }

func (*Renderer) Init(*goke.SysInit) {}

// Compose has the clouds drawn among what is seen through, at the layer's height over the middle
// of the view.
func (r *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	if _, ok := r.field(cam); !ok {
		return
	}
	w, h := cam.Viewport()
	x, y := cam.Unproject(w/2, h/2, 0)
	f.DrawAt(f.At(x, y, r.base()))
}

func (r *Renderer) base() float32 { return r.air().Base(r.layer) }

// the lines of sight cam looks along, while there are clouds to see from above: none from an eye
// under the layer's top, which sees them on the sky
func (r *Renderer) field(cam camera.Camera) (camera.RayField, bool) {
	if w := r.air(); !r.shown || w.Share(w.Most(), r.layer).Cover <= 0 {
		return camera.RayField{}, false
	}
	rays, ok := cam.(camera.Rays)
	if !ok {
		return camera.RayField{}, false
	}
	f, ok := rays.Rays()
	if !ok {
		return camera.RayField{}, false
	}
	// an eye among the clouds sees them on the sky, behind the hills: from above, past a ridge
	// they showed through it
	if f.DX == ([3]float32{}) && f.DY == ([3]float32{}) && f.Origin[2] <= r.air().Top(r.layer) {
		return camera.RayField{}, false
	}
	return f, true
}

// Draw draws the clouds into the target's screen through cam, hidden inside the ground as the
// frame's render.GroundHeights have it.
func (r *Renderer) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	f, ok := r.field(cam)
	if t.Screen == nil || !ok {
		return
	}
	u.Into(r.opts.Uniforms)
	w, h := cam.Viewport()
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
	r.set("AloftShown", shownAt)
	r.set("AloftOrtho", flag(f.DDX == ([3]float32{}) && f.DDY == ([3]float32{})))
	r.set("AloftSeeing", float32(r.air().Visibility(air.Under(cam))))
	b := t.Screen.Bounds()
	r.opts.Vertices = 3
	lw, lh := (b.Dx()+coarse-1)/coarse, (b.Dy()+coarse-1)/coarse
	if r.coarse == nil || r.coarse.Bounds().Dx() != lw || r.coarse.Bounds().Dy() != lh {
		if r.coarse != nil {
			r.coarse.Deallocate()
		}
		r.coarse = render.NewImage(lw, lh)
	}
	r.coarse.Clear()
	r.coarse.DrawMesh(nil, shader, &r.opts)
	if r.layer == air.High || !r.bodies {
		// sheets, met once a pixel: no grain to blur
		var op render.DrawImageOptions
		op.GeoM.Scale(float64(b.Dx())/float64(lw), float64(b.Dy())/float64(lh))
		op.Filter = render.FilterLinear
		t.Screen.DrawImage(r.coarse, &op)
		return
	}
	r.set("LayView", float32(b.Dx()), float32(b.Dy()))
	r.set("LayCoarse", coarse)
	r.layOpts.Uniforms, r.layOpts.Vertices = r.opts.Uniforms, 3
	r.layOpts.Images[0] = r.coarse
	t.Screen.DrawMesh(nil, layShader, &r.layOpts)
}

func (r *Renderer) set(name string, v ...float32) {
	s, ok := r.uniforms[name]
	if !ok || len(s) != len(v) {
		s = make([]float32, len(v))
		r.uniforms[name] = s
	}
	copy(s, v)
	r.opts.Uniforms[name] = s
}

func flag(on bool) float32 {
	if on {
		return 1
	}
	return 0
}
