package players_test

import (
	"bytes"
	"encoding/gob"
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
)

// splitRig is two local players with cameras of their own, the screen split into two columns of
// 400 by 600, each player's orders tagged by the number its binding gives.
func splitRig(t *testing.T) (*rig, *players.Player, *players.Player) {
	t.Helper()
	r := newRig(t)
	left, right := r.local.OwnCamera(), r.p.Local("right").OwnCamera()
	if err := left.Bind(control.Command(control.KeyHeld{Key: ebiten.KeyW}, "Up", orderOf(1)),
		control.Command(control.ButtonPress{Button: ebiten.MouseButtonLeft}, "Here", func(c control.Context) (order, bool) {
			return order{Cell: 100 + int(c.Cursor.X)}, true
		})); err != nil {
		t.Fatal(err)
	}
	if err := right.Bind(control.Command(control.KeyHeld{Key: ebiten.KeyArrowUp}, "Up", orderOf(2)),
		control.Command(control.ButtonPress{Button: ebiten.MouseButtonLeft}, "Here", func(c control.Context) (order, bool) {
			return order{Cell: 200 + int(c.Cursor.X)}, true
		})); err != nil {
		t.Fatal(err)
	}
	r.start()
	r.p.Viewports(image.Rect(0, 0, 800, 600))
	return r, left, right
}

// cells is what the players ordered this pass, by who ordered it.
func cells(r *rig) map[control.PlayerID][]int {
	out := map[control.PlayerID][]int{}
	for _, o := range r.drained() {
		out[o.Player] = append(out[o.Player], o.Command.Cell)
	}
	return out
}

func TestSplitScreen_KeysReachEveryPlayerAndKeyHeldFiresWhileDown(t *testing.T) {
	r, left, right := splitRig(t)
	if left.Area() != image.Rect(0, 0, 400, 600) || right.Area() != image.Rect(400, 0, 800, 600) {
		t.Fatalf("areas %v and %v, want the left and the right half", left.Area(), right.Area())
	}
	ev := &control.InputEvents{}
	ev.AddKeyEvent(ebiten.KeyW, control.ActionPress)
	ev.AddKeyEvent(ebiten.KeyArrowUp, control.ActionPress)
	r.handle(ev)
	got := cells(r)
	if len(got[left.ID]) != 1 || got[left.ID][0] != 1 || len(got[right.ID]) != 1 || got[right.ID][0] != 2 {
		t.Fatalf("orders %v, want one 1 from the left player and one 2 from the right", got)
	}
	r.handle(&control.InputEvents{}) // nothing new: both keys still down
	if got := cells(r); len(got[left.ID]) != 1 || len(got[right.ID]) != 1 {
		t.Errorf("a pass with the keys still down gave %v, want one order each again", got)
	}
	ev = &control.InputEvents{}
	ev.AddKeyEvent(ebiten.KeyW, control.ActionRelease)
	r.handle(ev)
	if got := cells(r); len(got[left.ID]) != 0 || len(got[right.ID]) != 1 {
		t.Errorf("after W came up: %v, want only the right player still going", got)
	}
}

func TestSplitScreen_TheMouseReachesThePlayerUnderItInItsOwnPixels(t *testing.T) {
	r, left, right := splitRig(t)
	ev := &control.InputEvents{MousePos: geom.NewVec(650, 300)}
	ev.AddClickEvent(650, 300, ebiten.MouseButtonLeft, control.ActionPress)
	r.handle(ev)
	got := cells(r)
	if len(got[left.ID]) != 0 || len(got[right.ID]) != 1 || got[right.ID][0] != 450 {
		t.Errorf("a click at x 650: %v, want only the right player, at x 250 of its half", got)
	}
}

func TestOwnCamera_IsSavedAndRestoredWithTheGame(t *testing.T) {
	newPlayers := func() (*players.Plugin, *players.Player) {
		w := world.NewPlugin(world.Config{
			Space:    world.SpaceCfg{Width: 1000, Height: 1000},
			Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
		})
		p := players.NewPlugin(w)
		p.Local("first")
		return p, p.Local("second").OwnCamera()
	}
	saved, second := newPlayers()
	second.Camera.Translate(120, 80)
	var buf bytes.Buffer
	for _, v := range saved.Serializable().Persisted() {
		if err := gob.NewEncoder(&buf).Encode(v); err != nil {
			t.Fatal(err)
		}
	}
	loaded, again := newPlayers()
	dec := gob.NewDecoder(&buf)
	for _, v := range loaded.Serializable().Persisted() {
		if err := dec.Decode(v); err != nil {
			t.Fatal(err)
		}
	}
	loaded.Restore()
	if got, want := again.Camera.Bounds(), second.Camera.Bounds(); got != want {
		t.Errorf("the second player's camera after a load shows %v, want %v", got, want)
	}
}
