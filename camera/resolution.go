package camera

// Resolved is a camera drawn at a resolution of the pixels it is shown at: SetResolution has its
// Viewport, Zoom and every screen conversion speak those camera pixels, the shown ones times s,
// its window on the world unchanged.
type Resolved interface {
	SetResolution(s float32)
	Resolution() float32
}

// ResolutionOf is cam's resolution, camera pixels a shown one: 1 for a camera without one.
func ResolutionOf(cam Camera) float32 {
	if r, ok := cam.(Resolved); ok {
		return r.Resolution()
	}
	return 1
}
