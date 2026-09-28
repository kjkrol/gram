package behavior

import (
	"math"
	"testing"
	"time"
)

// clock hands perSecond a time that moves only when the test says so.
type clock struct{ now time.Time }

func newClock() *clock { return &clock{now: time.Unix(1000, 0)} }
func (c *clock) after(ms int) time.Time {
	c.now = c.now.Add(time.Duration(ms) * time.Millisecond)
	return c.now
}

func near(got, want, within float64) bool { return math.Abs(got-want) <= within }

func TestPerSecond_OneEventInAWindowThatRanLong_IsNotRoundedAway(t *testing.T) {
	c, p := newClock(), perSecond{}
	p.observe(0, c.after(0))

	got := p.observe(1, c.after(1016))

	if !near(got, 1/1.016, 1e-9) {
		t.Errorf("rate = %v, want ~0.98 — one contact in 1.016 s", got)
	}
}

func TestPerSecond_AFewEventsASecond_ReadAsARateNotAsFlicker(t *testing.T) {
	c, p := newClock(), perSecond{}
	total := 0
	p.observe(total, c.after(0))

	var rates []float64
	for i, grew := range []int{2, 1, 3, 2, 1, 2, 2, 1} {
		total += grew
		rates = append(rates, p.observe(total, c.after(1000+i%3*8)))
	}

	for i, r := range rates[2:] {
		if r < 1 || r > 2.7 {
			t.Errorf("window %d: rate = %.2f, want it to stay near 1.8 rather than jump about", i+2, r)
		}
	}
}

func TestPerSecond_FollowsAChangeWithinAFewSeconds(t *testing.T) {
	c, p := newClock(), perSecond{}
	total := 0
	p.observe(total, c.after(0))

	var last float64
	for range smoothedWindows {
		total += 1000
		got := p.observe(total, c.after(1000))
		if got < last {
			t.Fatalf("rate fell from %v to %v while the total kept growing at the same pace", last, got)
		}
		last = got
	}
	if !near(last, 1000, 1e-6) {
		t.Errorf("rate after %d seconds at 1000 a second = %v, want 1000", smoothedWindows, last)
	}

	for range smoothedWindows {
		last = p.observe(total, c.after(1000))
	}
	if last != 0 {
		t.Errorf("rate %d seconds after the contacts stopped = %v, want 0 — not the last value it saw", smoothedWindows, last)
	}
}

func TestPerSecond_BetweenWindows_HoldsTheLastRate(t *testing.T) {
	c, p := newClock(), perSecond{}
	p.observe(0, c.after(0))
	closed := p.observe(100, c.after(1000))

	if got := p.observe(190, c.after(600)); got != closed {
		t.Errorf("rate mid-window = %v, want the %v of the last closed one", got, closed)
	}
}

func TestPerSecond_FirstLookAndAResetTotal_StartFromNothing(t *testing.T) {
	c, p := newClock(), perSecond{}

	if got := p.observe(40, c.after(0)); got != 0 {
		t.Errorf("rate on the first look = %v, want 0 — there is nothing to compare with yet", got)
	}
	p.observe(140, c.after(1000))

	if got := p.observe(5, c.after(1000)); got != 0 {
		t.Errorf("rate right after the total was reset = %v, want 0, never negative", got)
	}
	if got := p.observe(65, c.after(1000)); !near(got, 60, 1e-9) {
		t.Errorf("rate a second after the reset = %v, want 60 — counted from the reset on", got)
	}
}
