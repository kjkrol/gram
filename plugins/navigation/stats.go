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
	// Climbs are the routes found through a hierarchy of regions over the board's cells; none
	// while navigation keeps no hierarchy.
	Climbs uint64
	// Builds are the regions of such a hierarchy built; none while navigation keeps none.
	Builds uint64
	// Expansions are the nodes the searches expanded: the work they did.
	Expansions uint64
}

type reporter struct{ r *Read }

var _ render.Reporter = reporter{}

func (rp reporter) Report(line func(label, value string)) {
	c := rp.r.Counts()
	line("Routes", fmt.Sprintf("searches %d, walks %d, fills %d, expansions %d", c.Searches,
		c.Walks, c.Fills, c.Expansions))
}
