package navigation

import (
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/cond"
)

// StarCraft II's rules
func crowd() []act.Rule {
	return []act.Rule{makeWay(), joinTheGroup(), goRound()}
}

func makeWay() act.Rule {
	return act.Branch[Touch]{
		Name: "navigation.make way",
		When: cond.Self.If(PushedByAlly()),
		Then: act.Self.Do(StepAside{}),
	}
}

// the group gathers round the point, nobody fights for its exact spot
func joinTheGroup() act.Rule {
	return act.Branch[Touch]{
		Name: "navigation.join the group",
		When: cond.Self.If(ReachedTheGroup()),
		Then: act.Self.Do(Stop{}),
	}
}

func goRound() act.Rule {
	return act.Branch[Touch]{
		Name: "navigation.go round",
		When: cond.Self.If(cond.Always),
		Then: act.OneOf(
			act.Branch[Touch]{When: cond.Self.If(GoalTaken()), Then: act.Self.Do(Settle{})},
			act.Branch[Touch]{When: cond.Self.If(WaitsFirst()), Then: act.Self.Do(Hold{})},
			act.Branch[Touch]{When: cond.Self.If(NoWayRound()), Then: act.Self.Do(StepAside{})},
			act.Branch[Touch]{When: cond.Self.If(ClearsTheWay()), Then: act.Self.Do(Pass{})},
			act.Branch[Touch]{When: cond.Self.If(Blocks()), Then: act.Self.Do(Detour{})},
		),
	}
}
