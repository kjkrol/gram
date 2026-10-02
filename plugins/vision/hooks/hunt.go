package hooks

import (
	"math"
	"time"

	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

// Chase has every Predator head at the nearest Prey it sees. Hook it on the vision plugin.
func Chase(tags Tags) rule.Rule {
	return rule.On("vision.chase", rule.Between(tags.Predator, tags.Prey), func(m *rule.Moment[vision.Sighting]) rule.Step {
		return m.Order(steering.Toward{})
	})
}

// Looked defines on w the look round, the effect of a Predator having turned aside to search,
// lasting d of game time. Define it in Init, before the kinds.
func Looked(w *world.Plugin, d time.Duration) effect.Effect {
	return w.Effects().Define("looked", effect.Spec{effect.Lasts(d)})
}

// Search has every Predator that sees no Prey turn a quarter aside, to whichever side the chance
// falls, unless it still has looked about it. Hook it on the vision plugin.
func Search(tags Tags, looked effect.Effect) rule.Rule {
	return rule.On("vision.search", rule.Between(tags.Predator, tags.Prey), func(m *rule.Moment[vision.Sighting]) rule.Step {
		return m.If(func(s vision.Sighting) bool { return len(s.Seen) == 0 }, m.Unless(looked, m.Steps(
			m.Apply(looked),
			m.OneOf(m.Chance(0.5, m.Order(steering.Turn{Angle: math.Pi / 2})), m.Order(steering.Turn{Angle: -math.Pi / 2})),
		)))
	})
}
