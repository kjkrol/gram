package world

import (
	"fmt"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/world/sector"
)

// Pace is the pace of a system going as paces says in the world's sectors, an option of its query
// walked with goke.Query.Paced: the steps numbered by the clock, a sector k levels coarser than
// the finest any observer sees going paces[k] steps at once in the last of every paces[k], none
// at Omitted.
func (p *Plugin) Pace(paces lod.Paces) goke.Opt {
	paces.Check()
	m := p.module
	if m.sectors.grid == nil {
		return goke.PacedBy(m.clock.Read().Steps)
	}
	rules := []goke.PaceRule{goke.Never[sector.L6]()}
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
	case 3:
		return goke.Every[sector.Far3](n)
	case 4:
		return goke.Every[sector.Far4](n)
	}
	return goke.Every[sector.Far5](n)
}

func never(far int) goke.PaceRule {
	switch far {
	case 1:
		return goke.Never[sector.Far1]()
	case 2:
		return goke.Never[sector.Far2]()
	case 3:
		return goke.Never[sector.Far3]()
	case 4:
		return goke.Never[sector.Far4]()
	}
	return goke.Never[sector.Far5]()
}

// Within keeps a system's query to the entities in sectors of the scale beginning at first
// (lod.Close, Standard or Strategic), its two levels; a world of no sectors lies at Standard.
// A system of a scale's detail joins it with Pace(lod.Exact), going where someone looks so close.
func (p *Plugin) Within(first lod.Level) goke.Opt {
	if first != lod.Close && first != lod.Standard && first != lod.Strategic {
		panic(fmt.Sprintf("world: within the scale of level %d, want Close, Standard or Strategic",
			first))
	}
	switch {
	case p.module.sectors.grid != nil:
		return joined(excludeMarks(first, first+1)...)
	case first == lod.Standard:
		return joined[goke.Opt]()
	}
	return goke.Include[sector.L0]()
}

// joined is one option of a query that is every one of opts
func joined[O ~func(S, D) error, S, D any](opts ...O) O {
	return func(s S, d D) error {
		for _, o := range opts {
			if err := o(s, d); err != nil {
				return err
			}
		}
		return nil
	}
}
