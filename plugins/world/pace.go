package world

import (
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/world/sector"
)

// Pace is the pace of a system going at the levels of the world's sectors as paces says, an
// option of its query walked with goke.Query.Paced: the steps numbered by the clock, a level
// going n steps at once in the last of every n, none at L4.
func (p *Plugin) Pace(paces lod.Paces) goke.Opt {
	paces.Check()
	m := p.module
	if m.sectors.grid == nil {
		return goke.PacedBy(m.clock.Read().Steps)
	}
	rules := []goke.PaceRule{goke.Never[sector.L4]()}
	for k := lod.L1; k < lod.L4; k++ {
		switch n := paces[k]; {
		case n == 0:
			rules = append(rules, never(k))
		case n > 1:
			rules = append(rules, every(k, uint64(n)))
		}
	}
	return goke.PacedBy(m.clock.Read().Steps, rules...)
}

func every(level lod.Level, n uint64) goke.PaceRule {
	switch level {
	case lod.L1:
		return goke.Every[sector.L1](n)
	case lod.L2:
		return goke.Every[sector.L2](n)
	}
	return goke.Every[sector.L3](n)
}

func never(level lod.Level) goke.PaceRule {
	switch level {
	case lod.L1:
		return goke.Never[sector.L1]()
	case lod.L2:
		return goke.Never[sector.L2]()
	}
	return goke.Never[sector.L3]()
}
