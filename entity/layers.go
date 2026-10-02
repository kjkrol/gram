package entity

// Layers is the planes an entity is on, one bit each: two entities meet only where they share a
// bit, and one without Layers, or with none set, is on every plane. Collision and sight read it.
type Layers uint8

// Meets reports whether the two are on a common plane, none set standing for all of them.
func (l Layers) Meets(o Layers) bool { return l == 0 || o == 0 || l&o != 0 }

// LayersOf is the Layers an entity carries, every plane for one carrying none.
func LayersOf(p *Layers) Layers {
	if p == nil {
		return 0
	}
	return *p
}
