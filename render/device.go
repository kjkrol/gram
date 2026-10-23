package render

import (
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	"github.com/kjkrol/gram/render/internal/gfx"
	"github.com/kjkrol/gram/render/internal/gpu"
)

// UseDevice has render draw on dev, the device of the engine's window.
func UseDevice(dev *wgpu.Device) error { return gpu.Use(dev) }

// Windowed says the device will be the engine's window's: none is made without it.
func Windowed() { gpu.Windowed() }

// Present draws screen over the window's surface view, w by h pixels of format, into enc, the
// window's frame encoder. Everything drawn before is submitted first.
func Present(enc *wgpu.CommandEncoder, view *wgpu.TextureView, format gputypes.TextureFormat,
	w, h int, screen *Image) error {
	return gpu.Present(enc, view, format, w, h, gfx.GPU(screen).Texture)
}

// Submit sends every draw gathered so far to the GPU.
func Submit() { gpu.Submit() }
