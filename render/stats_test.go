package render

import (
	"testing"
	"time"
)

func TestSpent_RoundTrips(t *testing.T) {
	SetSpent(time.Millisecond, 2*time.Millisecond, 3*time.Millisecond)
	u, d, p := Spent()
	if u != time.Millisecond || d != 2*time.Millisecond || p != 3*time.Millisecond {
		t.Errorf("spent %v %v %v, want 1ms 2ms 3ms", u, d, p)
	}
}
