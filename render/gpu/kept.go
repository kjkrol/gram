package gpu

import (
	"encoding/binary"
	"fmt"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// Kept is triangles kept on the GPU: their vertices and indices written once, at their first draw,
// and drawn frame after frame without going there again.
type Kept struct {
	verts   []Vertex
	indices []uint16
	vbuf    *wgpu.Buffer
	ibuf    *wgpu.Buffer
	count   uint32
}

// NewKept keeps the triangles indices picks out of verts, copied.
func NewKept(verts []Vertex, indices []uint16) *Kept {
	return &Kept{verts: append([]Vertex(nil), verts...), indices: append([]uint16(nil), indices...), count: uint32(len(indices))}
}

// buffers are k's buffers on the GPU, written at the first call.
func (k *Kept) buffers(d *device) (*wgpu.Buffer, *wgpu.Buffer) {
	if k.vbuf != nil {
		return k.vbuf, k.ibuf
	}
	verts := make([]byte, 0, vertexSize*len(k.verts))
	for _, v := range k.verts {
		for _, f := range [12]float32{v.DstX, v.DstY, v.SrcX, v.SrcY, v.ColorR, v.ColorG, v.ColorB, v.ColorA, v.Custom0, v.Custom1, v.Custom2, v.Custom3} {
			verts = appendF32(verts, f)
		}
	}
	indices := make([]byte, 0, 2*len(k.indices)+2)
	for _, i := range k.indices {
		indices = binary.LittleEndian.AppendUint16(indices, i)
	}
	for len(indices)%4 != 0 {
		indices = append(indices, 0)
	}
	k.vbuf = d.keep("kept vertices", verts, wgpu.BufferUsageVertex)
	k.ibuf = d.keep("kept indices", indices, wgpu.BufferUsageIndex)
	k.verts, k.indices = nil, nil
	return k.vbuf, k.ibuf
}

// keep is a buffer on the GPU holding data, used as usage says.
func (d *device) keep(label string, data []byte, usage gputypes.BufferUsage) *wgpu.Buffer {
	buf, err := d.dev.CreateBuffer(&wgpu.BufferDescriptor{Label: label, Size: uint64(max(len(data), 4)), Usage: usage | wgpu.BufferUsageCopyDst})
	if err != nil {
		panic(fmt.Errorf("gpu: %s: %w", label, err))
	}
	if err := d.queue.WriteBuffer(buf, 0, data); err != nil {
		panic(fmt.Errorf("gpu: writing %s: %w", label, err))
	}
	return buf
}

// Release frees k on the GPU.
func (k *Kept) Release() {
	for _, b := range []*wgpu.Buffer{k.vbuf, k.ibuf} {
		if b != nil {
			b.Release()
		}
	}
	k.vbuf, k.ibuf = nil, nil
}

// DrawKept draws the kept triangles k as dw says.
func DrawKept(dw *Draw, k *Kept) {
	if k.count == 0 {
		return
	}
	d := must()
	d.mu.Lock()
	defer d.mu.Unlock()
	t := dw.Target.Texture.gpuTarget(d)
	clip := dw.Target.rect().intersect(Rect{0, 0, t.w, t.h})
	if clip.W <= 0 || clip.H <= 0 {
		return
	}
	var imgs [4]*Texture
	for i, im := range dw.Images {
		imgs[i] = im.Texture
	}
	pipeline := dw.Program.pipeline(d, dw.Blend, t.format, noDepth)
	group1 := d.images(imgs)
	vbuf, ibuf := k.buffers(d)
	d.ensure(0, 0)
	dOff, uOff := d.drawBlock(t, dw.Images, dw.Uniforms, dw.Place, dw.Tint)
	pass := d.begin(t, nil, false)
	pass.SetPipeline(pipeline)
	pass.SetBindGroup(0, d.group0, []uint32{uint32(dOff), uint32(uOff)})
	pass.SetBindGroup(1, group1, nil)
	pass.SetBindGroup(2, d.noDepth.readGroup(d), nil)
	pass.SetVertexBuffer(0, vbuf, 0)
	pass.SetIndexBuffer(ibuf, gputypes.IndexFormatUint16, 0)
	pass.SetScissorRect(gputypes.ScissorRect{X: uint32(clip.X), Y: uint32(clip.Y), Width: uint32(clip.W), Height: uint32(clip.H)})
	pass.DrawIndexed(gputypes.DrawIndexedArgs{IndexCount: k.count, InstanceCount: 1})
}
