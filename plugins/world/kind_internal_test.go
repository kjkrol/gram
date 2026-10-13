package world

import (
	"github.com/kjkrol/ceremony/comp"
	ikinds "github.com/kjkrol/gram/plugins/world/internal/kinds"
)

func testKind(pos Position, vel Velocity, comps ...comp.Comp) ikinds.Kind {
	return ikinds.Kind{Name: "test", Position: comp.Const(pos), Velocity: comp.Const(vel), Comps: comps}
}
