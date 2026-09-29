package vision

import (
	"embed"
	"image/color"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/render"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

// sighted bakes the sight of every view drawn (shaders/sighted.wgsl) over the ground and its cover.
var sighted = render.NewShaderWith("vision sight", render.Files(shaders, "shaders/ground.wgsl", "shaders/sighted.wgsl"), []render.Uniform{
	{Name: "GroundStep", Size: 1}, {Name: "GroundCount", Size: 2}, {Name: "Low", Size: 1}, {Name: "Span", Size: 1},
	{Name: "SightBend", Size: 1}, {Name: "March", Size: 1}, {Name: "Spokes", Size: 1}, {Name: "Rings", Size: 1},
})

// viewing lays the views over what the frame drew (shaders/views.wgsl), an observer an instance.
var viewing = render.NewMeshShaderWith("vision views", render.Files(shaders, "shaders/ground.wgsl", "shaders/views.wgsl"), []render.Uniform{
	{Name: "Unproject", Size: 16}, {Name: "Eye", Size: 3}, {Name: "Bend", Size: 1}, {Name: "Perspective", Size: 1},
	{Name: "ViewSize", Size: 2}, {Name: "ViewAt", Size: 2}, {Name: "PixelSpan", Size: 2},
	{Name: "GroundStep", Size: 1}, {Name: "GroundCount", Size: 2}, {Name: "Low", Size: 1}, {Name: "Span", Size: 1},
	{Name: "Spokes", Size: 1}, {Name: "Rings", Size: 1}, {Name: "ShadowColor", Size: 4}, {Name: "ConeColor", Size: 4},
}).Instanced(3)

const (
	// spokes and rings are how many texels across and out an observer's sight is baked in;
	// maxViews how many views a frame draws at most
	spokes, rings = 64, 64
	maxViews      = 128
	// maxGround is how many texels a side the copy of the ground holds at most
	maxGround = 4096
)

// views are the views of a world with heights drawn on the GPU: the ground and its cover copied
// into images as they change, every observer's sight baked each frame into a block of its own,
// then laid over the ground the frame's meshes drew, read from their depth — the ground out of
// sight veiled, the cone stroked.
type views struct {
	observers []observer // the frame's, in sight of its camera

	ground, bands *render.Image // the ground and its cover, a texel every step
	groundAt      [2]uint64     // the ground's version and the cover's the copy holds, 1 more; 0 none
	nx, ny        int
	step          float32
	low, span     float32
	heights       []float64
	covers        []coverTexel
	pix, bandPix  []byte

	sight     *render.Image // spokes by rings texels an observer
	verts     []render.Vertex
	indices   []uint16
	baking    render.DrawTrianglesShaderOptions
	drawing   render.DrawMeshOptions
	instances []float32
	// the uniforms of the bake and of the draw, kept between frames
	bakeOwn, drawOwn map[string][]float32
}

// observer is one view to draw: the eye (X, Y, Eye its level) and how far it reaches, the way it
// faces and half its angle, radians.
type observer struct{ X, Y, Eye, Reach, Facing, Half float32 }

// coverTexel is the cover at a texel of the copy: how see-through, its bottom and its top.
type coverTexel struct{ tau, bottom, top float64 }

func newViews() *views {
	v := &views{bakeOwn: map[string][]float32{}, drawOwn: map[string][]float32{}}
	v.baking.Uniforms = map[string]any{}
	v.drawing = render.DrawMeshOptions{Vertices: 6, Uniforms: map[string]any{}}
	return v
}

// look notes the view of an observer in sight of cam for the frame being composed.
func (v *views) look(cam camera.Camera, o observer) {
	r := float64(o.Reach)
	if len(v.observers) >= maxViews || !cam.Visible(geom.NewAABBAt(geom.NewVec(float64(o.X)-r, float64(o.Y)-r), 2*r, 2*r)) {
		return
	}
	v.observers = append(v.observers, o)
}

// draw bakes the frame's views and draws them into the target through cam, over ground and cover,
// the world w by h, the ground copied every step; shadow veils the ground out of sight, bend sinks
// it under an eye's level per d².
func (v *views) draw(t render.Target, cam camera.Camera, ground board.Heights, cover board.Cover, w, h, step float32, bend float64, shadow Shadow) {
	defer func() { v.observers = v.observers[:0] }()
	if t.Screen == nil || t.Depth == nil || len(v.observers) == 0 || ground == nil || step <= 0 {
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
	v.copy(ground, cover, w, h, step)
	v.bake(bend)
	persp, span := float32(0), [2]float32{length(f.DX), 0}
	if f.DDX != ([3]float32{}) || f.DDY != ([3]float32{}) {
		persp, span = 1, [2]float32{0, length(f.DDX)}
	}
	v.instances = v.instances[:0]
	for k, o := range v.observers {
		r := v.covered(tr, o, vw, vh)
		v.instances = append(v.instances, o.X, o.Y, o.Eye, o.Reach, o.Facing, o.Half, float32(k), 0, r[0], r[1], r[2], r[3])
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
	v.groundUniforms(u, own)
	sc, cc := unitColor(shadow.Color), unitColor(coneColor)
	set(u, own, "ShadowColor", sc[:]...)
	set(u, own, "ConeColor", cc[:]...)
	v.drawing.ReadDepth, v.drawing.Images, v.drawing.Instances = t.Depth, [4]*render.Image{v.ground, v.bands, v.sight, nil}, v.instances
	t.Screen.DrawMesh(nil, viewing, &v.drawing)
}

// covered is the rectangle of the viewport w by h the cone of o may cover through tr, pixels: the
// cone's rim and apex at the lowest and the highest the copy of the ground holds, a little wider;
// all of the viewport where any of it lies behind the eye.
func (v *views) covered(tr camera.Transform, o observer, w, h float32) [4]float32 {
	x0, y0, x1, y1 := float32(math.Inf(1)), float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.Inf(-1))
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
		a := float64(o.Facing - o.Half + 2*o.Half*float32(i)/rim)
		if !take(o.X+o.Reach*float32(math.Cos(a)), o.Y+o.Reach*float32(math.Sin(a))) {
			return all
		}
	}
	pad := 8 + 0.05*max(x1-x0, y1-y0)
	return [4]float32{max(x0-pad, 0), max(y0-pad, 0), min(x1+pad, w), min(y1+pad, h)}
}

// length is how long a is.
func length(a [3]float32) float32 {
	return float32(math.Sqrt(float64(a[0]*a[0] + a[1]*a[1] + a[2]*a[2])))
}

// bake works out every view's sight into its block of the sight image.
func (v *views) bake(bend float64) {
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
	set(u, own, "SightBend", float32(bend))
	set(u, own, "March", v.step/2)
	v.baking.Images = [4]*render.Image{v.ground, v.bands, nil, nil}
	v.sight.DrawTrianglesShader(v.verts, v.indices, sighted, &v.baking)
}

// groundUniforms hands a shader where the copy of the ground lies and what it holds.
func (v *views) groundUniforms(u map[string]any, own map[string][]float32) {
	set(u, own, "GroundStep", v.step)
	set(u, own, "GroundCount", float32(v.nx), float32(v.ny))
	set(u, own, "Low", v.low)
	set(u, own, "Span", v.span)
	set(u, own, "Spokes", spokes)
	set(u, own, "Rings", rings)
}

// set puts the uniform name, vals, into u, in a slice own keeps between frames and boxed once.
func set(u map[string]any, own map[string][]float32, name string, vals ...float32) {
	s, ok := own[name]
	if !ok || len(s) != len(vals) {
		s = make([]float32, len(vals))
		own[name] = s
		u[name] = s
	}
	copy(s, vals)
}

// copy copies the ground and its cover into images, a texel every step over the world w by h,
// anew where either has changed since — or every frame for one that does not count its changes.
func (v *views) copy(ground board.Heights, cover board.Cover, w, h, step float32) {
	nx, ny := min(int(w/step)+1, maxGround), min(int(h/step)+1, maxGround)
	at := [2]uint64{versionOf(ground), versionOf(cover)}
	if v.ground != nil && nx == v.nx && ny == v.ny && step == v.step && at[0] != 0 && at[1] != 0 && at == v.groundAt {
		return
	}
	v.groundAt, v.nx, v.ny, v.step = at, nx, ny, step
	n := nx * ny
	v.heights, v.covers = resize(v.heights, n), resizeCover(v.covers, n)
	x := func(i int) float64 { return math.Min(float64(i)*float64(step), float64(w)-1e-3) }
	y := func(j int) float64 { return math.Min(float64(j)*float64(step), float64(h)-1e-3) }
	low, high := math.Inf(1), math.Inf(-1)
	for j := range ny {
		for i := range nx {
			g := ground.At(geom.NewVec(x(i), y(j)))
			v.heights[j*nx+i] = g
			low, high = math.Min(low, g), math.Max(high, g)
		}
	}
	for k := range v.covers {
		v.covers[k] = coverTexel{tau: 1}
	}
	if cover != nil {
		for j := range ny {
			row := v.covers[j*nx : (j+1)*nx]
			cover.Walk(geom.NewVec(0, y(j)), geom.NewVec(1, 0), float64(w), 0, func(near, far, bottom, top, tau float64) bool {
				if math.IsInf(bottom, 0) || math.IsInf(top, 0) {
					return true
				}
				for i := max(int(math.Ceil(near/float64(step))), 0); i < nx && float64(i)*float64(step) <= far; i++ {
					if tau < row[i].tau {
						row[i] = coverTexel{tau: math.Max(tau, 0), bottom: bottom, top: top}
					}
				}
				low, high = math.Min(low, bottom), math.Max(high, top)
				return true
			})
		}
	}
	if math.IsInf(low, 0) {
		low, high = 0, 0
	}
	v.low, v.span = float32(low), float32(math.Max(high-low, 1))
	v.pix, v.bandPix = v.pix[:0], v.bandPix[:0]
	enc := func(z float64) (byte, byte) {
		q := uint32(math.Min(math.Max((z-low)/float64(v.span), 0), 1)*65535 + 0.5)
		return byte(q >> 8), byte(q & 255)
	}
	for k := range n {
		hi, lo := enc(v.heights[k])
		c := v.covers[k]
		v.pix = append(v.pix, hi, lo, byte(math.Min(c.tau, 1)*255+0.5), 255)
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

// versionOf tells a copy of a ground or a cover when to be made anew: a number that changes with
// its Version and, where it keeps one, the count of its cells' Changes; 1 for none at all, 0 for
// one that keeps no count, made anew every frame.
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

func resize(s []float64, n int) []float64 {
	if cap(s) < n {
		return make([]float64, n)
	}
	return s[:n]
}

func resizeCover(s []coverTexel, n int) []coverTexel {
	if cap(s) < n {
		return make([]coverTexel, n)
	}
	return s[:n]
}

// pow2 is the least power of two at least n.
func pow2(n int) int {
	p := 1
	for p < n {
		p *= 2
	}
	return p
}

// unitColor is c, premultiplied as the frame takes a colour, 0 to 1 a channel.
func unitColor(c color.RGBA) [4]float32 {
	return [4]float32{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255, float32(c.A) / 255}
}
