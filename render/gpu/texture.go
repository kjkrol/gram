package gpu

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// Texture is an image's pixels: on the GPU once there is a device, until then kept here.
type Texture struct {
	w, h     int
	tex      *wgpu.Texture
	tgt      target
	cpu      []byte // written before there was a device, RGBA
	released bool
}

type target struct {
	view   *wgpu.TextureView
	w, h   int
	format gputypes.TextureFormat
	load   gputypes.LoadOp
	clear  gputypes.Color
}

// NewTexture is a w by h texture, transparent.
func NewTexture(w, h int) *Texture {
	if w <= 0 || h <= 0 {
		panic(fmt.Sprintf("gpu: a texture of %dx%d", w, h))
	}
	return &Texture{w: w, h: h}
}

// Size is the texture's width and height in pixels.
func (t *Texture) Size() (w, h int) { return t.w, t.h }

func (t *Texture) realize(d *device) {
	if t.released {
		panic("gpu: a texture used after Release")
	}
	if t.tex != nil {
		return
	}
	if t.cpu == nil { // transparent from the start, as WebGPU has it and gogpu does not
		t.cpu = make([]byte, 4*t.w*t.h)
	}
	tex, err := d.dev.CreateTexture(&wgpu.TextureDescriptor{
		Size: wgpu.Extent3D{Width: uint32(t.w), Height: uint32(t.h), DepthOrArrayLayers: 1}, MipLevelCount: 1, SampleCount: 1,
		Dimension: gputypes.TextureDimension2D, Format: Format,
		Usage: gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst | gputypes.TextureUsageCopySrc | gputypes.TextureUsageRenderAttachment,
	})
	if err != nil {
		panic(fmt.Errorf("gpu: a %dx%d texture: %w", t.w, t.h, err))
	}
	view, err := d.dev.CreateTextureView(tex, &wgpu.TextureViewDescriptor{Format: Format, Dimension: gputypes.TextureViewDimension2D, MipLevelCount: 1, ArrayLayerCount: 1})
	if err != nil {
		panic(fmt.Errorf("gpu: a texture view: %w", err))
	}
	t.tex, t.tgt = tex, target{view: view, w: t.w, h: t.h, format: Format, load: gputypes.LoadOpLoad}
	if t.cpu != nil {
		d.write(t, 0, 0, t.w, t.h, t.cpu)
		t.cpu = nil
	}
}

func (t *Texture) gpuView(d *device) *wgpu.TextureView {
	t.realize(d)
	return t.tgt.view
}

func (t *Texture) gpuTarget(d *device) *target {
	t.realize(d)
	t.tgt.load = gputypes.LoadOpLoad
	return &t.tgt
}

// WritePixels puts pix, RGBA w by h, at (x, y) of the texture: after every draw made so far.
func (t *Texture) WritePixels(x, y, w, h int, pix []byte) {
	if len(pix) < 4*w*h {
		panic(fmt.Sprintf("gpu: %d bytes written as %dx%d pixels", len(pix), w, h))
	}
	if x < 0 || y < 0 || x+w > t.w || y+h > t.h {
		panic(fmt.Sprintf("gpu: %dx%d pixels at (%d, %d) outside a %dx%d texture", w, h, x, y, t.w, t.h))
	}
	if cur == nil && (windowed || !ready()) {
		if t.cpu == nil {
			t.cpu = make([]byte, 4*t.w*t.h)
		}
		for row := range h {
			copy(t.cpu[4*((y+row)*t.w+x):], pix[4*row*w:4*(row+1)*w])
		}
		return
	}
	d := cur
	d.mu.Lock()
	defer d.mu.Unlock()
	t.realize(d)
	switch {
	case w*h <= maxPut:
		d.put(t, x, y, w, h, pix)
	case x == 0 && y == 0 && w == t.w && h == t.h:
		d.submit()
		d.write(t, 0, 0, w, h, pix)
	default:
		d.blitIn(t, x, y, w, h, pix)
	}
}

// the most pixels a write puts in place by drawing them, a draw's uniforms at a time
const maxPut = 1 << 16

// a draw's uniforms, uniformBlock bytes, hold the piece's width and putPixels pixels
const putPixels = (uniformBlock - 16) / 4

var putProgram = NewProgram("put", "    At: vec4<f32>,\n    Pixels: array<vec4<u32>, "+
	strconv.Itoa(putPixels/4)+">,\n", shader("put"))

// put draws the pixels in place among the draws, after every draw made so far, the pixels in the
// draws' uniforms: gogpu's Queue.WriteTexture discards what the texture held and waits for a
// submission of its own, and its copy into a texture drawn in the same commands is refused
func (d *device) put(t *Texture, x, y, w, h int, pix []byte) {
	pw := min(w, putPixels)
	ph := max(putPixels/pw, 1)
	u := make([]byte, uniformBlock)
	for py := 0; py < h; py += ph {
		for px := 0; px < w; px += pw {
			cw, ch := min(pw, w-px), min(ph, h-py)
			binary.LittleEndian.PutUint32(u, math.Float32bits(float32(cw)))
			for r := range ch {
				at := 4 * ((py+r)*w + px)
				copy(u[16+4*r*cw:], pix[at:at+4*cw])
			}
			d.triangles(t.gpuTarget(d), Rect{x + px, y + py, cw, ch},
				&Draw{Program: putProgram, Uniforms: u[:int(alignUp(uint64(16+4*cw*ch), 16))],
					Blend: Copy},
				quad(x+px, y+py, cw, ch), []uint16{0, 1, 2, 1, 2, 3})
		}
	}
}

// gogpu's write at an origin but the corner spoils the texture left of it: the pixels go into
// a texture of their own, whole, and are drawn into place from there
func (d *device) blitIn(t *Texture, x, y, w, h int, pix []byte) {
	part := NewTexture(w, h)
	part.cpu = pix[:4*w*h]
	part.realize(d)
	d.triangles(t.gpuTarget(d), Rect{x, y, w, h}, &Draw{Program: blitProgram,
		Images: [4]Image{{Texture: part}}, Uniforms: make([]byte, blitLayout.Size), Blend: Copy},
		quad(x, y, w, h), []uint16{0, 1, 2, 1, 2, 3})
	d.submit()
	d.forget(part)
	// the draw into place may still read it
	d.retire(func() { part.tgt.view.Release(); part.tex.Release() })
}

// a quad over w by h pixels at (x, y), its source the piece's own pixels
func quad(x, y, w, h int) []Vertex {
	one := func(dx, dy float32) Vertex {
		return Vertex{DstX: float32(x) + dx, DstY: float32(y) + dy, SrcX: dx, SrcY: dy,
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}
	}
	fw, fh := float32(w), float32(h)
	return []Vertex{one(0, 0), one(fw, 0), one(0, fh), one(fw, fh)}
}

func (d *device) write(t *Texture, x, y, w, h int, pix []byte) {
	row := int(alignUp(uint64(4*w), 256))
	padded := make([]byte, row*h)
	for r := range h {
		copy(padded[r*row:], pix[4*r*w:4*(r+1)*w])
	}
	err := d.queue.WriteTexture(&wgpu.ImageCopyTexture{Texture: t.tex, Origin: wgpu.Origin3D{X: uint32(x), Y: uint32(y)}},
		padded, &wgpu.ImageDataLayout{BytesPerRow: uint32(row), RowsPerImage: uint32(h)},
		&wgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1})
	if err != nil {
		panic(fmt.Errorf("gpu: writing %dx%d pixels: %w", w, h, err))
	}
	d.flushWrites()
}

// a submission of nothing has gogpu carry out the writes it holds: two held at once spoil each
// other in its staging
func (d *device) flushWrites() {
	enc, err := d.dev.CreateCommandEncoder(nil)
	if err != nil {
		panic(fmt.Errorf("gpu: a command encoder: %w", err))
	}
	cmds, err := enc.Finish()
	if err != nil {
		panic(fmt.Errorf("gpu: finishing the commands: %w", err))
	}
	if _, err := d.queue.Submit(cmds); err != nil {
		panic(fmt.Errorf("gpu: submitting the writes: %w", err))
	}
}

// ReadPixels copies the texture, RGBA row by row, into dst: after every draw made so far.
func (t *Texture) ReadPixels(dst []byte) {
	if len(dst) < 4*t.w*t.h {
		panic(fmt.Sprintf("gpu: %d bytes for %dx%d pixels", len(dst), t.w, t.h))
	}
	d := must()
	d.mu.Lock()
	defer d.mu.Unlock()
	t.realize(d)
	d.submit()
	row := alignUp(uint64(4*t.w), 256)
	size := row * uint64(t.h)
	buf, err := d.dev.CreateBuffer(&wgpu.BufferDescriptor{Label: "read", Size: size, Usage: wgpu.BufferUsageCopyDst | wgpu.BufferUsageMapRead})
	if err != nil {
		panic(fmt.Errorf("gpu: a read buffer: %w", err))
	}
	defer buf.Release()
	enc := d.encoder()
	enc.CopyTextureToBuffer(t.tex, buf, []wgpu.BufferTextureCopy{{
		BufferLayout: wgpu.ImageDataLayout{BytesPerRow: uint32(row), RowsPerImage: uint32(t.h)},
		TextureBase:  wgpu.ImageCopyTexture{Texture: t.tex},
		Size:         wgpu.Extent3D{Width: uint32(t.w), Height: uint32(t.h), DepthOrArrayLayers: 1},
	}})
	d.submit()
	if err := buf.Map(context.Background(), wgpu.MapModeRead, 0, size); err != nil {
		panic(fmt.Errorf("gpu: mapping the read buffer: %w", err))
	}
	mapped, err := buf.MappedRange(0, size)
	if err != nil {
		panic(fmt.Errorf("gpu: the read buffer's range: %w", err))
	}
	src := mapped.Bytes()
	for y := range t.h {
		copy(dst[4*y*t.w:4*(y+1)*t.w], src[uint64(y)*row:])
	}
	mapped.Release()
	if err := buf.Unmap(); err != nil {
		panic(fmt.Errorf("gpu: unmapping the read buffer: %w", err))
	}
}

func ready() bool { return cur != nil }

// Release frees the texture; it may not be used after.
func (t *Texture) Release() {
	if t.released {
		return
	}
	t.released = true
	t.cpu = nil
	if t.tex == nil || cur == nil {
		return
	}
	d := cur
	d.mu.Lock()
	defer d.mu.Unlock()
	d.forget(t)
	view, tex := t.tgt.view, t.tex
	d.retire(func() { view.Release(); tex.Release() })
	t.tex = nil
}
