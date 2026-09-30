package world

import (
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/uid"
)

// Bodies spawns entities of a reserved kind that carry a Base and no Appearance: terrain and
// other geometry a plugin materializes itself. Build one in a system's Init, use it in Update.
type Bodies struct {
	w       *module
	typeID  kind.ID
	base    goke.Comp[Base]
	factory *goke.Factory
}

// NewBodies builds a Bodies whose entities carry Base plus the caller's comps under typeID; the
// comps are this factory's own columns and must not be shared with a query or another factory.
func (p *Plugin) NewBodies(si *goke.SysInit, typeID kind.ID, comps ...goke.Addable) *Bodies {
	b := &Bodies{w: p.module, typeID: typeID}
	b.factory = si.NewFactory(append([]goke.Addable{&b.base}, comps...)...)
	return b
}

// Spawn places one entity per box at once, counted against Config.Entities.MaxCount and free of
// its size bounds; write fills the caller's columns for the entity at i under cursor.
func (b *Bodies) Spawn(boxes []plane.AABB, write func(i int, id uid.UID64, cursor *goke.Cursor)) {
	if len(boxes) == 0 {
		return
	}
	b.w.reserve(len(boxes))
	b.w.telemetry.Count += len(boxes)
	b.factory.Create(len(boxes))
	index := 0
	for b.factory.Next() {
		bases := b.base.Slice(&b.factory.Cursor)
		for i, id := range b.factory.IDs {
			bases[i] = Base{Pos: Position{AABB: boxes[index]}, TypeID: b.typeID}
			b.w.space.Place(&bases[i].Pos.AABB)
			write(i, id, &b.factory.Cursor)
			index++
		}
	}
}

// Remove takes a body out of the ECS at the end of the tick.
func (b *Bodies) Remove(cb *goke.CmdBuf, id uid.UID64) { b.w.despawn(cb, id) }
