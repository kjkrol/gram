package navigation

import (
	"github.com/kjkrol/gram/rule"
)

// StarCraft II's rules
func crowd() []rule.Rule {
	return []rule.Rule{makeWay(), joinTheGroup(), goRound()}
}

func makeWay() rule.Rule {
	return rule.Branch[Touch]{
		Name: "navigation.make way",
		When: Touch.PushedByAlly,
		On:   rule.Self,
		Do:   rule.Apply(StepAside{}),
	}
}

// the group gathers round the point, nobody fights for its exact spot
func joinTheGroup() rule.Rule {
	return rule.Branch[Touch]{
		Name: "navigation.join the group",
		When: Touch.ReachedTheGroup,
		On:   rule.Self,
		Do:   rule.Apply(Stop{}),
	}
}

func goRound() rule.Rule {
	return rule.Branch[Touch]{
		Name: "navigation.go round",
		On:   rule.Self,
		Do: rule.OneOf(
			rule.Branch[Touch]{When: Touch.GoalTaken, Do: rule.Apply(Settle{})},
			rule.Branch[Touch]{When: Touch.WaitsFirst, Do: rule.Apply(Hold{})},
			rule.Branch[Touch]{When: Touch.NoWayRound, Do: rule.Apply(StepAside{})},
			rule.Branch[Touch]{When: Touch.ClearsTheWay, Do: rule.Apply(Pass{})},
			rule.Branch[Touch]{When: Touch.Blocks, Do: rule.Apply(Detour{})},
		),
	}
}
