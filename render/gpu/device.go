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
	vertexSize = 12 * 4 // Vertex: dst, src, colour, custom
	drawSize   = 7 * 16 // the Draw uniform: the target's size, four images' rectangles, the vertices' place and tint
	// uniformBlock is the most a program's uniforms may take, the size every block is bound at.
	uniformBlock = 4096
)

// device is the GPU and what is being gathered for its next submission.
type device struct {
	mu       sync.Mutex
	dev      *wgpu.Device
	queue    *wgpu.Queue
	instance *wgpu.Instance // held by a headless device only
	align    uint64         // how uniform blocks are aligned in their buffer

	enc        *wgpu.CommandEncoder
	pass       *wgpu.RenderPassEncoder
	passTarget *target
	passDepth  *Depth

	// what the encoded draws read, written to the buffers as the submission goes
	verts, indices, draws, uniforms []byte
	vbuf, ibuf, dbuf, ubuf          *wgpu.Buffer
	group0                          *wgpu.BindGroup

	layout0, layout1 *wgpu.BindGroupLayout
	layout2          *wgpu.BindGroupLayout // a depth buffer to read, a group of its own (Depth.group)
	noDepth          *Depth                // read where a draw reads none
	layout           *wgpu.PipelineLayout
	nearest, linear  *wgpu.Sampler
	blank            *Texture
	groups           map[[4]*Texture]*wgpu.BindGroup

	// what Present draws the window's frame with, apart from what is gathered for submissions, and
	// what they hold, written again only as it changes
	present      [4]*wgpu.Buffer // vertices, indices, the draw, the uniforms
	presentGroup *wgpu.BindGroup
	presented    [4][]byte

	stages []*staging // the gathered bytes' way to the GPU, mapped
}

var cur *device

// Use has the package draw on dev, the device of the engine's window.
func Use(dev *wgpu.Device) error {
	if cur != nil && cur.dev == dev {
		return nil
	}
	d := &device{dev: dev, queue: dev.Queue(), groups: map[[4]*Texture]*wgpu.BindGroup{}}
	if err := d.setup(); err != nil {
		return err
	}
	cur = d
	return nil
}

// Headless gives the package a device of its own, with no window: the best GPU there is, or the
// software rasteriser when software — gogpu's draws this package's programs wrong still.
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

// windowed is set by the engine before its window opens: the device is to be the window's, so a
// draw before it is an error, not a reason to make one without a window.
var windowed bool

// Windowed is the engine's: the device will be its window's (Use); none is made without it.
func Windowed() { windowed = true }

// must is the device drawn on: without a window, one made at first need (Headless), as a test or
// a tool draws.
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
	d.align = uint64(max(d.dev.Limits().MinUniformBufferOffsetAlignment, 256)) // gogpu validates 256 whatever the device says
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
	if d.layout, err = d.dev.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{Label: "gpu", BindGroupLayouts: []*wgpu.BindGroupLayout{d.layout0, d.layout1, d.layout2}}); err != nil {
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

// trySubmit is submit reporting what failed rather than panicking, and what was gathered dropped.
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
	// gogpu binds the window's swapchain to the next submission: this one is drawn off the window
	// and is kept off it; the window's frame goes with gogpu's own at the frame's end (Present)
	d.queue.SetSwapchainSuppressed(true)
	defer d.queue.SetSwapchainSuppressed(false)
	all := []*wgpu.CommandBuffer{cmds}
	if up != nil {
		all = []*wgpu.CommandBuffer{up, cmds}
	}
	if _, err := d.queue.Submit(all...); err != nil {
		return fmt.Errorf("gpu: submitting: %w", err)
	}
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

// begin has the pass being encoded draw into t, with the depth buffer dp or none, beginning one
// where it does not — clearing the depth when asked, which always begins one.
func (d *device) begin(t *target, dp *Depth, clearDepth bool) *wgpu.RenderPassEncoder {
	if d.pass != nil && d.passTarget.view == t.view && d.passDepth == dp && !clearDepth {
		return d.pass
	}
	d.endPass()
	desc := &wgpu.RenderPassDescriptor{Label: "gpu", ColorAttachments: []wgpu.RenderPassColorAttachment{{
		View: t.view, LoadOp: t.load, StoreOp: gputypes.StoreOpStore, ClearValue: t.clear,
	}}}
	if dp != nil {
		load := gputypes.LoadOpLoad
		if clearDepth {
			load = gputypes.LoadOpClear
		}
		desc.DepthStencilAttachment = &wgpu.RenderPassDepthStencilAttachment{View: dp.gpuView(d), DepthLoadOp: load, DepthStoreOp: gputypes.StoreOpStore, DepthClearValue: 0}
	}
	pass, err := d.encoder().BeginRenderPass(desc)
	if err != nil {
		panic(fmt.Errorf("gpu: beginning a pass: %w", err))
	}
	t.load = gputypes.LoadOpLoad
	d.pass, d.passTarget, d.passDepth = pass, t, dp
	return pass
}

// ensure makes room in the buffers for one more draw of nv vertices and ni indices, submitting
// what is gathered and growing a buffer where it would not fit.
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
			(*b).Release()
		}
		nb, err := d.dev.CreateBuffer(&wgpu.BufferDescriptor{Label: label, Size: size, Usage: usage | wgpu.BufferUsageCopyDst})
		if err != nil {
			panic(fmt.Errorf("gpu: a %s buffer of %d bytes: %w", label, size, err))
		}
		*b = nb
		return true
	}
	grow(&d.vbuf, uint64(nv*vertexSize), wgpu.BufferUsageVertex, "vertices")
	grow(&d.ibuf, uint64((ni*2+3)&^3), wgpu.BufferUsageIndex, "indices")
	regrouped := grow(&d.dbuf, drawSize+d.align, wgpu.BufferUsageUniform, "draws")
	regrouped = grow(&d.ubuf, uniformBlock+d.align, wgpu.BufferUsageUniform, "uniforms") || regrouped
	if regrouped || d.group0 == nil {
		g, err := d.dev.CreateBindGroup(&wgpu.BindGroupDescriptor{Label: "gpu draw", Layout: d.layout0, Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: d.dbuf, Size: drawSize},
			{Binding: 1, Buffer: d.ubuf, Size: uniformBlock},
		}})
		if err != nil {
			panic(fmt.Errorf("gpu: the draw group: %w", err))
		}
		d.group0 = g
	}
}

// images is the bind group of the four images a draw reads, made once for each four.
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
			g.Release()
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

// forget drops every bind group holding t.
func (d *device) forget(t *Texture) {
	for k, g := range d.groups {
		if k[0] == t || k[1] == t || k[2] == t || k[3] == t {
			g.Release()
			delete(d.groups, k)
		}
	}
}

func alignUp(n, a uint64) uint64 { return (n + a - 1) / a * a }

func appendF32(b []byte, v float32) []byte {
	return binary.LittleEndian.AppendUint32(b, math.Float32bits(v))
}
