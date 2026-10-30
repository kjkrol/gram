package navigation

import (
	"fmt"

	"github.com/kjkrol/gram/render"
)

// Counts are what navigation's routing did since the plugin was made, for a probe or the
// telemetry panel ([Read.Reporter]).
type Counts struct {
	// Searches are the routes searched for; an answer kept searched for none.
	Searches uint64
	// Walks are the routes read off a group's field.
	Walks uint64
	// Fills are the groups' fields filled.
	Fills uint64
	// Climbs are the long routes climbed through the hierarchy of their units' class (Hierarchy).
	Climbs uint64
	// Builds are the regions of the hierarchies built, again where the board changed under them.
	Builds uint64
	// Expansions are the nodes the searches expanded: the work they did.
	Expansions uint64
}

type reporter struct{ r *Read }

var _ render.Reporter = reporter{}

func (rp reporter) Report(line func(label, value string)) {
	c := rp.r.Counts()
	line("Routes", fmt.Sprintf("searches %d, climbs %d, walks %d, fills %d, builds %d, "+
		"expansions %d", c.Searches, c.Climbs, c.Walks, c.Fills, c.Builds, c.Expansions))
}
