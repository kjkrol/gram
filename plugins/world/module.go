package world

import (
	"fmt"
	cgoke "github.com/kjkrol/ceremony/goke"
	"log"
	"math"
	"math/bits"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/ceremony/plan"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/entity/kind"
	ikinds "github.com/kjkrol/gram/plugins/world/internal/kinds"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
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

	drawing render.Rules

	// a spawn or a despawn since the space last took every entity
	unindexed     bool
	indexRunnable goke.Runnable

	views        []*view.View
	viewRunnable goke.Runnable

	sectors         *sectors
	sectorsRunnable goke.Runnable

	clock           *clock.Clock
	moments         moments
	effects         *cgoke.Effects
	clockRunnable   goke.Runnable
	momentsRunnable goke.Runnable

	selves selves

	effectCmds         effectCommands
	effectCmdsRunnable goke.Runnable

	plans         *plan.Plans
	plansSys      *cgoke.Plans
	plansRunnable goke.Runnable

	commands command.Carrier
	pauses   command.Queue[Pause]
	fasters  command.Queue[Faster]
	slowers  command.Queue[Slower]
	spawns   command.Queue[Spawn]
	despawns command.Queue[Despawn]

	spawnRunnable   goke.Runnable
	despawnRunnable goke.Runnable
}

var _ goke.Module = (*module)(nil)

func newModule(cfg Config) *module {
	clk := clock.New(cfg.Clock)
	w := &module{config: cfg, space: buildSpace(cfg), despawned: make(map[uid.UID64]struct{}),
		clock: clk}
	w.moments = moments{clock: clk, tick: w.tick}
	w.sectors = newSectors(w)
	return w
}

func (w *module) tick(d time.Duration) host.Tick {
	return host.Tick{Now: time.Now(), Dt: d, Commands: &w.commands, Effects: w.effects,
		Time: w.clock.Time() + d, Seed: w.config.Seed, World: w.clock.Entity(), Roles: w.effectCmds.RolesOf}
}

func (w *module) RegSystems(ecs *goke.ECS) {
	if w.spawnRunnable != nil {
		return
	}
	// before the clock's, which finds its State on the world's own entity
	ecs.RegSys(w.selves.system())
	// first in a step, so its commands land with the step's effects
	w.effectCmdsRunnable = ecs.RegSys(w.effectCmds.system())
	w.spawnRunnable = ecs.RegSys(newSpawnSystem(w))
	w.despawnRunnable = ecs.RegSys(newDespawnSystem(w))
	w.indexRunnable = ecs.RegSys(newIndexSystem(w))
	w.viewRunnable = ecs.RegSys(view.NewSystem(w.space, &w.views, w.config.Space.Width, w.config.Space.Height))
	w.sectorsRunnable = ecs.RegSys(w.sectors)
	w.clockRunnable = ecs.RegSys(w.clock.System())
	w.plansRunnable = ecs.RegSys(w.plansSys)
	w.momentsRunnable = ecs.RegSys(w.moments.system())
	w.effects.Module().RegSystems(ecs)
}

func (w *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	w.pauses.Drain(func(command.Issued[Pause]) { w.clock.TogglePause() })
	w.fasters.Drain(func(command.Issued[Faster]) { w.clock.Faster() })
	w.slowers.Drain(func(command.Issued[Slower]) { w.clock.Slower() })
	ctx.Run(w.clockRunnable, d)
	ctx.Run(w.sectorsRunnable, d)
	ctx.Run(w.viewRunnable, d)
	ctx.Sync()
	w.clock.Simulate(ctx, w.simulate)
}

func (w *module) simulate(ctx goke.RunCtx, step time.Duration) {
	clear(w.despawned)
	ctx.Run(w.plansRunnable, step)
	ctx.Sync()
	ctx.Run(w.spawnRunnable, step)
	ctx.Run(w.despawnRunnable, step)
	ctx.Run(w.effectCmdsRunnable, step)
	ctx.Run(w.momentsRunnable, step)
	// lands the despawns, so the space is indexed without them
	ctx.Sync()
	ctx.Run(w.indexRunnable, step)
	w.effects.Module().RunPlan(ctx, step)
}

func (w *module) SetupSystems() []goke.System { return w.seeds }

func (w *module) LoadComps() []goke.CompToken {
	tokens := append([]goke.CompToken{
		goke.LoadComp[Base](),
		goke.LoadComp[render.Appearance](),
		goke.LoadComp[Layers](),
		goke.LoadComp[Z](),
		goke.LoadComp[Eye](),
		goke.LoadComp[Lie](),
		goke.LoadComp[Shadow](),
		goke.LoadComp[tag.Tags[clock.Phase]](),
		goke.LoadComp[ceremony.Label](),
		goke.LoadComp[tag.Tags[cond.Roles]](),
		goke.LoadComp[sector.Band2](),
		goke.LoadComp[sector.Band3](),
		goke.LoadComp[sector.Band4](),
		goke.LoadComp[sector.Band5](),
		goke.LoadComp[sector.Off](),
	}, w.effects.Module().LoadComps()...)
	tokens = append(tokens, w.selves.tokens()...)
	return append(tokens, w.plansSys.LoadComps()...)
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
	w.unindexed = true
	cb.RemoveOne(id)
	w.spawnedCount--
	w.telemetry.Count--
}

func (w *module) populate(k ikinds.Kind, rows []any) { w.populateEntries(k, rows, nil) }

// entries, each row's own, may be nil
func (w *module) populateEntries(k ikinds.Kind, rows []any, entries []kind.Entry) {
	count := len(rows)
	sp := spawnerOf(k)

	w.seeds = append(w.seeds, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		for i, row := range rows {
			if err := w.sizeOf(k.Position.Resolve(row)); err != nil {
				panic(fmt.Sprintf("world: kind %q, entry %d: %v", k.Name, i, err))
			}
		}
		w.reserve(count)
		w.telemetry.Count += count

		var baseComp goke.Comp[Base]
		var labelComp goke.Comp[ceremony.Label]
		comps := append([]goke.Addable{&baseComp, &labelComp}, sp.Columns()...)
		w.spawnRows(si.NewFactory(comps...), &baseComp, &labelComp, sp, k, rows, entries)
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
