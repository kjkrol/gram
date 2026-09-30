package world

import (
	"fmt"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/world/act"
	"github.com/kjkrol/gram/plugins/world/act/effect"
	"github.com/kjkrol/gram/plugins/world/clock"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/uid"
)

// module owns the world's topology, entities, and movement — the foundation
// any Stage with moving, drawable entities builds on.
type module struct {
	config Config
	space  *aabbworld.Space
	ecs    *goke.ECS

	// declared is what the game said it attaches at runtime — see Plugin.Declare.
	declared []goke.CompToken

	spawnedCount int
	seeds        []goke.System
	telemetry    Telemetry

	// despawned is this tick's removals.
	despawned map[uid.UID64]struct{}
	items     []aabbworld.Item

	kinds *Kinds

	behaviors         []Behavior
	behaviorRunnables []goke.Runnable
	leavers           *host.EachHost[Leaving]
	movers            *host.EachHost[Moving]
	drawers           *host.EachHost[Drawing]

	steeringRunnable goke.Runnable
	velocityRunnable goke.Runnable
	moveRunnable     goke.Runnable
	exitRunnable     goke.Runnable

	// views are refreshed each tick — see Plugin.NewView.
	views        []*view.View
	viewRunnable goke.Runnable

	// the tactical clock and the effects, the world's own: the clock's system goes first in the
	// tick, the effects last in every step of the simulation
	clock         *clock.Clock
	effects       *effect.Effects
	clockRunnable goke.Runnable

	// the entities' trees, run in every step of the simulation after the behaviors
	trees         *act.Trees
	treesRunnable goke.Runnable

	// commands takes the commands the entities give themselves to the plugins that handle them;
	// despawns are the world's own
	commands control.Carrier
	despawns control.Queue[Despawn]
}

var _ goke.Module = (*module)(nil)

// newModule builds the world's topology and spatial index from cfg, its clock and its effect.
func newModule(cfg Config) *module {
	clk := clock.New(cfg.Clock)
	w := &module{config: cfg, space: buildSpace(cfg), despawned: make(map[uid.UID64]struct{}),
		leavers: &host.EachHost[Leaving]{}, movers: &host.EachHost[Moving]{}, drawers: &host.EachHost[Drawing]{},
		clock: clk}
	w.effects = effect.New(clk, &w.commands)
	w.trees = act.New(clk.Time, w.effects, &w.commands)
	return w
}

// =================================================================
// goke.Module contract
// =================================================================

// RegSystems registers world's movement systems — see [goke.Module].
func (w *module) RegSystems(ecs *goke.ECS) {
	if w.velocityRunnable != nil {
		return
	}
	for _, b := range w.behaviors {
		w.behaviorRunnables = append(w.behaviorRunnables, ecs.RegSys(b))
	}
	w.steeringRunnable = ecs.RegSys(steering.NewSystem())
	velocity := NewVelocitySystem(w.movers)
	velocity.commands = &w.commands
	w.velocityRunnable = ecs.RegSys(velocity)
	w.moveRunnable = ecs.RegSys(NewMoveSystem(w.space))
	w.exitRunnable = ecs.RegSys(newExitSystem(w, w.leavers))
	w.viewRunnable = ecs.RegSys(view.NewSystem(w.space, &w.views, w.config.Space.Width, w.config.Space.Height))
	w.clockRunnable = ecs.RegSys(w.clock.System())
	w.treesRunnable = ecs.RegSys(w.trees.System())
	w.effects.Module().RegSystems(ecs)
}

// RunPlan runs world's tick. At once: the clock's commands and the views of the cameras, which
// move in the tactical pause too. In the simulation, every step: the decisions — the behaviors,
// then the entities' trees — steering, the Moving triggers, movement, then the leavers, then the
// effect. The sync after movement lands
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
	for _, b := range w.behaviorRunnables {
		ctx.Run(b, step)
		ctx.Sync()
	}
	ctx.Run(w.treesRunnable, step)
	ctx.Sync()
	ctx.Run(w.steeringRunnable, step)
	ctx.Run(w.velocityRunnable, step)
	ctx.Run(w.moveRunnable, step)
	ctx.Sync()
	ctx.Run(w.exitRunnable, step)
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
		goke.LoadComp[Outside](),
		goke.LoadComp[Layers](),
		goke.LoadComp[Z](),
		goke.LoadComp[Eye](),
		goke.LoadComp[steering.Driven](),
		goke.LoadComp[clock.State](),
		goke.LoadComp[tag.Tags[clock.Phase]](),
	}, w.effects.Module().LoadComps()...)
	tokens = append(tokens, w.trees.LoadComps()...)
	return append(tokens, w.declared...)
}

// =================================================================
// plugin.PostLoader contract
// =================================================================

// PostLoad recomputes Count and hands the space every loaded entity.
func (w *module) PostLoad() goke.System {
	return goke.SystemFn{OnInit: func(si *goke.SysInit) {
		w.remapTypes(si)
		w.kinds.remapTags(si)
		w.telemetry.Count = len(w.reindex(si))
	}}
}

// reindex hands the space every entity there is, and returns them.
func (w *module) reindex(si *goke.SysInit) []aabbworld.Item {
	var base goke.Comp[Base]
	w.items = rebuild(w.space, si.NewQueryBuilder(&base).Build(), &base, w.items)
	return w.items
}

// remapTypes rewrites every loaded Base.TypeID from the saved kind order to this build's.
func (w *module) remapTypes(si *goke.SysInit) {
	saved := w.kinds.saved

	lut := make([]kind.ID, len(saved))
	moved := false
	for old, name := range saved {
		k, ok := w.kinds.entries[name]
		if !ok {
			panic(fmt.Sprintf("world: the save names kind %q, which this build no longer defines", name))
		}
		lut[old] = k.typeID
		moved = moved || k.typeID != kind.ID(old)
	}
	if !moved {
		return
	}

	var base goke.Comp[Base]
	query := si.NewQueryBuilder(&base).Build()
	query.All()
	for query.Next() {
		cursor := query.Cursor()
		bases := base.Slice(cursor)
		for i := range cursor.IDs {
			if int(bases[i].TypeID) >= len(lut) {
				panic(fmt.Sprintf("world: loaded entity carries TypeID %d, beyond the %d the save named", bases[i].TypeID, len(lut)))
			}
			bases[i].TypeID = lut[bases[i].TypeID]
		}
	}
}

// =================================================================
// world-specific
// =================================================================

// Hook adds b to the decision pass that runs before movement.
func (w *module) Hook(b Behavior) { w.behaviors = append(w.behaviors, b) }

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
func (w *module) populate(k registered, rows []any) {
	count := len(rows)
	writers := []comp.Spawner{
		comp.Const(Appearance{SpriteID: k.spriteID}).Spawner(),
	}
	for _, c := range k.comps {
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
				pos := k.position.Resolve(row, id)
				w.validateSize(id, pos)
				bases[i] = Base{Pos: pos, Vel: k.velocity.Resolve(row, id), TypeID: k.typeID}
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
