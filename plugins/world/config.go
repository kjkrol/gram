package world

import (
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/world/clock"
)

// Config configures world's spatial shape, the bounds its entity population must respect,
// whether it has heights and how its clock goes.
type Config struct {
	Space    SpaceCfg
	Entities EntitiesCfg
	Camera   camera.Config
	// Quasi3D gives the world heights: entities carry a Z, terrain an altitude, sight an eye. A flat
	// world (the default) is a set of planes — see Layers — and refuses heights where it meets them.
	Quasi3D bool
	// Clock is how the tactical clock goes: its tempos, and whether a tempo is one bigger step or
	// as many steps; the zero Config is ½, 1, 2 and 4 in steps.
	Clock clock.Config
}

type SpaceCfg struct {
	Width, Height uint32
	Edges         aabbworld.Edges
}

// EntitiesCfg bounds how many entities the world holds and the sizes they may spawn with.
type EntitiesCfg struct {
	MaxCount int
	MinSize  uint32
	MaxSize  uint32
}

type Telemetry struct {
	Count int
}
