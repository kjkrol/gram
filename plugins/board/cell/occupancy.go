package cell

import (
	"slices"

	"github.com/kjkrol/uid"
)

// OccupancyRead is who holds each cell, on which of its levels and in which domains, read.
type OccupancyRead interface {
	// CanEnter reports whether entity, moving in domain, may hold c on the deck over it or on
	// its ground.
	CanEnter(c ID, deck bool, entity uid.UID64, domain Domain) bool
	// Holders hands visit every hold of c.
	Holders(c ID, visit func(entity uid.UID64, domain Domain, deck bool))
}

// Occupancy tracks who holds each cell, on which of its levels and in which domains.
type Occupancy interface {
	OccupancyRead
	// Enter has entity hold c on a level in domain, in place of the hold of c it had.
	Enter(c ID, deck bool, entity uid.UID64, domain Domain)
	Leave(c ID, entity uid.UID64)
	// Release lets go of every hold of an entity gone says is no more.
	Release(gone func(uid.UID64) bool)
}

type holder struct {
	entity uid.UID64
	domain Domain
	deck   bool
}

// SingleOccupancy lets one entity per domain into a cell on each of its levels. The zero value
// is ready.
type SingleOccupancy struct {
	holders map[ID][]holder
	spare   [][]holder // the emptied cells' slices, for the cells entered next
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
	o.holders = enter(o.holders, &o.spare, c, holder{entity, domain, deck})
}

func (o *SingleOccupancy) Leave(c ID, entity uid.UID64) {
	if o.holders != nil {
		o.holders[c] = leave(o.holders[c], entity)
	}
}

// Holder is the entity holding c on a level in a domain domain shares, if any.
func (o *SingleOccupancy) Holder(c ID, deck bool, domain Domain) (uid.UID64, bool) {
	return holderOf(o.holders[c], deck, domain)
}

func (o *SingleOccupancy) Holders(c ID, visit func(uid.UID64, Domain, bool)) {
	holdersOf(o.holders[c], visit)
}

func (o *SingleOccupancy) Release(gone func(uid.UID64) bool) { release(o.holders, &o.spare, gone) }

// MultipleOccupancy lets any number of entities share a cell. The zero value is ready.
type MultipleOccupancy struct {
	holders map[ID][]holder
	spare   [][]holder // the emptied cells' slices, for the cells entered next
}

var _ Occupancy = (*MultipleOccupancy)(nil)

func (o *MultipleOccupancy) CanEnter(ID, bool, uid.UID64, Domain) bool { return true }

func (o *MultipleOccupancy) Enter(c ID, deck bool, entity uid.UID64, domain Domain) {
	o.holders = enter(o.holders, &o.spare, c, holder{entity, domain, deck})
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

func (o *MultipleOccupancy) Release(gone func(uid.UID64) bool) {
	release(o.holders, &o.spare, gone)
}

func release(holders map[ID][]holder, spare *[][]holder, gone func(uid.UID64) bool) {
	for c, hs := range holders {
		if hs = slices.DeleteFunc(hs, func(h holder) bool { return gone(h.entity) }); len(hs) == 0 {
			delete(holders, c)
			if cap(hs) > 0 {
				*spare = append(*spare, hs)
			}
		} else {
			holders[c] = hs
		}
	}
}

func enter(holders map[ID][]holder, spare *[][]holder, c ID, h holder) map[ID][]holder {
	if holders == nil {
		holders = make(map[ID][]holder)
	}
	hs := holders[c]
	for i, o := range hs {
		if o.entity == h.entity {
			hs[i] = h
			return holders
		}
	}
	if n := len(*spare); cap(hs) == 0 && n > 0 {
		hs, *spare = (*spare)[n-1], (*spare)[:n-1]
	}
	holders[c] = append(hs, h)
	return holders
}

func holdersOf(holders []holder, visit func(uid.UID64, Domain, bool)) {
	for _, h := range holders {
		visit(h.entity, h.domain, h.deck)
	}
}

func holderOf(holders []holder, deck bool, domain Domain) (uid.UID64, bool) {
	for _, h := range holders {
		if h.deck == deck && h.domain&domain != 0 {
			return h.entity, true
		}
	}
	return 0, false
}

func leave(holders []holder, entity uid.UID64) []holder {
	for i, h := range holders {
		if h.entity == entity {
			return append(holders[:i], holders[i+1:]...)
		}
	}
	return holders
}
