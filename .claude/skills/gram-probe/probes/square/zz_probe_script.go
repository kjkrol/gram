package main

import (
	"log"

	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/navigation"
)

// the units sent together to the bridge's foot past the wall's gap (a crowd there), then back
// to the west, then to the bridge again
func (p *probe) play() {
	g := p.a.board.Read(0).Board().Grid
	var to cell.ID
	switch p.tick {
	case 600, 4200:
		to = g.At(13, 8)
	case 2400:
		to = g.At(3, 4)
	default:
		return
	}
	kinds := map[kind.ID]bool{}
	for _, name := range []string{RedKind, BlueKind, YellowKind, WagonKind} {
		if id, _, _, ok := p.a.world.Kinds().Lookup(name); ok {
			kinds[id] = true
		}
	}
	n := 0
	for p.q.All(); p.q.Next(); {
		cur := p.q.Cursor()
		for i, b := range p.base.Slice(cur) {
			if kinds[b.TypeID] {
				p.a.world.Carrier().PutFrom(cur.IDs[i], navigation.MoveTo{Cell: to})
				n++
			}
		}
	}
	log.Printf("P:t%d %d units sent to %v", p.tick, n, to)
}
