package collision

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/uid"
)

// Sweep marks an entity that moves itself further in a step than the world's cap: From is where
// its centre was as the step began, its box where it ended, and the nearest contact along the
// step alone is kept. Ignore, when Ignoring, is the one entity the sweep passes through.
type Sweep struct {
	From     geom.Vec
	Ignore   uid.UID64
	Ignoring bool
}
