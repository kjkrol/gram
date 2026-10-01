package relief

import (
	"github.com/kjkrol/gram/plugins/board/cell"
)

// Climbing is what slopes do to whoever goes over them, by the slope: rise over run. A climb takes
// 1 + Up·slope times as long as the flat. A gentle descent is quicker, the quickest — 1 − Down
// times as long, Down below 1 — at a fall of Ease; a steeper one slows again by Steep for every
// unit of fall past Ease: a steep way down is picked carefully. Whoever moves in a Free domain
// flies over.
type Climbing struct {
	Up, Down    float64
	Ease, Steep float64
	Free        cell.Domain
}

// DefaultClimbing has a climb of 1 in 10 take twice as long as the flat, a descent of 1 in 10 the
// quickest, 0.7 as long, one of 1 in 5 slower than the flat, and Air fly over.
var DefaultClimbing = Climbing{Up: 10, Down: 0.3, Ease: 0.1, Steep: 5, Free: cell.Air}

// Factor is how many times as long a step over slope takes as one on the flat.
func (c Climbing) Factor(slope float64) float64 {
	if slope >= 0 {
		return 1 + c.Up*slope
	}
	fall := -slope
	switch {
	case c.Ease <= 0:
		return 1 + c.Steep*fall // no descent is quick
	case fall <= c.Ease:
		return 1 - c.Down*fall/c.Ease
	}
	return 1 - c.Down + c.Steep*(fall-c.Ease)
}

// Least is the smallest Factor, on a descent of Ease.
func (c Climbing) Least() float64 {
	if c.Ease <= 0 {
		return 1
	}
	return min(1-c.Down, 1)
}

// Feels reports whether an entity moving in d climbs: none of its domains is Free.
func (c Climbing) Feels(d cell.Domain) bool { return d&c.Free == 0 }

// Climb is how many times as long the step from a to its neighbour to takes an entity moving in d
// as it would on the flat, as climbing says: on a square grid the slope of to's ground the way the
// step goes, read off its corners; on a hex grid, whose cells are level, the rise between the two
