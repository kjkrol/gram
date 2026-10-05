package rules

import (
	"math"
	"time"

	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

// Chase is the rule of those who hunt: head at the nearest one playing prey in view. A role
// obeys it: rule.Role("predator").Obeys(vrules.Chase(prey)).
func Chase(prey *rule.Part) rule.Rule {
	return rule.Then[vision.Sighting]("vision.chase", rule.Other(prey), rule.Order(steering.Toward{}))
}

// Looked defines on w the look round, the effect of a hunter having turned aside to search,
// lasting d of game time. Define it in Init, before the kinds.
func Looked(w *world.Plugin, d time.Duration) effect.Effect {
	return w.Effects().Define("looked", effect.Spec{effect.Lasts(d)})
}

// Search is the rule of those who look for their prey: seeing none playing prey, turn a quarter
// aside, to whichever side the chance falls, unless it still has looked about it. A role obeys
// it, after Chase.
func Search(prey *rule.Part, looked effect.Effect) rule.Rule {
	return rule.Then[vision.Sighting]("vision.search", rule.Other(prey),
		rule.If(func(s vision.Sighting) bool { return len(s.Seen) == 0 }, rule.Unless(looked, rule.Steps(
			rule.Apply(looked),
			rule.OneOf(rule.Chance(0.5, rule.Order(steering.Turn{Angle: math.Pi / 2})), rule.Order(steering.Turn{Angle: -math.Pi / 2})),
		))))
}
