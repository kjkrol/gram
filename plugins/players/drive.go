package players

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/uid"
)

// Hand is what a player's Drives of a tick add up to — each way held within -1 and 1, the Ways
// summed, Sprint while any said so — read by the plugin that moves the player's units.
type Hand struct {
	Ahead, Turn int8
	Way         geom.Vec
	Sprint      bool
}

// add adds a Drive to the hand.
func (h Hand) add(d Drive) Hand {
	h.Ahead, h.Turn = clampWay(h.Ahead+d.Ahead), clampWay(h.Turn+d.Turn)
	h.Way, h.Sprint = h.Way.Add(d.Way), h.Sprint || d.Sprint
	return h
}

func clampWay(v int8) int8 { return max(min(v, 1), -1) }

// hands are the players' and the entities' Drives of the tick, summed as they are first asked
// for after the players' pass.
type hands struct {
	drives   control.Queue[Drive]
	stale    bool // the pass went by: the next reader drains the Drives given since
	byPlayer map[control.PlayerID]Hand
	byEntity map[uid.UID64]Hand
}

func (h *hands) drain() {
	if !h.stale && h.byPlayer != nil {
		return
	}
	h.stale = false
	if h.byPlayer == nil {
		h.byPlayer, h.byEntity = map[control.PlayerID]Hand{}, map[uid.UID64]Hand{}
	}
	clear(h.byPlayer)
	clear(h.byEntity)
	h.drives.Drain(func(i control.Issued[Drive]) {
		if i.ByEntity {
			h.byEntity[i.Entity] = h.byEntity[i.Entity].add(i.Command)
		} else {
			h.byPlayer[i.Player] = h.byPlayer[i.Player].add(i.Command)
		}
	})
}

// Hand is what player by drives with this tick: its Drives summed; false with none. A plugin that
// moves units reads it in its pass — navigation, for the units the player's hand is on: the one
// its camera is fastened to, else those it has selected.
func (p *Plugin) Hand(by control.PlayerID) (Hand, bool) {
	p.hands.drain()
	h, ok := p.hands.byPlayer[by]
	return h, ok
}

// OwnHand is what entity e drives itself with this tick — a Drive it gave itself (rule.Order);
// false with none.
func (p *Plugin) OwnHand(e uid.UID64) (Hand, bool) {
	p.hands.drain()
	h, ok := p.hands.byEntity[e]
	return h, ok
}

// DriveBindings are W, S, A and D held into a Drive of the player's units — on, back, left,
// right — while the camera is outside any entity, for a game to bind in place of the camera's
// own keys on them (CameraBindings), which the DefaultBindings leave alone. Riding in an entity
// the same keys are bound by default.
func DriveBindings() []control.Binding {
	return driveKeys(control.KeyW, control.KeyS, control.KeyA, control.KeyD, camera.Outside,
		"Walk the selected units on; with Shift, sprint", "Brake the selected units, then back them away",
		"Turn the selected units left", "Turn the selected units right")
}

// DriveKeys are up, down, left and right held into a Drive of the player's units the way the
// key says — the ways held adding up, so two make a diagonal — while the camera is outside any
// entity: a player's own keys, where several share the keyboard.
func DriveKeys(up, down, left, right control.Key) []control.Binding {
	way := func(dx, dy float64) func(control.Context) (Drive, bool) {
		return func(control.Context) (Drive, bool) { return Drive{Ahead: 1, Way: geom.NewVec(dx, dy)}, true }
	}
	return []control.Binding{
		control.Command(control.KeyHeld{Key: up}, "Drive up", way(0, -1)).In(camera.Outside),
		control.Command(control.KeyHeld{Key: down}, "Drive down", way(0, 1)).In(camera.Outside),
		control.Command(control.KeyHeld{Key: left}, "Drive left", way(-1, 0)).In(camera.Outside),
		control.Command(control.KeyHeld{Key: right}, "Drive right", way(1, 0)).In(camera.Outside),
	}
}

// driveKeys binds on, back, left and right held to a Drive — on with Shift sprinting — in hows.
func driveKeys(on, back, left, right control.Key, hows camera.How, onLabel, backLabel, leftLabel, rightLabel string) []control.Binding {
	drive := func(ahead, turn int8) func(control.Context) (Drive, bool) {
		return func(control.Context) (Drive, bool) { return Drive{Ahead: ahead, Turn: turn}, true }
	}
	// the held key's context has the modifiers as they are
	walkOn := func(c control.Context) (Drive, bool) { return Drive{Ahead: 1, Sprint: c.Mods.Shift}, true }
	return []control.Binding{
		control.Command(control.KeyHeld{Key: on}, onLabel, walkOn).In(hows),
		control.Command(control.KeyHeld{Key: back}, backLabel, drive(-1, 0)).In(hows),
		control.Command(control.KeyHeld{Key: left}, leftLabel, drive(0, -1)).In(hows),
		control.Command(control.KeyHeld{Key: right}, rightLabel, drive(0, 1)).In(hows),
	}
}

// ridingBindings are the keys of a hand on the unit the camera is fastened to: W, S, A and D
// riding inside it, the arrows behind it.
func ridingBindings() []control.Binding {
	return append(
		driveKeys(control.KeyW, control.KeyS, control.KeyA, control.KeyD, camera.Inside,
			"Walk on; with Shift, sprint", "Brake, then back away", "Turn left", "Turn right"),
		driveKeys(control.KeyArrowUp, control.KeyArrowDown, control.KeyArrowLeft, control.KeyArrowRight, camera.Behind,
			"Walk the followed unit on; with Shift, sprint", "Brake the followed unit, then back it away",
			"Turn the followed unit anticlockwise", "Turn the followed unit clockwise")...)
}
