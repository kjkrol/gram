package camera

// Mode is what a player is doing with their camera, for the bindings that hold in some modes only
// (control.Binding.In): Free, moving it over the world, or FirstPerson, riding in an entity the
// player steers, the camera the entity's eyes. Modes are bits, so a binding may hold in several.
type Mode uint8

const (
	Free Mode = 1 << iota
	FirstPerson
)

// Rider is a camera that can ride in an entity; FirstPerson reports whether it rides in one now.
type Rider interface{ FirstPerson() bool }

// ModeOf is the mode of cam: FirstPerson for a Rider riding, Free otherwise.
func ModeOf(cam Camera) Mode {
	if r, ok := cam.(Rider); ok && r.FirstPerson() {
		return FirstPerson
	}
	return Free
}
