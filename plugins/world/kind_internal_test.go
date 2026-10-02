package world

import (
	"github.com/kjkrol/gram/entity/kind/comp"
	ikinds "github.com/kjkrol/gram/plugins/world/internal/kinds"
)

// testKind is a kind as the registry would hold it, built straight from its parts.
func testKind(pos Position, vel Velocity, comps ...comp.Comp) ikinds.Kind {
	return ikinds.Kind{Name: "test", Position: comp.Const(pos), Velocity: comp.Const(vel), Comps: comps}
}
