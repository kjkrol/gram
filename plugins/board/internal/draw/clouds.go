package draw

import (
	"embed"
	"math"

	"github.com/kjkrol/gram/camera"
	_ "github.com/kjkrol/gram/plugins/atmosphere/air" // the clouds' functions the shader calls
	"github.com/kjkrol/gram/render"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

var cloudsShader = render.NewMeshShaderWith("clouds' shadows",
	render.Files(shaders, "shaders/clouds.wgsl"), []render.Uniform{
		{Name: "ViewSize", Size: 2},
		{Name: "Pieces", Size: 2},
		{Name: "RayAt", Size: 3},
		{Name: "RayDX", Size: 3},
		{Name: "RayDY", Size: 3},
		{Name: "RayDir", Size: 3},
		{Name: "RayDDX", Size: 3},
		{Name: "RayDDY", Size: 3},
	})

// Clouds lays the clouds' shadows over a flat board's ground on the GPU, as much as the clouds
// over each pixel take of the sun.
type Clouds struct {
	opts render.DrawMeshOptions
	own  map[string][]float32 // boxed once in the options
}

// cloudPiece in pixels; shaders/clouds.wgsl has the same
const cloudPiece = 8

// Draw lays the shadows of clouds covering this much of the sky over the ground drawn into t
// through cam, the weather in the frame's uniforms u; nothing under a clear sky or through a
// camera without lines of sight.
func (c *Clouds) Draw(t render.Target, cam camera.Camera, u render.Uniforms, clouds float32) {
	if t.Screen == nil || clouds <= 0 {
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
	// over whatever the pass has drawn so far, past the world's edge too, where the sky's colour
	// later replaces it
	c.opts.Depth, c.opts.Untested = t.Depth, true
	t.Screen.DrawMesh(nil, cloudsShader, &c.opts)
}

func (c *Clouds) set(name string, v ...float32) {
	s, ok := c.own[name]
	if !ok || len(s) != len(v) {
		s = make([]float32, len(v))
		c.own[name] = s
	}
	copy(s, v)
	c.opts.Uniforms[name] = s
}
