package world_test

import (
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/render"
)

type installCtx struct {
	hosts   []host.Host
	ecs     *goke.ECS
	pending []func() []goke.System
}

func (c *installCtx) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}
func (c *installCtx) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *installCtx) RegSys(factory func() goke.System) goke.Runnable { return c.ecs.RegSys(factory()) }
func (c *installCtx) ECS() *goke.ECS                                  { return c.ecs }

func (c *installCtx) Hosts(h ...host.Host)   { c.hosts = append(c.hosts, h...) }
func (c *installCtx) Draws(...render.Source) {}
func (c *installCtx) Looks(func())           {}

func (c *installCtx) Deliver(rules ...act.Rule) error { return ceremony.Deliver(c.hosts, rules...) }
