package trigger

import (
	"time"

	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/act"
	"github.com/kjkrol/gram/plugins/world/act/effect"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
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

// ShowHits casts the hit on an entity that struck something, afresh every tick it strikes.
func ShowHits(h Hits) act.Instant {
	var r act.Reaction[collision.Struck]
	return r.If(collision.Struck.Hit, r.Apply(h.Effect))
}

// HitOverlay is a Drawing trigger for the world plugin: with is drawn on top of an entity while its
// hit marker is on — a bit read from its row, no effect looked up.
func HitOverlay(h Hits, with world.Appearance) plugin.Trigger {
	return act.Trigger[world.Drawing]("hit overlay").Self(h.Mark).Runs(func(_ plugin.Tick, d world.Drawing) { d.Overlay(with) })
}
