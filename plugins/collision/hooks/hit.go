package hooks

import (
	"time"

	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/gram/plugins/world/rule"
	"github.com/kjkrol/gram/plugins/world/rule/effect"
)

// States is the family of this package's markers: a hit showing. A kind carries it for good with
// comp.Marks[States](); an entity without it gets it at its first hit.
type States struct{}

// Hits is the hit: the effect of having struck something, and the marker it has on while it lasts.
type Hits struct {
	Effect effect.Effect
	Mark   tag.Tag[States]
}

// Hit defines on w the hit, lasting d of game time: the marker "collision.hit", given to every unit
// the world's roster makes, and the effect having it on. Define it in Init, before the kinds.
func Hit(w *world.Plugin, d time.Duration) Hits {
	mark := w.Kinds().DefineTag[States]("collision.hit")
	w.Roster().Unit.Default(comp.Marks[States]())
	return Hits{Effect: w.Effects().Define("hit", effect.Spec{effect.Lasts(d), effect.Grant(mark)}), Mark: mark}
}

// ShowHits casts the hit on an entity that struck something, afresh every tick it strikes; hook it
// on collision.
func ShowHits(h Hits) plugin.Rule {
	return rule.On("collision.show hits", rule.All, func(m *rule.Moment[collision.Struck]) rule.Step {
		return m.If(collision.Struck.Hit, m.Apply(h.Effect))
	})
}

// HitOverlay is a Drawing rule for the world plugin: with is drawn on top of an entity while its
// hit marker is on — a bit read from its row, no effect looked up.
func HitOverlay(h Hits, with world.Appearance) plugin.Rule {
	return rule.On("collision.hit overlay", rule.Self(h.Mark), func(m *rule.Moment[world.Drawing]) rule.Step {
		return m.Call(func(_ plugin.Tick, d world.Drawing) { d.Overlay(with) })
	})
}
