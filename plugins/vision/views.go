package vision

import (
	"embed"
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/render"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

var sighted = render.NewShaderWith("vision sight", render.Files(shaders, "shaders/ground.wgsl", "shaders/sighted.wgsl"), []render.Uniform{
	{Name: "GroundStep", Size: 1}, {Name: "GroundCount", Size: 2}, {Name: "GroundWrap", Size: 2}, {Name: "Low", Size: 1}, {Name: "Span", Size: 1},
	{Name: "SightBend", Size: 1}, {Name: "March", Size: 1}, {Name: "Spokes", Size: 1}, {Name: "Rings", Size: 1},
})

var viewing = render.NewMeshShaderWith("vision views", render.Files(shaders, "shaders/ground.wgsl", "shaders/views.wgsl"), []render.Uniform{
	{Name: "Unproject", Size: 16}, {Name: "Eye", Size: 3}, {Name: "Bend", Size: 1}, {Name: "Perspective", Size: 1},
	{Name: "ViewSize", Size: 2}, {Name: "ViewAt", Size: 2}, {Name: "PixelSpan", Size: 2}, {Name: "FlatGround", Size: 1},
	{Name: "GroundStep", Size: 1}, {Name: "GroundCount", Size: 2}, {Name: "GroundWrap", Size: 2}, {Name: "Low", Size: 1}, {Name: "Span", Size: 1},
	{Name: "Spokes", Size: 1}, {Name: "Rings", Size: 1}, {Name: "ShadowColor", Size: 4},
}).Instanced(5)

const (
	// texels across and out an observer's sight is baked in
	spokes, rings = 64, 64
	maxViews      = 128
	// texels a side of the ground's copy at most
	maxGround = 4096
)

type views struct {
	observers []observer

	ground, bands *render.Image
	groundAt      [2]uint64
	nx, ny        int
	step          float32
	wrap          [2]bool
	low, span     float32
	heights       []float32
	pix, bandPix  []byte

	sight            *render.Image
	verts            []render.Vertex
	indices          []uint16
	baking           render.DrawTrianglesShaderOptions
	drawing          render.DrawMeshOptions
	instances        []float32
	bakeOwn, drawOwn map[string][]float32
}

// Facing and Half in radians; Color the cone's edge, a unit colour; Tilt and VHalf the vertical
// field, radians up, VHalf 0 every height
type observer struct {
	X, Y, Eye, Reach, Facing, Half float32
	Color                          [4]float32
	Tilt, VHalf                    float32
}

type coverTexel struct{ tau, bottom, top float32 }

// coverCopy is the cover as a Publish walked it, a texel every step over a grid nx by ny: what
// the views drawn are held back by
type coverCopy struct {
	at        [2]uint64 // the ground's and the cover's versions walked
	nx, ny    int
	step      float32
	texels    []coverTexel // nil for no cover
	low, high float32      // the bands walked, ±Inf for none
}

// update walks cover into c over a world w by h, a texel every step, anew where it, the ground or
// the grid changed since c was walked
func (c *coverCopy) update(
	ground ground.Heights, cover ground.Cover, w, h, step float32, wrap [2]bool,
) {
	nx, ny := groundCount(w, h, step, wrap)
	at := [2]uint64{versionOf(ground), versionOf(cover)}
	if c.texels != nil && at[0] != 0 && at[1] != 0 && at == c.at && nx == c.nx && ny == c.ny &&
		step == c.step {
		return
	}
	c.at, c.nx, c.ny, c.step = at, nx, ny, step
	c.low, c.high = f32.Inf(1), f32.Inf(-1)
	if cover == nil {
		c.texels = nil
		return
	}
	c.texels = resizeCover(c.texels, nx*ny)
	for k := range c.texels {
		c.texels[k] = coverTexel{tau: 1}
	}
	y := func(j int) float32 { return min(float32(j)*step, h-1e-3) }
	var row []coverTexel
	visit := func(near, far, bottom, top, tau float32) bool {
		first := max(int(f32.Ceil(near/step)), 0)
		for i := first; i < nx && float32(i)*step <= far; i++ {
			if tau < row[i].tau {
				row[i] = coverTexel{tau: max(tau, 0), bottom: bottom, top: top}
			}
		}
		if !f32.IsInf(bottom, 0) && !f32.IsInf(top, 0) {
			c.low, c.high = min(c.low, bottom), max(c.high, top)
		}
		return true
	}
	for j := range ny {
		row = c.texels[j*nx : (j+1)*nx]
		cover.Walk(geom.NewVec(0, y(j)), geom.NewVec(1, 0), w, 0, visit)
	}
}

// groundCount is how many texels a step apart the ground's copy has across a world w by h
func groundCount(w, h, step float32, wrap [2]bool) (nx, ny int) {
	count := func(size float32, wraps bool) int {
		if wraps {
			return min(max(int(size/step+0.5), 1), maxGround)
		}
		return min(int(size/step)+1, maxGround)
	}
	return count(w, wrap[0]), count(h, wrap[1])
}

func newViews() *views {
	v := &views{bakeOwn: map[string][]float32{}, drawOwn: map[string][]float32{}}
	v.baking.Uniforms = map[string]any{}
	v.drawing = render.DrawMeshOptions{Vertices: 6, Uniforms: map[string]any{}}
	return v
}

func (v *views) look(cam camera.Camera, o observer) {
	r := o.Reach
	if len(v.observers) >= maxViews || !cam.Visible(geom.NewAABBAt(geom.NewVec(o.X-r, o.Y-r), 2*r, 2*r)) {
		return
	}
	v.observers = append(v.observers, o)
}

func (v *views) draw(
	t render.Target, cam camera.Camera, ground ground.Heights, cover *coverCopy, w, h,
	step float32, wrap [2]bool, bend float32, shadow Shadow,
) {
	defer func() { v.observers = v.observers[:0] }()
	if t.Screen == nil || ground != nil && t.Ground == nil || len(v.observers) == 0 || step <= 0 {
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
	vw, vh := cam.Viewport()
	tr, ok := camera.SceneTransform(f, vw, vh)
	if !ok {
		return
	}
	unproject, ok := tr.Unproject()
	if !ok {
		return
	}
	v.copy(ground, cover, w, h, step, wrap)
	v.bake(bend)
	persp, span := float32(0), [2]float32{length(f.DX), 0}
	if f.DDX != ([3]float32{}) || f.DDY != ([3]float32{}) {
		persp, span = 1, [2]float32{0, length(f.DDX)}
	}
	// a view is laid again past each seam of a wrapping world
	images := [][2]float32{{0, 0}}
	for axis, size := range [2]float32{w, h} {
		if !wrap[axis] {
			continue
		}
		for _, img := range images {
			for _, d := range [2]float32{-size, size} {
				shifted := img
				shifted[axis] += d
				images = append(images, shifted)
			}
		}
	}
	v.instances = v.instances[:0]
	for k, o := range v.observers {
		for _, d := range images {
			o := o
			o.X, o.Y = o.X+d[0], o.Y+d[1]
			r := v.covered(tr, o, vw, vh)
			if r[0] >= r[2] || r[1] >= r[3] {
				continue
			}
			v.instances = append(v.instances, o.X, o.Y, o.Eye, o.Reach, o.Facing, o.Half, float32(k), 0, r[0], r[1], r[2], r[3],
				o.Color[0], o.Color[1], o.Color[2], o.Color[3], o.Tilt, o.VHalf, 0, 0)
		}
	}
	if len(v.instances) == 0 {
		return
	}
	at := t.Screen.Bounds().Min
	u, own := v.drawing.Uniforms, v.drawOwn
	set(u, own, "Unproject", unproject[:]...)
	set(u, own, "Eye", tr.Eye[:]...)
	set(u, own, "Bend", tr.Bend)
	set(u, own, "Perspective", persp)
	set(u, own, "ViewSize", vw, vh)
	set(u, own, "ViewAt", float32(at.X), float32(at.Y))
	set(u, own, "PixelSpan", span[:]...)
	flat := float32(0)
	if ground == nil {
		flat = 1
	}
	set(u, own, "FlatGround", flat)
	v.groundUniforms(u, own)
	sc := unitColor(shadow.Color)
	set(u, own, "ShadowColor", sc[:]...)
	v.drawing.Images = [4]*render.Image{v.ground, v.bands, v.sight, nil}
	v.drawing.Instances = v.instances
	v.drawing.Depth, v.drawing.ReadDepth = nil, nil
	if ground != nil {
		v.drawing.Depth, v.drawing.ReadDepth = t.Depth, t.Ground
	}
	t.Screen.DrawMesh(nil, viewing, &v.drawing)
}

func (v *views) covered(tr camera.Transform, o observer, w, h float32) [4]float32 {
	x0, y0, x1, y1 := f32.Inf(1), f32.Inf(1), f32.Inf(-1), f32.Inf(-1)
	take := func(x, y float32) bool {
		for _, z := range [3]float32{v.low, v.low + v.span, o.Eye} {
			sx, sy, _, ok := tr.Apply(x, y, z, w, h)
			if !ok {
				return false
			}
			x0, y0, x1, y1 = min(x0, sx), min(y0, sy), max(x1, sx), max(y1, sy)
		}
		return true
	}
	all := [4]float32{0, 0, w, h}
	if !take(o.X, o.Y) {
		return all
	}
	const rim = 16
	for i := range rim + 1 {
		a := o.Facing - o.Half + 2*o.Half*float32(i)/rim
		if !take(o.X+o.Reach*f32.Cos(a), o.Y+o.Reach*f32.Sin(a)) {
			return all
		}
	}
	pad := 8 + 0.05*max(x1-x0, y1-y0)
	return [4]float32{max(x0-pad, 0), max(y0-pad, 0), min(x1+pad, w), min(y1+pad, h)}
}

func length(a [3]float32) float32 {
	return f32.Sqrt(a[0]*a[0] + a[1]*a[1] + a[2]*a[2])
}

func (v *views) bake(bend float32) {
	n := len(v.observers)
	if height := rings * n; v.sight == nil || v.sight.Bounds().Dy() < height {
		if v.sight != nil {
			v.sight.Deallocate()
		}
		v.sight = render.NewImage(spokes, rings*max(pow2(n), 8))
	}
	v.verts, v.indices = v.verts[:0], v.indices[:0]
	for k, o := range v.observers {
		base := uint16(len(v.verts))
		for _, c := range [4][2]float32{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
			v.verts = append(v.verts, render.Vertex{
				DstX: c[0] * spokes, DstY: float32(k*rings) + c[1]*rings, SrcX: c[0] * spokes, SrcY: c[1] * rings,
				ColorR: o.Facing, ColorG: o.Half, Custom0: o.X, Custom1: o.Y, Custom2: o.Eye, Custom3: o.Reach,
			})
		}
		v.indices = append(v.indices, base, base+1, base+2, base+1, base+2, base+3)
	}
	u, own := v.baking.Uniforms, v.bakeOwn
	v.groundUniforms(u, own)
	set(u, own, "SightBend", bend)
	set(u, own, "March", v.step/2)
	v.baking.Images = [4]*render.Image{v.ground, v.bands, nil, nil}
	v.sight.DrawTrianglesShader(v.verts, v.indices, sighted, &v.baking)
}

func (v *views) groundUniforms(u map[string]any, own map[string][]float32) {
	set(u, own, "GroundStep", v.step)
	set(u, own, "GroundCount", float32(v.nx), float32(v.ny))
	set(u, own, "GroundWrap", flag(v.wrap[0]), flag(v.wrap[1]))
	set(u, own, "Low", v.low)
	set(u, own, "Span", v.span)
	set(u, own, "Spokes", spokes)
	set(u, own, "Rings", rings)
}

// own keeps the slices between frames, so each is boxed into u once
func set(u map[string]any, own map[string][]float32, name string, vals ...float32) {
	s, ok := own[name]
	if !ok || len(s) != len(vals) {
		s = make([]float32, len(vals))
		own[name] = s
		u[name] = s
	}
	copy(s, vals)
}

// copy has the ground's texture hold ground and the cover as published, cover nil for none
func (v *views) copy(ground ground.Heights, cover *coverCopy, w, h, step float32, wrap [2]bool) {
	nx, ny := groundCount(w, h, step, wrap)
	var covers []coverTexel
	at := [2]uint64{versionOf(ground), versionOf(nil)}
	if cover != nil && cover.texels != nil && cover.nx == nx && cover.ny == ny &&
		cover.step == step {
		covers, at[1] = cover.texels, cover.at[1]
	}
	if v.ground != nil && nx == v.nx && ny == v.ny && step == v.step && wrap == v.wrap && at[0] != 0 && at[1] != 0 && at == v.groundAt {
		return
	}
	v.groundAt, v.nx, v.ny, v.step, v.wrap = at, nx, ny, step, wrap
	n := nx * ny
	v.heights = resize(v.heights, n)
	x := func(i int) float32 { return min(float32(i)*step, w-1e-3) }
	y := func(j int) float32 { return min(float32(j)*step, h-1e-3) }
	low, high := f32.Inf(1), f32.Inf(-1)
	for j := range ny {
		for i := range nx {
			g := float32(0)
			if ground != nil {
				g = ground.At(geom.NewVec(x(i), y(j)))
			}
			v.heights[j*nx+i] = g
			low, high = min(low, g), max(high, g)
		}
	}
	if covers != nil {
		low, high = min(low, cover.low), max(high, cover.high)
	}
	if f32.IsInf(low, 0) {
		low, high = 0, 0
	}
	v.low, v.span = low, max(high-low, 1)
	v.pix, v.bandPix = v.pix[:0], v.bandPix[:0]
	enc := func(z float32) (byte, byte) {
		q := uint32(min(max((z-low)/v.span, 0), 1)*65535 + 0.5)
		return byte(q >> 8), byte(q & 255)
	}
	for k := range n {
		hi, lo := enc(v.heights[k])
		c := coverTexel{tau: 1}
		if covers != nil {
			c = covers[k]
		}
		// a band without end, a cover of no height, has it all: alpha none
		whole := byte(255)
		if f32.IsInf(c.bottom, 0) || f32.IsInf(c.top, 0) {
			whole = 0
		}
		v.pix = append(v.pix, hi, lo, byte(min(c.tau, 1)*255+0.5), whole)
		th, tl := enc(c.top)
		bh, bl := enc(c.bottom)
		v.bandPix = append(v.bandPix, th, tl, bh, bl)
	}
	if v.ground == nil || v.ground.Bounds().Dx() != nx || v.ground.Bounds().Dy() != ny {
		if v.ground != nil {
			v.ground.Deallocate()
			v.bands.Deallocate()
		}
		v.ground, v.bands = render.NewImage(nx, ny), render.NewImage(nx, ny)
	}
	v.ground.WritePixels(v.pix)
	v.bands.WritePixels(v.bandPix)
}

// 1 for none at all, 0 for one that keeps no count: its copy is made anew every frame
func versionOf(x any) uint64 {
	if x == nil {
		return 1
	}
	c, ok := x.(interface{ Version() uint64 })
	if !ok {
		return 0
	}
	v := c.Version()<<1 | 1
	if ch, ok := x.(interface{ Changes() uint64 }); ok {
		v += ch.Changes() << 33
	}
	return v
}

func resize(s []float32, n int) []float32 {
	if cap(s) < n {
		return make([]float32, n)
	}
	return s[:n]
}

func resizeCover(s []coverTexel, n int) []coverTexel {
	if cap(s) < n {
		return make([]coverTexel, n)
	}
	return s[:n]
}

func pow2(n int) int {
	p := 1
	for p < n {
		p *= 2
	}
	return p
}

func unitColor(c color.RGBA) [4]float32 {
	a := float32(c.A) / 255
	return [4]float32{float32(c.R) / 255 * a, float32(c.G) / 255 * a, float32(c.B) / 255 * a, a}
}

func flag(b bool) float32 {
	if b {
		return 1
	}
	return 0
}
