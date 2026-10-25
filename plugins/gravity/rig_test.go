package gravity_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/gravity"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
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
func (c *installCtx) RegSys(factory func() goke.System) goke.Runnable {
	return c.ecs.RegSys(factory())
}
func (c *installCtx) ECS() *goke.ECS                  { return c.ecs }
func (c *installCtx) Hosts(h ...host.Host)            { c.hosts = append(c.hosts, h...) }
func (c *installCtx) Draws(...render.Source)          {}
func (c *installCtx) Looks(func())                    {}
func (*installCtx) Pool() *parallel.Pool              { return nil }
func (c *installCtx) Deliver(rules ...act.Rule) error { return ceremony.Deliver(c.hosts, rules...) }
func (c *installCtx) systems() []goke.System {
	var systems []goke.System
	for _, produce := range c.pending {
		systems = append(systems, produce()...)
	}
	return systems
}

type landed struct{ Hard bool }

func (landed) Command() {}

type heard struct{ command.Queue[landed] }

func (h *heard) Queues() []command.CommandQueue     { return []command.CommandQueue{&h.Queue} }
func (h *heard) DefaultBindings() []control.Binding { return nil }

func (h *heard) given() []command.Issued[landed] {
	var got []command.Issued[landed]
	h.Drain(func(i command.Issued[landed]) { got = append(got, i) })
	return got
}

const hard = 8.0

// a deck over from ≤ x < to, top high and thick, over the ground
type land struct {
	ground               func(p geom.Vec) float32
	from, to, top, thick float32
}

func (l land) Surface(p geom.Vec, _ cell.Domain, feet, step float32) (floor, ceiling float32) {
	floor, ceiling = 0, f32.Inf(1)
	if l.ground != nil {
		floor = l.ground(p)
	}
	if l.top <= 0 || p.X < l.from || p.X >= l.to {
		return floor, ceiling
	}
	if l.top <= feet+step {
		floor = max(floor, l.top)
	}
	if under := l.top - l.thick; l.thick > 0 && under > feet {
		ceiling = under
	}
	return floor, ceiling
}

type body struct {
	x, y, alt float32
	facing    geom.Vec
}

// a world 40 by 40 of cells 10 wide, flat where level draws the board
type scene struct {
	level  []string // '#' solid, the rest open
	land   *land
	leap   float32
	fence  bool
	deck   bool // a raised band with railings in cell (1, 1), the board's own surfaces
	water  bool
	bodies []body
	rules  []act.Rule
	// cut into sectors round the anchor
	sectors world.SectorsCfg
	anchor  geom.Vec
}

type anchor geom.Vec

func (a anchor) Anchors(at func(x, y float32)) { at(float32(a.X), float32(a.Y)) }

type rig struct {
	t     *testing.T
	w     *world.Plugin
	m     *motion.Plugin
	c     *collision.Plugin
	g     *gravity.Plugin
	brd   *board.Plugin
	heard heard
	ecs   *goke.ECS
	ids   []uid.UID64

	q      *goke.Query
	base   goke.Comp[world.Base]
	z      goke.OptComp[world.Z]
	drift  goke.OptComp[motion.Drift]
	pulls  *goke.Query
	pull   goke.Comp[gravity.Pull]
	marks  goke.Comp[tag.Tags[unit.States]]
	steer  goke.Comp[steering.Steering]
	course goke.OptComp[steering.Course]
}

const tps = 60

func newRig(t *testing.T, sc scene) *rig {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 40, Height: 40},
		Entities: world.EntitiesCfg{MaxCount: len(sc.bodies) + 8, MinSize: 1, MaxSize: 10},
		Heights:  true,
		Sectors:  sc.sectors,
	})
	w.Write().Anchor(anchor(sc.anchor))
	r := &rig{t: t, w: w}
	if err := w.Carry(&r.heard); err != nil {
		t.Fatal(err)
	}
	r.m = motion.NewPlugin(w)
	r.c = collision.NewPlugin(w, r.m)
	g := grid.DefaultGrids{}.Square(4, 4, 10)
	r.brd = board.NewPlugin(g, &cell.MultipleOccupancy{}, w).WithMotion(r.m)
	r.brd.Write().Board().SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	for y, line := range sc.level {
		for x, c := range line {
			if c == '#' {
				r.brd.Write().Board().Set(g.At(uint32(x), uint32(y)), cell.Kind{Solid: true})
			}
		}
	}
	if sc.level != nil {
		r.brd.WithCollision(r.c)
	}
	if sc.water {
		for y := range uint32(4) {
			r.brd.Write().Board().Set(g.At(2, y), cell.Kind{Cost: 1, Allows: cell.Water})
		}
		if sc.deck { // the deck runs over water too
			r.brd.Write().Board().Set(g.At(1, 1), cell.Kind{Cost: 1, Allows: cell.Water})
		}
	}
	if sc.fence {
		r.brd.WithCollision(r.c)
		fences := board.NewEntities(r.brd, board.Shape{Size: 10},
			func(c cell.ID) cell.ID { return c })
		north, _ := grid.Direction(g, 0, -1)
		south, _ := grid.Direction(g, 0, 1)
		fences.Define("fence",
			comp.Const(cell.Floor{Allows: cell.Land, Links: 1<<north | 1<<south, Width: 10}),
			comp.Const(cell.Rail{Keeps: cell.Land, Height: 1}))
		w.Write().Seed(fences.Named("fence").Entry(g.At(1, 1)))
	}
	if sc.deck {
		r.brd.WithCollision(r.c)
		decks := board.NewEntities(r.brd, board.Shape{Size: 10, Height: 1},
			func(c cell.ID) cell.ID { return c })
		north, _ := grid.Direction(g, 0, -1)
		south, _ := grid.Direction(g, 0, 1)
		decks.Define("deck",
			comp.Const(cell.Floor{Allows: cell.Land, Links: 1<<north | 1<<south, Width: 10,
				Height: 10, Thickness: 1}),
			comp.Const(cell.Rail{Keeps: cell.Land, Height: 1}))
		w.Write().Seed(decks.Named("deck").Entry(g.At(1, 1)))
	}
	pull := gravity.Pull{Z: -10}
	if sc.level != nil {
		pull = gravity.Pull{Y: 10}
	}
	r.g = gravity.NewPlugin(w, r.m, gravity.Config{Pull: pull})
	if sc.land != nil {
		l := *sc.land
		r.g.WithSurfaces(func() ground.Surfaces { return l })
	}
	if sc.deck { // the board's own surfaces are the topography's: a stub of the deck here
		l := land{from: 10, to: 20, top: 10, thick: 1}
		r.g.WithSurfaces(func() ground.Surfaces { return l })
	}
	if err := w.Carry(r.g); err != nil {
		t.Fatal(err)
	}
	units := board.NewUnits(r.brd, board.Shape{Size: 2, Height: 2},
		func(b body) geom.Vec { return geom.NewVec(b.x, b.y) })
	units.Define("body", unit.Mover{Domain: cell.Land},
		steering.Steering{MaxSpeed: 5},
		comp.Const(gravity.Body{Leap: sc.leap}),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Mass: 1}),
		comp.Load(func(b body) world.Velocity { return world.Velocity{Dir: b.facing} }))
	for _, b := range sc.bodies {
		w.Write().Seed(units.Named("body").Entry(b))
	}
	r.start(sc)
	return r
}

func (r *rig) start(sc scene) {
	t := r.t
	t.Helper()
	if err := r.w.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx := &installCtx{ecs: goke.New()}
	for _, p := range []plugin.Plugin{r.w, r.m, r.c, r.g, r.brd} {
		if err := p.Install(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := ctx.Deliver(sc.rules...); err != nil {
		t.Fatal(err)
	}
	systems := append(ctx.systems(), goke.SystemFn{OnInit: func(si *goke.SysInit) {
		r.q = si.NewQueryBuilder(&r.base, &r.marks, &r.steer).
			Optional(&r.z).Optional(&r.drift).Optional(&r.course).Build()
		r.pulls = si.NewQueryBuilder(&r.pull).Build()
	}})
	r.ecs = ctx.ecs
	r.ecs.Setup(systems...)
	r.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		r.w.RunPlan(rc, d)
		r.m.RunPlan(rc, d)
		r.c.RunPlan(rc, d)
		r.g.RunPlan(rc, d)
		r.brd.RunPlan(rc, d)
		rc.Sync()
		r.w.Write().Clock().Replay(rc, d)
	})
	r.ids = make([]uid.UID64, len(sc.bodies))
	for r.q.All(); r.q.Next(); {
		cur := r.q.Cursor()
		bases, zs := r.base.Slice(cur), r.z.Slice(cur)
		for i, id := range cur.IDs {
			for k, b := range sc.bodies {
				if c := bases[i].Pos.Center(); c.X == b.x && c.Y == b.y {
					r.ids[k] = id
					if zs != nil {
						zs[i].Altitude = b.alt
					}
				}
			}
		}
	}
}

func (r *rig) tick() { r.ecs.Tick(time.Second / tps) }

func (r *rig) leap(k int) { r.w.Write().Carrier().PutFrom(r.ids[k], gravity.Leap{}) }

func (r *rig) walk(k int, speed float32) {
	r.t.Helper()
	if !r.q.Seek(r.ids[k]) {
		r.t.Fatalf("body %d is gone", k)
	}
	cur := r.q.Cursor()
	h := steering.Helm{Steering: r.steer.At(cur), Course: r.course.At(cur)}
	if !h.Steerable() {
		r.t.Fatalf("body %d has no course", k)
	}
	h.Request(r.base.At(cur).Vel.Dir)
	h.RequestSpeed(speed)
}

type state struct {
	at       geom.Vec
	z        world.Z
	drift    motion.Drift
	airborne bool
}

func (r *rig) of(k int) state {
	r.t.Helper()
	if !r.q.Seek(r.ids[k]) {
		r.t.Fatalf("body %d is gone", k)
	}
	cur := r.q.Cursor()
	s := state{at: r.base.At(cur).Pos.Center(), airborne: r.marks.At(cur).Has(unit.Airborne)}
	if z := r.z.At(cur); z != nil {
		s.z = *z
	}
	if d := r.drift.At(cur); d != nil {
		s.drift = *d
	}
	return s
}

func (r *rig) setPull(p gravity.Pull) {
	for r.pulls.All(); r.pulls.Next(); {
		r.pull.Slice(r.pulls.Cursor())[0] = p
	}
}

func awayFrom(from uid.UID64, speed, rise float32) gravity.Away {
	a := gravity.Away{Speed: speed, Rise: rise}
	a.Aim(from)
	return a
}

func landing() act.Rule {
	return act.Branch[gravity.Landing]{
		Name: "landed",
		When: cond.Self.If(cond.Always),
		Then: act.OneOf(
			act.Branch[gravity.Landing]{
				When: cond.Self.If(cond.NewWord("hard", func(l gravity.Landing) bool { return l.Speed > hard })),
				Then: act.Self.Do(landed{Hard: true}),
			},
			act.Self.Do(landed{}),
		),
	}
}
