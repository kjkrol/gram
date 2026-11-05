package lod

import (
	"fmt"
	"time"
)

// Level is how fully a thing is ticked and drawn: three scales of two rungs each, exact and
// coarse, the close scale's L0 the fullest, and L6 neither ticked nor drawn.
type Level uint8

const (
	L0 Level = iota
	L1
	L2
	L3
	L4
	L5
	L6
	// Levels is how many levels there are.
	Levels = 7
)

// The exact rungs the scales begin at, and the omission.
const (
	// Close is the close scale's first rung, used only where a ladder sets its thresholds.
	Close = L0
	// Standard is the standard scale's first rung, a thing in full.
	Standard = L2
	// Strategic is the strategic scale's first rung, a thing's shape and aggregates folded.
	Strategic = L4
	// Omitted is the level neither ticked nor drawn.
	Omitted = L6
)

// Ladder is the world's levels: Scales[k] the fewest shown pixels a world unit must span for
// level k (L0 to L5), strictly decreasing where set, a zero leaving a close level out and met by
// any scale from Standard on; Sight how far from an observer the world is ticked and drawn at
// all (zero, without end), its last Fog fading.
// The zero Ladder is Standard everywhere without end round an observer, Strategic with none.
type Ladder struct {
	Scales [Levels - 1]float32
	Sight  float32
	Fog    float32
	// Unobserved is the level a world no camera observes is ticked at.
	Unobserved Fallback
	// Keep is the share of a level's entities a camera's publisher keeps, in [0, 1], zero for
	// all; the rest are thinned by their identity, and Omitted's means nothing, none being
	// published.
	Keep [Levels]float32
	// Texel is how many world units a texel of the figures' colour summed into the ground spans,
	// zero a sector's side over 64. The world raises it to span 2048 texels a side at most.
	Texel float32
	// Fold is how many seconds an aggregate's sector lies at Strategic or coarser before it
	// folds, zero for 3: a hold past the levels' own, so one on the edge of L3 and L4 does not
	// flicker.
	Fold float32
}

// FoldAfter is how long an aggregate's sector lies at Strategic or coarser before it folds: Fold
// seconds, 3 for none.
func (l Ladder) FoldAfter() time.Duration {
	if l.Fold == 0 {
		return 3 * time.Second
	}
	return time.Duration(float64(l.Fold) * float64(time.Second))
}

// Fallback is the level a world no camera observes is ticked at, its zero L4.
type Fallback uint8

// The Fallbacks, each the level its name says: UnobservedL4 the default, UnobservedL6 a world no
// camera observes asleep.
const (
	UnobservedL4 Fallback = iota
	UnobservedL0
	UnobservedL1
	UnobservedL2
	UnobservedL3
	UnobservedL5
	UnobservedL6
)

// Level is the level f stands for; it panics on a Fallback none of the constants is.
func (f Fallback) Level() Level {
	switch f {
	case UnobservedL0:
		return L0
	case UnobservedL1:
		return L1
	case UnobservedL2:
		return L2
	case UnobservedL3:
		return L3
	case UnobservedL4:
		return L4
	case UnobservedL5:
		return L5
	case UnobservedL6:
		return L6
	}
	panic(fmt.Sprintf("lod: an unobserved world at Fallback %d, want one of the seven", f))
}

// At is the level of what a world unit spans scale shown pixels of, dist from the observer: the
// first whose threshold the scale meets, a close one only where set, Omitted under the last or
// past the Sight.
func (l Ladder) At(scale, dist float32) Level {
	if l.Sight > 0 && dist > l.Sight {
		return Omitted
	}
	for k, s := range l.Scales {
		if scale >= s && (s > 0 || Level(k) >= Standard) {
			return Level(k)
		}
	}
	return Omitted
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
// before where set, a Fog past the Sight, a Keep past 1, an unknown Unobserved.
func (l Ladder) Check() {
	l.Unobserved.Level()
	if !(l.Fold >= 0) {
		panic(fmt.Sprintf("lod: aggregates folding after %v seconds, want none negative", l.Fold))
	}
	for k, keep := range l.Keep {
		if !(keep >= 0 && keep <= 1) {
			panic(fmt.Sprintf("lod: level %d keeps a share of %v, want one in [0, 1]", k, keep))
		}
	}
	if !(l.Texel >= 0) {
		panic(fmt.Sprintf("lod: a texel of %v, want none negative", l.Texel))
	}
	// the level whose threshold the next one set must be under, -1 for none yet
	before := -1
	for k, s := range l.Scales {
		switch {
		case s < 0:
			panic(fmt.Sprintf("lod: level %d's threshold %v is negative", k, s))
		case s == 0 && Level(k) < Standard:
			continue
		case before >= 0 && s > 0 && s >= l.Scales[before]:
			panic(fmt.Sprintf("lod: level %d's threshold %v is not under level %d's %v", k, s,
				before, l.Scales[before]))
		}
		before = k
	}
	switch {
	case l.Sight < 0 || l.Fog < 0:
		panic(fmt.Sprintf("lod: a sight of %v with a fog of %v, want neither negative", l.Sight,
			l.Fog))
	case l.Fog > l.Sight:
		panic(fmt.Sprintf("lod: a fog of %v past a sight of %v", l.Fog, l.Sight))
	}
}

// Paces are the steps of the world a system takes one step over in a sector k levels coarser
// than the finest any observer sees, at Paces[k], 0 none; at Omitted none ever.
type Paces [Levels]uint8

var (
	// Exact goes every step on the finest level seen alone.
	Exact = Paces{1, 0, 0, 0, 0, 0, 0}
	// Steady goes every step at every level but Omitted.
	Steady = Paces{1, 1, 1, 1, 1, 1, 0}
	// Contact goes every step up to two levels coarser than the finest seen, none beyond.
	Contact = Paces{1, 1, 1, 0, 0, 0, 0}
)

// Slowing goes every step on the finest level seen and a step every n one to five levels coarser
// in turn, the last n repeated where fewer are given; it panics on no n, more than five, a 0 or
// a falling one.
func Slowing(n ...uint8) Paces {
	if len(n) == 0 || len(n) > Levels-2 {
		panic(fmt.Sprintf("lod: slowing by %v, want one to five steps", n))
	}
	p := Paces{1}
	for k := 1; k < Levels-1; k++ {
		p[k] = n[min(k-1, len(n)-1)]
		if p[k] == 0 {
			panic(fmt.Sprintf("lod: slowing by %v, want no 0: Exact is the pace of none", n))
		}
	}
	p.Check()
	return p
}

// Check panics on paces that cannot be: the finest level seen not every step, any six levels
// coarser, a level slower than the one after it or going after one that does not.
func (p Paces) Check() {
	switch {
	case p[0] != 1:
		panic(fmt.Sprintf("lod: paces %v, want the finest level seen every step", p))
	case p[Levels-1] != 0:
		panic(fmt.Sprintf("lod: paces %v, want none %d levels coarser", p, Levels-1))
	}
	for k := 1; k < Levels-1; k++ {
		switch {
		case p[k] == 0:
		case p[k-1] == 0:
			panic(fmt.Sprintf("lod: paces %v, want none %d levels coarser after none before", p,
				k))
		case p[k] < p[k-1]:
			panic(fmt.Sprintf("lod: paces %v, want %d levels coarser no faster than %d", p, k,
				k-1))
		}
	}
}
