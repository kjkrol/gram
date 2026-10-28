package spacing

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/uid"
)

// more units than placeCells each get a cell of their own round the target
func TestCellKeeping_Orders_EveryMemberGetsACellOfItsOwn(t *testing.T) {
	g := grid.DefaultGrids{}.Square(20, 20, 16)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	occ := &cell.SingleOccupancy{}
	keep := NewCellKeeping(NewFinder(g, terrain, nil, OpenOccupancy{}), occ)
	var members []bodies.Member
	for i := range 3 * placeCells {
		c := g.At(uint32(i%20), uint32(i/20))
		members = append(members, bodies.Member{ID: uid.UID64(i + 1), Cell: c, From: c,
			Domain: cell.Land})
	}
	targets := map[cell.ID]bool{}
	keep.Orders(members, order.MoveTo{Cell: g.At(15, 15)}, func(_ bodies.Member, o order.MoveOrder) {
		targets[o.Target] = true
	})
	if len(targets) != len(members) {
		t.Errorf("%d cells given, want one to each of the %d", len(targets), len(members))
	}
}
