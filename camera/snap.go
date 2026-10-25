package camera

// Snapped is a camera drawn from a copy of itself (Snap), taken as the frame is laid out on its
// barrier, so the frame draws it as it stood while the ticks move it on. Drawn is the copy, the
// camera itself before the first; Origin is the camera a copy was taken of, the camera itself
// for one that is no copy. A copy is for drawing alone: it picks no ground and takes no command.
type Snapped interface {
	Snap()
	Drawn() Camera
	Origin() Camera
}

// DrawnOf is cam's copy for the frame drawn, cam itself for a camera that is not Snapped.
func DrawnOf(cam Camera) Camera {
	if s, ok := cam.(Snapped); ok {
		return s.Drawn()
	}
	return cam
}

// OriginOf is the camera cam is a copy of, cam itself for one that is no copy.
func OriginOf(cam Camera) Camera {
	if s, ok := cam.(Snapped); ok {
		return s.Origin()
	}
	return cam
}
