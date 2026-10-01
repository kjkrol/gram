package cell

import "github.com/kjkrol/gram/plugins/world/entity/tag"

// Family is the family of a game's tags of cells — a trapdoor, a plate, a zone — which every cell
// carries for good. A game defines them by name through the world's kinds
// (Kinds.DefineTag[cell.Family]) and gives them in the board's Layout (board.CellEntry.Tags); rules
// of a Now filter cells by them (rule.Self), as rules of units filter units by theirs, and a
// board.Standing tells those of the cell under a unit.
type Family struct{}

// Tag is one of the game's tags of cells.
type Tag = tag.Tag[Family]

// Tags are the tags a cell carries.
type Tags = tag.Tags[Family]
