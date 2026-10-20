package camera

// Layer is a layer of a picture drawn in layers: the central Share of the frame (1 the whole)
// drawn at Resolution of its shown pixels.
type Layer struct{ Share, Resolution float32 }

// Layered is a camera whose picture whoever shows it draws in layers, the widest first, each a
// centred crop over the last; none or one is drawn as a Resolved camera's. The innermost
// layer's Resolution is the camera's own, so its screen conversions stay the centre's.
type Layered interface {
	Layers() []Layer
	SetLayers(layers []Layer)
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

// IsCropped reports whether cam is drawn a part of its frame at the moment.
func IsCropped(cam Camera) bool {
	if c, ok := cam.(Cropping); ok {
		return c.Cropped()
	}
	return false
}
