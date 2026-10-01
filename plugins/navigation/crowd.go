package navigation

import (
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world/rule"
)

// crowd is how units get on among others, as in StarCraft II: one standing makes way for an ally
// on the move and stays aside, while that one goes on; one on the move stops on touching one of
// its order that has arrived, and goes round anyone else in its way. The plugin hooks it unless a
// game gives its own (Plugin.WithCrowd).
func crowd() []plugin.Rule {
	return []plugin.Rule{makeWay(), joinTheGroup(), goRound()}
}

// makeWay has one standing step off the way of an ally on the move, where it stays.
func makeWay() plugin.Rule {
	return rule.On("navigation.make way", rule.All, func(m *rule.Moment[Touch]) rule.Step {
		return m.If(Touch.PushedByAlly, m.Order(StepAside{}))
	})
}

// joinTheGroup has one on the move stop where it is on touching one of its order that has arrived:
// the group gathers round the point, nobody fights for its exact spot.
func joinTheGroup() plugin.Rule {
	return rule.On("navigation.join the group", rule.All, func(m *rule.Moment[Touch]) rule.Step {
		return m.If(Touch.ReachedTheGroup, m.Order(Stop{}))
	})
}

// goRound has one on the move stand beside its goal when someone stands on it who does not make
// way, go on past an ally making way for it, and go round anyone else in its way — with no way
// round, step aside a while and go on; of two coming at each other the first waits.
func goRound() plugin.Rule {
	return rule.On("navigation.go round", rule.All, func(m *rule.Moment[Touch]) rule.Step {
		return m.OneOf(
			m.If(Touch.GoalTaken, m.Order(Settle{})),
			m.If(Touch.WaitsFirst, m.Order(Hold{})),
			m.If(Touch.NoWayRound, m.Order(StepAside{})),
			m.If(Touch.ClearsTheWay, m.Order(Pass{})),
			m.If(Touch.Blocks, m.Order(Detour{})),
		)
	})
}
