package navigation

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ driving.Keeping = (*handKeeping)(nil)

type handKeeping struct {
	keep keeping           // set at Install
	nav  *navigationSystem // set at Install
}

func (k *handKeeping) MayStep(id uid.UID64, pos world.Position, at, ahead cell.ID, deck bool,
	domain cell.Domain, heading geom.Vec) bool {
	if k.keep == nil {
		return true
	}
	m := member{id: id, cell: at, from: at, domain: domain, pos: pos}
	return k.keep.mayStep(m, ahead, deck, heading)
}

func (k *handKeeping) Ordered(id uid.UID64) bool { return k.nav != nil && k.nav.ordered(id) }
