package aloft

import (
	"embed"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/world"
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
		{Name: "AloftShown", Size: 1},
		{Name: "AloftOrtho", Size: 1},
	})

// Renderer is the clouds seen from above, a render.Direct over the world, drawn while shown.
type Renderer struct {
	base  float32
	air   func() air.Weather
	shown bool

	opts     render.DrawMeshOptions
	uniforms map[string][]float32 // boxed once in the options
	low      *render.Image        // the clouds, coarser than the screen
}

// New is the clouds of the weather air gives, seen from above, on a world of scale.
func New(scale world.Scale, weather func() air.Weather) *Renderer {
	r := &Renderer{base: air.Base(scale), air: weather, uniforms: map[string][]float32{}}
	r.opts.Uniforms = map[string]any{}
	return r
}

// how much of what is behind the thickest cloud it hides; how many pixels of the screen a pixel of
// the clouds spans each way: they are soft, worked out a sixteenth as often and laid over enlarged
const (
	shownAt = 0.85
	coarse  = 4
)

// Show has the clouds drawn from above, or none: only their shadows on the ground.
func (r *Renderer) Show(shown bool) { r.shown = shown }

// Shown reports whether the clouds are drawn from above.
func (r *Renderer) Shown() bool { return r.shown }

func (*Renderer) Init(*goke.SysInit) {}

// Compose has the clouds drawn among what is seen through, at the layer's height over the middle
// of the view.
func (r *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	if _, ok := r.field(cam); !ok {
		return
	}
	w, h := cam.Viewport()
	x, y := cam.Unproject(w/2, h/2, 0)
	f.DrawAt(f.At(x, y, r.base))
}

// the lines of sight cam looks along, while there are clouds to see from above: none from an eye
// under the layer, which sees them on the sky
func (r *Renderer) field(cam camera.Camera) (camera.RayField, bool) {
	if !r.shown || r.air().Most().Cover <= 0 {
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
	if f.DX == ([3]float32{}) && f.DY == ([3]float32{}) && f.Origin[2] <= r.base {
		return camera.RayField{}, false
	}
	return f, true
}

// Draw draws the clouds into the target's screen through cam.
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
	r.set("AloftBase", r.base)
	r.set("AloftShown", shownAt)
	ortho := float32(0)
	if f.DDX == ([3]float32{}) && f.DDY == ([3]float32{}) {
		ortho = 1
	}
	r.set("AloftOrtho", ortho)
	r.opts.Vertices = 3
	b := t.Screen.Bounds()
	lw, lh := (b.Dx()+coarse-1)/coarse, (b.Dy()+coarse-1)/coarse
	if r.low == nil || r.low.Bounds().Dx() != lw || r.low.Bounds().Dy() != lh {
		if r.low != nil {
			r.low.Deallocate()
		}
		r.low = render.NewImage(lw, lh)
	}
	r.low.Clear()
	r.low.DrawMesh(nil, shader, &r.opts)
	var op render.DrawImageOptions
	op.GeoM.Scale(float64(b.Dx())/float64(lw), float64(b.Dy())/float64(lh))
	op.Filter = render.FilterLinear
	t.Screen.DrawImage(r.low, &op)
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
