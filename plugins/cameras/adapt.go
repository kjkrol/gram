package cameras

import (
	"time"

	"github.com/kjkrol/gram/camera"
)

// adapter steps the pictures' resolution after the frame rate, once a second at most: coarser
// once it has fallen under the budget twice running, finer, tentatively, once it has met it five
// times running. The vsync caps the rate at the screen's, so a budget met says nothing of the
// room above it: a finer step is tried, and one that costs the budget within ten seconds burns
// the trying for thirty.
type adapter struct {
	on     bool
	budget time.Duration
	under  int
	met    int
	last   time.Time // the last evaluation
	finer  time.Time // the last finer step
	burnt  time.Time // no finer step before
}

const (
	adaptEvery = time.Second
	// of the budget's rate: under the first the picture goes coarser, over the second finer
	adaptUnder = 0.92
	adaptMet   = 0.98
	// a finer step costing the budget within burnFast stops the finer steps for burnFor
	burnFast = 10 * time.Second
	burnFor  = 30 * time.Second
	// evaluations running under the budget before a coarser step, meeting it before a finer one
	underSteps = 2
	metSteps   = 5
)

// step is the factor to draw the pictures' resolution by at now, 1 for none, after fps frames a
// second; 0 frames (headless) is no evaluation.
func (a *adapter) step(now time.Time, fps float64) float32 {
	if !a.on || fps == 0 || now.Sub(a.last) < adaptEvery {
		return 1
	}
	a.last = now
	want := float64(time.Second) / float64(a.budget)
	switch {
	case fps < adaptUnder*want:
		a.under, a.met = a.under+1, 0
		if a.under < underSteps {
			return 1
		}
		a.under = 0
		if now.Sub(a.finer) < burnFast {
			a.burnt = now.Add(burnFor)
		}
		return ResolutionStep
	case fps >= adaptMet*want:
		a.met, a.under = a.met+1, 0
		if a.met < metSteps || !now.After(a.burnt) {
			return 1
		}
		a.met, a.finer = 0, now
		return 1 / ResolutionStep
	}
	a.under, a.met = 0, 0
	return 1
}

// clamped is s between LeastResolution and all the pixels
func clamped(s float32) float32 { return min(max(s, LeastResolution), 1) }

// the rings a picture drawn in rings starts with, the outermost first, and the least each is
// stepped down to
var (
	defaultRings = []camera.Ring{
		{Share: 1, Resolution: 0.6},
		{Share: 0.6, Resolution: 0.8},
		{Share: 0.35, Resolution: 1},
	}
	ringFloors = []float32{LeastResolution, 0.6, 0.7}
)

// stepRings steps one of c's rings by factor, coarser the outermost still over its floor, finer
// the innermost still under all its pixels, and is the centre's resolution after
func stepRings(c camera.Ringed, factor float32) float32 {
	rings := c.Rings()
	switch {
	case factor < 1:
		for i, r := range rings {
			floor := float32(LeastResolution)
			if i < len(ringFloors) {
				floor = ringFloors[i]
			}
			if r.Resolution > floor {
				rings[i].Resolution = max(r.Resolution*factor, floor)
				break
			}
		}
	case factor > 1:
		for i := len(rings) - 1; i >= 0; i-- {
			if r := rings[i].Resolution; r < 1 {
				rings[i].Resolution = min(r*factor, 1)
				break
			}
		}
	}
	c.SetRings(rings)
	return rings[len(rings)-1].Resolution
}
