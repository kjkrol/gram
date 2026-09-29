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

// Vertex is one corner of a triangle drawn: where in the target (Dst), where in the first image
// (Src), both in pixels, its colour and four numbers the fragment reads (Custom), interpolated.
type Vertex struct {
	DstX, DstY                     float32
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
)

// Program is a WGSL fragment on the package's vertex stage (see the package's doc), compiled at
// first draw.
type Program struct {
	label     string
	source    string
	mesh      bool // its own vertex stage, reading no vertex buffer
	instance  int  // a mesh's: how many vec4s an instance hands its vertex stage, 0 none
	module    *wgpu.ShaderModule
	pipelines map[pipelineKey]*wgpu.RenderPipeline
	err       error
}

type pipelineKey struct {
	blend  Blend
	format gputypes.TextureFormat
	depth  depthMode
}

// depthMode is how a draw meets the depth buffer: none, tested, tested and written, written alone
// by fs_prime, met exactly.
type depthMode uint8

const (
	noDepth depthMode = iota
	depthTest
	depthWrite
	depthPrime
	depthEqual
)

// NewProgram is the fragment source over the package's vertex stage, its uniforms the WGSL
// struct fields given (none: an empty string).
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

// NewMesh is a program with a vertex stage of its own — source defines vs_main, reading the vertex
// it draws from its @builtin(vertex_index) and the images, and fs_main — over the package's
// bindings, its uniforms the WGSL struct fields given; it draws with a depth buffer (Mesh).
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

// Instanced has the mesh program p take vec4s vectors an instance (MeshDraw.Instances), at the
// vertex stage's locations 0 up; it returns p.
func (p *Program) Instanced(vec4s int) *Program {
	p.instance = vec4s
	return p
}

// Source is the program's whole WGSL.
func (p *Program) Source() string { return p.source }

// Compile compiles the program now rather than at its first draw: it needs a device.
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

func (p *Program) pipeline(d *device, blend Blend, format gputypes.TextureFormat, depth depthMode) *wgpu.RenderPipeline {
	key := pipelineKey{blend, format, depth}
	if pl, ok := p.pipelines[key]; ok {
		return pl
	}
	if err := p.compile(d); err != nil {
		panic(err)
	}
	var state *gputypes.BlendState
	if blend == SourceOver {
		over := gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd}
		state = &gputypes.BlendState{Color: over, Alpha: over}
	}
	buffers := []gputypes.VertexBufferLayout{{
		ArrayStride: vertexSize, StepMode: gputypes.VertexStepModeVertex, Attributes: []gputypes.VertexAttribute{
			{Format: gputypes.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0},
			{Format: gputypes.VertexFormatFloat32x2, Offset: 8, ShaderLocation: 1},
			{Format: gputypes.VertexFormatFloat32x4, Offset: 16, ShaderLocation: 2},
			{Format: gputypes.VertexFormatFloat32x4, Offset: 32, ShaderLocation: 3},
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
	if depth != noDepth { // the depth reversed: nearer is greater
		ds = &wgpu.DepthStencilState{Format: DepthFormat, DepthWriteEnabled: depth == depthWrite || depth == depthPrime, DepthCompare: gputypes.CompareFunctionGreater}
		if depth == depthEqual {
			ds.DepthCompare = gputypes.CompareFunctionEqual
		}
	}
	entry, mask := "fs_main", gputypes.ColorWriteMaskAll
	if depth == depthPrime {
		entry, mask = "fs_prime", gputypes.ColorWriteMaskNone
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
	Target   Image
	Program  *Program
	Images   [4]Image
	Uniforms []byte // the program's Uniforms, packed by WGSL's layout (Layout)
	Blend    Blend
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
	pipeline := dw.Program.pipeline(d, dw.Blend, t.format, noDepth)
	group1 := d.images(imgs) // realizes the images before the buffers are measured
	d.ensure(len(verts), len(indices))

	vOff := uint64(len(d.verts))
	for _, v := range verts {
		for _, f := range [12]float32{v.DstX, v.DstY, v.SrcX, v.SrcY, v.ColorR, v.ColorG, v.ColorB, v.ColorA, v.Custom0, v.Custom1, v.Custom2, v.Custom3} {
			d.verts = appendF32(d.verts, f)
		}
	}
	iOff := uint64(len(d.indices))
	for _, i := range indices {
		d.indices = binary.LittleEndian.AppendUint16(d.indices, i)
	}
	for len(d.indices)%4 != 0 {
		d.indices = append(d.indices, 0)
	}
	dOff, uOff := d.drawBlock(t, dw.Images, dw.Uniforms)

	pass := d.begin(t, nil, false)
	pass.SetPipeline(pipeline) // gogpu's Vulkan binds a group through the pipeline's layout: first
	pass.SetBindGroup(0, d.group0, []uint32{uint32(dOff), uint32(uOff)})
	pass.SetBindGroup(1, group1, nil)
	pass.SetBindGroup(2, d.noDepth.readGroup(d), nil)
	pass.SetVertexBuffer(0, d.vbuf, vOff)
	pass.SetIndexBuffer(d.ibuf, gputypes.IndexFormatUint16, iOff)
	pass.SetScissorRect(gputypes.ScissorRect{X: uint32(clip.X), Y: uint32(clip.Y), Width: uint32(clip.W), Height: uint32(clip.H)})
	pass.DrawIndexed(gputypes.DrawIndexedArgs{IndexCount: uint32(len(indices)), InstanceCount: 1})
}

// drawBlock gathers a draw's block — the target's size and the images' rectangles — and its
// uniforms, and says where each lies in its buffer.
func (d *device) drawBlock(t *target, images [4]Image, uniforms []byte) (dOff, uOff uint64) {
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
	uOff = alignUp(uint64(len(d.uniforms)), d.align)
	d.uniforms = append(d.uniforms, make([]byte, int(uOff)-len(d.uniforms))...)
	d.uniforms = append(d.uniforms, uniforms...)
	d.uniforms = append(d.uniforms, make([]byte, uniformBlock-len(uniforms))...)
	return dOff, uOff
}

func (r Rect) intersect(o Rect) Rect {
	x0, y0 := max(r.X, o.X), max(r.Y, o.Y)
	x1, y1 := min(r.X+r.W, o.X+o.W), min(r.Y+r.H, o.Y+o.H)
	if x1 <= x0 || y1 <= y0 {
		return Rect{}
	}
	return Rect{x0, y0, x1 - x0, y1 - y0}
}
