package hooks

import (
	"time"

	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/rule"
	"github.com/kjkrol/gram/plugins/world/rule/effect"
)

// Hit defines on w the hit, the effect of having struck something, lasting d of game time; its
// marker, Mark, is on while it lasts. Define it in Init, before the kinds.
func Hit(w *world.Plugin, d time.Duration) effect.Effect {
	return w.Effects().Define("hit", effect.Spec{effect.Lasts(d)})
}

// ShowHits casts the hit on an entity that struck something, afresh every tick it strikes; hook it
// on collision.
func ShowHits(hit effect.Effect) plugin.Rule {
	return rule.On("collision.show hits", rule.All, func(m *rule.Moment[collision.Struck]) rule.Step {
		return m.If(collision.Struck.Hit, m.Apply(hit))
	})
}

// HitOverlay is a Drawing rule for the world plugin: with is drawn on top of an entity while the
// hit's marker is on — a bit read from its row, no effect looked up.
func HitOverlay(hit effect.Effect, with world.Appearance) plugin.Rule {
	return rule.On("collision.hit overlay", rule.Self(hit.Mark()), func(m *rule.Moment[world.Drawing]) rule.Step {
		return m.Call(func(_ plugin.Tick, d world.Drawing) { d.Overlay(with) })
	})
}
