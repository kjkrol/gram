package look

// ComposedOnce reports whether r's last frame drew the board's tiles composed once (render.Still).
func ComposedOnce(r *Renderer) bool { return r.stillOn && r.still != nil && r.still.Len() > 0 }

// GridLift is how far over the ground the lines of a grid other than square lie.
const GridLift = gridLift
