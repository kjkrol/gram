package world

import (
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world/sector"
)

// Pace is the pace of a system going by policy in the bands of the world's sectors
// (Config.Sectors), an option of its query walked with goke.Query.Paced: the steps numbered by
// the clock, the bands farther than the first all at once in the last of every Pace steps.
func (p *Plugin) Pace(policy sector.Policy) goke.Opt {
	m := p.module
	if m.sectors.grid == nil {
		return goke.PacedBy(m.clock.Read().Steps)
	}
	rules := []goke.PaceRule{goke.Never[sector.Off]()}
	for k, b := range m.config.Sectors.Bands[1:] {
		switch policy {
		case sector.Near:
			rules = append(rules, never(k+1))
		case sector.Paced:
			rules = append(rules, every(k+1, uint64(b.Pace)))
		}
	}
	return goke.PacedBy(m.clock.Read().Steps, rules...)
}

func every(band int, n uint64) goke.PaceRule {
	switch band {
	case 1:
		return goke.Every[sector.Band2](n)
	case 2:
		return goke.Every[sector.Band3](n)
	case 3:
		return goke.Every[sector.Band4](n)
	}
	return goke.Every[sector.Band5](n)
}

func never(band int) goke.PaceRule {
	switch band {
	case 1:
		return goke.Never[sector.Band2]()
	case 2:
		return goke.Never[sector.Band3]()
	case 3:
		return goke.Never[sector.Band4]()
	}
	return goke.Never[sector.Band5]()
}
