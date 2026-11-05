package navigation

import (
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/uid"
)

const (
	armyKind    = "army"
	soldierKind = "soldier"
	armyRole    = "army"
	soldierRole = "soldier"
	// how long the army's sector lies at L4 before it folds
	foldAfter = 3 * time.Second
	frame     = time.Second / 60
)

type oneStageGame struct{ stage game.Stage }

func (oneStageGame) Props() game.Props { return game.Props{} }

func (g oneStageGame) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{g.stage.Name(): g.stage}, g.stage.Name()
}

type armyRow struct {
	at, to cell.ID
}

// a soldier's place off the army's centre, across its way
type soldierRow struct{ dy float32 }

// an army, an aggregate sent along a board of cols by rows cells cut into sectors as the
// abstract rig's, unfolding into three soldiers as a camera comes near and folding as it goes
type handover struct {
	eng    *engine.Engine
	ecs    *goke.ECS
	w      *world.Plugin
	nav    *Plugin
	g      grid.Grid
	seen   *observed
	occ    cell.Occupancy
	update func(goke.RunCtx, time.Duration)

	q      *goke.Query
	base   goke.Comp[world.Base]
	orders goke.OptComp[MoveOrder]
	partOf goke.OptComp[world.PartOf]
	whole  goke.OptComp[world.Aggregate]
	mark   goke.OptComp[sector.Abstract]
	open   goke.OptComp[world.Unfolded]
	at     goke.OptComp[unit.At]
}

// newHandover is the handover over a board all land but where lay changes it
func newHandover(t *testing.T, cols, rows uint32, from, to cell.ID,
	lay ...func(b *board.Plugin)) *handover {
	t.Helper()
	h := &handover{g: grid.DefaultGrids{}.Square(cols, rows, legCellSize),
		seen: &observed{w: cols * legCellSize, h: rows * legCellSize}}
	st := stage.New("handover").
		World(world.Config{
			Space:    world.SpaceCfg{Width: cols * legCellSize, Height: rows * legCellSize},
			Entities: world.EntitiesCfg{MaxCount: 16, MinSize: 1, MaxSize: legCellSize},
			Heights:  true,
			Sectors:  world.SectorsCfg{Buckets: 2},
			LOD:      lod.Ladder{Scales: abstractLadder.Scales, Fold: float32(foldAfter.Seconds())},
		}).
		Plugins(func(w *world.Plugin) {
			h.w = w
			m := motion.NewPlugin(w)
			brd := board.NewPlugin(h.g, &cell.SingleOccupancy{}, w).
				WithMotion(m, cell.DefaultHiking)
			brd.Write().Board().SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
			h.occ = brd.Write().Occupancy()
			for _, l := range lay {
				l(brd)
			}
			sel := selection.NewPlugin(w)
			h.nav = NewPlugin(brd, w, m, sel, driving.NewPlugin(w, m, sel)).WithSpacing(CellSpacing)
			w.Write().Observe(h.seen)
			h.update = func(rc goke.RunCtx, d time.Duration) {
				w.RunPlan(rc, d)
				m.RunPlan(rc, d)
				brd.RunPlan(rc, d)
				h.nav.RunPlan(rc, d)
				rc.Sync()
			}
		}).
		Effects(func(ctx game.Initializer) {
			h.ecs = ctx.ECS()
			ctx.Setup(h)
		}).
		Rules(func() { h.rules() }).
		Kinds(func() { h.kinds() }).
		Spawn(func() {
			h.w.Write().Seed(kind.Named[armyRow](h.w.Kinds(), armyKind).Entry(armyRow{from, to}))
		}).
		Update(func(rc goke.RunCtx, d time.Duration) { h.update(rc, d) })
	h.eng = engine.NewEngine(oneStageGame{stage: st})
	if err := h.eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return h
}

func (h *handover) SetupSystems() []goke.System {
	return []goke.System{asKinds(h.nav), goke.SystemFn{OnInit: func(si *goke.SysInit) {
		h.q = si.NewQueryBuilder(&h.base).
			Optional(&h.orders, &h.partOf, &h.whole, &h.mark, &h.open, &h.at).Build()
	}}}
}

// the army unfolds into its soldiers, each taking its order over as it is made, and takes back
// the order of one of them as it folds; a soldier folds in by going
func (h *handover) rules() {
	var parts []act.Step
	cell := float32(legCellSize)
	for _, dy := range []float32{-cell, 0, cell} {
		e := kind.EntryOf(soldierKind, soldierRow{dy: dy}).Told(Inherit{})
		parts = append(parts, act.Self.Do(world.SpawnPart{Entry: e}))
	}
	h.w.Roles().Define(armyRole,
		act.Branch[world.Unfold]{
			Name: "unfold",
			When: cond.Self.If(cond.Always),
			Then: act.Steps(parts...),
		},
		act.Branch[world.Fold]{
			Name: "fold",
			When: cond.Self.If(cond.Always),
			Then: act.Self.Do(Inherit{}),
		})
	h.w.Roles().Define(soldierRole, act.Branch[world.Folding]{
		Name: "fold in",
		When: cond.Self.If(cond.Always),
		Then: act.Self.Do(world.Despawn{}),
	})
}

func (h *handover) kinds() {
	size := float32(legEntitySize)
	profile := steering.Steering{MaxSpeed: 8 * float32(legCellSize)}
	roles := h.w.Roles()
	kind.Define[armyRow](h.w.Kinds(), armyKind, kind.Spec{
		comp.Load(func(r armyRow) world.Position {
			return world.Position{AABB: cellBox(h.g, r.at, legEntitySize)}
		}),
		comp.Const(world.Velocity{}),
		comp.Load(func(r armyRow) unit.At { return unit.At{Cell: r.at} }),
		comp.Const(profile),
		comp.Load(func(r armyRow) MoveOrder { return MoveOrder{Target: r.to} }),
		comp.Const(world.Aggregate{}),
		ceremony.Plays(roles.Named(armyRole)),
	})
	kind.Define[soldierRow](h.w.Kinds(), soldierKind, kind.Spec{
		comp.Load(func(r soldierRow) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(-size/2, r.dy-size/2), size,
				size)}
		}),
		comp.Const(world.Velocity{}),
		comp.Const(unit.At{}),
		comp.Const(profile),
		ceremony.Plays(roles.Named(soldierRole)),
	})
}

func (h *handover) run(ticks int) {
	for range ticks {
		h.ecs.Tick(frame)
	}
}

// one taking part: the army or a soldier, its order, nil with none, its box's middle, whether it
// travels the abstract graph, whether it is Unfolded and its cell
type taking struct {
	id       uid.UID64
	order    *MoveOrder
	at       geom.Vec
	marked   bool
	unfolded bool
	cell     cell.ID
}

// the army and its soldiers, by id, as they stand
func (h *handover) cast() (army taking, soldiers []taking) {
	for h.q.All(); h.q.Next(); {
		cur := h.q.Cursor()
		bases, orders := h.base.Slice(cur), h.orders.Slice(cur)
		for i, id := range cur.IDs {
			one := taking{id: id, at: bases[i].Pos.Center(), marked: h.mark.Present(cur),
				unfolded: h.open.Present(cur)}
			if at := h.at.At(cur); at != nil {
				one.cell = at.Cell
			}
			if orders != nil {
				o := orders[i]
				one.order = &o
			}
			switch {
			case h.whole.Present(cur):
				army = one
			case h.partOf.Present(cur):
				soldiers = append(soldiers, one)
			}
		}
	}
	return army, soldiers
}

// walksOn says whether b's route is a's walked on from where a came, a step further at most
func walksOn(a, b Path) bool {
	return a.Length == b.Length && b.Index >= a.Index && b.Index <= a.Index+1 &&
		slices.Equal(a.Steps[b.Index:a.Length], b.Steps[b.Index:b.Length])
}

// takesOver says whether b is a's order taken over: its goals, group and aim, its route walked on
func takesOver(a, b *MoveOrder) bool {
	return b.Target == a.Target && b.Level == a.Level && b.Spot == a.Spot &&
		b.Waypoints == a.Waypoints && b.Queued == a.Queued && b.Group == a.Group &&
		b.Party == a.Party && b.Aim == a.Aim && b.Aimed == a.Aimed && walksOn(a.Path, b.Path)
}

func TestInherit_HandsTheRouteOverAtAFoldAndAnUnfold(t *testing.T) {
	const cols, rows = 192, 16
	g := grid.DefaultGrids{}.Square(cols, rows, legCellSize)
	goal := g.At(186, 8)
	h := newHandover(t, cols, rows, g.At(2, 8), goal)

	var army taking
	for range 10 * 60 {
		h.run(1)
		if army, _ = h.cast(); army.order != nil && army.marked && army.order.Path.Index > 1 {
			break
		}
	}
	if army.order == nil || !army.marked || army.order.Path.Index <= 1 {
		t.Fatalf("the army, far, is not along its route: %+v", army)
	}

	h.seen.look(army.at.X, army.at.Y, 1)
	took := map[uid.UID64]bool{}
	for range 60 {
		h.run(1)
		army, soldiers := h.cast()
		for _, s := range soldiers {
			if s.order == nil || took[s.id] {
				continue
			}
			took[s.id] = true
			if !takesOver(army.order, s.order) {
				t.Errorf("unfolded, soldier %d took over %v along %+v;\nwant the army's %v along "+
					"%+v", s.id, s.order.Target, s.order.Path, army.order.Target, army.order.Path)
			}
			if c, _ := h.g.CellAt(s.at); order.At(&s.order.Path) != c {
				t.Errorf("unfolded on %v, soldier %d took over a route standing at %v; want it "+
					"walked on from its own cell", c, s.id, order.At(&s.order.Path))
			}
		}
		if len(took) == 3 {
			break
		}
	}
	if len(took) != 3 {
		t.Fatalf("unfolded, %d soldiers took the army's order over, want 3", len(took))
	}
	unfolded, soldiers := h.cast()
	if unfolded.order == nil || unfolded.order.Path.Index <= 1 {
		t.Fatalf("unfolded, the army's order is %+v, want it kept along its route",
			unfolded.order)
	}
	h.run(2 * 60)
	stood, walked := h.cast()
	if moved := stood.at.X - unfolded.at.X; moved > float32(legCellSize) {
		t.Errorf("unfolded, the army went %v on, want it standing for its soldiers", moved)
	}
	along := false
	for _, s := range soldiers {
		for _, w := range walked {
			if w.id == s.id && w.at.X-s.at.X < 4*float32(legCellSize) {
				t.Errorf("unfolded, soldier %d went from %v to %v, want it on its way", s.id, s.at,
					w.at)
			}
			along = along || w.id == s.id && w.order != nil &&
				w.order.Path.Steps == unfolded.order.Path.Steps &&
				w.order.Path.Index > unfolded.order.Path.Index+4
		}
	}
	// the others come round the one ahead of them on its first step
	if !along {
		t.Error("unfolded, no soldier walks the army's route on")
	}

	h.seen.cams = nil
	// the fold is told the step before the soldiers go: the army then, its soldiers before it
	var folded taking
	var before []taking
	_, last := h.cast()
	gone := false
	for range 2 * int(foldAfter/frame) {
		h.run(1)
		army, soldiers := h.cast()
		if gone = len(soldiers) == 0; gone {
			break
		}
		before, last, folded = last, soldiers, army
	}
	if !gone || before == nil {
		t.Fatalf("the soldiers folded in %v, %d steps watched before; want them gone after the "+
			"fold", gone, len(before))
	}
	var first *taking
	for k, s := range before {
		if s.order != nil && (first == nil || s.id < first.id) {
			first = &before[k]
		}
	}
	if first == nil || first.order.Path.Length == 0 {
		t.Fatalf("folding, the first soldier with an order is %+v, want one along a route", first)
	}
	if o, of := folded.order, first.order; o == nil || o.Target != goal || !takesOver(of, o) {
		t.Fatalf("folded, the army took over %+v;\nwant soldier %d's to %v along %+v", o,
			first.id, goal, of.Path)
	}
	h.run(30)
	if army, _ := h.cast(); army.order == nil || army.order.Target != goal ||
		army.at.X-folded.at.X < float32(legCellSize) {
		t.Errorf("folded, the army went from %v to %v under %+v; want it on to %v", folded.at,
			army.at, army.order, goal)
	}
}

func TestHandover_AnAggregateComesDownAndStandsTillItHasFolded(t *testing.T) {
	const cols, rows = 64, 16
	g := grid.DefaultGrids{}.Square(cols, rows, legCellSize)
	goal := g.At(40, 8)
	// a wall across the first sector but for its top row
	h := newHandover(t, cols, rows, g.At(2, 8), goal, func(b *board.Plugin) {
		for x := uint32(6); x < 10; x++ {
			for y := uint32(1); y < rows; y++ {
				b.Write().Board().Set(g.At(x, y), wall)
			}
		}
	})
	var army taking
	for range 60 {
		h.run(1)
		if army, _ = h.cast(); army.marked && army.order != nil && army.order.Path.Length > 0 {
			break
		}
	}
	if !army.marked || army.order == nil || army.order.Path.Length == 0 {
		t.Fatalf("the army is not marked on a route: %+v", army)
	}
	route := army.order.Path
	// put in the wall, it comes down onto the nearest cell taking it
	in := g.CellCenter(g.At(7, 8))
	half := float32(legEntitySize) / 2
	h.q.Seek(army.id)
	h.base.At(h.q.Cursor()).Pos.AABB = plane.NewAABB(in.Sub(geom.NewVec(half, half)), 2*half,
		2*half)

	h.seen.look(in.X, in.Y, 1)
	h.run(2)
	down, _ := h.cast()
	if c := g.CellCenter(down.cell); down.marked || !down.unfolded ||
		f32.Abs(c.X-down.at.X) > 1 || f32.Abs(c.Y-down.at.Y) > 1 ||
		h.nav.board.Kind(down.cell) == wall {
		t.Fatalf("unfolded at %v on cell %v, marked %v, Unfolded %v; want it unmarked on the "+
			"middle of a cell out of the wall", down.at, down.cell, down.marked, down.unfolded)
	}
	if c := h.nav.Read(0).Counts(); c.Snaps != 1 {
		t.Errorf("%d snaps, want the one out of the wall", c.Snaps)
	}
	if down.order == nil || down.order.Path.Steps != route.Steps ||
		down.order.Path.Length != route.Length {
		t.Errorf("unfolded under %+v, want its route %v kept for its soldiers", down.order,
			route.Steps[:route.Length])
	}

	h.run(60)
	if stood, _ := h.cast(); stood.at != down.at {
		t.Errorf("unfolded, the army went from %v to %v, want it standing", down.at, stood.at)
	}
	g.EachCell(func(c cell.ID) {
		h.occ.Holders(c, func(e uid.UID64, _ cell.Domain, _ bool) {
			if e == down.id {
				t.Errorf("unfolded, the army holds cell %v, want none", c)
			}
		})
	})
	h.seen.cams = nil
	h.run(int((foldAfter - foldAfter/4) / frame))
	if stood, _ := h.cast(); stood.at != down.at || stood.marked || !stood.unfolded {
		t.Errorf("at L4, its soldiers yet to fold, the army went from %v to %v, marked %v, "+
			"Unfolded %v; want it standing, unmarked, Unfolded", down.at, stood.at, stood.marked,
			stood.unfolded)
	}
	gone := false
	for range 2 * int(foldAfter/frame) {
		h.run(1)
		if _, soldiers := h.cast(); len(soldiers) == 0 {
			gone = true
			break
		}
	}
	if !gone {
		t.Fatal("the soldiers never folded")
	}
	folded, _ := h.cast()
	h.run(30)
	if on, _ := h.cast(); !on.marked || on.unfolded || on.order == nil ||
		f32.Hypot(on.at.X-folded.at.X, on.at.Y-folded.at.Y) < float32(legCellSize) {
		t.Errorf("folded at %v, the army went to %v, marked %v, Unfolded %v, under %+v; want it "+
			"marked on its way", folded.at, on.at, on.marked, on.unfolded, on.order)
	}
}
