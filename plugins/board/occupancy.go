package board

import "github.com/kjkrol/uid"

// Occupancy tracks who holds each cell and in which domains, gating and recording every step
// navigation takes: a land unit and a flyer may share a cell, two land units may not.
type Occupancy interface {
	// CanEnter reports whether entity, moving in domain, may hold c.
	CanEnter(c CellID, entity uid.UID64, domain Domain) bool
	Enter(c CellID, entity uid.UID64, domain Domain)
	Leave(c CellID, entity uid.UID64)
}

// holder is one entity on a cell and the domains it holds it in.
type holder struct {
	entity uid.UID64
	domain Domain
}

// SingleOccupancy lets one entity per domain into a cell: whoever shares a domain with a holder
// is refused. Zero-value ready — no constructor needed.
type SingleOccupancy struct {
	holders map[CellID][]holder
}

var _ Occupancy = (*SingleOccupancy)(nil)

func (o *SingleOccupancy) CanEnter(c CellID, entity uid.UID64, domain Domain) bool {
	for _, h := range o.holders[c] {
		if h.entity != entity && h.domain&domain != 0 {
			return false
		}
	}
	return true
}

func (o *SingleOccupancy) Enter(c CellID, entity uid.UID64, domain Domain) {
	if o.holders == nil {
		o.holders = make(map[CellID][]holder)
	}
	for i, h := range o.holders[c] {
		if h.entity == entity {
			o.holders[c][i].domain = domain
			return
		}
	}
	o.holders[c] = append(o.holders[c], holder{entity, domain})
}

func (o *SingleOccupancy) Leave(c CellID, entity uid.UID64) {
	o.holders[c] = leave(o.holders[c], entity)
}

// Holder is the entity holding c in a domain domain shares — whom a step into c would meet — if
// any.
func (o *SingleOccupancy) Holder(c CellID, domain Domain) (uid.UID64, bool) {
	return holderOf(o.holders[c], domain)
}

// MultipleOccupancy lets any number of entities share a cell — tokens on a board square. Such
// entities carry no Physics: bodies cannot overlap. Zero-value ready — no constructor needed.
type MultipleOccupancy struct {
	holders map[CellID][]holder
}

var _ Occupancy = (*MultipleOccupancy)(nil)

func (o *MultipleOccupancy) CanEnter(CellID, uid.UID64, Domain) bool { return true }

func (o *MultipleOccupancy) Enter(c CellID, entity uid.UID64, domain Domain) {
	if o.holders == nil {
		o.holders = make(map[CellID][]holder)
	}
	for i, h := range o.holders[c] {
		if h.entity == entity {
			o.holders[c][i].domain = domain
			return
		}
	}
	o.holders[c] = append(o.holders[c], holder{entity, domain})
}

func (o *MultipleOccupancy) Leave(c CellID, entity uid.UID64) {
	o.holders[c] = leave(o.holders[c], entity)
}

// Holder is the first entity holding c in a domain domain shares, if any.
func (o *MultipleOccupancy) Holder(c CellID, domain Domain) (uid.UID64, bool) {
	return holderOf(o.holders[c], domain)
}

// holderOf is the first of holders in a domain domain shares.
func holderOf(holders []holder, domain Domain) (uid.UID64, bool) {
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
