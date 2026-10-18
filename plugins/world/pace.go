package world

import (
	"time"

	cgoke "github.com/kjkrol/ceremony/goke"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world/sector"
)

// Pace is how a system goes in the bands of the world's sectors (Config.Sectors), as its policy
// says: a plugin's system lays it Over its query and asks it of every block.
type Pace struct {
	w      *module
	policy sector.Policy
}

var _ cgoke.Pace = (*Pace)(nil)

// Pace is the pace of a system going by policy in the world's bands.
func (p *Plugin) Pace(policy sector.Policy) *Pace { return &Pace{w: p.module, policy: policy} }

// Over adds the bands' marks to a system's query, before it is built, and gives the pace of its
// blocks; a nil Pace, or a world without bands, lets every block go every step.
func (pc *Pace) Over(qb *goke.QueryBuilder) cgoke.Blocks {
	if pc == nil || pc.w.sectors.grid == nil {
		return always{}
	}
	b := &blocks{pace: pc}
	qb.Optional(&b.band2, &b.band3, &b.band4, &b.band5, &b.off)
	return b
}

type always struct{}

func (always) Step(_ *goke.Cursor, dt time.Duration) (time.Duration, bool) { return dt, true }

type blocks struct {
	pace  *Pace
	band2 goke.OptComp[sector.Band2]
	band3 goke.OptComp[sector.Band3]
	band4 goke.OptComp[sector.Band4]
	band5 goke.OptComp[sector.Band5]
	off   goke.OptComp[sector.Off]
}

// the bands farther than the first go all at once, in the last of every Pace steps
func (b *blocks) Step(cur *goke.Cursor, dt time.Duration) (time.Duration, bool) {
	s := b.pace.w.sectors
	band := b.band(cur)
	switch {
	case band == 0:
		return dt, true
	case band == s.grid.Beyond(), b.pace.policy == sector.Near:
		return 0, false
	case b.pace.policy == sector.Everywhere:
		return dt, true
	}
	pace := uint64(s.bands[band].Pace)
	if b.pace.w.clock.Steps()%pace != pace-1 {
		return 0, false
	}
	return dt * time.Duration(pace), true
}

func (b *blocks) band(cur *goke.Cursor) int {
	switch {
	case b.off.Present(cur):
		return b.pace.w.sectors.grid.Beyond()
	case b.band2.Present(cur):
		return 1
	case b.band3.Present(cur):
		return 2
	case b.band4.Present(cur):
		return 3
	case b.band5.Present(cur):
		return 4
	}
	return 0
}
