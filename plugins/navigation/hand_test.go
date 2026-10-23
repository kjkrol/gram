package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/uid"
)

func (rw *roadWorld) tick(n int) {
	for range n {
		rw.ecs.Tick(time.Second / 60)
	}
}

func TestHand_EndsTheOrderOfTheUnitItIsOn(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0}})
	rw = newRoadWorld(t, 10, []roadUnit{{start: rw.at(2, 1), target: rw.at(8, 1), ordered: true, selected: true, owner: 1}})
	id := rw.byRow[0]
	rw.tick(2)
	at, o := rw.state(id)
	if o == nil || !o.Leg.Active {
		t.Fatalf("no hand on it, the unit has order %+v, want one on its way", o)
	}
	ahead := o.Leg.To
	if !rw.nav.worldPlugin.Write().Carrier().Put(1, driving.Turn{Camera: rw.cams[0], Way: 1}) {
		t.Fatal("the world carries no Turn")
	}
	rw.tick(1)
	now, o := rw.state(id)
	if o != nil {
		t.Fatalf("the unit keeps its order %+v under a hand, want it given up", o)
	}
	occupancy := rw.nav.boardPlugin.Write().Occupancy()
	stranger := uid.UID64(1 << 20)
	if ahead != now && !occupancy.CanEnter(ahead, false, stranger, cell.Land) {
		t.Error("the cell the order's step was heading into is still held")
	}
	if occupancy.CanEnter(now, false, stranger, cell.Land) {
		t.Errorf("the cell the unit stands on (%v, was %v) is no longer held", now, at)
	}
}
