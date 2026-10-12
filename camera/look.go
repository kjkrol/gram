package camera

// MouseLooker is a camera whose eye, riding Inside an entity, may look round with the mouse.
type MouseLooker interface {
	MouseLook() bool
	SetMouseLook(on bool)
}

// MouseLooks reports whether cam looks round with the mouse while it rides inside an entity.
func MouseLooks(cam Camera) bool {
	m, ok := cam.(MouseLooker)
	return ok && m.MouseLook()
}
