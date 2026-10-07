package gpu

import (
	"bytes"
	"fmt"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

var (
	fillProgram = NewProgram("fill", "", shader("fill"))
	blitLayout  = NewLayout([]Uniform{{"filter", 4}})
	blitProgram = NewProgram("blit", blitLayout.Fields, shader("blit"))
)

// Fill puts the colour (r, g, b, a), premultiplied, 0 to 1, in place of all of dst.
func Fill(dst Image, r, g, b, a float32) {
	rc := dst.rect()
	if cur == nil && windowed { // before the window's GPU is up: into what waits for it
		px := [4]byte{byte(r*255 + 0.5), byte(g*255 + 0.5), byte(b*255 + 0.5), byte(a*255 + 0.5)}
		pix := make([]byte, 0, 4*rc.W*rc.H)
		for range rc.W * rc.H {
			pix = append(pix, px[:]...)
		}
		dst.Texture.WritePixels(rc.X, rc.Y, rc.W, rc.H, pix)
		return
	}
	if rc == (Rect{0, 0, dst.Texture.w, dst.Texture.h}) { // all of it: a pass that clears, no draw
		d := must()
		d.mu.Lock()
		defer d.mu.Unlock()
		t := dst.Texture.gpuTarget(d)
		d.endPass()
		t.load, t.clear = gputypes.LoadOpClear, gputypes.Color{R: float64(r), G: float64(g), B: float64(b), A: float64(a)}
		d.begin(t, nil, false)
		return
	}
	x0, y0, x1, y1 := float32(rc.X), float32(rc.Y), float32(rc.X+rc.W), float32(rc.Y+rc.H)
	v := func(x, y float32) Vertex { return Vertex{DstX: x, DstY: y, ColorR: r, ColorG: g, ColorB: b, ColorA: a} }
	Triangles(&Draw{Target: dst, Program: fillProgram, Blend: Copy},
		[]Vertex{v(x0, y0), v(x1, y0), v(x0, y1), v(x1, y1)}, []uint16{0, 1, 2, 1, 2, 3})
}

// Affine maps a point of a source image to the target: x' = A·x + C·y + TX, y' = B·x + D·y + TY.
type Affine struct{ A, B, C, D, TX, TY float32 }

// Identity is the affine that leaves a point where it is.
var Identity = Affine{A: 1, D: 1}

// Apply is where the affine takes (x, y).
func (m Affine) Apply(x, y float32) (float32, float32) {
	return m.A*x + m.C*y + m.TX, m.B*x + m.D*y + m.TY
}

// DrawImage draws src into dst, its top-left corner at the origin moved by m, its colours times
// color (premultiplied), sampled blended between texels where linear, laid over as blend says.
func DrawImage(dst, src Image, m Affine, color [4]float32, linear bool, blend Blend) {
	r := src.rect()
	w, h := float32(r.W), float32(r.H)
	sx, sy := float32(r.X), float32(r.Y)
	v := func(x, y float32) Vertex {
		dx, dy := m.Apply(x, y)
		return Vertex{DstX: dx, DstY: dy, SrcX: sx + x, SrcY: sy + y, ColorR: color[0], ColorG: color[1], ColorB: color[2], ColorA: color[3]}
	}
	u := make([]byte, blitLayout.Size)
	if linear {
		blitLayout.Put(u, "filter", []float32{1})
	}
	Triangles(&Draw{Target: dst, Program: blitProgram, Images: [4]Image{src}, Uniforms: u, Blend: blend},
		[]Vertex{v(0, 0), v(w, 0), v(0, h), v(w, h)}, []uint16{0, 1, 2, 1, 2, 3})
}

// DrawImageIn draws src into dst, its top-left corner at (x, y), only inside the convex polygon pts
// (dst's pixels, a fan round the first point), laid over as blend says.
func DrawImageIn(dst, src Image, x, y float32, pts [][2]float32, blend Blend) {
	if len(pts) < 3 {
		return
	}
	r := src.rect()
	sx, sy := float32(r.X)-x, float32(r.Y)-y
	verts := make([]Vertex, len(pts))
	for k, p := range pts {
		verts[k] = Vertex{DstX: p[0], DstY: p[1], SrcX: sx + p[0], SrcY: sy + p[1], ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}
	}
	indices := make([]uint16, 0, 3*(len(pts)-2))
	for k := 1; k+1 < len(pts); k++ {
		indices = append(indices, 0, uint16(k), uint16(k+1))
	}
	Triangles(&Draw{Target: dst, Program: blitProgram, Images: [4]Image{src}, Uniforms: make([]byte, blitLayout.Size), Blend: blend}, verts, indices)
}

// Present draws screen over the window's surface view — w by h pixels of format, its texels
// blended where the sizes differ — into enc, the window's frame encoder, which the window submits
// and presents; everything drawn before is submitted first.
func Present(enc *wgpu.CommandEncoder, view *wgpu.TextureView, format gputypes.TextureFormat, w, h int, screen *Texture) error {
	d := must()
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.trySubmit(); err != nil {
		return err
	}
	if err := d.presentBuffers(); err != nil {
		return err
	}
	sw, sh := float32(screen.w), float32(screen.h)
	fw, fh := float32(w), float32(h)
	var verts []byte
	for _, v := range [4][4]float32{{0, 0, 0, 0}, {fw, 0, sw, 0}, {0, fh, 0, sh}, {fw, fh, sw, sh}} {
		for _, f := range [12]float32{v[0], v[1], v[2], v[3], 1, 1, 1, 1, 0, 0, 0, 0} {
			verts = appendF32(verts, f)
		}
	}
	indices := []byte{0, 0, 1, 0, 2, 0, 1, 0, 2, 0, 3, 0}
	var draw []byte
	for _, f := range [28]float32{fw, fh, 0, 0, 0, 0, sw, sh, 20: 1, 21: 1, 24: 1, 25: 1, 26: 1, 27: 1} {
		draw = appendF32(draw, f)
	}
	uniforms := make([]byte, uniformBlock)
	if screen.w != w || screen.h != h {
		blitLayout.Put(uniforms, "filter", []float32{1})
	}
	for k, data := range [][]byte{verts, indices, draw, uniforms} {
		if bytes.Equal(d.presented[k], data) {
			continue // written as it is: Queue.WriteBuffer waits for the GPU
		}
		if err := d.queue.WriteBuffer(d.present[k], 0, data); err != nil {
			return fmt.Errorf("gpu: writing the frame's quad: %w", err)
		}
		d.presented[k] = data
	}
	pipeline := blitProgram.pipeline(d, Copy, format, noDepth)
	images := d.images([4]*Texture{screen})
	pass, err := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{Label: "present", ColorAttachments: []wgpu.RenderPassColorAttachment{{
		View: view, LoadOp: gputypes.LoadOpClear, StoreOp: gputypes.StoreOpStore, ClearValue: gputypes.Color{A: 1},
	}}})
	if err != nil {
		return fmt.Errorf("gpu: the frame's pass: %w", err)
	}
	pass.SetPipeline(pipeline)
	pass.SetBindGroup(0, d.presentGroup, []uint32{0, 0})
	pass.SetBindGroup(1, images, nil)
	pass.SetBindGroup(2, d.noDepth.readGroup(d), nil)
	pass.SetVertexBuffer(0, d.present[0], 0)
	pass.SetIndexBuffer(d.present[1], gputypes.IndexFormatUint16, 0)
	pass.SetScissorRect(gputypes.ScissorRect{Width: uint32(w), Height: uint32(h)})
	pass.DrawIndexed(gputypes.DrawIndexedArgs{IndexCount: 6, InstanceCount: 1})
	return pass.End()
}

// presentBuffers makes, once, the buffers Present draws the window's frame with.
func (d *device) presentBuffers() error {
	if d.presentGroup != nil {
		return nil
	}
	for k, b := range []struct {
		size  uint64
		usage gputypes.BufferUsage
	}{{4 * vertexSize, wgpu.BufferUsageVertex}, {16, wgpu.BufferUsageIndex}, {drawSize, wgpu.BufferUsageUniform}, {uniformBlock, wgpu.BufferUsageUniform}} {
		buf, err := d.dev.CreateBuffer(&wgpu.BufferDescriptor{Label: "present", Size: max(b.size, 256), Usage: b.usage | wgpu.BufferUsageCopyDst})
		if err != nil {
			return fmt.Errorf("gpu: a buffer for the frame: %w", err)
		}
		d.present[k] = buf
	}
	g, err := d.dev.CreateBindGroup(&wgpu.BindGroupDescriptor{Label: "present", Layout: d.layout0, Entries: []wgpu.BindGroupEntry{
		{Binding: 0, Buffer: d.present[2], Size: drawSize},
		{Binding: 1, Buffer: d.present[3], Size: uniformBlock},
	}})
	if err != nil {
		return fmt.Errorf("gpu: the frame's group: %w", err)
	}
	d.presentGroup = g
	return nil
}

// Sprites draws the triangles of verts, their Src in src's texture pixels, sampled nearest or
// blended between texels, their colours times the vertices', laid over as blend says.
func Sprites(dst, src Image, verts []Vertex, indices []uint16, linear bool, blend Blend) {
	u := make([]byte, blitLayout.Size)
	if linear {
		blitLayout.Put(u, "filter", []float32{1})
	}
	Triangles(&Draw{Target: dst, Program: blitProgram, Images: [4]Image{src}, Uniforms: u, Blend: blend}, verts, indices)
}

// Colored draws the triangles of verts in their colours, premultiplied, laid over as blend says.
func Colored(dst Image, verts []Vertex, indices []uint16, blend Blend) {
	Triangles(&Draw{Target: dst, Program: fillProgram, Blend: blend}, verts, indices)
}
