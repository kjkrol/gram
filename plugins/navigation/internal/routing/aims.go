package routing

import (
	"slices"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/uid"
)

// keeps each aggregate the orders are aimed at ticked at L1 while one is, its own Keen kept in a
// Sought, a Keen it gives itself meanwhile taken for its own, and put back as the last ends; the
// orders counted afresh each tick over the units marked Aiming, those asleep too
type aims struct {
	aimers    *goke.Query
	aimerBase goke.Comp[world.Base]
	order     goke.OptComp[order.MoveOrder]
	aimingID  goke.CompID

	sought   *goke.Query
	record   goke.Comp[order.Sought]
	soughtID goke.CompID

	targets *goke.Query
	base    goke.Comp[world.Base]
	keen    goke.OptComp[sector.Keen]
	keenID  goke.CompID

	at   []uid.UID64 // the aggregates aimed at this tick, sorted
	kept []bool      // whether each was kept already
}

func (a *aims) init(si *goke.SysInit) {
	a.aimingID = si.RegComp[order.Aiming]()
	a.soughtID = si.RegComp[order.Sought]()
	a.keenID = si.RegComp[sector.Keen]()
	a.aimers = si.NewQueryBuilder(&a.aimerBase).Include(goke.Include[order.Aiming]()).
		Optional(&a.order).Build()
	a.sought = si.NewQueryBuilder(&a.record).Build()
	a.targets = si.NewQueryBuilder(&a.base).Optional(&a.keen).Build()
}

// mark has the unit id counted among those aiming
func (a *aims) mark(cb *goke.CmdBuf, id uid.UID64) { cb.AddOne(id, a.aimingID, order.Aiming{}) }

func (a *aims) keep(cb *goke.CmdBuf) {
	a.at = a.at[:0]
	for a.aimers.All(); a.aimers.Next(); {
		cur := a.aimers.Cursor()
		orders := a.order.Slice(cur)
		for i, id := range cur.IDs {
			if orders != nil && orders[i].Aimed {
				a.at = append(a.at, orders[i].Aim)
			} else {
				cb.RemoveCompOne(id, a.aimingID)
			}
		}
	}
	slices.Sort(a.at)
	a.at = slices.Compact(a.at)
	a.kept = slices.Grow(a.kept[:0], len(a.at))[:len(a.at)]
	clear(a.kept)
	for a.sought.All(); a.sought.Next(); {
		cur := a.sought.Cursor()
		records := a.record.Slice(cur)
		for i, id := range cur.IDs {
			if k, ok := slices.BinarySearch(a.at, id); ok {
				a.kept[k] = true
				a.heed(cb, id, &records[i])
				continue
			}
			if r := records[i]; r.Had {
				cb.AddOne(id, a.keenID, r.Keen)
			} else {
				cb.RemoveCompOne(id, a.keenID)
			}
			cb.RemoveCompOne(id, a.soughtID)
		}
	}
	for k, id := range a.at {
		if a.kept[k] || !a.targets.Seek(id) {
			continue
		}
		var r order.Sought
		if keen := a.keen.At(a.targets.Cursor()); keen != nil {
			r.Keen, r.Had = *keen, true
		}
		cb.AddOne(id, a.soughtID, r)
		// the finer of the two holds
		if !r.Had || r.Keen.Level > lod.L1 {
			cb.AddOne(id, a.keenID, sector.Keen{Level: lod.L1})
		}
	}
}

// heed takes a Keen the aggregate id kept carries other than the one it is kept at, r its record,
// for its own, given it since or taken off (world.Calm), keeping it at L1 still where that is the
// finer; its own given the same as the one it is kept at is not told apart
func (a *aims) heed(cb *goke.CmdBuf, id uid.UID64, r *order.Sought) {
	if !a.targets.Seek(id) {
		return
	}
	held := sector.Keen{Level: lod.L1}
	if r.Had && r.Keen.Level <= lod.L1 {
		held = r.Keen
	}
	now := a.keen.At(a.targets.Cursor())
	if now != nil && *now == held {
		return
	}
	*r = order.Sought{}
	if now != nil {
		r.Keen, r.Had = *now, true
	}
	if !r.Had || r.Keen.Level > lod.L1 {
		cb.AddOne(id, a.keenID, sector.Keen{Level: lod.L1})
	}
}
