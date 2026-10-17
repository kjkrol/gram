package stick

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

// Over is cfg seeing cage: the ground the top of what is solid under a point, the extent the
// cage's Span, a pick stopping where the cage is not free, as finely as it says. A nil cage
// leaves level ground at sea level.
func Over(cfg Config, cage Cage) (Config, func(x, y float32) float32, func() (low, high float32)) {
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
