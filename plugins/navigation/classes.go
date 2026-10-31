package navigation

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/pathfind"
)

// classify has the units planned as the classes of the bodies of the kinds of unit defined over
// the board: a kind's body is known whole where the board defines it, a unit's box being its
// row's to the world
func (p *Plugin) classify() {
	var kinds []pathfind.Kind
	for _, k := range p.boardPlugin.UnitKinds() {
		body := bodies.BodyOf(k.Z(), k.Shape.Box(geom.Vec{}).Size, k.Mover.Weight)
		kinds = append(kinds, pathfind.Kind{Domain: k.Mover.Domain, Body: body})
	}
	p.classes.Set(kinds)
}
