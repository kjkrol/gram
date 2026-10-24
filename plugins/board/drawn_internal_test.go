package board

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

func TestDrawnBoard_HoldsTheCellsAsPublishedEachBufferCatchingUp(t *testing.T) {
	b := newBoard(grid.DefaultGrids{}.Square(4, 4, 32))
	grass := cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land, SpriteID: 1}
	sand := cell.Kind{Name: cell.Named("sand"), Cost: 1, Allows: cell.Land, SpriteID: 2}
	c := b.At(1, 2)
	drawn := b.drawnBoard()
	b.write.Set(c, grass)
	if drawn.Kind(c) != grass {
		t.Fatalf("before any publish the drawn cell is %v, want the board's own", drawn.Kind(c))
	}
	frame := func() {
		b.drawn.publish()
		b.drawn.swap()
	}
	frame()
	b.write.Set(c, sand)
	if drawn.Kind(c) != grass {
		t.Errorf("changed after the frame was published, the drawn cell is %v, want grass "+
			"till the next", drawn.Kind(c).Name)
	}
	frame()
	if drawn.Kind(c) != sand || drawn.Changes() != b.read.Changes() {
		t.Errorf("published again, the drawn cell is %v at %d changes, want sand at %d",
			drawn.Kind(c).Name, drawn.Changes(), b.read.Changes())
	}
	// the buffer drawn next held grass, two frames ago: it catches up with nothing new changed
	frame()
	if drawn.Kind(c) != sand {
		t.Errorf("the buffer two frames old shows %v, want it caught up: sand", drawn.Kind(c).Name)
	}
}
