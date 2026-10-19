package gpu

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	_ "github.com/gogpu/wgpu/hal/allbackends" // the GPU backends a headless device picks from
)

// Format is the colour format every texture holds: 8-bit RGBA, premultiplied, no sRGB.
const Format = gputypes.TextureFormatRGBA8Unorm

const (
	vertexSize = 13 * 4 // dst, src, colour, custom
	// the target's size, four images' rectangles, the vertices' place and tint, the sight
	drawSize = 16 * 16
	// the most a program's uniforms may take, the size every block is bound at
	uniformBlock = 4096
)

// FrameImages is how many images a frame hands every program (group 3, Draw.Frame).
const FrameImages = 8

type device struct {
	mu       sync.Mutex
	dev      *wgpu.Device
	queue    *wgpu.Queue
	instance *wgpu.Instance // a headless device's only
	align    uint64

	enc        *wgpu.CommandEncoder
	pass       *wgpu.RenderPassEncoder
	passTarget *target
	passDepth  *Depth
	passFrame  *wgpu.BindGroup // the frame's images bound in the pass, kept till they change
	// the pass's descriptor, kept: gogpu converts it before BeginRenderPass returns
	passDesc     wgpu.RenderPassDescriptor
	passColor    [1]wgpu.RenderPassColorAttachment
	passDepthAtt wgpu.RenderPassDepthStencilAttachment
	cmds         [2]*wgpu.CommandBuffer // the upload and the draws, handed to Submit

	verts, indices, draws, uniforms []byte
	vbuf, ibuf, dbuf, ubuf          *wgpu.Buffer
	group0                          *wgpu.BindGroup
	// the draws' dynamic offsets, two a draw, let go after Submit: gogpu's GLES backend keeps the
	// slice handed to SetBindGroup in a deferred command replayed at Submit
	offs []uint32
	// DrawPieces' triangles, gathered into the frame's buffers before it returns
	pieceVerts   []Vertex
	pieceIndices []uint16

	layout0, layout1         *wgpu.BindGroupLayout
	layout2, layout3         *wgpu.BindGroupLayout
	noDepth                  *Depth // read where a draw reads none
	layout                   *wgpu.PipelineLayout
	nearest, linear, wrapped *wgpu.Sampler
	blank                    *Texture
	groups                   map[[4]*Texture]*wgpu.BindGroup
	frames                   map[[FrameImages]*Texture]*wgpu.BindGroup

	present      [4]*wgpu.Buffer // vertices, indices, the draw, the uniforms
	presentGroup *wgpu.BindGroup
	presented    [4][]byte // what the buffers hold, copies of the device's own
	quad         [4][]byte // where Present builds them, never what presented holds

	stages []*staging

	// let go while a submission may still read it: gogpu frees a resource at once
	retired []func()
}

var cur *device

// Use has the package draw on dev, the device of the engine's window.
func Use(dev *wgpu.Device) error {
	if cur != nil && cur.dev == dev {
		return nil
	}
	d := &device{dev: dev, queue: dev.Queue(), groups: map[[4]*Texture]*wgpu.BindGroup{},
		frames: map[[FrameImages]*Texture]*wgpu.BindGroup{}}
	if err := d.setup(); err != nil {
		return err
	}
	cur = d
	return nil
}

// Headless gives the package a device of its own, with no window: the best GPU there is, or the
// software rasteriser when software, which draws this package's programs wrong still.
func Headless(software bool) error {
	if cur != nil {
		return nil
	}
	inst, err := wgpu.CreateInstance(nil)
	if err != nil {
		return fmt.Errorf("gpu: instance: %w", err)
	}
	adapter, err := inst.RequestAdapter(&wgpu.RequestAdapterOptions{ForceFallbackAdapter: software})
	if err != nil {
		inst.Release()
		return fmt.Errorf("gpu: adapter: %w", err)
	}
	if !software && adapter.Info().DeviceType == gputypes.DeviceTypeCPU {
		adapter.Release()
		inst.Release()
		return errors.New("gpu: no GPU, only the software rasteriser, which draws wrong yet")
	}
	dev, err := adapter.RequestDevice(nil)
	if err != nil {
		inst.Release()
		return fmt.Errorf("gpu: device: %w", err)
	}
	if err := Use(dev); err != nil {
		return err
	}
	cur.instance = inst
	return nil
}

// Ready reports whether there is a device to draw on.
func Ready() bool { return cur != nil }

// Device is the device drawn on, nil before there is one: for a program of the engine's own.
func Device() *wgpu.Device {
	if cur == nil {
		return nil
	}
	return cur.dev
}

var errNoDevice = errors.New("gpu: drawn before the window's GPU is up: draw from a frame, not from a stage's Init")

// a draw before the window's device is an error, not a reason to make one without a window
var windowed bool

// Windowed says the device will be the engine's window's: none is made without it.
func Windowed() { windowed = true }

func must() *device {
	if cur == nil && !windowed {
		if err := Headless(false); err != nil {
			panic(fmt.Errorf("gpu: no device to draw on: %w", err))
		}
	}
	if cur == nil {
		panic(errNoDevice)
	}
	return cur
}

func (d *device) setup() error {
	// gogpu validates 256 whatever the device says
	d.align = uint64(max(d.dev.Limits().MinUniformBufferOffsetAlignment, 256))
	vf := wgpu.ShaderStageVertex | wgpu.ShaderStageFragment
	var err error
	if d.layout0, err = d.dev.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{Label: "gpu draw", Entries: []gputypes.BindGroupLayoutEntry{
		{Binding: 0, Visibility: vf, Buffer: &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, HasDynamicOffset: true, MinBindingSize: drawSize}},
		{Binding: 1, Visibility: vf, Buffer: &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, HasDynamicOffset: true, MinBindingSize: uniformBlock}},
	}}); err != nil {
		return err
	}
	tex := func(b uint32) gputypes.BindGroupLayoutEntry {
		return gputypes.BindGroupLayoutEntry{Binding: b, Visibility: vf, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D}}
	}
	if d.layout1, err = d.dev.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{Label: "gpu images", Entries: []gputypes.BindGroupLayoutEntry{
		tex(0), tex(1), tex(2), tex(3),
		{Binding: 4, Visibility: vf, Sampler: &gputypes.SamplerBindingLayout{Type: gputypes.SamplerBindingTypeFiltering}},
		{Binding: 5, Visibility: vf, Sampler: &gputypes.SamplerBindingLayout{Type: gputypes.SamplerBindingTypeFiltering}},
	}}); err != nil {
		return err
	}
	if d.layout2, err = d.dev.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{Label: "gpu depth", Entries: []gputypes.BindGroupLayoutEntry{
		{Binding: 0, Visibility: vf, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeDepth, ViewDimension: gputypes.TextureViewDimension2D}},
	}}); err != nil {
		return err
	}
	frame := make([]gputypes.BindGroupLayoutEntry, 0, FrameImages+1)
	for i := range FrameImages {
		frame = append(frame, tex(uint32(i)))
	}
	frame = append(frame, gputypes.BindGroupLayoutEntry{Binding: FrameImages, Visibility: vf,
		Sampler: &gputypes.SamplerBindingLayout{Type: gputypes.SamplerBindingTypeFiltering}})
	d.layout3, err = d.dev.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label:   "gpu frame",
		Entries: frame,
	})
	if err != nil {
		return err
	}
	d.layout, err = d.dev.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "gpu",
		BindGroupLayouts: []*wgpu.BindGroupLayout{d.layout0, d.layout1, d.layout2, d.layout3},
	})
	if err != nil {
		return err
	}
	d.noDepth = &Depth{w: 1, h: 1, readable: true}
	clamp := gputypes.AddressModeClampToEdge
	if d.nearest, err = d.dev.CreateSampler(&wgpu.SamplerDescriptor{Label: "nearest", AddressModeU: clamp, AddressModeV: clamp, AddressModeW: clamp, MagFilter: gputypes.FilterModeNearest, MinFilter: gputypes.FilterModeNearest, LodMaxClamp: 32}); err != nil {
		return err
	}
	if d.linear, err = d.dev.CreateSampler(&wgpu.SamplerDescriptor{Label: "linear", AddressModeU: clamp, AddressModeV: clamp, AddressModeW: clamp, MagFilter: gputypes.FilterModeLinear, MinFilter: gputypes.FilterModeLinear, LodMaxClamp: 32}); err != nil {
		return err
	}
	repeat := gputypes.AddressModeRepeat
	if d.wrapped, err = d.dev.CreateSampler(&wgpu.SamplerDescriptor{Label: "wrapped",
		AddressModeU: repeat, AddressModeV: repeat, AddressModeW: repeat,
		MagFilter: gputypes.FilterModeLinear, MinFilter: gputypes.FilterModeLinear,
		LodMaxClamp: 32}); err != nil {
		return err
	}
	d.blank = NewTexture(1, 1)
	return nil
}

// Submit sends every draw gathered so far to the GPU.
func Submit() {
	if cur == nil {
		return
	}
	cur.mu.Lock()
	defer cur.mu.Unlock()
	cur.submit()
}

func (d *device) submit() {
	if err := d.trySubmit(); err != nil {
		panic(err)
	}
}

func (d *device) trySubmit() error {
	if d.enc == nil {
		return nil
	}
	d.endPass()
	up, st, err := d.upload()
	if err != nil { // no staging buffer to be had: written the slow way, the GPU waited for
		st, up = nil, nil
		if err := d.writeGathered(); err != nil {
			return err
		}
	}
	enc := d.enc
	d.enc = nil
	d.verts, d.indices, d.draws, d.uniforms = d.verts[:0], d.indices[:0], d.draws[:0], d.uniforms[:0]
	cmds, err := enc.Finish()
	if err != nil {
		return fmt.Errorf("gpu: finishing the commands: %w", err)
	}
	// gogpu binds the window's swapchain to the next submission: this one is kept off the window
	d.queue.SetSwapchainSuppressed(true)
	defer d.queue.SetSwapchainSuppressed(false)
	n := 0
	if up != nil {
		d.cmds[n] = up
		n++
	}
	d.cmds[n] = cmds
	n++
	_, err = d.queue.Submit(d.cmds[:n]...)
	d.cmds = [2]*wgpu.CommandBuffer{}
	d.offs = d.offs[:0]
	if err != nil {
		return fmt.Errorf("gpu: submitting: %w", err)
	}
	d.bury()
	if st != nil {
		return st.remap()
	}
	return nil
}

func (d *device) encoder() *wgpu.CommandEncoder {
	if d.enc == nil {
		enc, err := d.dev.CreateCommandEncoder(nil)
		if err != nil {
			panic(fmt.Errorf("gpu: a command encoder: %w", err))
		}
		d.enc = enc
	}
	return d.enc
}

func (d *device) endPass() {
	if d.pass != nil {
		if err := d.pass.End(); err != nil {
			panic(fmt.Errorf("gpu: ending a pass: %w", err))
		}
		d.pass, d.passTarget, d.passDepth = nil, nil, nil
	}
}

func (d *device) begin(t *target, dp *Depth, clearDepth bool) *wgpu.RenderPassEncoder {
	if d.pass != nil && d.passTarget.view == t.view && d.passDepth == dp && !clearDepth {
		return d.pass
	}
	d.endPass()
	pass, err := d.encoder().BeginRenderPass(d.passDescriptor(t, dp, clearDepth))
	if err != nil {
		panic(fmt.Errorf("gpu: beginning a pass: %w", err))
	}
	t.load = gputypes.LoadOpLoad
	d.pass, d.passTarget, d.passDepth, d.passFrame = pass, t, dp, nil
	return pass
}

// passDescriptor fills the kept descriptor for a pass into t over dp, none without one
func (d *device) passDescriptor(t *target, dp *Depth, clearDepth bool) *wgpu.RenderPassDescriptor {
	d.passColor[0] = wgpu.RenderPassColorAttachment{
		View: t.view, LoadOp: t.load, StoreOp: gputypes.StoreOpStore, ClearValue: t.clear,
	}
	d.passDesc = wgpu.RenderPassDescriptor{Label: "gpu", ColorAttachments: d.passColor[:]}
	if dp != nil {
		load := gputypes.LoadOpLoad
		if clearDepth {
			load = gputypes.LoadOpClear
		}
		d.passDepthAtt = wgpu.RenderPassDepthStencilAttachment{
			View: dp.gpuView(d), DepthLoadOp: load, DepthStoreOp: gputypes.StoreOpStore,
		}
		d.passDesc.DepthStencilAttachment = &d.passDepthAtt
	}
	return &d.passDesc
}

// offsets are a draw's dynamic offsets for SetBindGroup, kept till the next Submit
func (d *device) offsets(dOff, uOff uint64) []uint32 {
	n := len(d.offs)
	d.offs = append(d.offs, uint32(dOff), uint32(uOff))
	return d.offs[n : n+2 : n+2]
}

// bindFrame binds the frame's images g in the pass, unless bound already: every pipeline shares
// one layout, so a group bound stays bound across them. Call it after the pass's pipeline is set.
func (d *device) bindFrame(pass *wgpu.RenderPassEncoder, g *wgpu.BindGroup) {
	if g != d.passFrame {
		pass.SetBindGroup(3, g, nil)
		d.passFrame = g
	}
}

func (d *device) ensure(nv, ni int) {
	needV := uint64(len(d.verts) + nv*vertexSize)
	needI := uint64(len(d.indices) + (ni*2+3)&^3)
	needD := alignUp(uint64(len(d.draws)), d.align) + drawSize
	needU := alignUp(uint64(len(d.uniforms)), d.align) + uniformBlock
	fits := func(b *wgpu.Buffer, n uint64) bool { return b != nil && b.Size() >= n }
	if fits(d.vbuf, needV) && fits(d.ibuf, needI) && fits(d.dbuf, needD) && fits(d.ubuf, needU) {
		return
	}
	d.submit()
	grow := func(b **wgpu.Buffer, n uint64, usage gputypes.BufferUsage, label string) bool {
		if fits(*b, n) {
			return false
		}
		size := uint64(1 << 16)
		for size < 2*n {
			size *= 2
		}
		if *b != nil {
			d.retire((*b).Release) // the submissions before may still read it
		}
		nb, err := d.dev.CreateBuffer(&wgpu.BufferDescriptor{Label: label, Size: size, Usage: usage | wgpu.BufferUsageCopyDst})
		if err != nil {
			panic(fmt.Errorf("gpu: a %s buffer of %d bytes: %w", label, size, err))
		}
		*b = nb
		return true
	}
	// as big as what was gathered, so the next frame gathers as much without a submission between
	grow(&d.vbuf, needV, wgpu.BufferUsageVertex, "vertices")
	grow(&d.ibuf, needI, wgpu.BufferUsageIndex, "indices")
	regrouped := grow(&d.dbuf, needD, wgpu.BufferUsageUniform, "draws")
	regrouped = grow(&d.ubuf, needU, wgpu.BufferUsageUniform, "uniforms") || regrouped
	if regrouped || d.group0 == nil {
		g, err := d.dev.CreateBindGroup(&wgpu.BindGroupDescriptor{Label: "gpu draw", Layout: d.layout0, Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: d.dbuf, Size: drawSize},
			{Binding: 1, Buffer: d.ubuf, Size: uniformBlock},
		}})
		if err != nil {
			panic(fmt.Errorf("gpu: the draw group: %w", err))
		}
		if d.group0 != nil {
			d.retire(d.group0.Release) // the submissions before may still bind it
		}
		d.group0 = g
	}
}

func (d *device) images(imgs [4]*Texture) *wgpu.BindGroup {
	for i := range imgs {
		if imgs[i] == nil {
			imgs[i] = d.blank
		}
	}
	if g, ok := d.groups[imgs]; ok {
		return g
	}
	if len(d.groups) > 4096 {
		for k, g := range d.groups {
			d.retire(g.Release)
			delete(d.groups, k)
		}
	}
	entries := make([]wgpu.BindGroupEntry, 0, 6)
	for i, t := range imgs {
		entries = append(entries, wgpu.BindGroupEntry{Binding: uint32(i), TextureView: t.gpuView(d)})
	}
	entries = append(entries, wgpu.BindGroupEntry{Binding: 4, Sampler: d.nearest}, wgpu.BindGroupEntry{Binding: 5, Sampler: d.linear})
	g, err := d.dev.CreateBindGroup(&wgpu.BindGroupDescriptor{Label: "gpu images", Layout: d.layout1, Entries: entries})
	if err != nil {
		panic(fmt.Errorf("gpu: an images group: %w", err))
	}
	d.groups[imgs] = g
	return g
}

// the frame's images' group, one for each set of them
func (d *device) frame(imgs [FrameImages]*Texture) *wgpu.BindGroup {
	for i := range imgs {
		if imgs[i] == nil {
			imgs[i] = d.blank
		}
	}
	if g, ok := d.frames[imgs]; ok {
		return g
	}
	if len(d.frames) > 256 {
		for k, g := range d.frames {
			d.retire(g.Release)
			delete(d.frames, k)
		}
	}
	entries := make([]wgpu.BindGroupEntry, 0, FrameImages+1)
	for i, t := range imgs {
		entries = append(entries,
			wgpu.BindGroupEntry{Binding: uint32(i), TextureView: t.gpuView(d)})
	}
	entries = append(entries, wgpu.BindGroupEntry{Binding: FrameImages, Sampler: d.wrapped})
	g, err := d.dev.CreateBindGroup(&wgpu.BindGroupDescriptor{Label: "gpu frame", Layout: d.layout3,
		Entries: entries})
	if err != nil {
		panic(fmt.Errorf("gpu: a frame's group: %w", err))
	}
	d.frames[imgs] = g
	return g
}

func (d *device) forget(t *Texture) {
	for k, g := range d.groups {
		if k[0] == t || k[1] == t || k[2] == t || k[3] == t {
			d.retire(g.Release)
			delete(d.groups, k)
		}
	}
	for k, g := range d.frames {
		for _, f := range k {
			if f == t {
				d.retire(g.Release)
				delete(d.frames, k)
				break
			}
		}
	}
}

func (d *device) retire(free func()) { d.retired = append(d.retired, free) }

func (d *device) bury() {
	if len(d.retired) == 0 {
		return
	}
	d.dev.Poll(wgpu.PollWait)
	for _, free := range d.retired {
		free()
	}
	d.retired = d.retired[:0]
}

func retire(free func()) {
	if cur == nil {
		free()
		return
	}
	cur.mu.Lock()
	defer cur.mu.Unlock()
	cur.retire(free)
}

func alignUp(n, a uint64) uint64 { return (n + a - 1) / a * a }

func appendF32(b []byte, v float32) []byte {
	return binary.LittleEndian.AppendUint32(b, math.Float32bits(v))
}
