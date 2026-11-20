package cameras

import "github.com/kjkrol/gram/camera"

// Cage is where the cameras may be: Free says whether nothing solid fills the point (x, y, z) —
// the ground, what stands on it, a deck, a cave's rock — and Span the lowest and highest heights
// the solid reaches. A line of sight stops where a point is not Free.
type Cage interface {
	Free(x, y, z float32) bool
	Span() (low, high float32)
}

// Grained is a Cage with things thinner than half a cell: Grain is how far a line of sight may
// step at (x, y) and how thin the thinnest thing there is, zero where half a cell will do.
type Grained interface {
	Grain(x, y float32) (step, thin float32)
}

// Opens is a Cage letting a camera into some of its solids as it is fastened: FreeTo is Free as
// a camera fastened how sees it.
type Opens interface {
	FreeTo(how camera.How, x, y, z float32) bool
}

// Cages is the cage of cages: free where each is, spanning them all, as fine as the finest, a
// camera let in where each lets it in.
func Cages(cages ...Cage) Cage {
	var all cageSet
	for _, c := range cages {
		if c != nil {
			all = append(all, c)
		}
	}
	return all
}

type cageSet []Cage

var (
	_ Grained = cageSet(nil)
	_ Opens   = cageSet(nil)
)

func (s cageSet) Free(x, y, z float32) bool {
	for _, c := range s {
		if !c.Free(x, y, z) {
			return false
		}
	}
	return true
}

func (s cageSet) FreeTo(how camera.How, x, y, z float32) bool {
	for _, c := range s {
		if o, ok := c.(Opens); ok {
			if !o.FreeTo(how, x, y, z) {
				return false
			}
		} else if !c.Free(x, y, z) {
			return false
		}
	}
	return true
}

func (s cageSet) Span() (low, high float32) {
	for i, c := range s {
		l, h := c.Span()
		if i == 0 {
			low, high = l, h
		}
		low, high = min(low, l), max(high, h)
	}
	return low, high
}

func (s cageSet) Grain(x, y float32) (step, thin float32) {
	for _, c := range s {
		g, ok := c.(Grained)
		if !ok {
			continue
		}
		st, th := g.Grain(x, y)
		if st > 0 && (step == 0 || st < step) {
			step = st
		}
		if th > 0 && (thin == 0 || th < thin) {
			thin = th
		}
	}
	return step, thin
}
