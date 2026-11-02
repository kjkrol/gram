package roads

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/plugins/roads/site"
)

// Read reads the roads in the players' hands and the roads built.
type Read struct{ p *Plugin }

// Read reads the plugin's state; w is the worker's index, 0 the game's own goroutine.
func (p *Plugin) Read(w int) *Read { return &p.read }

// Estimate is the plan of the road in the hand of issuer and its price; false while the hand
// holds no plan.
func (r *Read) Estimate(issuer command.Issuer) (site.Estimate, float32, bool) {
	if h := r.p.handFor(issuer); h != nil && h.planned {
		return h.plan, h.price, true
	}
	return site.Estimate{}, 0, false
}

// Built is every road built since Built was last read, in the order built.
func (r *Read) Built() []Receipt {
	built := r.p.Res.Receipts
	r.p.Res.Receipts = nil
	return built
}

// Levelling reports whether the tool levels the ground under the roads it lays
// (Settings.Levelling).
func (r *Read) Levelling() bool { return r.p.Res.Settings.Levelling }
