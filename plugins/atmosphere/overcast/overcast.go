package overcast

import (
	"embed"
	"math"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/render"
)

var _ render.Direct = (*Renderer)(nil)

//go:embed shaders/*.wgsl
var shaders embed.FS

var shader = render.NewMeshShaderWith("clouds' shadows", render.Files(shaders, "shaders/overcast.wgsl"), []render.Uniform{
	{Name: "ViewSize", Size: 2}, {Name: "Pieces", Size: 2}, {Name: "RayAt", Size: 3}, {Name: "RayDX", Size: 3}, {Name: "RayDY", Size: 3},
	{Name: "RayDir", Size: 3}, {Name: "RayDDX", Size: 3}, {Name: "RayDDY", Size: 3},
})

// Renderer lays the clouds' shadows over a flat world, a render.Direct over what stands on the
// ground and under the overlays.
type Renderer struct {
	sun func() sky.Sun
	air func() air.Weather

	opts render.DrawMeshOptions
	own  map[string][]float32 // boxed once in the options
}

// cloudPiece in pixels; shaders/overcast.wgsl has the same
const (
	cloudTier  = render.Objects + 50
	cloudPiece = 8
)

// New is the clouds' shadows of the weather weather gives, under sun.
func New(sun func() sky.Sun, weather func() air.Weather) *Renderer {
	return &Renderer{sun: sun, air: weather}
}

func (*Renderer) Init(*goke.SysInit) {}

// Compose hands the frame the sun and the weather; the shadows are drawn Direct.
func (c *Renderer) Compose(f *render.Frame, _ camera.Camera) {
	sun := c.sun()
	sun.Frame(f)
	c.air().Frame(f, sun)
}

// Tier is render.Objects + 50.
func (*Renderer) Tier() render.Tier { return cloudTier }

// Draw lays the shadows over the target's screen through cam; nothing under a clear sky or
// through a camera without lines of sight.
func (c *Renderer) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	if t.Screen == nil || c.air().Clouds <= 0 {
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
	if c.own == nil {
		c.own, c.opts.Uniforms = map[string][]float32{}, map[string]any{}
	}
	u.Into(c.opts.Uniforms)
	w, h := cam.Viewport()
	nx, ny := int(math.Ceil(float64(w/cloudPiece))), int(math.Ceil(float64(h/cloudPiece)))
	c.set("ViewSize", w, h)
	c.set("Pieces", float32(nx), float32(ny))
	c.set("RayAt", f.Origin[:]...)
	c.set("RayDX", f.DX[:]...)
	c.set("RayDY", f.DY[:]...)
	c.set("RayDir", f.Dir[:]...)
	c.set("RayDDX", f.DDX[:]...)
	c.set("RayDDY", f.DDY[:]...)
	c.opts.Vertices = 6 * nx * ny
	t.Screen.DrawMesh(nil, shader, &c.opts)
}

func (c *Renderer) set(name string, v ...float32) {
	s, ok := c.own[name]
	if !ok || len(s) != len(v) {
		s = make([]float32, len(v))
		c.own[name] = s
	}
	copy(s, v)
	c.opts.Uniforms[name] = s
}
