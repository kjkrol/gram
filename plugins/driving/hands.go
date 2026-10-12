package driving

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/uid"
)

type hand struct {
	ahead, turn int8
	way         geom.Vec
	sprint      bool
	jump        bool
}

func (h hand) drives() bool { return h.ahead != 0 || h.turn != 0 || h.way != geom.Vec{} }

func clampWay(v int8) int8 { return max(min(v, 1), -1) }

type held struct {
	player control.PlayerID
	camera camera.Camera
	hand   hand
}

type hands struct {
	aheads  control.Queue[Ahead]
	backs   control.Queue[Back]
	turns   control.Queue[Turn]
	towards control.Queue[Toward]
	jumps   control.Queue[Jump]

	players  []held
	entities map[uid.UID64]hand
}

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
	h.jumps.Drain(func(i control.Issued[Jump]) {
		h.add(i.Player, i.Entity, i.ByEntity, i.Command.Camera, func(x *hand) { x.jump = true })
	})
}

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
