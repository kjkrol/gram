package render

import (
	"fmt"
	"image"
	"image/color"

	"github.com/kjkrol/gram/render/gpu"
)

// Vertex is one corner of a triangle drawn: where in the target (Dst) and in the first image (Src),
// both in pixels, its colour and four numbers the fragment reads (Custom).
type Vertex = gpu.Vertex

// Uniform is one of a shader's uniforms: Size floats, 1 to 4.
type Uniform = gpu.Uniform

// Image is a picture on the GPU: a texture, or a rectangle of one (SubImage), in whose pixels what
// is drawn into it is placed and to which it is clipped. A texture made before the window's GPU
// is up keeps what is written into it until then.
type Image struct {
	tex   *gpu.Texture
	rect  image.Rectangle
	owner bool // made by NewImage: Deallocate frees the texture
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

func (i *Image) gpu() gpu.Image {
	r := i.rect
	return gpu.Image{Texture: i.tex, Rect: gpu.Rect{X: r.Min.X, Y: r.Min.Y, W: r.Dx(), H: r.Dy()}}
}

// Fill puts c in place of every pixel of the image.
func (i *Image) Fill(c color.Color) {
	r, g, b, a := rgbaOf(c)
	gpu.Fill(i.gpu(), r, g, b, a)
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
	a1, b, c, d1, tx, ty float64 // a1 and d1 are the diagonal minus one, so the zero is the identity
}

// Reset makes g the identity.
func (g *GeoM) Reset() { *g = GeoM{} }

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

// Apply is where g takes (x, y).
func (g GeoM) Apply(x, y float64) (float64, float64) {
	return (g.a1+1)*x + g.c*y + g.tx, g.b*x + (g.d1+1)*y + g.ty
}

func (g GeoM) affine() gpu.Affine {
	return gpu.Affine{A: float32(g.a1 + 1), B: float32(g.b), C: float32(g.c), D: float32(g.d1 + 1), TX: float32(g.tx), TY: float32(g.ty)}
}

// DrawImageOptions is how DrawImage draws: through GeoM, sampled by Filter.
type DrawImageOptions struct {
	GeoM   GeoM
	Filter Filter
}

// DrawImage lays src over the image, its top-left corner at the origin moved by op's GeoM.
func (i *Image) DrawImage(src *Image, op *DrawImageOptions) {
	var o DrawImageOptions
	if op != nil {
		o = *op
	}
	gpu.DrawImage(i.gpu(), src.gpu(), o.GeoM.affine(), [4]float32{1, 1, 1, 1}, o.Filter == FilterLinear, gpu.SourceOver)
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
				imgs[k] = im.gpu()
			}
		}
		u = op.Uniforms
	}
	gpu.Triangles(&gpu.Draw{Target: i.gpu(), Program: s.program(), Images: imgs, Uniforms: s.pack(u), Blend: gpu.SourceOver}, verts, indices)
}

// rgbaOf is c's red, green, blue and alpha, 0 to 1, the colour premultiplied by alpha.
func rgbaOf(c color.Color) (r, g, b, a float32) {
	cr, cg, cb, ca := c.RGBA()
	return float32(cr) / 0xffff, float32(cg) / 0xffff, float32(cb) / 0xffff, float32(ca) / 0xffff
}

// Shader is a WGSL fragment with its uniforms (see render/gpu), for DrawTrianglesShader.
type Shader struct {
	label  string
	source func() string // its WGSL once the materials are all in
	layout func() *gpu.Layout
	mesh   bool // a vertex stage of its own (NewMeshShaderWith)
	vec4s  int  // a mesh's vectors an instance (Instanced), 0 none
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

// Compile compiles the shader now, rather than at its first draw: for a test.
func (s *Shader) Compile() error { return s.program().Compile() }

// Source is the shader's whole WGSL.
func (s *Shader) Source() string { return s.program().Source() }

// pack is the uniforms u laid out as the shader's struct holds them, every other zero.
func (s *Shader) pack(u map[string]any) []byte {
	s.program()
	if len(s.buf) != s.lay.Size {
		s.buf = make([]byte, s.lay.Size)
	}
	clear(s.buf)
	for name, v := range u {
		f, ok := v.([]float32)
		if !ok {
			panic(fmt.Sprintf("render: uniform %s is a %T, not a []float32", name, v))
		}
		s.lay.Put(s.buf, name, f)
	}
	return s.buf
}

// Texture is the image's texture: for the engine, which presents it on the window.
func (i *Image) Texture() *gpu.Texture { return i.tex }
