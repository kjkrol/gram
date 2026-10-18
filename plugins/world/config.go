package world

import (
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/plugins/world/sector"
)

// Config is the world's shape, its bounds on entities, whether it has heights and how its clock
// goes.
type Config struct {
	Space    SpaceCfg
	Entities EntitiesCfg
	// Heights gives the world heights: entities carry a Z, and a board stands in it. A flat world
	// (the default) is a set of planes and refuses heights where it meets them.
	Heights bool
	// Clock is how the tactical clock goes; the zero Config is tempos ½, 1, 2 and 4 in steps.
	Clock clock.Config
	// Scale is how many metres a world unit spans; zero, a board.
	Scale Scale
	// Seed is what the rules' and the plans' Chance draws from, the same after a load.
	Seed uint64
	// Sectors cuts the world into sectors in bands of pace round the anchors; zero, the whole
	// world at the world's pace.
	Sectors SectorsCfg
}

// SectorsCfg is the world's sectors: Buckets of the space's buckets a side (zero, 4) and their
// bands, nearest first, at most sector.MaxBands, each reaching farther than the one before.
type SectorsCfg struct {
	Buckets int
	Bands   []sector.Band
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
