package steering_test

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/motion/steering"
)

var (
	east  = geom.NewVec(1.0, 0.0)
	north = geom.NewVec(0.0, 1.0)
)

func heading(v geom.Vec) float64 { return math.Atan2(v.Y, v.X) }

func TestSteering_RequestIsRefusedWhileStillReacting(t *testing.T) {
	s := steering.Helm{Steering: &steering.Steering{Reflex: 3}, Course: &steering.Course{}}

	if !s.Request(east) {
		t.Fatal("first Request refused on an idle Steering")
	}
	if s.Request(north) {
		t.Error("second Request taken while the first was still being reacted to")
	}
	if s.Pending != east {
		t.Errorf("Pending = %v, want the first request kept", s.Pending)
	}
}

func TestSteering_RequestNormalisesWhateverItIsHanded(t *testing.T) {
	s := steering.Helm{Steering: &steering.Steering{}, Course: &steering.Course{}}
	s.Request(geom.NewVec(3.0, 4.0))

	if n := math.Hypot(s.Want.X, s.Want.Y); math.Abs(n-1) > 1e-12 {
		t.Errorf("|Want| = %v, want 1", n)
	}
	if math.Abs(heading(s.Want)-math.Atan2(4, 3)) > 1e-12 {
		t.Errorf("Want points at %v, want the direction it was handed", heading(s.Want))
	}
}

func TestSteering_SpeedIsHeldWithinTheProfile(t *testing.T) {
	s := steering.Helm{Steering: &steering.Steering{MaxSpeed: 100}, Course: &steering.Course{}}
	s.RequestSpeed(500)
	if s.WantSpeed != 100 {
		t.Errorf("RequestSpeed(500) asked for %v, want MaxSpeed 100", s.WantSpeed)
	}
	s.RequestSpeed(-1)
	if s.WantSpeed != 0 {
		t.Errorf("RequestSpeed(-1) asked for %v, want 0", s.WantSpeed)
	}
	none := steering.Helm{Steering: &steering.Steering{}, Course: &steering.Course{}}
	none.RequestSpeed(50)
	if none.WantSpeed != 0 {
		t.Errorf("RequestSpeed without a profile asked for %v, want nothing", none.WantSpeed)
	}
}
