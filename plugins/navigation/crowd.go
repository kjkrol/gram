package navigation

import (
	"github.com/kjkrol/gram/rule"
)

// crowd is how units get on among others, as in StarCraft II: one standing makes way for an ally
// on the move and stays aside, while that one goes on; one on the move stops on touching one of
// its order that has arrived, and goes round anyone else in its way. The plugin has every unit obey it unless a
// game gives its own (Plugin.WithCrowd).
func crowd() []rule.Rule {
	return []rule.Rule{makeWay(), joinTheGroup(), goRound()}
}

// makeWay has one standing step off the way of an ally on the move, where it stays.
func makeWay() rule.Rule {
	return rule.Then[Touch]("navigation.make way", rule.All, rule.If(Touch.PushedByAlly, rule.Order(StepAside{})))
}

// joinTheGroup has one on the move stop where it is on touching one of its order that has arrived:
// the group gathers round the point, nobody fights for its exact spot.
func joinTheGroup() rule.Rule {
	return rule.Then[Touch]("navigation.join the group", rule.All, rule.If(Touch.ReachedTheGroup, rule.Order(Stop{})))
}

// goRound has one on the move stand beside its goal when someone stands on it who does not make
// way, go on past an ally making way for it, and go round anyone else in its way — with no way
// round, step aside a while and go on; of two coming at each other the first waits.
func goRound() rule.Rule {
	return rule.Then[Touch]("navigation.go round", rule.All, rule.OneOf(
		rule.If(Touch.GoalTaken, rule.Order(Settle{})),
		rule.If(Touch.WaitsFirst, rule.Order(Hold{})),
		rule.If(Touch.NoWayRound, rule.Order(StepAside{})),
		rule.If(Touch.ClearsTheWay, rule.Order(Pass{})),
		rule.If(Touch.Blocks, rule.Order(Detour{})),
	))
}
