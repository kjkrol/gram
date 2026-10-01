package board

import "github.com/kjkrol/gram/plugins/world/entity/tag"

// Places is the family of a game's tags of cells — a trapdoor, a spawn point, a zone — which every
// cell carries for good. A game defines them by name through the world's kinds
// (Kinds.DefineTag[board.Places]) and gives them in the Layout (CellEntry.Tags); rules of a Cell
// filter cells by them (rule.Self), as rules of units filter units by theirs, and a Standing tells
// those of the cell under a unit.
type Places struct{}

// tagCell gives cell c the tags as the cells are made; the Layout gives them, before the setup.
func (b *Board) tagCell(c CellID, tags tag.Tags[Places]) {
	if b.cells != nil {
		panic("board: a cell's tags are given in the Layout, before the cells are made")
	}
	if b.places == nil {
		b.places = map[CellID]tag.Tags[Places]{}
	}
	b.places[c] |= tags
}
