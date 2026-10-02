package clock_test

import (
	"testing"
	"time"

	"github.com/kjkrol/gram/clock"
)

// At holds in the one step reaching its time; Every in each step reaching another period after
// its offset, however long the steps.
func TestMoment_AtAndEveryHoldInTheStepsReachingThem(t *testing.T) {
	fired := func(holds func(clock.Moment) bool, dt time.Duration) []time.Duration {
		var at []time.Duration
		for last := time.Duration(0); last < 12; last += dt {
			if m := (clock.Moment{Last: last, Now: last + dt}); holds(m) {
				at = append(at, m.Now)
			}
		}
		return at
	}
	for _, dt := range []time.Duration{1, 3, 5} {
		if got := fired(clock.At(5), dt); len(got) != 1 || got[0] < 5 || got[0]-dt >= 5 {
			t.Errorf("step %d: At(5) held at %v, want once in the step reaching 5", dt, got)
		}
		every := fired(clock.Every(4, 2), dt)
		want := 0
		for at := time.Duration(2); at <= every[len(every)-1]; at += 4 {
			want++
		}
		if dt < 4 && len(every) != want {
			t.Errorf("step %d: Every(4, 2) held at %v, want once for each of 2, 6, 10", dt, every)
		}
		if dt == 1 && (len(every) != 3 || every[0] != 2 || every[1] != 6 || every[2] != 10) {
			t.Errorf("step 1: Every(4, 2) held at %v, want 2, 6 and 10", every)
		}
	}
	if clock.Every(4, 2)(clock.Moment{Last: 3, Now: 5}) {
		t.Error("Every(4, 2) held between 3 and 5, reaching nothing")
	}
}
