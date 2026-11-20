package stick

import contract "github.com/kjkrol/gram/camera"

// Cage is where the cameras may be, as the cameras plugin's Cage says: Free whether nothing
// solid fills a point, Span the lowest and highest heights the solid reaches.
type Cage interface {
	Free(x, y, z float32) bool
	Span() (low, high float32)
}

// Grained is a Cage with things thinner than half a cell, as the cameras plugin's Grained says.
type Grained interface {
	Grain(x, y float32) (step, thin float32)
}

// Opens is a Cage letting a camera into some of its solids, as the cameras plugin's Opens says.
type Opens interface {
	FreeTo(how contract.How, x, y, z float32) bool
}

// Walls is what holds a camera's eye and stops its cursor: Free whether nothing solid fills a
// point as a camera fastened how sees it, Fine how finely to step near what is thin (nil: half a
// cell everywhere), Span the lowest and highest heights the solid reaches.
type Walls struct {
	Free func(how contract.How, x, y, z float32) bool
	Fine func(x, y float32) (step, thin float32)
	Span func() (low, high float32)
}

// wallsOf is cage as Walls
func wallsOf(cage Cage) *Walls {
	w := &Walls{Span: cage.Span}
	if g, ok := cage.(Grained); ok {
		w.Fine = g.Grain
	}
	if o, ok := cage.(Opens); ok {
		w.Free = o.FreeTo
	} else {
		w.Free = func(_ contract.How, x, y, z float32) bool { return cage.Free(x, y, z) }
	}
	return w
}

// solid is w's Free as a pick takes it, for a camera fastened how
func (w *Walls) solid(how contract.How) func(x, y, z float32) (float32, bool) {
	return func(x, y, z float32) (float32, bool) { return z, !w.Free(how, x, y, z) }
}

// Over is cfg seeing cage, its eye held and its cursor stopped by walls too: the ground the top
// of what cage makes solid under a point, the extent the cage's Span, the stick's foot standing on
// it, a pick and the eye stopping where walls are not free (cage alone where walls is nil), as
// finely as each says. A nil cage leaves level ground at sea level.
func Over(cfg Config, cage, walls Cage) (Config, func(x, y float32) float32,
	func() (low, high float32)) {
	if walls == nil {
		walls = cage
	}
	if walls != nil {
		cfg.Walls = wallsOf(walls)
	}
	if cage == nil {
		return cfg, nil, nil
	}
	if g, ok := cage.(Grained); ok {
		cfg.Fine = g.Grain
	}
	cfg.Solid = func(x, y, z float32) (float32, bool) { return z, !cage.Free(x, y, z) }
	headroom := cfg.Headroom
	if headroom == 0 {
		headroom = defaultHeadroom
	}
	ground := func(x, y float32) float32 { return topOf(cage, cfg.Fine, x, y, cfg.Cell, headroom) }
	return cfg, ground, cage.Span
}

const defaultHeadroom = 64

// the top of what is solid under (x, y), from over the highest down: never through a deck,
// stepping no further than half its thickness where the cage is grained
func topOf(
	cage Cage,
	fine func(x, y float32) (float32, float32),
	x, y, cell, headroom float32,
) float32 {
	low, high := cage.Span()
	z := high + headroom
	step := max(cell/2, (z-low)/64)
	if fine != nil {
		if _, thin := fine(x, y); thin > 0 {
			step = min(step, thin/2)
		}
	}
	for ; z > low; z -= step {
		if !cage.Free(x, y, z-step) {
			return bisect(cage.Free, x, y, z-step, z)
		}
	}
	return low
}

// the first free height over z, stepping up by half a cell
func rise(free func(x, y, z float32) bool, x, y, z, cell float32) float32 {
	step := cell / 2
	for range 1 << 14 {
		if free(x, y, z+step) {
			return bisect(free, x, y, z, z+step)
		}
		z += step
	}
	return z
}

// the lowest free height between solid at below and free at above, within pickPrecision
func bisect(free func(x, y, z float32) bool, x, y, below, above float32) float32 {
	for k := 0; k < 48 && above-below > pickPrecision; k++ {
		if mid := (below + above) / 2; free(x, y, mid) {
			above = mid
		} else {
			below = mid
		}
	}
	return above
}
