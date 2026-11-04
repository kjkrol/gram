package world

import (
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/world/sector"
)

// Pace is the pace of a system going as paces says in the world's sectors, an option of its query
// walked with goke.Query.Paced: the steps numbered by the clock, a sector k levels coarser than
// the finest any observer sees going paces[k] steps at once in the last of every paces[k], none
// at L4.
func (p *Plugin) Pace(paces lod.Paces) goke.Opt {
	paces.Check()
	m := p.module
	if m.sectors.grid == nil {
		return goke.PacedBy(m.clock.Read().Steps)
	}
	rules := []goke.PaceRule{goke.Never[sector.L4]()}
	for far := 1; far < len(paces)-1; far++ {
		switch n := paces[far]; {
		case n == 0:
			rules = append(rules, never(far))
		case n > 1:
			rules = append(rules, every(far, uint64(n)))
		}
	}
	return goke.PacedBy(m.clock.Read().Steps, rules...)
}

func every(far int, n uint64) goke.PaceRule {
	switch far {
	case 1:
		return goke.Every[sector.Far1](n)
	case 2:
		return goke.Every[sector.Far2](n)
	}
	return goke.Every[sector.Far3](n)
}

func never(far int) goke.PaceRule {
	switch far {
	case 1:
		return goke.Never[sector.Far1]()
	case 2:
		return goke.Never[sector.Far2]()
	}
	return goke.Never[sector.Far3]()
}
