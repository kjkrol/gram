package camera

// Scaler is a camera whose scale changes over the screen — a perspective's. ScaleAt is how many
// screen units a world unit spans at the world point (x, y, z), across the way the eye looks, and
// 0 for a point not in front of the eye.
type Scaler interface {
	ScaleAt(x, y, z float32) float32
}

// ScaleAt is how many screen units a world unit spans at the point through cam: a Scaler's own,
// else its Zoom, the same everywhere; 0 for a point not in front of the eye.
func ScaleAt(cam Camera, x, y, z float32) float32 {
	if s, ok := cam.(Scaler); ok {
		return s.ScaleAt(x, y, z)
	}
	return cam.Zoom()
}
