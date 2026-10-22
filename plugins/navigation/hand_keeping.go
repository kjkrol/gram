package navigation

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ driving.Keeping = (*handKeeping)(nil)

type handKeeping struct {
	keep spacing.Keeping   // set at Install
	nav  *navigationSystem // set at Install
	near bodies.Nearby
}

func (k *handKeeping) MayStep(id uid.UID64, pos world.Position, at, ahead cell.ID, deck bool,
	domain cell.Domain, heading geom.Vec) bool {
	if k.keep == nil {
		return true
	}
	m := bodies.Member{ID: id, Cell: at, From: at, Domain: domain, Pos: pos}
	return k.keep.MayStep(m, ahead, deck, heading, &k.near)
}

func (k *handKeeping) Ordered(id uid.UID64) bool { return k.nav != nil && k.nav.ordered(id) }
