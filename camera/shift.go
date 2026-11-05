package camera

import "github.com/kjkrol/gram/lod"

// Shifted is a camera whose picture is drawn LevelShift levels coarser than the world sees its
// sectors at, the frame budget's shift; a sector seen at lod.Standard or finer stays full.
type Shifted interface {
	SetLevelShift(n lod.Level)
	LevelShift() lod.Level
}

// LevelShiftOf is how many levels coarser than the world sees them cam draws its sectors seen
// past lod.Standard, 0 for a camera without a shift.
func LevelShiftOf(cam Camera) lod.Level {
	if s, ok := cam.(Shifted); ok {
		return s.LevelShift()
	}
	return 0
}
