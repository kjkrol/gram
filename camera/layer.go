package camera

import "math"

// Layer is a layer of a picture drawn in layers: Share of the frame wide and high (1 the whole)
// drawn at Resolution of its shown pixels, laid in the frame as the picture's Layout says.
type Layer struct{ Share, Resolution float32 }

// Layout is how a picture's layers lie in its frame: Inset, each the central part of the frame,
// inset on every side; Footed, each centred across the frame and standing on its bottom edge.
type Layout uint8

const (
	Inset Layout = iota
	Footed
)

// Layered is a camera whose picture whoever shows it draws in layers, the widest first, each a
// crop of the frame over the last, laid out as its Layout says; none or one is drawn as a
// Resolved camera's. The innermost layer's Resolution is the camera's own, so its screen
// conversions stay the centre's.
type Layered interface {
	Layers() []Layer
	Layout() Layout
	SetLayers(layers []Layer, layout Layout)
}

// Window is a part of a camera's frame in its shown pixels: its top-left corner and its size.
type Window struct{ X, Y, W, H float32 }

// Arrange is the window of each of layers in a frame fw by fh shown pixels laid out as layout
// says, in whole pixels, an inset part's margins even so it lies a whole pixel in.
func Arrange(layout Layout, layers []Layer, fw, fh int) []Window {
	wins := make([]Window, len(layers))
	for i, l := range layers {
		w, h := part(fw, l.Share, true), part(fh, l.Share, layout == Inset)
		y := float32(fh-h) / 2
		if layout == Footed {
			y = float32(fh - h)
		}
		wins[i] = Window{X: float32(fw-w) / 2, Y: y, W: float32(w), H: float32(h)}
	}
	return wins
}

// part is share of n in whole pixels, 2 up to n, the rest even where the part is centred
func part(n int, share float32, centred bool) int {
	c := min(max(int(math.Round(float64(n)*float64(share))), 2), n)
	if centred && (n-c)%2 != 0 {
		c++
	}
	return c
}

// Cropping is a camera drawn a part at a time: Crop has its Viewport be the central w by h
// shown pixels of its frame at s of them, its window on the world and its zoom as they are,
// until Uncrop puts the frame back exactly; Cropped is true for a part of the frame; Frame is
// the shown size of the whole frame, cropped or not.
type Cropping interface {
	Crop(w, h, s float32)
	Uncrop()
	Cropped() bool
	Frame() (w, h float32)
}

// InFrame is where the frame's shown pixel (sx, sy) lies in cam's pixels as it is drawn now:
// outside them when a cropped camera's part of the frame leaves it out.
func InFrame(cam Camera, sx, sy float32) (x, y float32) {
	k := ResolutionOf(cam)
	c, ok := cam.(Cropping)
	if !ok || !c.Cropped() {
		return sx * k, sy * k
	}
	fw, fh := c.Frame()
	w, h := cam.Viewport()
	return (sx - (fw-w/k)/2) * k, (sy - (fh-h/k)/2) * k
}

// LayersOf is the layers cam's picture is drawn in, a copy; nil for a camera without.
func LayersOf(cam Camera) []Layer {
	if r, ok := cam.(Layered); ok {
		return r.Layers()
	}
	return nil
}

// LayoutOf is how cam's layers lie in its frame; Inset for a camera without.
func LayoutOf(cam Camera) Layout {
	if l, ok := cam.(Layered); ok {
		return l.Layout()
	}
	return Inset
}

// IsCropped reports whether cam is drawn a part of its frame at the moment.
func IsCropped(cam Camera) bool {
	if c, ok := cam.(Cropping); ok {
		return c.Cropped()
	}
	return false
}
