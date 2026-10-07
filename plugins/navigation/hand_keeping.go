package navigation

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ driving.Keeping = (*handKeeping)(nil)

// handKeeping is what the driving asks of navigation: whether a unit driven by hand may walk on —
// the spacing's say, as for a step of a route — and whether a unit is steered along an order.
type handKeeping struct {
	keep keeping           // set as navigation is installed
	nav  *navigationSystem // set as navigation is installed
}

func (k *handKeeping) MayStep(id uid.UID64, pos world.Position, at, ahead cell.ID, domain cell.Domain, heading geom.Vec) bool {
	if k.keep == nil {
		return true
	}
	return k.keep.mayStep(member{id: id, cell: at, from: at, domain: domain, pos: pos}, ahead, heading)
}

func (k *handKeeping) Ordered(id uid.UID64) bool { return k.nav != nil && k.nav.ordered(id) }
