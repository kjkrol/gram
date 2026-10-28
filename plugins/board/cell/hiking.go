package cell

import "math"

// Hiking is how whoever goes on foot fares on a slope: past a rise or a fall of Shift the pace
// falls by exp(-Rate·slope), a descent never quicker than the flat, and a domain in Free flies
// over at full speed.
type Hiking struct {
	Rate, Shift float64
	Free        Domain
}

// DefaultHiking is Tobler's hiking function cut at the flat: half the pace at a rise of 1 in 5
// and at a fall of 3 in 10, Air flying over.
var DefaultHiking = Hiking{Rate: 3.5, Shift: 0.05, Free: Air}

var _ Mobility = Hiking{}

// Pace is the kind's share for the domain, slowed by the slope.
func (h Hiking) Pace(s Step) float64 {
	if s.Domain&h.Free != 0 {
		return 1
	}
	share := s.Kind.PaceFor(s.Domain)
	if s.Slope == 0 {
		return share
	}
	return share * min(1, math.Exp(-h.Rate*(math.Abs(s.Slope+h.Shift)-h.Shift)))
}
