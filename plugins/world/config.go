package world

import (
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/lod"
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
	// Sectors cuts the world into sectors ticked by level round the observers; zero, the whole
	// world at the world's pace.
	Sectors SectorsCfg
	// LOD is the world's ladder of levels, its sectors' by the scale they are seen at and how far
	// they lie; zero, Standard everywhere without end round an observer, Strategic with none. It
	// needs Sectors.
	LOD lod.Ladder
	// Far is how the loose units are drawn at the strategic scale; zero, Points.
	Far FarLook
	// Counts has the number of a kind's units in a sector drawn by them at the strategic scale.
	Counts bool
	// Sheets is where the sheets of what stands are kept between runs; zero, they are made anew
	// each run and live in memory alone.
	Sheets SheetsCfg
}

// SheetsCfg is the files of the sheets of what stands: tiles of them read as the game starts,
// so a sector photographed in an earlier run is not photographed again unless what stands in it
// has changed.
type SheetsCfg struct {
	// Dir is the folder of the tiles and their manifest; empty, no files.
	Dir string
	// Keep has the tiles of the sectors photographed written back to Dir; false, Dir only read.
	Keep bool
	// Version is the game's word for how it plants its fields: tiles written under another are
	// ignored, so a game changes it as it changes its rules of planting.
	Version string
}

// FarLook is how the loose units of a kind are drawn at the strategic scale: a point each, or one
// banner a sector where the kind declares a look of its own there.
type FarLook uint8

// The FarLooks but AsConfig.
const (
	// Points draws every unit a point of its own.
	Points FarLook = iota + 1
	// Banners draws a kind's units in a sector as one, in its look at L4, where it declares one.
	Banners
)

// AsConfig is a setting left as the Config says (Settings.Far, Settings.Counts): the zero an older
// save loads such a setting as.
const AsConfig = 0

// SectorsCfg is the world's sectors: Buckets of the space's buckets a side, the world cut into
// sectors where it is over zero.
type SectorsCfg struct {
	Buckets int
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
