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
	if cur == nil && windowed {
		px := [4]byte{byte(r*255 + 0.5), byte(g*255 + 0.5), byte(b*255 + 0.5), byte(a*255 + 0.5)}
		pix := make([]byte, 0, 4*rc.W*rc.H)
		for range rc.W * rc.H {
			pix = append(pix, px[:]...)
		}
		dst.Texture.WritePixels(rc.X, rc.Y, rc.W, rc.H, pix)
		return
	}
	if rc == (Rect{0, 0, dst.Texture.w, dst.Texture.h}) {
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

// FillBehind puts the colour (r, g, b, a), premultiplied, 0 to 1, over all of dst where nothing was
// drawn into dp: at the furthest depth, tested.
func FillBehind(dst Image, dp *Depth, r, g, b, a float32) {
	rc := dst.rect()
	x0, y0, x1, y1 := float32(rc.X), float32(rc.Y), float32(rc.X+rc.W), float32(rc.Y+rc.H)
	v := func(x, y float32) Vertex {
		return Vertex{DstX: x, DstY: y, ColorR: r, ColorG: g, ColorB: b, ColorA: a}
	}
	Triangles(&Draw{Target: dst, Program: fillProgram, Blend: Copy, Depth: dp, Test: Tested},
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
// color (premultiplied), sampled between texels where linear.
func DrawImage(dst, src Image, m Affine, color [4]float32, linear bool, blend Blend) {
	r := src.rect()
	w, h := float32(r.W), float32(r.H)
	sx, sy := float32(r.X), float32(r.Y)
	v := func(x, y float32) Vertex {
		dx, dy := m.Apply(x, y)
		return Vertex{DstX: dx, DstY: dy, SrcX: sx + x, SrcY: sy + y, ColorR: color[0], ColorG: color[1], ColorB: color[2], ColorA: color[3]}
	}
	dw := Draw{Target: dst, Program: blitProgram, Images: [4]Image{src}, Uniforms: blitOf(linear),
		Blend: blend}
	Triangles(&dw, []Vertex{v(0, 0), v(w, 0), v(0, h), v(w, h)}, []uint16{0, 1, 2, 1, 2, 3})
}

// Piece is a W by H rectangle of a source image, at SrcX, SrcY in it, drawn at DstX, DstY.
type Piece struct{ DstX, DstY, SrcX, SrcY, W, H float32 }

// never written: a draw copies its uniforms into the frame's buffer
var (
	blitZero   = make([]byte, blitLayout.Size)
	blitLinear = func() []byte {
		u := make([]byte, blitLayout.Size)
		blitLayout.Put(u, "filter", []float32{1})
		return u
	}()
)

// blitOf is the blit's uniforms, sampling between texels where linear
func blitOf(linear bool) []byte {
	if linear {
		return blitLinear
	}
	return blitZero
}

// DrawPieces draws pieces of src into dst, their colours times color (premultiplied).
func DrawPieces(dst, src Image, pieces []Piece, color [4]float32, blend Blend) {
	const most = 65536 / 4 // pieces a draw's 16-bit indices reach
	if len(pieces) == 0 {
		return
	}
	d := must()
	d.mu.Lock()
	defer d.mu.Unlock()
	r := src.rect()
	dw := Draw{
		Target: dst, Program: blitProgram, Images: [4]Image{src}, Uniforms: blitZero, Blend: blend,
	}
	for len(pieces) > 0 {
		n := min(len(pieces), most)
		verts, indices := d.pieceVerts[:0], d.pieceIndices[:0]
		for k, p := range pieces[:n] {
			v := func(dx, dy float32) Vertex {
				return Vertex{DstX: p.DstX + dx, DstY: p.DstY + dy, SrcX: float32(r.X) + p.SrcX + dx, SrcY: float32(r.Y) + p.SrcY + dy,
					ColorR: color[0], ColorG: color[1], ColorB: color[2], ColorA: color[3]}
			}
			verts = append(verts, v(0, 0), v(p.W, 0), v(0, p.H), v(p.W, p.H))
			b := uint16(4 * k)
			indices = append(indices, b, b+1, b+2, b+1, b+2, b+3)
		}
		d.pieceVerts, d.pieceIndices = verts, indices
		d.triangles(dst.Texture.gpuTarget(d), dst.rect(), &dw, verts, indices)
		pieces = pieces[n:]
	}
}

// DrawImageIn draws src into dst as DrawImage does, its top-left corner at the origin moved by
// m, sampled between texels where linear, only inside the convex polygon pts, in dst's pixels.
func DrawImageIn(dst, src Image, m Affine, linear bool, pts [][2]float32, blend Blend) {
	back, ok := m.inverse()
	if len(pts) < 3 || !ok {
		return
	}
	d := must()
	d.mu.Lock()
	defer d.mu.Unlock()
	r := src.rect()
	verts, indices := d.pieceVerts[:0], d.pieceIndices[:0]
	for _, p := range pts {
		x, y := back.Apply(p[0], p[1])
		verts = append(verts, Vertex{DstX: p[0], DstY: p[1], SrcX: float32(r.X) + x,
			SrcY: float32(r.Y) + y, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1})
	}
	for k := 1; k+1 < len(pts); k++ {
		indices = append(indices, 0, uint16(k), uint16(k+1))
	}
	d.pieceVerts, d.pieceIndices = verts, indices
	dw := Draw{Target: dst, Program: blitProgram, Images: [4]Image{src}, Uniforms: blitOf(linear),
		Blend: blend}
	d.triangles(dst.Texture.gpuTarget(d), dst.rect(), &dw, verts, indices)
}

// the affine taking a point back to where m took it from; false for one flattening the plane
func (m Affine) inverse() (Affine, bool) {
	det := m.A*m.D - m.B*m.C
	if det == 0 {
		return Affine{}, false
	}
	a, b, c, d := m.D/det, -m.B/det, -m.C/det, m.A/det
	return Affine{A: a, B: b, C: c, D: d, TX: -(a*m.TX + c*m.TY), TY: -(b*m.TX + d*m.TY)}, true
}

// Present draws screen over the window's surface view, w by h pixels of format, into enc, the
// window's frame encoder. Everything drawn before is submitted first.
func Present(
	enc *wgpu.CommandEncoder, view *wgpu.TextureView, format gputypes.TextureFormat, w, h int,
	screen *Texture,
) error {
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
	verts := d.quad[0][:0]
	for _, v := range [4][4]float32{{0, 0, 0, 0}, {fw, 0, sw, 0}, {0, fh, 0, sh}, {fw, fh, sw, sh}} {
		verts = appendVertex(
			verts,
			Vertex{
				DstX:   v[0],
				DstY:   v[1],
				SrcX:   v[2],
				SrcY:   v[3],
				ColorR: 1,
				ColorG: 1,
				ColorB: 1,
				ColorA: 1,
			},
		)
	}
	indices := append(d.quad[1][:0], 0, 0, 1, 0, 2, 0, 1, 0, 2, 0, 3, 0)
	draw := d.quad[2][:0]
	// the target's size, the image's rectangle, the place and the tint as they come
	for _, f := range [drawSize / 4]float32{
		fw, fh, 0, 0, 0, 0, sw, sh, 20: 1, 21: 1, 24: 1, 25: 1, 26: 1, 27: 1,
	} {
		draw = appendF32(draw, f)
	}
	uniforms := d.quad[3]
	if cap(uniforms) < uniformBlock {
		uniforms = make([]byte, uniformBlock)
	}
	uniforms = uniforms[:uniformBlock]
	clear(uniforms)
	if screen.w != w || screen.h != h {
		blitLayout.Put(uniforms, "filter", []float32{1})
	}
	d.quad = [4][]byte{verts, indices, draw, uniforms}
	for k, data := range d.quad {
		if _, err := d.keepPresented(k, data); err != nil {
			return err
		}
	}
	pipeline := blitProgram.pipeline(d, Copy, format, noDepth)
	images := d.images([4]*Texture{screen})
	frame := d.frame([FrameImages]*Texture{})
	d.passColor[0] = wgpu.RenderPassColorAttachment{
		View: view, LoadOp: gputypes.LoadOpClear, StoreOp: gputypes.StoreOpStore,
		ClearValue: gputypes.Color{A: 1},
	}
	d.passDesc = wgpu.RenderPassDescriptor{Label: "present", ColorAttachments: d.passColor[:]}
	pass, err := enc.BeginRenderPass(&d.passDesc)
	if err != nil {
		return fmt.Errorf("gpu: the frame's pass: %w", err)
	}
	pass.SetPipeline(pipeline)
	pass.SetBindGroup(0, d.presentGroup, zeroOffsets)
	pass.SetBindGroup(1, images, nil)
	pass.SetBindGroup(2, d.noDepth.readGroup(d), nil)
	pass.SetBindGroup(3, frame, nil)
	pass.SetVertexBuffer(0, d.present[0], 0)
	pass.SetIndexBuffer(d.present[1], gputypes.IndexFormatUint16, 0)
	pass.SetScissorRect(gputypes.ScissorRect{Width: uint32(w), Height: uint32(h)})
	pass.DrawIndexed(gputypes.DrawIndexedArgs{IndexCount: 6, InstanceCount: 1})
	return pass.End()
}

// never written: gogpu submits the frame's pass after Present returns, once the draws' offsets
// are let go, and its GLES backend reads the slice then
var zeroOffsets = []uint32{0, 0}

// keepPresented writes data into the quad's buffer k unless it holds it already, and keeps a
// copy of its own to tell by: Queue.WriteBuffer waits for the GPU
func (d *device) keepPresented(k int, data []byte) (written bool, err error) {
	if bytes.Equal(d.presented[k], data) {
		return false, nil
	}
	if err := d.queue.WriteBuffer(d.present[k], 0, data); err != nil {
		return false, fmt.Errorf("gpu: writing the frame's quad: %w", err)
	}
	d.presented[k] = append(d.presented[k][:0], data...)
	return true, nil
}

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

// Sprites draws the triangles of verts, their Src in src's texture pixels, sampled between texels
// where linear.
func Sprites(dst, src Image, verts []Vertex, indices []uint16, linear bool, blend Blend) {
	Triangles(&Draw{Target: dst, Program: blitProgram, Images: [4]Image{src},
		Uniforms: blitOf(linear), Blend: blend}, verts, indices)
}

// Colored draws the triangles of verts in their colours, premultiplied.
func Colored(dst Image, verts []Vertex, indices []uint16, blend Blend) {
	Triangles(&Draw{Target: dst, Program: fillProgram, Blend: blend}, verts, indices)
}
