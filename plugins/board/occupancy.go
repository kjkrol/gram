package board

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"slices"

	"github.com/kjkrol/uid"
)

// Occupancy tracks who holds each cell and in which domains, gating and recording every step
// navigation takes: a land unit and a flyer may share a cell, two land units may not.
type Occupancy interface {
	// CanEnter reports whether entity, moving in domain, may hold c.
	CanEnter(c cell.ID, entity uid.UID64, domain cell.Domain) bool
	Enter(c cell.ID, entity uid.UID64, domain cell.Domain)
	Leave(c cell.ID, entity uid.UID64)
	// Release lets go of every hold of an entity gone says is no more: the board calls it every
	// step, so one despawned — fallen in, say — blocks no cell.
	Release(gone func(uid.UID64) bool)
}

// holder is one entity on a cell and the domains it holds it in.
type holder struct {
	entity uid.UID64
	domain cell.Domain
}

// SingleOccupancy lets one entity per domain into a cell: whoever shares a domain with a holder
// is refused. Zero-value ready — no constructor needed.
type SingleOccupancy struct {
	holders map[cell.ID][]holder
}

var _ Occupancy = (*SingleOccupancy)(nil)

func (o *SingleOccupancy) CanEnter(c cell.ID, entity uid.UID64, domain cell.Domain) bool {
	for _, h := range o.holders[c] {
		if h.entity != entity && h.domain&domain != 0 {
			return false
		}
	}
	return true
}

func (o *SingleOccupancy) Enter(c cell.ID, entity uid.UID64, domain cell.Domain) {
	if o.holders == nil {
		o.holders = make(map[cell.ID][]holder)
	}
	for i, h := range o.holders[c] {
		if h.entity == entity {
			o.holders[c][i].domain = domain
			return
		}
	}
	o.holders[c] = append(o.holders[c], holder{entity, domain})
}

func (o *SingleOccupancy) Leave(c cell.ID, entity uid.UID64) {
	o.holders[c] = leave(o.holders[c], entity)
}

// Holder is the entity holding c in a domain domain shares — whom a step into c would meet — if
// any.
func (o *SingleOccupancy) Holder(c cell.ID, domain cell.Domain) (uid.UID64, bool) {
	return holderOf(o.holders[c], domain)
}

func (o *SingleOccupancy) Release(gone func(uid.UID64) bool) { release(o.holders, gone) }

// MultipleOccupancy lets any number of entities share a cell — tokens on a board square. Such
// entities carry no Physics: bodies cannot overlap. Zero-value ready — no constructor needed.
type MultipleOccupancy struct {
	holders map[cell.ID][]holder
}

var _ Occupancy = (*MultipleOccupancy)(nil)

func (o *MultipleOccupancy) CanEnter(cell.ID, uid.UID64, cell.Domain) bool { return true }

func (o *MultipleOccupancy) Enter(c cell.ID, entity uid.UID64, domain cell.Domain) {
	if o.holders == nil {
		o.holders = make(map[cell.ID][]holder)
	}
	for i, h := range o.holders[c] {
		if h.entity == entity {
			o.holders[c][i].domain = domain
			return
		}
	}
	o.holders[c] = append(o.holders[c], holder{entity, domain})
}

func (o *MultipleOccupancy) Leave(c cell.ID, entity uid.UID64) {
	o.holders[c] = leave(o.holders[c], entity)
}

// Holder is the first entity holding c in a domain domain shares, if any.
func (o *MultipleOccupancy) Holder(c cell.ID, domain cell.Domain) (uid.UID64, bool) {
	return holderOf(o.holders[c], domain)
}

func (o *MultipleOccupancy) Release(gone func(uid.UID64) bool) { release(o.holders, gone) }

// release drops from every cell of holders the entities gone says are no more.
func release(holders map[cell.ID][]holder, gone func(uid.UID64) bool) {
	for c, hs := range holders {
		if hs = slices.DeleteFunc(hs, func(h holder) bool { return gone(h.entity) }); len(hs) == 0 {
			delete(holders, c)
		} else {
			holders[c] = hs
		}
	}
}

// holderOf is the first of holders in a domain domain shares.
func holderOf(holders []holder, domain cell.Domain) (uid.UID64, bool) {
	for _, h := range holders {
		if h.domain&domain != 0 {
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
