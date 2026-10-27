package camera

import "github.com/kjkrol/uid"

// How is how a camera is fastened to an entity: Loose to none, Centred over it, Behind it on an
// arm turned the way it walks, Inside it in first person. Hows are bits, so a binding may hold in
// several.
type How uint8

const (
	Loose How = 1 << iota
	Centred
	Behind
	Inside
)

// Outside is where the bindings of a free hand hold: every How but Inside.
const Outside = Loose | Centred | Behind

// Free is where the hand scrolls the world: a camera fastened to nothing or Centred over an
// entity; one kept on an arm behind an entity, or inside it, is not scrolled.
const Free = Loose | Centred

// Fastening is what a camera is fastened to and how; the zero value, to nothing.
type Fastening struct {
	Entity uid.UID64
	How    How
}

// Fastenable is a camera that can be fastened to an entity.
type Fastenable interface {
	Fasten(Fastening)
	Fastening() Fastening
}

// HowOf is how cam is fastened: Loose for one fastened to nothing, or that cannot be.
func HowOf(cam Camera) How {
	if f, ok := cam.(Fastenable); ok {
		if h := f.Fastening().How; h != 0 {
			return h
		}
	}
	return Loose
}
