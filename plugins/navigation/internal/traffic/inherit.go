package traffic

import (
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// the entities an Inherit is given by or names, and the parts of an aggregate giving one
type heirs struct {
	lookup   *goke.Query
	base     goke.Comp[world.Base]
	z        goke.OptComp[world.Z]
	order    goke.OptComp[order.MoveOrder]
	partOf   goke.OptComp[world.PartOf]
	whole    goke.OptComp[world.Aggregate]
	aimingID goke.CompID

	parts     *goke.Query
	ofWhole   goke.Comp[world.PartOf]
	partOrder goke.Comp[order.MoveOrder]

	named *goke.Query
	label goke.Comp[ceremony.Label]
}

func (h *heirs) init(si *goke.SysInit) {
	h.lookup = si.NewQueryBuilder(&h.base).Optional(&h.z, &h.order, &h.partOf, &h.whole).Build()
	h.parts = si.NewQueryBuilder(&h.ofWhole, &h.partOrder).Build()
	h.named = si.NewQueryBuilder(&h.label).Build()
	h.aimingID = si.RegComp[order.Aiming]()
}

// inherit carries out the Inherits the units gave themselves before any of them moves: each takes
// over the order it names in place of its own, its step in flight kept and the route walked on
// from where it stands, or with none there is told to stop
func (s *System) inherit(cb *goke.CmdBuf) {
	if s.Given == nil || s.Given.inherits.Empty() {
		return
	}
	s.Given.inherits.Drain(func(i command.Issued[order.Inherit]) {
		if i.ByEntity {
			s.inheritFrom(cb, i.Entity, i.Command.From)
		}
	})
}

func (s *System) inheritFrom(cb *goke.CmdBuf, heir uid.UID64, of ceremony.Whom) {
	h := &s.heirs
	if !h.lookup.Seek(heir) {
		return
	}
	var from uid.UID64
	known := true
	if of.Nobody() {
		from, known = h.otherSide(heir)
	} else if from, known = h.one(of); !known {
		return
	}
	if known && from == heir {
		return
	}
	var took order.MoveOrder
	has := false
	if known && h.lookup.Seek(from) {
		if o := h.order.At(h.lookup.Cursor()); o != nil {
			took, has = *o, true
		}
	}
	h.lookup.Seek(heir)
	cur := h.lookup.Cursor()
	mine := h.order.At(cur)
	var leg order.Leg
	if mine != nil {
		leg = mine.Leg
	}
	o := order.Inherited(&took, leg)
	if b := h.base.At(cur); has && !leg.Active {
		off := s.offOf(b.Pos.Size)
		if c, deck, ok := spacing.LevelOf(s.grid, s.terrain, s.from(b.Pos.Center(), off),
			h.z.At(cur)); ok {
			s.anchor(&o.Path, c, deck)
		}
	}
	switch {
	case has && mine != nil:
		*mine = o
	case has:
		cb.AddOne(heir, s.orderID, o)
	case mine != nil:
		if s.told == nil {
			s.told = map[uid.UID64]told{}
		}
		t := s.told[heir]
		t.stop = true
		s.told[heir] = t
	}
	if has && took.Aimed {
		cb.AddOne(heir, h.aimingID, order.Aiming{})
	}
}

// one is the one entity of names, false for none or several
func (h *heirs) one(names ceremony.Whom) (uid.UID64, bool) {
	var id uid.UID64
	n := 0
	for _, e := range names.IDs() {
		if h.lookup.Seek(e) {
			id = e
			n++
		}
	}
	labelled := false
	names.Each(func(string, func(ceremony.Label) bool) { labelled = true })
	if !labelled {
		return id, n == 1
	}
	for h.named.All(); h.named.Next(); {
		cur := h.named.Cursor()
		for i, l := range h.label.Slice(cur) {
			if names.Holds(l) {
				id = cur.IDs[i]
				n++
			}
		}
	}
	return id, n == 1
}

// otherSide is the whole of heir, the lookup on it, a part; or as an aggregate its part of the
// least id with an order, those asleep too; false with neither
func (h *heirs) otherSide(heir uid.UID64) (uid.UID64, bool) {
	cur := h.lookup.Cursor()
	if of := h.partOf.At(cur); of != nil {
		return of.Whole, true
	}
	if !h.whole.Present(cur) {
		return 0, false
	}
	var first uid.UID64
	found := false
	for h.parts.All(); h.parts.Next(); {
		pc := h.parts.Cursor()
		for i, of := range h.ofWhole.Slice(pc) {
			if id := pc.IDs[i]; of.Whole == heir && (!found || id < first) {
				first, found = id, true
			}
		}
	}
	return first, found
}
