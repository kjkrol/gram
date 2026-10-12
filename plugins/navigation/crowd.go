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
		Do:   rule.Order(StepAside{}),
	}
}

// the group gathers round the point, nobody fights for its exact spot
func joinTheGroup() rule.Rule {
	return rule.Branch[Touch]{
		Name: "navigation.join the group",
		When: Touch.ReachedTheGroup,
		Do:   rule.Order(Stop{}),
	}
}

func goRound() rule.Rule {
	return rule.Branch[Touch]{
		Name: "navigation.go round",
		Do: rule.OneOf(
			rule.Branch[Touch]{When: Touch.GoalTaken, Do: rule.Order(Settle{})},
			rule.Branch[Touch]{When: Touch.WaitsFirst, Do: rule.Order(Hold{})},
			rule.Branch[Touch]{When: Touch.NoWayRound, Do: rule.Order(StepAside{})},
			rule.Branch[Touch]{When: Touch.ClearsTheWay, Do: rule.Order(Pass{})},
			rule.Branch[Touch]{When: Touch.Blocks, Do: rule.Order(Detour{})},
		),
	}
}
