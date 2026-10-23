package look

// ComposedOnce reports whether r's last frame drew the board's tiles composed once (render.Still).
func ComposedOnce(r *Renderer) bool { return r.stillOn && r.still != nil && r.still.Len() > 0 }

// Tiled reports whether r's last frame laid the board's tiles on the GPU.
func Tiled(r *Renderer) bool { return r.tiled }

// OnCPU has r compose the tiles on the CPU whatever its map, as it did before they were laid on
// the GPU.
func OnCPU(r *Renderer) *Renderer {
	r.cpu = true
	return r
}

// GridLift is how far over the ground the lines of a grid other than square lie.
const GridLift = gridLift

// GroundLift is how far over the tiles composed into the frame a GroundDressing lays its own.
const GroundLift = groundLift
