package gpu

import (
	"embed"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

func shader(name string) string {
	b, err := shaders.ReadFile("shaders/" + name + ".wgsl")
	if err != nil {
		panic(err)
	}
	return string(b)
}

// Vertex is one corner of a triangle drawn: where in the target (Dst) and in the first image
// (Src), both in pixels, its colour and four numbers the fragment reads (Custom). DstZ is its
// depth, or with the draw's Sight its height in the world.
type Vertex struct {
	DstX, DstY, DstZ               float32
	SrcX, SrcY                     float32
	ColorR, ColorG, ColorB, ColorA float32
	Custom0, Custom1               float32
	Custom2, Custom3               float32
}

// Rect is a rectangle of a texture, in pixels.
type Rect struct{ X, Y, W, H int }

// Image is a rectangle of a texture a draw reads or draws into; a zero Rect is all of it.
type Image struct {
	Texture *Texture
	Rect    Rect
}

func (i Image) rect() Rect {
	if i.Rect == (Rect{}) && i.Texture != nil {
		return Rect{0, 0, i.Texture.w, i.Texture.h}
	}
	return i.Rect
}

// Blend is how a draw lays its colour over what is there.
type Blend uint8

const (
	// SourceOver lays a premultiplied colour over what is there, as its alpha covers it.
	SourceOver Blend = iota
	// Copy puts the colour in place of what is there.
	Copy
	// Darkest keeps the lesser of the colour and what is there, channel by channel.
	Darkest
	// Multiply scales what is there by the colour, channel by channel, its alpha left as it is.
	Multiply
)

// Program is a WGSL fragment on the package's vertex stage, compiled at first draw.
type Program struct {
	label     string
	source    string
	mesh      bool
	instance  int // vec4s an instance, 0 none
	module    *wgpu.ShaderModule
	pipelines map[pipelineKey]*wgpu.RenderPipeline
	err       error
}

type pipelineKey struct {
	blend  Blend
	format gputypes.TextureFormat
	depth  depthMode
}

type depthMode uint8

const (
	noDepth depthMode = iota
	depthIgnore
	depthTest
	depthWrite
	depthPrime
	depthEqual
	depthCopy
)

// DepthTest is how a draw meets its depth buffer: drawn only where it lies as near as what is
// there or nearer, and the buffer kept or written; or drawn everywhere, the buffer kept.
type DepthTest uint8

const (
	Tested DepthTest = iota
	Written
	Untested
)

func (t DepthTest) mode() depthMode {
	switch t {
	case Written:
		return depthWrite
	case Untested:
		return depthIgnore
	}
	return depthTest
}

// Sight is how the heights of a draw's vertices become depths: the line of sight through the
// target's pixel (x, y) starts at Origin + x·DX + y·DY and runs along Dir + x·DDX + y·DDY; where it
// meets a vertex's height, sunk Bend·d² round Eye, the rows Z and W of the camera's matrix take it.
type Sight struct {
	Origin, DX, DY [3]float32
	Dir, DDX, DDY  [3]float32
	Z, W           [4]float32
	Eye            [3]float32
	Bend           float32
}

// NewProgram is the fragment source over the package's vertex stage, its uniforms the WGSL struct
// fields given, none an empty string.
func NewProgram(label, uniformFields, source string) *Program {
	if strings.TrimSpace(uniformFields) == "" {
		uniformFields = "unused: vec4<f32>,"
	}
	var b strings.Builder
	b.WriteString("struct Uniforms {\n")
	b.WriteString(uniformFields)
	b.WriteString("\n}\n\n")
	b.WriteString(shader("bindings"))
	b.WriteString("\n")
	b.WriteString(shader("quad"))
	b.WriteString("\n")
	b.WriteString(source)
	return &Program{label: label, source: b.String(), pipelines: map[pipelineKey]*wgpu.RenderPipeline{}}
}

// NewMesh is a program with a vertex stage of its own, source defining vs_main and fs_main over
// the package's bindings, its uniforms the WGSL struct fields given.
func NewMesh(label, uniformFields, source string) *Program {
	if strings.TrimSpace(uniformFields) == "" {
		uniformFields = "unused: vec4<f32>,"
	}
	var b strings.Builder
	b.WriteString("struct Uniforms {\n")
	b.WriteString(uniformFields)
	b.WriteString("\n}\n\n")
	b.WriteString(shader("bindings"))
	b.WriteString("\n")
	b.WriteString(source)
	return &Program{label: label, source: b.String(), mesh: true, pipelines: map[pipelineKey]*wgpu.RenderPipeline{}}
}

// Instanced has the mesh program p take vec4s vectors an instance, at the vertex stage's
// locations 0 up.
func (p *Program) Instanced(vec4s int) *Program {
	p.instance = vec4s
	return p
}

// Source is the program's whole WGSL.
func (p *Program) Source() string { return p.source }

// Compile compiles the program now rather than at its first draw.
func (p *Program) Compile() error {
	d := must()
	d.mu.Lock()
	defer d.mu.Unlock()
	return p.compile(d)
}

func (p *Program) compile(d *device) error {
	if p.module != nil || p.err != nil {
		return p.err
	}
	m, err := d.dev.CreateShaderModule(&wgpu.ShaderModuleDescriptor{Label: p.label, WGSL: p.source})
	if err != nil {
		p.err = fmt.Errorf("gpu: the %s program: %w", p.label, err)
		return p.err
	}
	p.module = m
	return nil
}

func (p *Program) pipeline(
	d *device, blend Blend, format gputypes.TextureFormat, depth depthMode,
) *wgpu.RenderPipeline {
	key := pipelineKey{blend, format, depth}
	if pl, ok := p.pipelines[key]; ok {
		return pl
	}
	if err := p.compile(d); err != nil {
		panic(err)
	}
	var state *gputypes.BlendState
	switch blend {
	case SourceOver:
		over := gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd}
		state = &gputypes.BlendState{Color: over, Alpha: over}
	case Darkest:
		least := gputypes.BlendComponent{
			SrcFactor: gputypes.BlendFactorOne,
			DstFactor: gputypes.BlendFactorOne,
			Operation: gputypes.BlendOperationMin,
		}
		state = &gputypes.BlendState{Color: least, Alpha: least}
	case Multiply:
		state = &gputypes.BlendState{
			Color: gputypes.BlendComponent{
				SrcFactor: gputypes.BlendFactorDst,
				DstFactor: gputypes.BlendFactorZero,
				Operation: gputypes.BlendOperationAdd,
			},
			Alpha: gputypes.BlendComponent{
				SrcFactor: gputypes.BlendFactorZero,
				DstFactor: gputypes.BlendFactorOne,
				Operation: gputypes.BlendOperationAdd,
			},
		}
	}
	buffers := []gputypes.VertexBufferLayout{{
		ArrayStride: vertexSize, StepMode: gputypes.VertexStepModeVertex, Attributes: []gputypes.VertexAttribute{
			{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},
			{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1},
			{Format: gputypes.VertexFormatFloat32x4, Offset: 20, ShaderLocation: 2},
			{Format: gputypes.VertexFormatFloat32x4, Offset: 36, ShaderLocation: 3},
		},
	}}
	if p.mesh {
		buffers = nil
		if p.instance > 0 {
			attrs := make([]gputypes.VertexAttribute, p.instance)
			for k := range attrs {
				attrs[k] = gputypes.VertexAttribute{Format: gputypes.VertexFormatFloat32x4, Offset: uint64(16 * k), ShaderLocation: uint32(k)}
			}
			buffers = []gputypes.VertexBufferLayout{{ArrayStride: uint64(16 * p.instance), StepMode: gputypes.VertexStepModeInstance, Attributes: attrs}}
		}
	}
	var ds *wgpu.DepthStencilState
	if depth != noDepth { // reversed: nearer is greater, and at one depth the later wins
		ds = &wgpu.DepthStencilState{Format: DepthFormat,
			DepthCompare: gputypes.CompareFunctionGreaterEqual,
		}
		switch depth {
		case depthWrite, depthPrime:
			ds.DepthWriteEnabled = true
		case depthEqual:
			ds.DepthCompare = gputypes.CompareFunctionEqual
		case depthIgnore:
			ds.DepthCompare = gputypes.CompareFunctionAlways
		case depthCopy:
			ds.DepthWriteEnabled, ds.DepthCompare = true, gputypes.CompareFunctionAlways
		}
	}
	entry, mask := "fs_main", gputypes.ColorWriteMaskAll
	switch depth {
	case depthPrime:
		entry, mask = "fs_prime", gputypes.ColorWriteMaskNone
	case depthCopy:
		mask = gputypes.ColorWriteMaskNone
	}
	pl, err := d.dev.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:        p.label,
		Layout:       d.layout,
		Vertex:       wgpu.VertexState{Module: p.module, EntryPoint: "vs_main", Buffers: buffers},
		DepthStencil: ds,
		Primitive:    gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList, FrontFace: gputypes.FrontFaceCCW, CullMode: gputypes.CullModeNone},
		Multisample:  gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &wgpu.FragmentState{Module: p.module, EntryPoint: entry, Targets: []gputypes.ColorTargetState{
			{Format: format, Blend: state, WriteMask: mask},
		}},
	})
	if err != nil {
		panic(fmt.Errorf("gpu: the %s pipeline: %w", p.label, err))
	}
	p.pipelines[key] = pl
	return pl
}

// Draw is a draw of triangles into a rectangle of a texture, clipped to it.
type Draw struct {
	Target  Image
	Program *Program
	Images  [4]Image
	// Frame is the images of the frame drawn, which every program reads by name; nil a blank.
	Frame    [FrameImages]*Texture
	Uniforms []byte // packed by WGSL's layout (Layout)
	Blend    Blend
	// Place lays the vertices' Dst on the target scaled by its first two and moved by its last
	// two; zero, as they are.
	Place [4]float32
	// Tint multiplies the vertices' colours; zero, white.
	Tint [4]float32
	// Depth is the depth buffer the draw meets as Test says; nil, none.
	Depth *Depth
	Test  DepthTest
	// Sight takes the vertices' DstZ for heights; nil, for depths as they are.
	Sight *Sight
}

// Triangles draws the triangles indices picks out of verts as dw says.
func Triangles(dw *Draw, verts []Vertex, indices []uint16) {
	if len(indices) == 0 {
		return
	}
	d := must()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.triangles(dw.Target.Texture.gpuTarget(d), dw.Target.rect(), dw, verts, indices)
}

func (d *device) triangles(t *target, clip Rect, dw *Draw, verts []Vertex, indices []uint16) {
	if len(dw.Uniforms) > uniformBlock {
		panic(fmt.Sprintf("gpu: %d bytes of uniforms, more than %d", len(dw.Uniforms), uniformBlock))
	}
	clip = clip.intersect(Rect{0, 0, t.w, t.h})
	if clip.W <= 0 || clip.H <= 0 {
		return
	}
	var imgs [4]*Texture
	for i, im := range dw.Images {
		imgs[i] = im.Texture
	}
	mode := dw.depthMode(t)
	pipeline := dw.Program.pipeline(d, dw.Blend, t.format, mode)
	group1 := d.images(imgs) // realises the images before the buffers are measured
	group3 := d.frame(dw.Frame)
	d.ensure(len(verts), len(indices))

	vOff := uint64(len(d.verts))
	for _, v := range verts {
		d.verts = appendVertex(d.verts, v)
	}
	iOff := uint64(len(d.indices))
	for _, i := range indices {
		d.indices = binary.LittleEndian.AppendUint16(d.indices, i)
	}
	for len(d.indices)%4 != 0 {
		d.indices = append(d.indices, 0)
	}
	dOff, uOff := d.drawBlock(t, dw.Images, dw.Uniforms, dw.Place, dw.Tint, dw.Sight)

	pass := d.begin(t, dw.Depth, false)
	pass.SetPipeline(pipeline) // gogpu's Vulkan binds a group through the pipeline's layout: first
	pass.SetBindGroup(0, d.group0, []uint32{uint32(dOff), uint32(uOff)})
	pass.SetBindGroup(1, group1, nil)
	pass.SetBindGroup(2, d.noDepth.readGroup(d), nil)
	d.bindFrame(pass, group3)
	pass.SetVertexBuffer(0, d.vbuf, vOff)
	pass.SetIndexBuffer(d.ibuf, gputypes.IndexFormatUint16, iOff)
	pass.SetScissorRect(gputypes.ScissorRect{X: uint32(clip.X), Y: uint32(clip.Y), Width: uint32(clip.W), Height: uint32(clip.H)})
	pass.DrawIndexed(gputypes.DrawIndexedArgs{IndexCount: uint32(len(indices)), InstanceCount: 1})
}

// depthMode fits the draw's depth buffer to t, the mode it is drawn in
func (dw *Draw) depthMode(t *target) depthMode {
	if dw.Depth == nil {
		return noDepth
	}
	if dw.Depth.w != t.w || dw.Depth.h != t.h {
		dw.Depth.fit(t.w, t.h)
	}
	return dw.Test.mode()
}

func appendVertex(b []byte, v Vertex) []byte {
	for _, f := range [13]float32{
		v.DstX, v.DstY, v.DstZ, v.SrcX, v.SrcY, v.ColorR, v.ColorG, v.ColorB, v.ColorA,
		v.Custom0, v.Custom1, v.Custom2, v.Custom3,
	} {
		b = appendF32(b, f)
	}
	return b
}

func (d *device) drawBlock(
	t *target, images [4]Image, uniforms []byte, place, tint [4]float32, sight *Sight,
) (dOff, uOff uint64) {
	if len(uniforms) > uniformBlock {
		panic(fmt.Sprintf("gpu: %d bytes of uniforms, more than %d", len(uniforms), uniformBlock))
	}
	dOff = alignUp(uint64(len(d.draws)), d.align)
	d.draws = append(d.draws, make([]byte, int(dOff)-len(d.draws))...)
	d.draws = appendF32(appendF32(appendF32(appendF32(d.draws, float32(t.w)), float32(t.h)), 0), 0)
	for _, im := range images {
		r := im.rect()
		if im.Texture == nil {
			r = Rect{}
		}
		d.draws = appendF32(appendF32(appendF32(appendF32(d.draws, float32(r.X)), float32(r.Y)), float32(r.W)), float32(r.H))
	}
	if place == ([4]float32{}) {
		place = [4]float32{1, 1, 0, 0}
	}
	if tint == ([4]float32{}) {
		tint = [4]float32{1, 1, 1, 1}
	}
	for _, v := range append(place[:], tint[:]...) {
		d.draws = appendF32(d.draws, v)
	}
	d.draws = appendSight(d.draws, sight)
	uOff = alignUp(uint64(len(d.uniforms)), d.align)
	d.uniforms = append(d.uniforms, make([]byte, int(uOff)-len(d.uniforms))...)
	d.uniforms = append(d.uniforms, uniforms...)
	d.uniforms = append(d.uniforms, make([]byte, uniformBlock-len(uniforms))...)
	return dOff, uOff
}

// appendSight writes the Draw's sight: the rays in five vec4s, the matrix's rows Z and W, the eye
// and the bend; w of the first 1 for a sight, 0 for none
func appendSight(b []byte, s *Sight) []byte {
	var f [36]float32
	if s != nil {
		vec := func(k int, v [3]float32, w float32) {
			f[4*k], f[4*k+1], f[4*k+2], f[4*k+3] = v[0], v[1], v[2], w
		}
		vec(0, s.Origin, 1)
		vec(1, s.DX, 0)
		vec(2, s.DY, 0)
		vec(3, s.Dir, 0)
		vec(4, s.DDX, 0)
		vec(5, s.DDY, 0)
		copy(f[24:28], s.Z[:])
		copy(f[28:32], s.W[:])
		vec(8, s.Eye, s.Bend)
	}
	for _, v := range f {
		b = appendF32(b, v)
	}
	return b
}

func (r Rect) intersect(o Rect) Rect {
	x0, y0 := max(r.X, o.X), max(r.Y, o.Y)
	x1, y1 := min(r.X+r.W, o.X+o.W), min(r.Y+r.H, o.Y+o.H)
	if x1 <= x0 || y1 <= y0 {
		return Rect{}
	}
	return Rect{x0, y0, x1 - x0, y1 - y0}
}
