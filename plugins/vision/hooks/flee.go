package hooks

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

// onCourse is the cosine of the widest angle at which one still counts as heading at the other.
const onCourse = 0.5

// Flee has every Skittish entity head away from the nearest Threat it sees, else from the nearest
// one in view when either is heading at the other; both rules run During fleeing, a state of the
// world (world.Apply). Hook them on the vision plugin in this order.
func Flee(tags Tags, fleeing effect.Effect) []rule.Rule {
	return []rule.Rule{
		rule.On("vision.flee a threat", rule.Between(tags.Skittish, tags.Threat), func(m *rule.Moment[vision.Sighting]) rule.Step {
			return m.During(fleeing, m.Order(steering.Away{}))
		}),
		rule.On("vision.give way", rule.Between(tags.Skittish, tag.Any), func(m *rule.Moment[vision.Sighting]) rule.Step {
			return m.During(fleeing, m.If(func(s vision.Sighting) bool { return len(s.Seen) > 0 && closing(s) }, m.Order(steering.Away{})))
		}),
	}
}

// closing reports whether the observer or the nearest one it sees is heading at the other.
func closing(s vision.Sighting) bool {
	towards := s.Seen[0].Base.Pos.Center().Sub(s.Base.Pos.Center())
	d := math.Hypot(towards.X, towards.Y)
	if d == 0 {
		return false
	}
	towards = geom.NewVec(towards.X/d, towards.Y/d)
	mine, theirs := s.Base.Vel.Dir, s.Seen[0].Base.Vel.Dir
	return mine.X*towards.X+mine.Y*towards.Y > onCourse || theirs.X*towards.X+theirs.Y*towards.Y < -onCourse
}
