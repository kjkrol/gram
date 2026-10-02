package unit

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/uid"
)

// Standing is where an entity on the board stands this tick: the cell under its centre, that
// cell's kind and the game's tags of its place (a plate under it), its box (Grid.CellsUnder lists
// every cell it touches) and the domains it moves in (its Mover's; Land without one). The board
// hosts rules of it (board.Plugin.Hook).
type Standing struct {
	ID     uid.UID64
	Cell   cell.ID
	Kind   cell.Kind
	Places cell.Tags // the game's tags of the cell's place: a plate, a zone
	Box    geom.AABB
	Domain cell.Domain
}

// Who is the entity standing: whose moment it is, for a rule.
func (s Standing) Who() uid.UID64 { return s.ID }

// Placed: a rule's Here acts on the cells under the entity's box, its Around on the rings round
// them too.
func (Standing) Placed() {}

// Fallen reports whether the entity stands where its domain may not be: in a hole, in water on
// foot.
func (s Standing) Fallen() bool { return !s.Kind.Admits(s.Domain) }
