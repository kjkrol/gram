package camera

import "github.com/kjkrol/uid"

// How is how a camera is fastened to an entity: Loose to none, moved by hand; Centred kept over
// it, at its altitude; Behind it, turned the way it walks; Inside it, the eye its own — first
// person. Hows are bits, so a binding may hold in several (control.Binding.In).
type How uint8

const (
	Loose How = 1 << iota
	Centred
	Behind
	Inside
)

// Outside is every How but Inside: where the bindings of a free hand hold.
const Outside = Loose | Centred | Behind

// Fastening is what a camera is fastened to and how; the zero value, to nothing.
type Fastening struct {
	Entity uid.UID64
	How    How
}

// Fastenable is a camera that can be fastened to an entity: whoever keeps it there reads its
// Fastening every tick.
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
