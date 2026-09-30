package behavior

import (
	"time"

	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
)

// HitMark is how long this entity shows a hit, and whether one is showing now —
// give it to a kind with comp.Const(behavior.HitMark{Duration: 100 * time.Millisecond}).
type HitMark struct {
	Duration      time.Duration
	ExpiresAtNano int64
}

// Active reports whether a hit is showing, as of the last tick the behavior ran.
func (m HitMark) Active() bool { return m.ExpiresAtNano != 0 }

// ShowHits marks an entity that struck something for its own Duration, or d if it sets none.
func ShowHits(d time.Duration) func(plugin.Tick, *HitMark, collision.Struck) {
	return func(t plugin.Tick, m *HitMark, struck collision.Struck) {
		now := t.Now.UnixNano()
		switch {
		case len(struck.Contacts) > 0:
			m.ExpiresAtNano = now + int64(m.lasting(d))
		case m.Active() && now > m.ExpiresAtNano:
			m.ExpiresAtNano = 0
		}
	}
}

// lasting is the entity's own hit duration, or fallback for one that sets none.
func (m HitMark) lasting(fallback time.Duration) time.Duration {
	if m.Duration > 0 {
		return m.Duration
	}
	return fallback
}

// HitOverlay is a Drawing behavior for the world plugin: with is drawn on top of an entity while
// its HitMark is active.
func HitOverlay(with world.Appearance) plugin.Behavior {
	return host.Each[HitMark](func(_ plugin.Tick, m *HitMark, d world.Drawing) {
		if m.Active() {
			d.Overlay(with)
		}
	})
}
