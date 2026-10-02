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
	"github.com/kjkrol/gram/entity/kind/comp"
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

// module owns the world's topology, entities, and movement — the foundation
// any Stage with moving, drawable entities builds on.
type module struct {
	config Config
	space  *aabbworld.Space

	spawnedCount int
	seeds        []goke.System
	telemetry    Telemetry

	// despawned is this tick's removals.
	despawned map[uid.UID64]struct{}
	items     []aabbworld.Item

	kinds *Kinds

	leavers *plugin.Rules[Leaving]
	movers  *plugin.Rules[Moving]
	drawing render.Rules // the renderer's

	steer            *steering.System
	steeringRunnable goke.Runnable
	velocityRunnable goke.Runnable
	moveRunnable     goke.Runnable
	exitRunnable     goke.Runnable

	// views are refreshed each tick — see Plugin.ViewFor.
	views        []*view.View
	viewRunnable goke.Runnable

	// the tactical clock and the effects, the world's own: the clock's system goes first in the
	// tick, the rules of its moments and then the effects last in every step of the simulation
	clock           *clock.Clock
	moments         moments
	effects         *effect.Effects
	clockRunnable   goke.Runnable
	momentsRunnable goke.Runnable

	// the wires a game defined, their entities and the Signals given to them
	wires         wires
	wiresRunnable goke.Runnable

	// the entities' plans, run first in every step of the simulation
	plans         *steps.Plans
	plansRunnable goke.Runnable

	// commands takes the commands the entities give themselves to the plugins that handle them;
	// despawns are the world's own
	commands control.Carrier
	despawns control.Queue[Despawn]
	applies  control.Queue[Apply]
	dispels  control.Queue[Dispel]
}

var _ goke.Module = (*module)(nil)

// newModule builds the world's topology and spatial index from cfg, its clock and its effect.
func newModule(cfg Config) *module {
	clk := clock.New(cfg.Clock)
	w := &module{config: cfg, space: buildSpace(cfg), despawned: make(map[uid.UID64]struct{}),
		leavers: &plugin.Rules[Leaving]{}, movers: &plugin.Rules[Moving]{},
		clock: clk, steer: steering.NewSystem()}
	w.moments = moments{clock: clk, tick: w.tick, applies: &w.applies, dispels: &w.dispels}
	return w
}

// tick is the Tick of a pass over d of the simulation: the world's carrier, the game time the
// step ends at and the world's seed.
func (w *module) tick(cb *goke.CmdBuf, d time.Duration) plugin.Tick {
	return plugin.Tick{CmdBuf: cb, Now: time.Now(), Dt: d, Commands: &w.commands, Effects: w.effects,
		Time: w.clock.Time() + d, Seed: w.config.Seed, World: w.clock.Entity(), Wires: w.wires.Of, Roles: w.wires.RolesOf}
}

// =================================================================
// goke.Module contract
// =================================================================

// RegSystems registers world's movement systems — see [goke.Module].
func (w *module) RegSystems(ecs *goke.ECS) {
	if w.velocityRunnable != nil {
		return
	}
	for _, name := range rule.RoleNames() { // the roles by name, as a save carries them
		w.kinds.DefineTag[rule.Roles](name)
	}
	w.wiresRunnable = ecs.RegSys(w.wires.system()) // first: the wires' Signals land with the step's effects
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

// RunPlan runs world's tick. At once: the clock's commands and the views of the cameras, which
// move in the tactical pause too. In the simulation, every step: the entities' plans, steering,
// the Moving rules, movement, then the leavers, then the effect. The sync after movement lands
// the Outside marks, so a leaver is dealt with the step it left.
func (w *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(w.clockRunnable, d)
	ctx.Run(w.viewRunnable, d)
	ctx.Sync()
	w.clock.Simulate(ctx, w.simulate)
}

// simulate is one step of the world's simulation.
func (w *module) simulate(ctx goke.RunCtx, step time.Duration) {
	clear(w.despawned)
	ctx.Run(w.plansRunnable, step)
	ctx.Sync()
	ctx.Run(w.steeringRunnable, step)
	ctx.Run(w.velocityRunnable, step)
	ctx.Run(w.moveRunnable, step)
	ctx.Sync()
	ctx.Run(w.exitRunnable, step)
	ctx.Sync()
	ctx.Run(w.wiresRunnable, step)
	ctx.Run(w.momentsRunnable, step)
	ctx.Sync()
	w.effects.Module().RunPlan(ctx, step)
}

// SetupSystems runs every queued Populate call, in call order.
func (w *module) SetupSystems() []goke.System { return w.seeds }

// LoadComps lists the component types world owns — see [goke.CompProvider].
func (w *module) LoadComps() []goke.CompToken {
	tokens := append([]goke.CompToken{
		goke.LoadComp[Base](),
		goke.LoadComp[Appearance](),
		goke.LoadComp[steering.Steering](),
		goke.LoadComp[steering.Course](),
		goke.LoadComp[steering.Pace](),
		goke.LoadComp[Outside](),
		goke.LoadComp[Layers](),
		goke.LoadComp[Z](),
		goke.LoadComp[Eye](),
		goke.LoadComp[steering.Driven](),
		goke.LoadComp[clock.State](),
		goke.LoadComp[tag.Tags[clock.Phase]](),
		goke.LoadComp[rule.Wiring](),
		goke.LoadComp[rule.Wired](),
		goke.LoadComp[tag.Tags[rule.Roles]](),
	}, w.effects.Module().LoadComps()...)
	return append(tokens, w.plans.LoadComps()...)
}

// =================================================================
// plugin.PostLoader contract
// =================================================================

// PostLoad recomputes Count and hands the space every loaded entity.
func (w *module) PostLoad() goke.System {
	return goke.SystemFn{OnInit: func(si *goke.SysInit) {
		w.kinds.r.RemapTypes(si)
		w.kinds.r.RemapTags(si)
		w.telemetry.Count = len(w.reindex(si))
	}}
}

// reindex hands the space every entity there is, and returns them.
func (w *module) reindex(si *goke.SysInit) []aabbworld.Item {
	var base goke.Comp[Base]
	w.items = rebuild(w.space, si.NewQueryBuilder(&base).Build(), &base, w.items)
	return w.items
}

// =================================================================
// world-specific
// =================================================================

// despawn drops id from the ECS, once per tick.
func (w *module) despawn(cb *goke.CmdBuf, id uid.UID64) {
	if _, gone := w.despawned[id]; gone {
		return
	}
	w.despawned[id] = struct{}{}
	cb.RemoveOne(id)
	w.spawnedCount--
	w.telemetry.Count--
}

// populate queues a spawn of one entity of k per row, each row feeding k's Loads.
func (w *module) populate(k ikinds.Kind, rows []any) {
	count := len(rows)
	writers := []comp.Spawner{
		comp.Const(Appearance{SpriteID: k.SpriteID}).Spawner(),
	}
	for _, c := range k.Comps {
		writers = append(writers, c.Spawner())
	}

	w.seeds = append(w.seeds, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		w.reserve(count)
		w.telemetry.Count += count

		var baseComp goke.Comp[Base]
		comps := []goke.Addable{&baseComp}
		for _, wr := range writers {
			comps = append(comps, wr.Columns()...)
		}
		factory := si.NewFactory(comps...)

		factory.Create(count)
		index := 0
		for factory.Next() {
			bases := baseComp.Slice(&factory.Cursor)
			for i, id := range factory.IDs {
				row := rows[index]
				pos := k.Position.Resolve(row)
				w.validateSize(id, pos)
				bases[i] = Base{Pos: pos, Vel: k.Velocity.Resolve(row), TypeID: k.TypeID}
				w.space.Place(&bases[i].Pos.AABB)
				for _, wr := range writers {
					wr.Write(&factory.Cursor, i, row, id)
				}
				index++
			}
		}
		w.reindex(si)
	}})
}

func (w *module) validateSize(id uid.UID64, pos Position) {
	if pos.Size.X < float64(w.config.Entities.MinSize) || pos.Size.X > float64(w.config.Entities.MaxSize) ||
		pos.Size.Y < float64(w.config.Entities.MinSize) || pos.Size.Y > float64(w.config.Entities.MaxSize) {
		panic(fmt.Sprintf("world: entity %d size %vx%v outside declared bounds [%d, %d]",
			id, pos.Size.X, pos.Size.Y, w.config.Entities.MinSize, w.config.Entities.MaxSize))
	}
}

func (w *module) reserve(count int) {
	if w.spawnedCount+count > w.config.Entities.MaxCount {
		panic(fmt.Sprintf("world: spawning %d more would exceed Config.Entities.MaxCount %d (already spawned %d)",
			count, w.config.Entities.MaxCount, w.spawnedCount))
	}
	w.spawnedCount += count
}

// buildSpace is the world's spatial index, its buckets sized to the entities it will hold.
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
