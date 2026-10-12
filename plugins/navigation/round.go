package navigation

import (
	"fmt"

	"github.com/kjkrol/gram/plugins/board/cell"
)

// Round is a unit's own round of cells, walked in turn and round again by the command Onward:
// what a patrol's plan reads, given to a kind with its row (comp.Load).
type Round struct {
	Cells [MaxWaypoints]cell.ID
	Count uint8
	Next  uint8 // the one Onward goes to next
}

// NewRound is a Round of cells, in that order. It panics with no cell or more than
// MaxWaypoints.
func NewRound(cells ...cell.ID) Round {
	if len(cells) == 0 || len(cells) > MaxWaypoints {
		panic(fmt.Sprintf("navigation: a round of %d cells, want 1 to %d",
			len(cells), MaxWaypoints))
	}
	r := Round{Count: uint8(len(cells))}
	copy(r.Cells[:], cells)
	return r
}

// Onward is the command a unit gives itself to go to the next cell of its Round; one without a
// Round goes nowhere.
type Onward struct{}

func (Onward) Command() {}
