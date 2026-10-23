package navigation

import (
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
)

// StarCraft II's rules
func crowd() []act.Rule {
	return []act.Rule{makeWay(), joinTheGroup(), goRound()}
}

func makeWay() act.Rule {
	return act.Branch[order.Touch]{
		Name: "navigation.make way",
		When: cond.Self.If(order.PushedByAlly()),
		Then: act.Self.Do(order.StepAside{}),
	}
}

// the group gathers round the point, nobody fights for its exact spot
func joinTheGroup() act.Rule {
	return act.Branch[order.Touch]{
		Name: "navigation.join the group",
		When: cond.Self.If(order.ReachedTheGroup()),
		Then: act.Self.Do(order.Stop{}),
	}
}

func goRound() act.Rule {
	return act.Branch[order.Touch]{
		Name: "navigation.go round",
		When: cond.Self.If(cond.Always),
		Then: act.OneOf(
			act.Branch[order.Touch]{When: cond.Self.If(order.GoalTaken()),
				Then: act.Self.Do(order.Settle{})},
			act.Branch[order.Touch]{When: cond.Self.If(order.WaitsFirst()), Then: act.Self.Do(order.Hold{})},
			act.Branch[order.Touch]{When: cond.Self.If(order.NoWayRound()),
				Then: act.Self.Do(order.StepAside{})},
			act.Branch[order.Touch]{When: cond.Self.If(order.ClearsTheWay()),
				Then: act.Self.Do(order.Pass{})},
			act.Branch[order.Touch]{When: cond.Self.If(order.Blocks()), Then: act.Self.Do(order.Detour{})},
		),
	}
}
