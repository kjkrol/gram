package mosaic

import (
	"io/fs"

	"github.com/kjkrol/gram/plugins/board/mapfile"
)

// Cutting is how a level is cut into tiles.
type Cutting struct {
	// Tile is a tile's side in world units, a power of two, and Margin how far past a tile a
	// feature may lie and still be the tile's; zero, 1024 and 32.
	Tile   float32
	Margin float32
	// Overview says whether a feature of these tags, extent world units across, is drawn from
	// far off, so lies in the overview tier too; nil, the roads and what is 64 or more across.
	Overview func(tags map[string]string, extent float32) bool
}

// Cut writes the level in one piece whose manifest is name in fsys into the folder out as
// tiles, which Load reads as it reads the level itself.
func Cut(fsys fs.FS, name, out string, c Cutting) error {
	if c.Tile == 0 {
		c.Tile = 1024
	}
	if c.Margin == 0 {
		c.Margin = 32
	}
	if c.Overview == nil {
		c.Overview = func(tags map[string]string, extent float32) bool {
			return tags["highway"] != "" || extent >= 64
		}
	}
	return mapfile.Cut(fsys, name, out, mapfile.Cutting{
		Tile:     c.Tile,
		Margin:   c.Margin,
		Overview: c.Overview,
	})
}
