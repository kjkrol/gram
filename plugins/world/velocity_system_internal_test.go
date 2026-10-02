package world

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
)

// scaling is a Moving hook scaling every entity's speed by a fixed factor.
func scaling(factor float64) plugin.Rule {
	return host.Every(func(_ plugin.Tick, m Moving) { m.Base.Vel.Value *= factor })
}

func TestVelocitySystem_Update_RunsTheMovingRulesInOrder(t *testing.T) {
	ecs := goke.New()
	var baseComp goke.Comp[Base]
	var q *goke.Query
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&baseComp)
		f.Create(1)
		f.Next()
		baseComp.Slice(&f.Cursor)[0].Vel = Velocity{Dir: geom.NewVec(1, 0), Value: 100}
		q = si.NewQueryBuilder(&baseComp).Build()
	}})

	host := &host.EachHost[Moving]{}
	for _, b := range []plugin.Rule{scaling(0.5), scaling(0.25)} {
		if err := host.Add(b); err != nil {
			t.Fatal(err)
		}
	}
	sys := newVelocitySystem(host)
	handle := ecs.RegSys(sys)
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(handle, d)
		ctx.Sync()
	})

	ecs.Tick(time.Second)

	q.All()
	for q.Next() {
		bases := baseComp.Slice(q.Cursor())
		if len(bases) == 0 {
			continue
		}
		if math.Abs(bases[0].Vel.Value-12.5) > 1e-9 {
			t.Errorf("Velocity.Value = %v, want 12.5 — each rule scales what the one before left", bases[0].Vel.Value)
		}
		return
	}
	t.Fatal("expected to find the seeded entity")
}
