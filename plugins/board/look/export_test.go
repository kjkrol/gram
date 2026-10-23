package look

// Tiled reports whether r's last frame laid the board's tiles on the GPU.
func Tiled(r *Renderer) bool { return r.tiled }

// GridLift is how far over the ground the lines of a grid other than square lie.
const GridLift = gridLift
