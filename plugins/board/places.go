package board

import (
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/gram/plugins/board/internal/moments"
)

// Around are the places within rings of where a unit stands, those under its box first, or of a
// cell: whom cond.At and act.At turn a rule of unit.Standing or cell.Now to, 0 the cell itself.
func Around(rings int) host.Places {
	return host.Places{Placing: moments.AroundPlaces, N: rings}
}

// Ahead are the places on the cells one to steps in front of a unit, the way it faces, the
// nearest first: whom cond.At and act.At turn a rule of unit.Standing to.
func Ahead(steps int) host.Places { return host.Places{Placing: moments.AheadPlaces, N: steps} }
