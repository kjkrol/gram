package render

import (
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	"github.com/kjkrol/gram/render/internal/gfx"
	"github.com/kjkrol/gram/render/internal/gpu"
)

// UseDevice has render draw on dev, the device of the engine's window, made by backend: Vulkan
// takes a program's SPIR-V kept on disk, the others want its WGSL.
func UseDevice(dev *wgpu.Device, backend gputypes.Backend) error { return gpu.Use(dev, backend) }

// Windowed says the device will be the engine's window's: none is made without it.
func Windowed() { gpu.Windowed() }

// Present draws screen over the window's surface view, w by h pixels of format, into enc, the
// window's frame encoder. Everything drawn before is submitted first.
func Present(enc *wgpu.CommandEncoder, view *wgpu.TextureView, format gputypes.TextureFormat,
	w, h int, screen *Image) error {
	return gpu.Present(enc, view, format, w, h, gfx.GPU(screen).Texture)
}

// Prepare compiles every shader, fills every image deferred and puts on the GPU every texture
// whose pixels wait on the CPU: what a stage's first frame would do on its own, done before it.
func Prepare() error { return gfx.Prepare() }

// Submit sends every draw gathered so far to the GPU.
func Submit() { gpu.Submit() }
