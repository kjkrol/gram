package gpu

import (
	"encoding/binary"
	"fmt"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// DepthFormat is what a depth buffer holds: 32-bit floats, reversed — 1 nearest, 0 furthest.
const DepthFormat = gputypes.TextureFormatDepth32Float

// Depth is a depth buffer for meshes, the size of the texture they are drawn into.
type Depth struct {
	w, h  int
	tex   *wgpu.Texture
	view  *wgpu.TextureView
	group *wgpu.BindGroup
	// the GPU keeps a buffer that is only drawn against packed, faster
	readable bool
}

// NewDepth is a depth buffer, sized at each draw to the texture drawn into.
func NewDepth() *Depth { return &Depth{} }

// Readable has dp read as well as drawn against (MeshDraw.ReadDepth, CopyDepth); asked before its
// first draw, it keeps what it holds. A buffer only drawn against is kept packed, faster.
func (dp *Depth) Readable() *Depth {
	if !dp.readable {
		dp.readable = true
		if dp.tex != nil {
			dp.Release()
		}
	}
	return dp
}

// Size is the buffer's width and height: those of the texture it was last drawn with.
func (dp *Depth) Size() (w, h int) { return dp.w, dp.h }

func (dp *Depth) fit(w, h int) {
	if dp.w == w && dp.h == h {
		return
	}
	dp.free()
	dp.w, dp.h = w, h
}

func (dp *Depth) gpuView(d *device) *wgpu.TextureView {
	if dp.view != nil {
		return dp.view
	}
	usage := gputypes.TextureUsageRenderAttachment
	if dp.readable {
		usage |= gputypes.TextureUsageTextureBinding
	}
	tex, err := d.dev.CreateTexture(&wgpu.TextureDescriptor{Label: "depth", Size: wgpu.Extent3D{Width: uint32(dp.w), Height: uint32(dp.h), DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: gputypes.TextureDimension2D, Format: DepthFormat, Usage: usage})
	if err != nil {
		panic(fmt.Errorf("gpu: a %dx%d depth buffer: %w", dp.w, dp.h, err))
	}
	view, err := d.dev.CreateTextureView(tex, &wgpu.TextureViewDescriptor{Format: DepthFormat, Dimension: gputypes.TextureViewDimension2D, Aspect: gputypes.TextureAspectDepthOnly, MipLevelCount: 1, ArrayLayerCount: 1})
	if err != nil {
		panic(fmt.Errorf("gpu: a depth view: %w", err))
	}
	dp.tex, dp.view = tex, view
	return view
}

func (dp *Depth) readGroup(d *device) *wgpu.BindGroup {
	if dp.group != nil && dp.view != nil {
		return dp.group
	}
	g, err := d.dev.CreateBindGroup(&wgpu.BindGroupDescriptor{Label: "gpu depth", Layout: d.layout2, Entries: []wgpu.BindGroupEntry{{Binding: 0, TextureView: dp.gpuView(d)}}})
	if err != nil {
		panic(fmt.Errorf("gpu: a depth group: %w", err))
	}
	dp.group = g
	return g
}

// Release frees the buffer.
func (dp *Depth) Release() {
	if dp.tex == nil || cur == nil {
		return
	}
	cur.mu.Lock()
	defer cur.mu.Unlock()
	dp.free()
}

func (dp *Depth) free() {
	if dp.tex == nil {
		return
	}
	group, view, tex := dp.group, dp.view, dp.tex
	cur.retire(func() {
		if group != nil {
			group.Release()
		}
		view.Release()
		tex.Release()
	})
	dp.tex, dp.view, dp.group = nil, nil, nil
}

// Indices are a mesh's triangles, three vertex indices each, kept on the GPU.
type Indices struct {
	data  []uint32
	buf   *wgpu.Buffer
	count uint32
}

// NewIndices keeps the triangles of indices, going to the GPU at their first draw.
func NewIndices(indices []uint32) *Indices {
	return &Indices{data: append([]uint32(nil), indices...), count: uint32(len(indices))}
}

func (x *Indices) gpuBuffer(d *device) *wgpu.Buffer {
	if x.buf != nil {
		return x.buf
	}
	data := make([]byte, 0, 4*len(x.data))
	for _, i := range x.data {
		data = binary.LittleEndian.AppendUint32(data, i)
	}
	buf, err := d.dev.CreateBuffer(&wgpu.BufferDescriptor{Label: "mesh indices", Size: uint64(max(len(data), 4)), Usage: wgpu.BufferUsageIndex | wgpu.BufferUsageCopyDst})
	if err != nil {
		panic(fmt.Errorf("gpu: a mesh's indices: %w", err))
	}
	if err := d.queue.WriteBuffer(buf, 0, data); err != nil {
		panic(fmt.Errorf("gpu: writing a mesh's indices: %w", err))
	}
	x.buf, x.data = buf, nil
	return buf
}

// Release frees the indices on the GPU.
func (x *Indices) Release() {
	if x.buf != nil {
		retire(x.buf.Release) // a draw gathered or submitted may still read it
		x.buf = nil
	}
}

// MeshDraw is a draw of a mesh (NewMesh) into a rectangle of a texture, tested against the depth
// buffer Depth and written to it where Write says; without a Depth it is drawn whole, untested.
// Primed, its depth is laid first alone by the program's fs_prime, and its colour then only where
// the depth is met exactly.
type MeshDraw struct {
	Target     Image
	Depth      *Depth
	ClearDepth bool
	Write      bool
	Primed     bool
	Program    *Program
	Images     [4]Image
	// Frame is the images of the frame drawn, which every program reads by name; nil a blank.
	Frame     [FrameImages]*Texture
	Uniforms  []byte
	Indices   *Indices
	Vertices  uint32    // without Indices: how many vertices, in order, three a triangle
	Instances []float32 // an Instanced program's: its vec4s an instance, the mesh drawn once each
	Blend     Blend
	// ReadDepth is a depth buffer the program reads (depthAt), another than Depth, at the size it
	// was drawn at.
	ReadDepth *Depth
	// Untested draws everywhere, the Depth kept as it is.
	Untested bool
}

// ClearDepth clears the depth buffer dp, fitted to target's texture.
func ClearDepth(target Image, dp *Depth) {
	d := must()
	d.mu.Lock()
	defer d.mu.Unlock()
	t := target.Texture.gpuTarget(d)
	if dp.w != t.w || dp.h != t.h {
		dp.fit(t.w, t.h)
	}
	d.begin(t, dp, true)
}

// Mesh draws the triangles of dw's Indices with its program.
func Mesh(dw *MeshDraw) {
	d := must()
	d.mu.Lock()
	defer d.mu.Unlock()
	t := dw.Target.Texture.gpuTarget(d)
	clip := dw.Target.rect().intersect(Rect{0, 0, t.w, t.h})
	count := dw.Vertices
	if dw.Indices != nil {
		count = dw.Indices.count
	}
	instances := uint32(1)
	if n := dw.Program.instance; n > 0 {
		instances = uint32(len(dw.Instances) / (4 * n))
	}
	if clip.W <= 0 || clip.H <= 0 || count == 0 || instances == 0 {
		return
	}
	mode := depthTest
	if dw.Write {
		mode = depthWrite
	}
	if dw.Primed {
		mode = depthEqual
	}
	if dw.Untested {
		mode = depthIgnore
	}
	if dw.Depth == nil {
		mode = noDepth
	}
	var imgs [4]*Texture
	for i, im := range dw.Images {
		imgs[i] = im.Texture
	}
	if dw.Depth != nil && (dw.Depth.w != t.w || dw.Depth.h != t.h) {
		dw.Depth.fit(t.w, t.h)
	}
	pipeline := dw.Program.pipeline(d, dw.Blend, t.format, mode)
	group1 := d.images(imgs)
	read := d.noDepth
	if dw.ReadDepth != nil {
		if dw.ReadDepth == dw.Depth {
			panic("gpu: a mesh reads the depth buffer it is drawn against")
		}
		read = dw.ReadDepth
		if read.w == 0 || read.h == 0 {
			read.fit(t.w, t.h) // never drawn; one drawn keeps its size, read by any target
		}
		if !read.readable {
			panic("gpu: a mesh reads a depth buffer that is not Readable")
		}
	}
	group2 := read.readGroup(d)
	group3 := d.frame(dw.Frame)
	var indices *wgpu.Buffer
	if dw.Indices != nil {
		indices = dw.Indices.gpuBuffer(d)
	}
	d.ensure((4*len(dw.Instances)+vertexSize-1)/vertexSize, 0)
	iOff := uint64(len(d.verts))
	if dw.Program.instance > 0 {
		for _, f := range dw.Instances[:int(instances)*4*dw.Program.instance] {
			d.verts = appendF32(d.verts, f)
		}
	}
	dOff, uOff := d.drawBlock(t, dw.Images, dw.Uniforms, [4]float32{}, [4]float32{}, nil)
	pass := d.begin(t, dw.Depth, dw.ClearDepth && dw.Depth != nil)
	if dw.Primed && dw.Depth != nil {
		r := dw.Target.rect()
		pass.SetViewport(gputypes.Viewport{X: float32(r.X), Y: float32(r.Y), Width: float32(r.W), Height: float32(r.H), MaxDepth: 1})
		pass.SetScissorRect(gputypes.ScissorRect{X: uint32(clip.X), Y: uint32(clip.Y), Width: uint32(clip.W), Height: uint32(clip.H)})
		pass.SetPipeline(dw.Program.pipeline(d, dw.Blend, t.format, depthPrime))
		pass.SetBindGroup(0, d.group0, []uint32{uint32(dOff), uint32(uOff)})
		pass.SetBindGroup(1, group1, nil)
		pass.SetBindGroup(2, group2, nil)
		d.bindFrame(pass, group3)
		if dw.Program.instance > 0 {
			pass.SetVertexBuffer(0, d.vbuf, iOff)
		}
		draw(pass, indices, count, instances)
	}
	pass.SetPipeline(pipeline)
	pass.SetBindGroup(0, d.group0, []uint32{uint32(dOff), uint32(uOff)})
	pass.SetBindGroup(1, group1, nil)
	pass.SetBindGroup(2, group2, nil)
	d.bindFrame(pass, group3)
	if dw.Program.instance > 0 {
		pass.SetVertexBuffer(0, d.vbuf, iOff)
	}
	// clip space spans the target's rectangle, not the whole texture
	r := dw.Target.rect()
	pass.SetViewport(gputypes.Viewport{X: float32(r.X), Y: float32(r.Y), Width: float32(r.W), Height: float32(r.H), MaxDepth: 1})
	pass.SetScissorRect(gputypes.ScissorRect{X: uint32(clip.X), Y: uint32(clip.Y), Width: uint32(clip.W), Height: uint32(clip.H)})
	draw(pass, indices, count, instances)
	pass.SetViewport(gputypes.Viewport{Width: float32(t.w), Height: float32(t.h), MaxDepth: 1})
}

var copyProgram = NewMesh("depth copy", "", shader("depth"))

// CopyDepth puts what from holds in place of what to holds, both fitted to target's texture: a
// draw, since a texture copy of a depth is wrong on some backends. From must be Readable.
func CopyDepth(target Image, from, to *Depth) {
	d := must()
	d.mu.Lock()
	defer d.mu.Unlock()
	if !from.readable {
		panic("gpu: a depth copied from is not Readable")
	}
	t := target.Texture.gpuTarget(d)
	for _, dp := range []*Depth{from, to} {
		if dp.w != t.w || dp.h != t.h {
			dp.fit(t.w, t.h)
		}
	}
	pipeline := copyProgram.pipeline(d, Copy, t.format, depthCopy)
	group1 := d.images([4]*Texture{})
	group2 := from.readGroup(d)
	group3 := d.frame([FrameImages]*Texture{})
	d.ensure(0, 0)
	dOff, uOff := d.drawBlock(t, [4]Image{}, nil, [4]float32{}, [4]float32{}, nil)
	pass := d.begin(t, to, false)
	pass.SetPipeline(pipeline)
	pass.SetBindGroup(0, d.group0, []uint32{uint32(dOff), uint32(uOff)})
	pass.SetBindGroup(1, group1, nil)
	pass.SetBindGroup(2, group2, nil)
	d.bindFrame(pass, group3)
	pass.SetViewport(gputypes.Viewport{Width: float32(t.w), Height: float32(t.h), MaxDepth: 1})
	pass.SetScissorRect(gputypes.ScissorRect{Width: uint32(t.w), Height: uint32(t.h)})
	pass.Draw(gputypes.DrawArgs{VertexCount: 3, InstanceCount: 1})
}

func draw(pass *wgpu.RenderPassEncoder, indices *wgpu.Buffer, count, instances uint32) {
	if indices == nil {
		pass.Draw(gputypes.DrawArgs{VertexCount: count, InstanceCount: instances})
		return
	}
	pass.SetIndexBuffer(indices, gputypes.IndexFormatUint32, 0)
	pass.DrawIndexed(gputypes.DrawIndexedArgs{IndexCount: count, InstanceCount: instances})
}
