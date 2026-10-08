package driving

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/uid"
)

// hand is what the commands of one hand add up to in a tick: each way held within -1 and 1, the
// ways of Toward summed, Sprint while any said so.
type hand struct {
	ahead, turn int8
	way         geom.Vec
	sprint      bool
}

func clampWay(v int8) int8 { return max(min(v, 1), -1) }

// held is one player's hand through one camera this tick.
type held struct {
	player control.PlayerID
	camera camera.Camera
	hand   hand
}

// hands are the commands of the tick, summed by whose hand gave them: a player's through the
// camera it was given through, an entity's its own.
type hands struct {
	aheads  control.Queue[Ahead]
	backs   control.Queue[Back]
	turns   control.Queue[Turn]
	towards control.Queue[Toward]

	players  []held
	entities map[uid.UID64]hand
}

// sum drains the queues into the tick's hands.
func (h *hands) sum() {
	h.players = h.players[:0]
	if h.entities == nil {
		h.entities = map[uid.UID64]hand{}
	}
	clear(h.entities)
	h.aheads.Drain(func(i control.Issued[Ahead]) {
		h.add(i.Player, i.Entity, i.ByEntity, i.Command.Camera, func(x *hand) {
			x.ahead, x.sprint = clampWay(x.ahead+1), x.sprint || i.Command.Sprint
		})
	})
	h.backs.Drain(func(i control.Issued[Back]) {
		h.add(i.Player, i.Entity, i.ByEntity, i.Command.Camera, func(x *hand) { x.ahead = clampWay(x.ahead - 1) })
	})
	h.turns.Drain(func(i control.Issued[Turn]) {
		h.add(i.Player, i.Entity, i.ByEntity, i.Command.Camera, func(x *hand) { x.turn = clampWay(x.turn + i.Command.Way) })
	})
	h.towards.Drain(func(i control.Issued[Toward]) {
		h.add(i.Player, i.Entity, i.ByEntity, i.Command.Camera, func(x *hand) {
			x.ahead, x.way = clampWay(x.ahead+1), x.way.Add(i.Command.Way)
		})
	})
}

// add changes the hand that gave a command: the entity's own, or the player's through cam.
func (h *hands) add(player control.PlayerID, entity uid.UID64, byEntity bool, cam camera.Camera, change func(*hand)) {
	if byEntity {
		x := h.entities[entity]
		change(&x)
		h.entities[entity] = x
		return
	}
	for k := range h.players {
		if h.players[k].player == player && h.players[k].camera == cam {
			change(&h.players[k].hand)
			return
		}
	}
	h.players = append(h.players, held{player: player, camera: cam})
	change(&h.players[len(h.players)-1].hand)
}
