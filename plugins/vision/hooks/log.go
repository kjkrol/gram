package hooks

import (
	"log"

	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/uid"
)

// LogSightings writes a line the first time one entity sees another.
func LogSightings() plugin.Rule {
	told := map[[2]uid.UID64]bool{}
	return host.Pair(tag.Any, tag.Any, func(_ plugin.Tick, s vision.Sighting) {
		for _, seen := range s.Seen {
			if pair := [2]uid.UID64{s.Self, seen.ID}; !told[pair] {
				told[pair] = true
				log.Printf("entity %d sees entity %d at %.0f", s.Self, seen.ID, seen.Dist)
			}
		}
	})
}
