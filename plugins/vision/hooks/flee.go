package hooks

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

// onCourse is the cosine of the widest angle at which one still counts as heading at the other.
const onCourse = 0.5

// Flee is the rules of those who steer clear: head away from the nearest one playing threat in
// view, else from the nearest one in view when either is heading at the other; both run During
// fleeing, a state of the world (rule.Cast on entity.World). A role obeys them, in this order:
// rule.Role("skittish").Obeys(hooks.Flee(threat, fleeing)...).
func Flee(threat *rule.Part, fleeing effect.Effect) []rule.Rule {
	return []rule.Rule{
		rule.Then[vision.Sighting]("vision.flee a threat", rule.Other(threat), rule.During(fleeing, rule.Order(steering.Away{}))),
		rule.Then[vision.Sighting]("vision.give way", rule.All,
			rule.During(fleeing, rule.If(func(s vision.Sighting) bool { return len(s.Seen) > 0 && closing(s) }, rule.Order(steering.Away{})))),
	}
}

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
