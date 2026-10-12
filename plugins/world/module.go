package world

import (
	"fmt"
	"log"
	"math"
	"math/bits"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/internal/steps"
	"github.com/kjkrol/gram/plugin"
	ikinds "github.com/kjkrol/gram/plugins/world/internal/kinds"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

type module struct {
	config Config
	space  *aabbworld.Space

	spawnedCount int
	seeds        []goke.System
	telemetry    Telemetry

	despawned map[uid.UID64]struct{}
	items     []aabbworld.Item

	kinds *Kinds

	leavers *plugin.Rules[Leaving]
	movers  *plugin.Rules[Moving]
	drawing render.Rules

	steer            *steering.System
	steeringRunnable goke.Runnable
	velocityRunnable goke.Runnable
	moveRunnable     goke.Runnable
	exitRunnable     goke.Runnable

	views        []*view.View
	viewRunnable goke.Runnable

	clock           *clock.Clock
	moments         moments
	effects         *effect.Effects
	clockRunnable   goke.Runnable
	momentsRunnable goke.Runnable

	selves selves

	effectCmds         effectCommands
	effectCmdsRunnable goke.Runnable

	plans         *steps.Plans
	plansRunnable goke.Runnable

	commands control.Carrier
	pauses   control.Queue[Pause]
	fasters  control.Queue[Faster]
	slowers  control.Queue[Slower]
	spawns   control.Queue[Spawn]
	despawns control.Queue[Despawn]

	spawnRunnable goke.Runnable
}

var _ goke.Module = (*module)(nil)

func newModule(cfg Config) *module {
	clk := clock.New(cfg.Clock)
	w := &module{config: cfg, space: buildSpace(cfg), despawned: make(map[uid.UID64]struct{}),
		leavers: &plugin.Rules[Leaving]{}, movers: &plugin.Rules[Moving]{},
		clock: clk, steer: steering.NewSystem()}
	w.moments = moments{clock: clk, tick: w.tick}
	return w
}

func (w *module) tick(cb *goke.CmdBuf, d time.Duration) plugin.Tick {
	return plugin.Tick{CmdBuf: cb, Now: time.Now(), Dt: d, Commands: &w.commands, Effects: w.effects,
		Time: w.clock.Time() + d, Seed: w.config.Seed, World: w.clock.Entity(), Roles: w.effectCmds.RolesOf}
}

func (w *module) RegSystems(ecs *goke.ECS) {
	if w.velocityRunnable != nil {
		return
	}
	// before the clock's, which finds its State on the world's own entity
	ecs.RegSys(w.selves.system())
	// first in a step, so its commands land with the step's effects
	w.effectCmdsRunnable = ecs.RegSys(w.effectCmds.system())
	w.spawnRunnable = ecs.RegSys(newSpawnSystem(w))
	w.steeringRunnable = ecs.RegSys(w.steer)
	velocity := newVelocitySystem(w.movers)
	velocity.tick = w.tick
	w.velocityRunnable = ecs.RegSys(velocity)
	w.moveRunnable = ecs.RegSys(newMoveSystem(w.space))
	w.exitRunnable = ecs.RegSys(newExitSystem(w, w.leavers))
	w.viewRunnable = ecs.RegSys(view.NewSystem(w.space, &w.views, w.config.Space.Width, w.config.Space.Height))
	w.clockRunnable = ecs.RegSys(w.clock.System())
	w.plansRunnable = ecs.RegSys(w.plans.System())
	w.momentsRunnable = ecs.RegSys(w.moments.system())
	w.effects.Module().RegSystems(ecs)
}

func (w *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	w.pauses.Drain(func(control.Issued[Pause]) { w.clock.TogglePause() })
	w.fasters.Drain(func(control.Issued[Faster]) { w.clock.Faster() })
	w.slowers.Drain(func(control.Issued[Slower]) { w.clock.Slower() })
	ctx.Run(w.clockRunnable, d)
	ctx.Run(w.viewRunnable, d)
	ctx.Sync()
	w.clock.Simulate(ctx, w.simulate)
}

func (w *module) simulate(ctx goke.RunCtx, step time.Duration) {
	clear(w.despawned)
	ctx.Run(w.plansRunnable, step)
	ctx.Sync()
	ctx.Run(w.spawnRunnable, step)
	ctx.Run(w.steeringRunnable, step)
	ctx.Run(w.velocityRunnable, step)
	ctx.Run(w.moveRunnable, step)
	// lands the Outside marks, so a leaver is dealt with the step it left
	ctx.Sync()
	ctx.Run(w.exitRunnable, step)
	ctx.Sync()
	ctx.Run(w.effectCmdsRunnable, step)
	ctx.Run(w.momentsRunnable, step)
	ctx.Sync()
	w.effects.Module().RunPlan(ctx, step)
}

func (w *module) SetupSystems() []goke.System { return w.seeds }

func (w *module) LoadComps() []goke.CompToken {
	tokens := append([]goke.CompToken{
		goke.LoadComp[Base](),
		goke.LoadComp[render.Appearance](),
		goke.LoadComp[steering.Steering](),
		goke.LoadComp[steering.Course](),
		goke.LoadComp[steering.Pace](),
		goke.LoadComp[Outside](),
		goke.LoadComp[Layers](),
		goke.LoadComp[Z](),
		goke.LoadComp[Eye](),
		goke.LoadComp[Drift](),
		goke.LoadComp[Lie](),
		goke.LoadComp[Block](),
		goke.LoadComp[steering.Driven](),
		goke.LoadComp[tag.Tags[clock.Phase]](),
		goke.LoadComp[entity.Label](),
		goke.LoadComp[tag.Tags[rule.Roles]](),
	}, w.effects.Module().LoadComps()...)
	tokens = append(tokens, w.selves.tokens()...)
	return append(tokens, w.plans.LoadComps()...)
}

func (w *module) PostLoad() goke.System {
	return goke.SystemFn{OnInit: func(si *goke.SysInit) {
		w.kinds.r.RemapTypes(si)
		w.kinds.r.RemapTags(si)
		w.spawnedCount = len(w.reindex(si))
		w.telemetry.Count = w.spawnedCount
	}}
}

func (w *module) reindex(si *goke.SysInit) []aabbworld.Item {
	var base goke.Comp[Base]
	w.items = rebuild(w.space, si.NewQueryBuilder(&base).Build(), &base, w.items)
	return w.items
}

func (w *module) despawn(cb *goke.CmdBuf, id uid.UID64) {
	if _, gone := w.despawned[id]; gone {
		return
	}
	w.despawned[id] = struct{}{}
	cb.RemoveOne(id)
	w.spawnedCount--
	w.telemetry.Count--
}

func (w *module) populate(k ikinds.Kind, rows []any) { w.populateEntries(k, rows, nil) }

// entries, each row's own, may be nil
func (w *module) populateEntries(k ikinds.Kind, rows []any, entries []kind.Entry) {
	count := len(rows)
	writers := writersOf(k)

	w.seeds = append(w.seeds, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		for i, row := range rows {
			if err := w.sizeOf(k.Position.Resolve(row)); err != nil {
				panic(fmt.Sprintf("world: kind %q, entry %d: %v", k.Name, i, err))
			}
		}
		w.reserve(count)
		w.telemetry.Count += count

		var baseComp goke.Comp[Base]
		var labelComp goke.Comp[entity.Label]
		comps := []goke.Addable{&baseComp, &labelComp}
		for _, wr := range writers {
			comps = append(comps, wr.Columns()...)
		}
		w.spawnRows(si.NewFactory(comps...), &baseComp, &labelComp, writers, k, rows, entries)
		w.reindex(si)
	}})
}

func (w *module) reserve(count int) {
	if w.spawnedCount+count > w.config.Entities.MaxCount {
		panic(fmt.Sprintf("world: spawning %d more would exceed Config.Entities.MaxCount %d (already spawned %d)",
			count, w.config.Entities.MaxCount, w.spawnedCount))
	}
	w.spawnedCount += count
}

func buildSpace(cfg Config) *aabbworld.Space {
	const minCapacity, maxCapacity = 2.0, 8.0

	worldArea := uint64(cfg.Space.Width) * uint64(cfg.Space.Height)
	entityArea := uint64(cfg.Entities.MaxSize) * uint64(cfg.Entities.MaxSize)
	density := float64(uint64(cfg.Entities.MaxCount)*entityArea) / float64(worldArea)

	raw := math.Round(1.0 / math.Sqrt(density))
	capacity := uint32(math.Max(minCapacity, math.Min(maxCapacity, raw)))
	bucketSide := uint32(1) << bits.Len32(cfg.Entities.MaxSize*capacity-1)

	log.Printf("[world] maxEntities=%d, density=%.2f%%, capacity=%d → bucket=%dx%d",
		cfg.Entities.MaxCount, density*100, capacity, bucketSide, bucketSide)

	space, err := aabbworld.NewSpace(aabbworld.Config{
		Width:      cfg.Space.Width,
		Height:     cfg.Space.Height,
		Edges:      cfg.Space.Edges,
		BucketSize: bucketSide,
	})
	if err != nil {
		panic(fmt.Sprintf("world: invalid space configuration: %v", err))
	}
	return space
}
