package players_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/render"
)

// splitRig is two local players with cameras of their own, the screen split into two columns of
// 400 by 600, each player's orders tagged by the number its binding gives.
func splitRig(t *testing.T) (*rig, *players.Player, *players.Player, *goke.ECS) {
	t.Helper()
	r := newRig(t)
	left, right := r.local, r.p.Local("right")
	if err := left.Bind(control.Command(control.KeyHeld{Key: control.KeyW}, "Up", orderOf(1)),
		control.Command(control.ButtonPress{Button: control.MouseButtonLeft}, "Here", func(c control.Context) (order, bool) {
			return order{Cell: 100 + int(c.Cursor.X)}, true
		})); err != nil {
		t.Fatal(err)
	}
	if err := right.Bind(control.Command(control.KeyHeld{Key: control.KeyArrowUp}, "Up", orderOf(2)),
		control.Command(control.ButtonPress{Button: control.MouseButtonLeft}, "Here", func(c control.Context) (order, bool) {
			return order{Cell: 200 + int(c.Cursor.X)}, true
		})); err != nil {
		t.Fatal(err)
	}
	ecs := r.start()
	// the scene's pictures, side by side: each tells its player where it lies
	r.wire.Over(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(400, 600)), render.NewFeed(r.cam, nil))
	r.p.Through(right).Over(geom.NewAABB(geom.NewVec(400, 0), geom.NewVec(800, 600)), render.NewFeed(r.cams.New(cameras.TopDown(), camera.Config{}), nil))
	return r, left, right, ecs
}

// cells is what the players ordered this pass, by who ordered it.
func cells(r *rig) map[control.PlayerID][]int {
	out := map[control.PlayerID][]int{}
	for _, o := range r.drained() {
		out[o.Player] = append(out[o.Player], o.Command.Cell)
	}
	return out
}

func TestSplitScreen_KeysReachEveryPlayerAndKeyHeldFiresEveryTickWhileDown(t *testing.T) {
	r, left, right, ecs := splitRig(t)
	if left.Area() != geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(400, 600)) || right.Area() != geom.NewAABB(geom.NewVec(400, 0), geom.NewVec(800, 600)) {
		t.Fatalf("areas %v and %v, want the left and the right half", left.Area(), right.Area())
	}
	ev := &control.InputEvents{}
	ev.AddKeyEvent(control.KeyW, control.ActionPress)
	ev.AddKeyEvent(control.KeyArrowUp, control.ActionPress)
	r.handle(ev)
	ecs.Tick(time.Second / 60) // the keys went down: their commands are issued for the next tick
	got := cells(r)
	if len(got[left.ID]) != 1 || got[left.ID][0] != 1 || len(got[right.ID]) != 1 || got[right.ID][0] != 2 {
		t.Fatalf("orders %v, want one 1 from the left player and one 2 from the right", got)
	}
	for range 3 { // several ticks, no input pass between them: once a tick each
		ecs.Tick(time.Second / 60)
		if got := cells(r); len(got[left.ID]) != 1 || len(got[right.ID]) != 1 {
			t.Fatalf("a tick with the keys still down gave %v, want one order each", got)
		}
	}
	ev = &control.InputEvents{}
	ev.AddKeyEvent(control.KeyW, control.ActionRelease)
	r.handle(ev)
	ecs.Tick(time.Second / 60)
	if got := cells(r); len(got[left.ID]) != 0 || len(got[right.ID]) != 1 {
		t.Errorf("after W came up: %v, want only the right player still going", got)
	}
}

func TestSplitScreen_TheMouseReachesThePlayerUnderItInItsOwnPixels(t *testing.T) {
	r, left, right, _ := splitRig(t)
	ev := &control.InputEvents{MousePos: geom.NewVec(650, 300)}
	ev.AddClickEvent(650, 300, control.MouseButtonLeft, control.ActionPress)
	r.handle(ev)
	got := cells(r)
	if len(got[left.ID]) != 0 || len(got[right.ID]) != 1 || got[right.ID][0] != 450 {
		t.Errorf("a click at x 650: %v, want only the right player, at x 250 of its half", got)
	}
}

func TestCursorOver_FiresEveryTickForThePlayerWhosePictureTheCursorLiesOver(t *testing.T) {
	r := newRig(t)
	right := r.p.Local("right")
	r.bind(control.Command(control.CursorOver{}, "Over", func(c control.Context) (order, bool) { return order{int(c.Cursor.X)}, true }))
	if err := right.Bind(control.Command(control.CursorOver{}, "Over", orderOf(-1))); err != nil {
		t.Fatal(err)
	}
	ecs := r.start()
	r.wire.Over(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(400, 600)), render.NewFeed(r.cam, nil))
	r.p.Through(right).Over(geom.NewAABB(geom.NewVec(400, 0), geom.NewVec(800, 600)), render.NewFeed(r.cams.New(cameras.TopDown(), camera.Config{}), nil))
	r.handle(&control.InputEvents{MousePos: geom.NewVec(100, 300)})
	for range 3 { // the cursor still, no input pass between the ticks: once a tick
		ecs.Tick(time.Second / 60)
		if got := cells(r); len(got[r.local.ID]) != 1 || got[r.local.ID][0] != 100 || len(got[right.ID]) != 0 {
			t.Fatalf("a tick with the cursor over the left picture gave %v, want one from the left player at x 100", got)
		}
	}
	r.handle(&control.InputEvents{MousePos: geom.NewVec(900, 300)}) // past both pictures
	ecs.Tick(time.Second / 60)
	if got := cells(r); len(got) != 0 {
		t.Errorf("with the cursor over no picture: %v, want nothing", got)
	}
}
