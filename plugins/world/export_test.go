package world

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/uid"
)

// Hook hands rules to the world's own hosts, as the engine does with the roles' rules.
func (p *Plugin) Hook(rules ...act.Rule) error {
	return ceremony.Deliver([]host.Host{&p.module.moments.host}, rules...)
}

// AggregateTexels is how many texels a world unit the aggregate spans, held or not.
func (p *Plugin) AggregateTexels() float32 { return p.renderer.aggregate.perUnit }

// Drawn is a record the renderer publishes: its entity, the box it is drawn over and whether it
// is a field's part in a sector.
type Drawn struct {
	ID    uid.UID64
	Box   geom.AABB
	Piece bool
}

// Published publishes what the cameras see as a frame is about to be drawn and hands it back.
func (p *Plugin) Published() []Drawn {
	p.renderer.Publish()
	p.renderer.Swap()
	var out []Drawn
	for _, e := range p.renderer.figures.Front().all {
		out = append(out, Drawn{ID: e.id, Box: e.box.AABB, Piece: e.piece})
	}
	return out
}
