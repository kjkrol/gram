// Package cast lays the live shadows of what stands on whatever ground the frame drew: patches
// and silhouettes, read off the ground's depth, so the ground may rise and fall under them or
// lie flat.
package cast

import (
	"math"

	"github.com/kjkrol/gram/camera"
	_ "github.com/kjkrol/gram/plugins/light/internal/distant" // the light in the shaders
	"github.com/kjkrol/gram/render"

	"embed"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

var shading = render.NewMeshShaderWith("casts", render.Files(shaders, "shaders/cast.wgsl"),
	[]render.Uniform{
		{Name: "Unproject", Size: 16},
		{Name: "ViewSize", Size: 2},
		{Name: "ViewAt", Size: 2},
	}).Instanced(4)

// Floats is a cast's floats as Caster.Draw takes them: a light.Umbra and the corners of the
// sprite whose shape it has, all 0 for a patch's own.
const Floats = 12

// world units over the ground's highest a shadow may lie, on what stands there
const headroom = 64

// Caster draws casts over the ground's depth.
type Caster struct {
	opts      render.DrawMeshOptions
	own       map[string][]float32
	instances []float32
}

// Draw lays casts, Floats each, the sprites' shapes from atlas, on the ground drawn through cam
// between low and high, read from the target's Ground depth: nothing without one.
func (c *Caster) Draw(
	t render.Target, cam camera.Camera, atlas render.AtlasSource, casts []float32,
	low, high float32,
) {
	if t.Screen == nil || t.Ground == nil || len(casts) == 0 {
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
	unproject, ok := tr.Unproject()
	if !ok {
		return
	}
	c.instances = c.instances[:0]
	for k := 0; k+Floats <= len(casts); k += Floats {
		in := casts[k : k+Floats]
		rect, ok := covers(tr, in[0], in[1], in[2], in[3], in[4], in[5], low, high+headroom, w, h)
		if !ok {
			continue
		}
		c.instances = append(c.instances, in...)
		c.instances = append(c.instances, rect[0], rect[1], rect[2], rect[3])
	}
	if len(c.instances) == 0 {
		return
	}
	if c.own == nil {
		c.own, c.opts.Uniforms = map[string][]float32{}, map[string]any{}
	}
	at := t.Screen.Bounds().Min
	c.set("Unproject", unproject[:]...)
	c.set("ViewSize", w, h)
	c.set("ViewAt", float32(at.X), float32(at.Y))
	c.opts.Vertices, c.opts.Instances = 6, c.instances
	c.opts.Depth, c.opts.ReadDepth = t.Depth, t.Ground
	c.opts.Images = [4]*render.Image{}
	if atlas != nil {
		c.opts.Images[0] = atlas.Atlas()
	}
	t.Screen.DrawMesh(nil, shading, &c.opts)
}

// the screen rectangle the cast's corners cover standing anywhere from low up to high: the whole
// viewport where a corner lies behind the eye; false off the viewport
func covers(tr camera.Transform, x, y, ux, uy, along, wide, low, high, w, h float32) (
	[4]float32, bool,
) {
	x0, y0 := float32(math.Inf(1)), float32(math.Inf(1))
	x1, y1 := float32(math.Inf(-1)), float32(math.Inf(-1))
	corners := [4][2]float32{{-along, -wide}, {along, -wide}, {-along, wide}, {along, wide}}
	for _, c := range corners {
		px, py := x+ux*c[0]-uy*c[1], y+uy*c[0]+ux*c[1]
		for _, z := range [2]float32{low, high} {
			sx, sy, _, ok := tr.Apply(px, py, z, w, h)
			if !ok {
				return [4]float32{0, 0, w, h}, true
			}
			x0, y0, x1, y1 = min(x0, sx), min(y0, sy), max(x1, sx), max(y1, sy)
		}
	}
	r := [4]float32{max(x0-2, 0), max(y0-2, 0), min(x1+2, w), min(y1+2, h)}
	return r, r[0] < r[2] && r[1] < r[3]
}

// kept between frames and boxed once
func (c *Caster) set(name string, v ...float32) {
	u, ok := c.own[name]
	if !ok || len(u) != len(v) {
		u = make([]float32, len(v))
		c.own[name] = u
	}
	copy(u, v)
	c.opts.Uniforms[name] = u
}
