package mosaic

import "github.com/kjkrol/gram/lod"

// Config is how finely the ground is drawn and how fast it follows the levels.
type Config struct {
	// Tolerances is how far, in world units, the ground drawn in a sector at each level may lie
	// from the map's; all zero, DefaultTolerances.
	Tolerances [lod.Levels]float32
	// Cells is how many cells are worked out anew a frame, Triangles how many are laid anew
	// (a sector at least) and Shown how many sectors are shown at another level a batch; zero,
	// 300, 20000 and 16.
	Cells     int
	Triangles int
	Shown     int
}

// DefaultTolerances draws the standard scale whole and the strategic one in outline.
var DefaultTolerances = [lod.Levels]float32{0, 0, 0, 0.25, 2, 4, 4}

func (c Config) filled() Config {
	if c.Tolerances == ([lod.Levels]float32{}) {
		c.Tolerances = DefaultTolerances
	}
	if c.Cells == 0 {
		c.Cells = 300
	}
	if c.Triangles == 0 {
		c.Triangles = 20000
	}
	if c.Shown == 0 {
		c.Shown = 16
	}
	return c
}
