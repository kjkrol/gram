package main

import (
	"log"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
)

// probe logs, every probeEvery engine ticks, how many entities moved their box since the tick
// before; its play (zz_probe_script.go) presses the keys and puts the commands of its demo
type probe struct {
	*world.Self
	a    *arena
	tick int
	q    *goke.Query
	base goke.Comp[world.Base]
	last []geom.Vec
	seen []bool
}

const probeEvery = 60

func newProbe(a *arena) *probe {
	p := &probe{a: a}
	p.Self = world.NewSelf(a.world, p)
	return p
}

func (p *probe) Name() string                       { return "probe" }
func (p *probe) EventHandler() control.EventHandler { return nil }
func (p *probe) Serializable() plugin.Serializable  { return nil }

func (p *probe) Install(ctx plugin.Installer) error {
	ctx.RegSys(func() goke.System {
		return goke.SystemFn{OnInit: func(si *goke.SysInit) {
			p.q = si.NewQueryBuilder(&p.base).Build()
		}}
	})
	return nil
}

func (p *probe) RunPlan(ctx goke.RunCtx, _ time.Duration) {
	ctx.Sync()
	p.tick++
	p.play()
	switch {
	case p.tick%probeEvery == 0:
		p.snapshot()
	case p.tick%probeEvery == 1 && p.tick > 1:
		p.moved()
	}
}

func (p *probe) snapshot() {
	for i := range p.seen {
		p.seen[i] = false
	}
	for p.q.All(); p.q.Next(); {
		cur := p.q.Cursor()
		for i, b := range p.base.Slice(cur) {
			k := int(cur.IDs[i].Index())
			for k >= len(p.last) {
				p.last = append(p.last, geom.Vec{})
				p.seen = append(p.seen, false)
			}
			p.last[k], p.seen[k] = b.Pos.TopLeft, true
		}
	}
}

func (p *probe) moved() {
	n, moved := 0, 0
	for p.q.All(); p.q.Next(); {
		cur := p.q.Cursor()
		for i, b := range p.base.Slice(cur) {
			k := int(cur.IDs[i].Index())
			if k >= len(p.seen) || !p.seen[k] {
				continue
			}
			n++
			if p.last[k] != b.Pos.TopLeft {
				moved++
			}
		}
	}
	log.Printf("P:t%d moved %d of %d", p.tick, moved, n)
}
