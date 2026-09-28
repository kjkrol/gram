package steering

import "testing"

func TestSteering_WithoutV0SetsOffAndAccelerates(t *testing.T) {
	s := Steering{MaxSpeed: 100, Accel: 60}
	s.RequestSpeed(100)
	s.advance(0.5)
	if s.Speed != 30 {
		t.Errorf("speed after half a second from standing without V0 = %v, want 30 at Accel 60", s.Speed)
	}
}
