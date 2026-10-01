package board

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
)

// tagCell gives cell c the tags as the cells are made; the Layout gives them, before the setup.
func (b *Board) tagCell(c cell.ID, tags tag.Tags[cell.Family]) {
	if b.cells != nil {
		panic("board: a cell's tags are given in the Layout, before the cells are made")
	}
	if b.places == nil {
		b.places = map[cell.ID]tag.Tags[cell.Family]{}
	}
	b.places[c] |= tags
}
