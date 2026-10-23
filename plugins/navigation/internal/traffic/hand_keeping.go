package traffic

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ driving.Keeping = (*HandKeeping)(nil)

// HandKeeping answers the driving whether a unit driven by hand may step on (driving.Keeping).
type HandKeeping struct {
	Keep spacing.Keeping // set at Install
	Nav  *System         // set at Install
	near bodies.Nearby
}

func (k *HandKeeping) MayStep(id uid.UID64, pos world.Position, at, ahead cell.ID, deck bool,
	domain cell.Domain, heading geom.Vec) bool {
	if k.Keep == nil {
		return true
	}
	m := bodies.Member{ID: id, Cell: at, From: at, Domain: domain, Pos: pos}
	return k.Keep.MayStep(m, ahead, deck, heading, &k.near)
}

func (k *HandKeeping) Ordered(id uid.UID64) bool { return k.Nav != nil && k.Nav.ordered(id) }
