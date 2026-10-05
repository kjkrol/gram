package hooks

import (
	"time"

	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

// Hit defines on w the hit, the effect of having struck something, lasting d of game time; its
// marker, Mark, is on while it lasts. Define it in Init, before the kinds.
func Hit(w *world.Plugin, d time.Duration) effect.Effect {
	return w.Effects().Define("hit", effect.Spec{effect.Lasts(d)})
}

// ShowHits casts the hit on an entity that struck something, afresh every tick it strikes; hook it
// on collision.
func ShowHits(hit effect.Effect) rule.Rule {
	return rule.Then[collision.Struck]("collision.show hits", rule.All, rule.Apply(hit))
}

// HitOverlay draws with on top of an entity while the hit's marker is on — a bit read from its
// row, no effect looked up; give it to the world's Draw.
func HitOverlay(hit effect.Effect, with world.Appearance) render.Rule {
	return render.Over(with, hit.Mark().In)
}
