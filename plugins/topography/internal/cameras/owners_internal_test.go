package cameras

import (
	"testing"
	"time"

	"github.com/kjkrol/gram/control"
)

// V fastens a player's camera to the one selected unit it owns — another player's selected unit
// beside it notwithstanding — and never to another player's.
func TestFollow_FastensToThePlayersOwnSelectedUnitAlone(t *testing.T) {
	r := newFollowRig(t)
	r.own(r.walkers[0], 1)
	r.own(r.walkers[1], 2)
	r.selectOnly(r.walkers[0], r.walkers[1])
	r.follow.Add(1, Follow{Camera: r.cam})
	r.ecs.Tick(time.Second / 60)
	if f := r.sys.fastened(r.cam); f == nil || f.target != r.walkers[0] {
		t.Fatalf("player 1's V fastened to %v; want its own walker %v", f, r.walkers[0])
	}
	r.follow.Add(1, Follow{Camera: r.cam}) // let go
	r.ecs.Tick(time.Second / 60)
	r.selectOnly(r.walkers[1])
	r.follow.Add(1, Follow{Camera: r.cam})
	r.ecs.Tick(time.Second / 60)
	if f := r.sys.fastened(r.cam); f != nil {
		t.Errorf("player 1's V fastened to player 2's walker %v", f.target)
	}
	r.follow.Add(control.Nobody, Follow{Camera: r.cam})
	r.ecs.Tick(time.Second / 60)
	if f := r.sys.fastened(r.cam); f != nil {
		t.Errorf("nobody's V fastened to player 2's walker %v", f.target)
	}
}

// Shift+V rides in the one selected unit the player owns, never in another player's.
func TestLookOut_RidesInThePlayersOwnSelectedUnitAlone(t *testing.T) {
	r := newFollowRig(t)
	r.ridged(1000, 1001)
	r.own(r.walkers[0], 1)
	r.own(r.walkers[1], 2)
	r.selectOnly(r.walkers[0], r.walkers[1])
	r.lookOuts.Add(2, LookOut{Camera: r.cam})
	r.ecs.Tick(time.Second / 60)
	if f := r.sys.fastened(r.cam); f == nil || !f.inside || f.target != r.walkers[1] {
		t.Fatalf("player 2's Shift+V rides %v; want inside its own walker %v", f, r.walkers[1])
	}
	r.lookOuts.Add(2, LookOut{Camera: r.cam}) // out again
	r.ecs.Tick(time.Second / 60)
	r.selectOnly(r.walkers[0])
	r.lookOuts.Add(2, LookOut{Camera: r.cam})
	r.ecs.Tick(time.Second / 60)
	if f := r.sys.fastened(r.cam); f != nil {
		t.Errorf("player 2's Shift+V rides player 1's walker %v", f.target)
	}
}
