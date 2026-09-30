package trigger_test

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
)

// installCtx is the least of a plugin.Installer that world and vision need.
type installCtx struct {
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
func (c *installCtx) RegSys(f func() goke.System) goke.Runnable { return c.ecs.RegSys(f()) }
func (c *installCtx) ECS() *goke.ECS                            { return c.ecs }

func heading(v geom.Vec) float64 { return math.Atan2(v.Y, v.X) }
