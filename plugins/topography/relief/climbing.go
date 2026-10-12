package relief

import (
	"github.com/kjkrol/gram/plugins/board/cell"
)

// Climbing is what slopes (rise over run) do to whoever goes over them: a climb takes 1 + Up·slope
// times as long as the flat, a descent is quickest at a fall of Ease and slows by Steep for every
// unit of fall past it. Whoever moves in a Free domain flies over.
type Climbing struct {
	Up, Down    float64
	Ease, Steep float64
	Free        cell.Domain
}

// DefaultClimbing has a climb of 1 in 10 take twice as long as the flat, a descent of 1 in 10
// the quickest at 0.7, and Air fly over.
var DefaultClimbing = Climbing{Up: 10, Down: 0.3, Ease: 0.1, Steep: 5, Free: cell.Air}

// Factor is how many times as long a step over slope takes as one on the flat.
func (c Climbing) Factor(slope float64) float64 {
	if slope >= 0 {
		return 1 + c.Up*slope
	}
	fall := -slope
	switch {
	case c.Ease <= 0:
		return 1 + c.Steep*fall
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
