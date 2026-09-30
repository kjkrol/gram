package trigger

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
)

// onCourse is the cosine of the widest angle at which one still counts as heading at the other.
const onCourse = 0.5

// Flee steers entities away from what is closing on them, and from any Threat on sight —
// register it between Skittish and Any.
type Flee struct {
	threat tag.Tag[Family]
	off    bool // zero value is on
}

// NewFlee is a Flee that is switched on, fleeing whatever carries tags.Threat.
func NewFlee(tags Tags) *Flee { return &Flee{threat: tags.Threat} }

// SetEnabled turns the trigger off and on again without unregistering it.
func (b *Flee) SetEnabled(on bool) { b.off = !on }

// Steer turns the observer away from what closes on it, and from any Threat in view.
func (b *Flee) Steer(_ plugin.Tick, s vision.Sighting) {
	if b.off || s.Steering == nil {
		return
	}
	if away, ok := awayFrom(s, b.threat); ok {
		s.Steering.Request(away)
	}
}

// awayFrom sums a push per sighting worth avoiding, nearest weighing most; Threats alone win.
func awayFrom(s vision.Sighting, threat tag.Tag[Family]) (geom.Vec, bool) {
	ox, oy := centre(&s.Base.Pos)
	heading := s.Base.Vel.Dir

	var threats, others geom.Vec
	for _, seen := range s.Seen {
		tx, ty := centre(&seen.Base.Pos)
		dx, dy := ox-tx, oy-ty
		d := math.Hypot(dx, dy)
		if d == 0 {
			continue
		}
		switch {
		case seen.Marks.Carries(threat):
			threats.X += dx / (d * d)
			threats.Y += dy / (d * d)
		case closing(heading, seen.Base.Vel.Dir, -dx/d, -dy/d):
			others.X += dx / (d * d)
			others.Y += dy / (d * d)
		}
	}
	if threats.X != 0 || threats.Y != 0 {
		return threats, true
	}
	return others, others.X != 0 || others.Y != 0
}

// closing reports whether either of the two is heading at the other.
func closing(heading, otherHeading geom.Vec, towardsX, towardsY float64) bool {
	if heading.X*towardsX+heading.Y*towardsY > onCourse {
		return true
	}
	return otherHeading.X*towardsX+otherHeading.Y*towardsY < -onCourse
}
