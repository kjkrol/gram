package cell

import "fmt"

// Level is which of a cell's two levels a step is on: the ground, or a raised deck over it.
type Level uint8

const (
	OnGround Level = iota
	OnDeck
)

// Step is one step a unit moving in Domain takes onto To's Kind on its Level, from a neighbour
// or from To itself, over Slope: the rise over the run between the two cells' middles, 0 under
// a Graded kind.
type Step struct {
	From, To ID
	Level    Level
	Kind     Kind
	Domain   Domain
	Slope    float64
}

// Mobility is how a game's units fare on the ground: the share of its top speed a unit keeps on
// a step, 1 full speed and nothing faster, 0 refusing the step. Factors multiply, and rest on
// the Step alone (whom the kind admits, its paces and grading, the slope and the domain): a
// share read off anything else goes stale, as the board counts only these in a cell's passage.
type Mobility interface {
	Pace(s Step) float64
}

// PaceOf is the share m gives s, a Graded kind's slope read as 0; without m the kind's own. A
// share out of [0, 1] panics, naming the kind, the domain and the slope.
func PaceOf(m Mobility, s Step) float64 {
	if s.Kind.Graded {
		s.Slope = 0
	}
	share := s.Kind.PaceFor(s.Domain)
	if m != nil {
		share = m.Pace(s)
	}
	if !(share >= 0 && share <= 1) {
		panic(fmt.Sprintf("cell: %s paces %v of the top speed for domain %d at slope %v; a share "+
			"is between 0 and 1", s.Kind.Name.String(), share, s.Domain, s.Slope))
	}
	return share
}
