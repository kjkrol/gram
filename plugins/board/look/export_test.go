package look

// ComposedOnce reports whether r's last frame drew the board's tiles composed once (render.Still).
func ComposedOnce(r *Renderer) bool { return r.stillOn && r.still != nil && r.still.Len() > 0 }

// GridTier is the tier the lines of a grid other than square lie on.
const GridTier = gridTier
