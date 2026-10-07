package camera

// MouseLooker is a camera whose eye, riding in an entity (Inside), may look round with the mouse:
// off unless its Config says MouseLook or a command switches it on.
type MouseLooker interface {
	MouseLook() bool
	SetMouseLook(on bool)
}

// MouseLooks reports whether cam looks round with the mouse while it rides inside an entity.
func MouseLooks(cam Camera) bool {
	m, ok := cam.(MouseLooker)
	return ok && m.MouseLook()
}
