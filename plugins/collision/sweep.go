package collision

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/uid"
)

// Sweep marks an entity that moved itself this step, further than the world's step allows: From
// is where its centre was as the step began, its box where it ended. Collision pairs the whole
// stretch between them, refines each pair to the segment — the other's box as it stands, its own
// motion in the step ignored — and keeps the nearest contact alone; Contact.Along says where
// along the step it lies, for both sides. A swept entity is only ever detected, never pushed and
// never pushing, whatever its Physics; two swept entities pass through each other; its Base.Vel
// is zero, whoever moves it moves its box and writes From every step. Ignore, when Ignoring, is
// an entity the sweep passes through — its shooter. A wrapping world refuses it.
type Sweep struct {
	From     geom.Vec
	Ignore   uid.UID64
	Ignoring bool
}
