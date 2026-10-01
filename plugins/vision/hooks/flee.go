package hooks

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
)

// onCourse is the cosine of the widest angle at which one still counts as heading at the other.
const onCourse = 0.5

// Flee steers every Skittish entity away from what is closing on it, and from any Threat on
// sight; its Rule is hooked on the vision plugin, and SetEnabled switches it off and on.
type Flee struct {
	skittish, threat tag.Tag[Family]
	off              bool // zero value is on
}

// NewFlee is a Flee that is switched on, for whatever carries tags.Skittish, fleeing whatever
// carries tags.Threat.
func NewFlee(tags Tags) *Flee { return &Flee{skittish: tags.Skittish, threat: tags.Threat} }

// SetEnabled turns the rule off and on again without unhooking it.
func (b *Flee) SetEnabled(on bool) { b.off = !on }

// Rule is the fleeing as a hook of a Sighting, for the vision plugin's Hook.
func (b *Flee) Rule() plugin.Rule { return host.Pair(b.skittish, tag.Any, b.steer) }

// steer turns the observer away from what closes on it, and from any Threat in view.
func (b *Flee) steer(_ plugin.Tick, s vision.Sighting) {
	if b.off || !s.Helm.Steerable() {
		return
	}
	if away, ok := awayFrom(s, b.threat); ok {
		s.Helm.Request(away)
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
