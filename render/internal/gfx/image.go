package gfx

import (
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/kjkrol/gram/render/internal/gpu"
	"github.com/kjkrol/gram/render/internal/wgsl"
)

// Vertex is one corner of a triangle drawn: where in the target (Dst) and in the first image (Src),
// both in pixels, its colour and four numbers the fragment reads (Custom).
type Vertex = gpu.Vertex

// Uniform is one of a shader's uniforms: Size floats, 1 to 4.
type Uniform = gpu.Uniform

// Image is a picture on the GPU: a texture, or a rectangle of one (SubImage), in whose pixels what
// is drawn into it is placed and to which it is clipped.
type Image struct {
	tex   *gpu.Texture
	rect  image.Rectangle
	owner bool             // made by NewImage: Deallocate frees the texture
	fill  func(dst *Image) // Defer's, run as a draw next reads the image
}

// NewImage is a w by h image, transparent.
func NewImage(w, h int) *Image {
	return &Image{tex: gpu.NewTexture(w, h), rect: image.Rect(0, 0, w, h), owner: true}
}

// Bounds is the image's rectangle, in its texture's pixels.
func (i *Image) Bounds() image.Rectangle { return i.rect }

// SubImage is the part r of the image, sharing its pixels.
func (i *Image) SubImage(r image.Rectangle) *Image {
	return &Image{tex: i.tex, rect: r.Intersect(i.rect)}
}

// Defer has fill make the image's pixels as a draw next reads it, when there surely is a GPU: a
// picture baked or written the first time it is needed. A later Defer before then replaces it.
func (i *Image) Defer(fill func(dst *Image)) {
	i.fill = fill
	deferred[i] = true
}

// the images whose fills wait and the shaders made, for Prepare
var (
	deferred = map[*Image]bool{}
	shaders  []*Shader
)

// Prepare compiles every shader made, fills every image deferred and puts on the GPU every
// texture whose pixels wait on the CPU.
func Prepare() error {
	for _, s := range shaders {
		if err := s.Compile(); err != nil {
			return fmt.Errorf("render: the %s shader: %w", s.label, err)
		}
	}
	for i := range deferred {
		i.ready()
	}
	gpu.RealizeAll()
	return nil
}

func (i *Image) ready() *Image {
	if fill := i.fill; fill != nil {
		i.fill = nil
		delete(deferred, i)
		fill(i)
	}
	return i
}

func (i *Image) gpu() gpu.Image {
	r := i.rect
	return gpu.Image{Texture: i.tex, Rect: gpu.Rect{X: r.Min.X, Y: r.Min.Y, W: r.Dx(), H: r.Dy()}}
}

// GPU is the image as the GPU draws into it and reads it.
func GPU(i *Image) gpu.Image { return i.gpu() }

// FrameOf are the textures of the frame's images the uniforms u hold, by their binding, ready
// to be read.
func FrameOf(u map[string]any) (f [gpu.FrameImages]*gpu.Texture) {
	for i, name := range wgsl.Images() {
		if img, _ := u[name].(*Image); img != nil {
			f[i] = img.ready().tex
		}
	}
	return f
}

// Fill puts c in place of every pixel of the image.
func (i *Image) Fill(c color.Color) {
	r, g, b, a := Premultiply(c)
	gpu.Fill(i.gpu(), r, g, b, a)
}

// FillBehind puts c where nothing was drawn into d: behind the whole world.
func (i *Image) FillBehind(c color.Color, d *Depth) {
	r, g, b, a := Premultiply(c)
	gpu.FillBehind(i.gpu(), d.d, r, g, b, a)
}

// Clear makes the image transparent.
func (i *Image) Clear() { gpu.Fill(i.gpu(), 0, 0, 0, 0) }

// WritePixels puts pix, RGBA premultiplied, row by row, over the whole image.
func (i *Image) WritePixels(pix []byte) {
	r := i.rect
	i.tex.WritePixels(r.Min.X, r.Min.Y, r.Dx(), r.Dy(), pix)
}

// ReadPixels copies the image, RGBA premultiplied, row by row, into pix.
func (i *Image) ReadPixels(pix []byte) {
	tw, th := i.tex.Size()
	r := i.rect
	if r == image.Rect(0, 0, tw, th) {
		i.tex.ReadPixels(pix)
		return
	}
	all := make([]byte, 4*tw*th)
	i.tex.ReadPixels(all)
	for y := range r.Dy() {
		copy(pix[4*y*r.Dx():4*(y+1)*r.Dx()], all[4*((r.Min.Y+y)*tw+r.Min.X):])
	}
}

// Deallocate frees an image made by NewImage; it may not be drawn with after.
func (i *Image) Deallocate() {
	if i.owner {
		i.tex.Release()
	}
}

// Filter is how an image drawn scaled is sampled.
type Filter uint8

const (
	FilterNearest Filter = iota
	FilterLinear
)

// GeoM is an affine transform of the plane, the identity as it comes.
type GeoM struct {
	a1, b, c, d1, tx, ty float64 // the diagonal minus one, so the zero is the identity
}

// Translate moves by (tx, ty) after g.
func (g *GeoM) Translate(tx, ty float64) { g.tx += tx; g.ty += ty }

// Scale scales by (x, y) after g.
func (g *GeoM) Scale(x, y float64) {
	a, d := (g.a1+1)*x, (g.d1+1)*y
	g.a1, g.d1 = a-1, d-1
	g.b *= y
	g.c *= x
	g.tx *= x
	g.ty *= y
}

// Rotate turns by theta radians after g, x towards y.
func (g *GeoM) Rotate(theta float64) {
	s, c := math.Sincos(theta)
	a, d := g.a1+1, g.d1+1
	na, nc := c*a-s*g.b, c*g.c-s*d
	nb, nd := s*a+c*g.b, s*g.c+c*d
	g.a1, g.c, g.b, g.d1 = na-1, nc, nb, nd-1
	g.tx, g.ty = c*g.tx-s*g.ty, s*g.tx+c*g.ty
}

// Apply is where g takes (x, y).
func (g GeoM) Apply(x, y float64) (float64, float64) {
	return (g.a1+1)*x + g.c*y + g.tx, g.b*x + (g.d1+1)*y + g.ty
}

func (g GeoM) affine() gpu.Affine {
	return gpu.Affine{A: float32(g.a1 + 1), B: float32(g.b), C: float32(g.c), D: float32(g.d1 + 1), TX: float32(g.tx), TY: float32(g.ty)}
}

// DrawImageOptions is how DrawImage draws: through GeoM, sampled by Filter; with a Fade, the
// pixels of the target each edge fades out over, sampled between texels whatever the Filter.
type DrawImageOptions struct {
	GeoM   GeoM
	Filter Filter
	Fade   Fade
}

//go:embed blit_soft.wgsl
var blitSoft string

var softBlit = NewShaderWith("soft blit", []byte(blitSoft+"\n"), nil)

// DrawImage lays src over the image, its top-left corner at the origin moved by op's GeoM.
func (i *Image) DrawImage(src *Image, op *DrawImageOptions) {
	var o DrawImageOptions
	if op != nil {
		o = *op
	}
	if o.Fade != (Fade{}) {
		i.drawSoft(src, o)
		return
	}
	gpu.DrawImage(i.gpu(), src.gpu(), o.GeoM.affine(), [4]float32{1, 1, 1, 1}, o.Filter == FilterLinear, gpu.SourceOver)
}

func (i *Image) drawSoft(src *Image, o DrawImageOptions) {
	r := src.rect
	corners := [4][2]float64{{0, 0}, {float64(r.Dx()), 0}, {0, float64(r.Dy())},
		{float64(r.Dx()), float64(r.Dy())}}
	var dst Corners
	for k, p := range corners {
		x, y := o.GeoM.Apply(p[0], p[1])
		dst[k] = [2]float32{float32(x), float32(y)}
	}
	customs := FadeCustoms(dst, o.Fade)
	var verts [4]Vertex
	for k, p := range corners {
		sx, sy := float32(r.Min.X)+float32(p[0]), float32(r.Min.Y)+float32(p[1])
		v := Vertex{DstX: dst[k][0], DstY: dst[k][1], SrcX: sx, SrcY: sy, ColorR: 1, ColorG: 1,
			ColorB: 1, ColorA: 1}
		v.Custom0, v.Custom1, v.Custom2, v.Custom3 = customs[k][0], customs[k][1], customs[k][2],
			customs[k][3]
		verts[k] = v
	}
	indices := [6]uint16{0, 1, 2, 1, 2, 3}
	i.DrawTrianglesShader(verts[:], indices[:], softBlit,
		&DrawTrianglesShaderOptions{Images: [4]*Image{src}})
}

// CopyFrom puts src's pixels, alpha and all, in place of the image's, from its top-left corner.
func (i *Image) CopyFrom(src *Image) {
	var at GeoM
	at.Translate(float64(i.rect.Min.X), float64(i.rect.Min.Y))
	gpu.DrawImage(i.gpu(), src.ready().gpu(), at.affine(), [4]float32{1, 1, 1, 1}, false,
		gpu.Copy)
}

// DrawImageIn lays src over the image as DrawImage does, only inside the convex polygon pts, in
// the image's pixels.
func (i *Image) DrawImageIn(src *Image, op *DrawImageOptions, pts [][2]float32) {
	var o DrawImageOptions
	if op != nil {
		o = *op
	}
	gpu.DrawImageIn(i.gpu(), src.gpu(), o.GeoM.affine(), o.Filter == FilterLinear, pts,
		gpu.SourceOver)
}

// DrawTrianglesShaderOptions is what a shader draws with: its uniforms by name, each a []float32,
// and up to four images.
type DrawTrianglesShaderOptions struct {
	Uniforms map[string]any
	Images   [4]*Image
}

// DrawTrianglesShader draws the triangles indices picks out of verts with s, laid over the image.
func (i *Image) DrawTrianglesShader(verts []Vertex, indices []uint16, s *Shader, op *DrawTrianglesShaderOptions) {
	var imgs [4]gpu.Image
	var u map[string]any
	if op != nil {
		for k, im := range op.Images {
			if im != nil {
				imgs[k] = im.ready().gpu()
			}
		}
		u = op.Uniforms
	}
	gpu.Triangles(&gpu.Draw{Target: i.gpu(), Program: s.program(), Images: imgs, Frame: FrameOf(u),
		Uniforms: s.pack(u), Blend: gpu.SourceOver}, verts, indices)
}

// Premultiply is a colour as gram takes it, straight alpha, premultiplied for the GPU, 0 to 1.
func Premultiply(c color.Color) (r, g, b, a float32) {
	if s, ok := c.(color.RGBA); ok {
		a = float32(s.A) / 0xff
		return float32(s.R) / 0xff * a, float32(s.G) / 0xff * a, float32(s.B) / 0xff * a, a
	}
	cr, cg, cb, ca := c.RGBA()
	return float32(cr) / 0xffff, float32(cg) / 0xffff, float32(cb) / 0xffff, float32(ca) / 0xffff
}

// Shader is a WGSL fragment with its uniforms, for DrawTrianglesShader.
type Shader struct {
	label  string
	source func() string // read once the materials are all in
	layout func() *gpu.Layout
	mesh   bool
	vec4s  int
	prog   *gpu.Program
	lay    *gpu.Layout
	buf    []byte
}

func (s *Shader) program() *gpu.Program {
	if s.prog == nil {
		s.lay = s.layout()
		if s.mesh {
			s.prog = gpu.NewMesh(s.label, s.lay.Fields, s.source()).Instanced(s.vec4s)
		} else {
			s.prog = gpu.NewProgram(s.label, s.lay.Fields, s.source())
		}
	}
	return s.prog
}

// NewShaderWith is a shader on the library and every material registered, with fragment's own
// uniforms and fragment after them. No material may be registered once it has been built.
func NewShaderWith(label string, fragment []byte, own []Uniform) *Shader {
	s := &Shader{label: label, source: func() string { return wgsl.SourceWith(fragment) },
		layout: func() *gpu.Layout { return wgsl.Layout(own) }}
	shaders = append(shaders, s)
	return s
}

// Program is the shader's program, built on the materials the first time it is asked for.
func Program(s *Shader) *gpu.Program { return s.program() }

// Pack is the uniforms u as the shader's program takes them, a frame's images left out.
func Pack(s *Shader, u map[string]any) []byte { return s.pack(u) }

// Compile compiles the shader now, rather than at its first draw.
func (s *Shader) Compile() error { return s.program().Compile() }

// Source is the shader's whole WGSL.
func (s *Shader) Source() string { return s.program().Source() }

func (s *Shader) pack(u map[string]any) []byte {
	s.program()
	if len(s.buf) != s.lay.Size {
		s.buf = make([]byte, s.lay.Size)
	}
	clear(s.buf)
	for name, v := range u {
		if _, img := v.(*Image); img {
			continue // a frame's image, bound apart
		}
		f, ok := v.([]float32)
		if !ok {
			panic(fmt.Sprintf("render: uniform %s is a %T, not a []float32", name, v))
		}
		s.lay.Put(s.buf, name, f)
	}
	return s.buf
}
