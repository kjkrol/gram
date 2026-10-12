package cell

import (
	"slices"

	"github.com/kjkrol/uid"
)

// Occupancy tracks who holds each cell, on which of its levels and in which domains, gating and
// recording every step navigation takes: a land unit and a flyer may share a cell, two land units
// may not, but for one on a bridge's deck and one on the ground under it.
type Occupancy interface {
	// CanEnter reports whether entity, moving in domain, may hold c on the raised deck over it,
	// deck, or on its ground.
	CanEnter(c ID, deck bool, entity uid.UID64, domain Domain) bool
	// Enter has entity hold c on a level in domain, in place of the hold of c it had.
	Enter(c ID, deck bool, entity uid.UID64, domain Domain)
	Leave(c ID, entity uid.UID64)
	// Holders hands visit every hold of c: who, in which domains, and whether on the deck.
	Holders(c ID, visit func(entity uid.UID64, domain Domain, deck bool))
	// Release lets go of every hold of an entity gone says is no more: the board calls it every
	// step, so one despawned — fallen in, say — blocks no cell.
	Release(gone func(uid.UID64) bool)
}

// holder is one entity on a cell, the domains it holds it in and whether on the deck over it.
type holder struct {
	entity uid.UID64
	domain Domain
	deck   bool
}

// SingleOccupancy lets one entity per domain into a cell on each of its levels: whoever shares a
// domain with a holder on its level is refused. Zero-value ready — no constructor needed.
type SingleOccupancy struct {
	holders map[ID][]holder
}

var _ Occupancy = (*SingleOccupancy)(nil)

func (o *SingleOccupancy) CanEnter(c ID, deck bool, entity uid.UID64, domain Domain) bool {
	for _, h := range o.holders[c] {
		if h.entity != entity && h.deck == deck && h.domain&domain != 0 {
			return false
		}
	}
	return true
}

func (o *SingleOccupancy) Enter(c ID, deck bool, entity uid.UID64, domain Domain) {
	o.holders = enter(o.holders, c, holder{entity, domain, deck})
}

func (o *SingleOccupancy) Leave(c ID, entity uid.UID64) {
	if o.holders != nil {
		o.holders[c] = leave(o.holders[c], entity)
	}
}

// Holder is the entity holding c on a level in a domain domain shares — whom a step into c there
// would meet — if any.
func (o *SingleOccupancy) Holder(c ID, deck bool, domain Domain) (uid.UID64, bool) {
	return holderOf(o.holders[c], deck, domain)
}

func (o *SingleOccupancy) Holders(c ID, visit func(uid.UID64, Domain, bool)) {
	holdersOf(o.holders[c], visit)
}

func (o *SingleOccupancy) Release(gone func(uid.UID64) bool) { release(o.holders, gone) }

// MultipleOccupancy lets any number of entities share a cell — tokens on a board square. Such
// entities carry no Physics: bodies cannot overlap. Zero-value ready — no constructor needed.
type MultipleOccupancy struct {
	holders map[ID][]holder
}

var _ Occupancy = (*MultipleOccupancy)(nil)

func (o *MultipleOccupancy) CanEnter(ID, bool, uid.UID64, Domain) bool { return true }

func (o *MultipleOccupancy) Enter(c ID, deck bool, entity uid.UID64, domain Domain) {
	o.holders = enter(o.holders, c, holder{entity, domain, deck})
}

func (o *MultipleOccupancy) Leave(c ID, entity uid.UID64) {
	if o.holders != nil {
		o.holders[c] = leave(o.holders[c], entity)
	}
}

// Holder is the first entity holding c on a level in a domain domain shares, if any.
func (o *MultipleOccupancy) Holder(c ID, deck bool, domain Domain) (uid.UID64, bool) {
	return holderOf(o.holders[c], deck, domain)
}

func (o *MultipleOccupancy) Holders(c ID, visit func(uid.UID64, Domain, bool)) {
	holdersOf(o.holders[c], visit)
}

func (o *MultipleOccupancy) Release(gone func(uid.UID64) bool) { release(o.holders, gone) }

// release drops from every cell of holders the entities gone says are no more.
func release(holders map[ID][]holder, gone func(uid.UID64) bool) {
	for c, hs := range holders {
		if hs = slices.DeleteFunc(hs, func(h holder) bool { return gone(h.entity) }); len(hs) == 0 {
			delete(holders, c)
		} else {
			holders[c] = hs
		}
	}
}

// enter has h hold c among holders, in place of the hold of c its entity had.
func enter(holders map[ID][]holder, c ID, h holder) map[ID][]holder {
	if holders == nil {
		holders = make(map[ID][]holder)
	}
	for i, o := range holders[c] {
		if o.entity == h.entity {
			holders[c][i] = h
			return holders
		}
	}
	holders[c] = append(holders[c], h)
	return holders
}

// holdersOf hands visit every one of holders.
func holdersOf(holders []holder, visit func(uid.UID64, Domain, bool)) {
	for _, h := range holders {
		visit(h.entity, h.domain, h.deck)
	}
}

// holderOf is the first of holders on a level in a domain domain shares.
func holderOf(holders []holder, deck bool, domain Domain) (uid.UID64, bool) {
	for _, h := range holders {
		if h.deck == deck && h.domain&domain != 0 {
			return h.entity, true
		}
	}
	return 0, false
}

// leave drops entity from holders, keeping the order of the rest.
func leave(holders []holder, entity uid.UID64) []holder {
	for i, h := range holders {
		if h.entity == entity {
			return append(holders[:i], holders[i+1:]...)
		}
	}
	return holders
}
