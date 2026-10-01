package relief

import (
	"math"
	"testing"
)

// A climb costs the more the steeper it is; a descent is quickest at Ease and costs the more the
// steeper it is past it; no slope is quicker than Least.
func TestClimbing_CostsTheMoreTheSteeperEitherWay(t *testing.T) {
	c := DefaultClimbing
	for _, s := range []struct{ slope, want float64 }{
		{0, 1}, {0.05, 1.5}, {0.1, 2}, {0.2, 3}, {0.5, 6}, {1, 11},
		{-0.05, 0.85}, {-0.1, 0.7}, {-0.2, 1.2}, {-0.5, 2.7}, {-1, 5.2},
	} {
		if got := c.Factor(s.slope); math.Abs(got-s.want) > 1e-9 {
			t.Errorf("a slope of %v costs ×%v, want ×%v", s.slope, got, s.want)
		}
		if c.Factor(s.slope) < c.Least() {
			t.Errorf("a slope of %v costs ×%v, under the least ×%v", s.slope, c.Factor(s.slope), c.Least())
		}
	}
	if c.Least() != 0.7 {
		t.Errorf("least ×%v, want ×0.7 at a descent of Ease", c.Least())
	}
	if flat := (Climbing{Up: 10, Steep: 5}); flat.Factor(-0.1) != 1.5 || flat.Least() != 1 {
		t.Errorf("with no Ease a descent of 1 in 10 costs ×%v, least ×%v; want ×1.5 and ×1", flat.Factor(-0.1), flat.Least())
	}
}
