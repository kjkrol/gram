package site

import (
	"fmt"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

// Estimate is a road planned: what is laid, what is levelled and what it comes to.
type Estimate struct {
	// Cells are the road's cells in order, Links each one's ways out, Ways the entries to lay
	// (board.BoardWrite.SetWay), the roads crossed or joined holding their old links too.
	Cells []cell.ID
	Links []cell.Links
	Ways  []cell.WayEntry
	// Corners are the corners the levelling moves, as Shaper.Grade leaves them grading the cells
	// of Ways in turn at their corners as Levelled says; Graded the cells whose corners move (the
	// road's first, then those the levelling ripples to), Blocked those something stands on.
	Corners []CornerMove
	Graded  []cell.ID
	Blocked []cell.ID
	// Crossings are the cells where the road meets a road standing, Decks and Fords those where
	// it runs over a bridge's span and through a ford, no way laid on them.
	Crossings []cell.ID
	Decks     []cell.ID
	Fords     []cell.ID
	// Laid is how many of the cells get a way laid or a new link, Length the length of the road
	// laid anew between its cells' middles, where no road ran already; Cut and Fill the earth dug
	// and raised, in world units cubed; Lift and Drop the most a corner is raised and dug.
	Laid      int
	Length    float32
	Cut, Fill float32
	Lift      float32
	Drop      float32
}

// Levels reports whether the plan moves any corner.
func (e Estimate) Levels() bool { return len(e.Corners) > 0 }

// CornerMove is one corner of the ground moved by the levelling: where it lies, from what
// height to what.
type CornerMove struct {
	At       geom.Vec
	From, To float32
}

// Refusal is why a road may not run or be built as planned, of the cell At.
type Refusal struct {
	At   cell.ID
	why  string
	args []any
	g    grid.Grid
}

// Why is the rule broken.
func (r *Refusal) Why() string { return fmt.Sprintf(r.why, r.args...) }

func (r *Refusal) Error() string {
	x, y, _ := r.g.Coords(r.At)
	return fmt.Sprintf("the road at [%d, %d] %s", x, y, r.Why())
}

// Refuse is a Refusal of the cell c, why said as a format with its args.
func Refuse(g grid.Grid, c cell.ID, why string, args ...any) *Refusal {
	return &Refusal{At: c, why: why, args: args, g: g}
}
