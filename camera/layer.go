package camera

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
)

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

// Cropping is a camera drawn a part at a time: Crop has its Viewport be the part win of its frame
// at s of its shown pixels, the camera and its zoom staying where they are, until Uncrop puts the
// frame back exactly. Cropped is true for a part of the frame, Frame is the whole frame's shown
// size and Window the part drawn, the whole frame uncropped.
type Cropping interface {
	Crop(win Window, s float32)
	Uncrop()
	Cropped() bool
	Frame() (w, h float32)
	Window() Window
}

// InFrame is where the frame's shown pixel (sx, sy) lies in cam's pixels as it is drawn now:
// outside them when a cropped camera's part of the frame leaves it out.
func InFrame(cam Camera, sx, sy float32) (x, y float32) {
	if c, ok := cam.(Cropping); ok {
		win := c.Window()
		sx, sy = sx-win.X, sy-win.Y
	}
	k := ResolutionOf(cam)
	return sx * k, sy * k
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

// Inner is the window of the layer drawn after the one cam is cropped to, in cam's own pixels:
// what that layer covers whole. False for the last layer and for a picture of one.
func Inner(cam Camera) (Window, bool) {
	crop, cropped := cam.(Cropping)
	layers := LayersOf(cam)
	if !cropped || !crop.Cropped() || len(layers) < 2 {
		return Window{}, false
	}
	fw, fh := crop.Frame()
	wins := Arrange(LayoutOf(cam), layers, int(fw), int(fh))
	at := crop.Window()
	for i, win := range wins[:len(wins)-1] {
		if win == at {
			k := ResolutionOf(cam)
			next := wins[i+1]
			return Window{X: (next.X - at.X) * k, Y: (next.Y - at.Y) * k, W: next.W * k,
				H: next.H * k}, true
		}
	}
	return Window{}, false
}

// Shows reports whether anything in the box, grown by margin and standing from low to top, may
// show through tr on a viewport w by h: not where it lies off one side of it, nor wholly
// within hole, the window a later layer covers (holed).
func (tr Transform) Shows(box geom.AABB, margin, low, top, w, h float32, hole Window,
	holed bool) bool {
	var left, right, over, under, inside int
	for _, x := range [2]float32{box.TopLeft.X - margin, box.BottomRight.X + margin} {
		for _, y := range [2]float32{box.TopLeft.Y - margin, box.BottomRight.Y + margin} {
			for _, z := range [2]float32{low, top} {
				sx, sy, _, ok := tr.Apply(x, y, z, w, h)
				if !ok {
					return true
				}
				if sx < 0 {
					left++
				}
				if sx > w {
					right++
				}
				if sy < 0 {
					over++
				}
				if sy > h {
					under++
				}
				if holed && sx > hole.X+2 && sx < hole.X+hole.W-2 && sy > hole.Y+2 &&
					sy < hole.Y+hole.H-2 {
					inside++
				}
			}
		}
	}
	return left < 8 && right < 8 && over < 8 && under < 8 && inside < 8
}
