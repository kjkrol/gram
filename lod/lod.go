package lod

import "fmt"

// Level is how fully a thing is ticked and drawn, L0 the fullest, L4 neither.
type Level uint8

const (
	L0 Level = iota
	L1
	L2
	L3
	L4
	// Levels is how many levels there are.
	Levels = 5
)

// Ladder is the world's levels: Scales[k] the fewest shown pixels a world unit must span for
// level k (L0 to L3), strictly decreasing where set, a zero met by any scale; Sight how far from
// an observer the world is ticked and drawn at all (zero, without end), its last Fog fading.
// The zero Ladder is L0 everywhere without end.
type Ladder struct {
	Scales [Levels - 1]float32
	Sight  float32
	Fog    float32
}

// At is the level of what a world unit spans scale shown pixels of, dist from the observer: the
// first whose threshold the scale meets, L4 under the last or past the Sight.
func (l Ladder) At(scale, dist float32) Level {
	if l.Sight > 0 && dist > l.Sight {
		return L4
	}
	for k, s := range l.Scales {
		if scale >= s {
			return Level(k)
		}
	}
	return L4
}

// Hysteresis is the share of a threshold a scale must fall under before its level goes coarser,
// so a sector at the edge of two levels does not flicker between them.
const Hysteresis = 0.9

// Hold is the level from goes to at scale, dist from the observer: a finer one at once, a coarser
// one only as far as it holds at the scale over Hysteresis and margin nearer.
func (l Ladder) Hold(from Level, scale, dist, margin float32) Level {
	n := l.At(scale, dist)
	if n <= from {
		return n
	}
	return max(from, l.At(scale/Hysteresis, dist-margin))
}

// Widened is l seeing factor times as far, its Fog kept; a Sight without end stays so.
func (l Ladder) Widened(factor float32) Ladder {
	l.Sight *= factor
	return l
}

// Check panics on a ladder that cannot be: a negative number, a threshold not under the one
// before where set, a Fog past the Sight.
func (l Ladder) Check() {
	for k, s := range l.Scales {
		switch {
		case s < 0:
			panic(fmt.Sprintf("lod: level %d's threshold %v is negative", k, s))
		case k > 0 && s > 0 && s >= l.Scales[k-1]:
			panic(fmt.Sprintf("lod: level %d's threshold %v is not under level %d's %v", k, s,
				k-1, l.Scales[k-1]))
		}
	}
	switch {
	case l.Sight < 0 || l.Fog < 0:
		panic(fmt.Sprintf("lod: a sight of %v with a fog of %v, want neither negative", l.Sight,
			l.Fog))
	case l.Fog > l.Sight:
		panic(fmt.Sprintf("lod: a fog of %v past a sight of %v", l.Fog, l.Sight))
	}
}

// Paces are the steps of the world a system takes one step over at each level, 0 none.
type Paces [Levels]uint8

var (
	// Exact goes every step at L0 alone.
	Exact = Paces{1, 0, 0, 0, 0}
	// Steady goes every step at every level but L4.
	Steady = Paces{1, 1, 1, 1, 0}
	// Contact goes every step through L2, where contacts are worked out, none beyond.
	Contact = Paces{1, 1, 1, 0, 0}
)

// Slowing goes every step at L0 and a step every n at L1, L2 and L3 in turn, the last n
// repeated where fewer are given; it panics on no n, more than three, a 0 or a falling one.
func Slowing(n ...uint8) Paces {
	if len(n) == 0 || len(n) > int(L3) {
		panic(fmt.Sprintf("lod: slowing by %v, want one to three steps", n))
	}
	p := Paces{L0: 1}
	for k := L1; k < L4; k++ {
		p[k] = n[min(int(k)-1, len(n)-1)]
		if p[k] == 0 {
			panic(fmt.Sprintf("lod: slowing by %v, want no 0: Exact is the pace of none", n))
		}
	}
	p.Check()
	return p
}

// Check panics on paces that cannot be: L0 not every step, L4 any, a level slower than the one
// after it or going after one that does not.
func (p Paces) Check() {
	switch {
	case p[L0] != 1:
		panic(fmt.Sprintf("lod: paces %v, want L0 every step", p))
	case p[L4] != 0:
		panic(fmt.Sprintf("lod: paces %v, want none at L4", p))
	}
	for k := L1; k < L4; k++ {
		switch {
		case p[k] == 0:
		case p[k-1] == 0:
			panic(fmt.Sprintf("lod: paces %v, want none at level %d after none before", p, k))
		case p[k] < p[k-1]:
			panic(fmt.Sprintf("lod: paces %v, want level %d no faster than level %d", p, k,
				k-1))
		}
	}
}
