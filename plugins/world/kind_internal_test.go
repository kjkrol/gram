package world

import "github.com/kjkrol/gram/plugins/world/entity/kind/comp"

// testKind is a kind as the registry would hold it, built straight from its parts.
func testKind(pos Position, vel Velocity, comps ...comp.Comp) registered {
	return registered{name: "test", position: comp.Const(pos), velocity: comp.Const(vel), comps: comps}
}
