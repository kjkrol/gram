package gravity_test

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/internal/hosts"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/gravity"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// installCtx is the plugin.Installer a Stage would hand over, minus the engine.
type installCtx struct {
	hosts   []plugin.Host
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
func (c *installCtx) ECS() *goke.ECS                   { return c.ecs }
func (c *installCtx) Hosts(h ...plugin.Host)           { c.hosts = append(c.hosts, h...) }
func (c *installCtx) Deliver(rules ...rule.Rule) error { return hosts.Deliver(c.hosts, rules...) }
func (c *installCtx) systems() []goke.System {
	var systems []goke.System
	for _, produce := range c.pending {
		systems = append(systems, produce()...)
	}
	return systems
}

// landed is the command a test's Landing rule gives, Hard for a fall faster than hard.
type landed struct{ Hard bool }

func (landed) Command() {}

// heard is where the landed commands land, for the world to carry.
type heard struct{ control.Queue[landed] }

func (h *heard) Queues() []control.CommandQueue     { return []control.CommandQueue{&h.Queue} }
func (h *heard) DefaultBindings() []control.Binding { return nil }

// given is every landed given since the last time.
func (h *heard) given() []control.Issued[landed] {
	var got []control.Issued[landed]
	h.Drain(func(i control.Issued[landed]) { got = append(got, i) })
	return got
}

// hard is how fast a fall the tests' Landing rule calls hard.
const hard = 8.0

// land is the ground of a test: its height at a point and a deck — over from ≤ x < to, top high
// and thick — carrying walkers, read as the topography's relief reads its own.
type land struct {
	ground               func(p geom.Vec) float64
	from, to, top, thick float64
}

func (l land) Surface(p geom.Vec, _ cell.Domain, feet, step float64) (floor, ceiling float64) {
	floor, ceiling = 0, math.Inf(1)
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

// body is a unit of a test: where it stands, at what altitude, the way it faces.
type body struct {
	x, y, alt float64
	facing    geom.Vec
}

// scene is what a rig is built with: a world 40 by 40, a board of cells 10 wide; with heights
// unless the board is drawn.
type scene struct {
	level  []string // a flat world's board, a row a line: '#' solid, the rest open
	land   *land    // nil: no surfaces, the ground at 0
	leap   float64  // how high the bodies leap
	fence  bool     // a railing 1 high along x 20 by the cell at 1, 1, struck by collision
	water  bool     // the cells at 2, y water
	bodies []body
	rules  []rule.Rule
}

// rig is a world of bodies under gravity, ticked by hand: the world, collision, gravity, the board.
type rig struct {
	t     *testing.T
	w     *world.Plugin
	c     *collision.Plugin
	g     *gravity.Plugin
	brd   *board.Plugin
	heard heard
	ecs   *goke.ECS
	ids   []uid.UID64

	q      *goke.Query
	base   goke.Comp[world.Base]
	z      goke.OptComp[world.Z]
	drift  goke.OptComp[world.Drift]
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
		Heights:  sc.level == nil,
	})
	r := &rig{t: t, w: w}
	if err := w.Carry(&r.heard); err != nil {
		t.Fatal(err)
	}
	r.c = collision.NewPlugin(w)
	g := grid.DefaultGrids{}.Square(4, 4, 10)
	r.brd = board.NewPlugin(g, &cell.MultipleOccupancy{}, w)
	r.brd.Board().SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	for y, line := range sc.level {
		for x, c := range line {
			if c == '#' {
				r.brd.Board().Set(g.At(uint32(x), uint32(y)), cell.Kind{Solid: true})
			}
		}
	}
	if sc.level != nil {
		r.brd.WithCollision(r.c)
	}
	if sc.water {
		for y := range uint32(4) {
			r.brd.Board().Set(g.At(2, y), cell.Kind{Cost: 1, Allows: cell.Water})
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
		w.Seed(fences.Named("fence").Entry(g.At(1, 1)))
	}
	pull := gravity.Pull{Z: -10}
	if sc.level != nil {
		pull = gravity.Pull{Y: 10}
	}
	r.g = gravity.NewPlugin(w, gravity.Config{Pull: pull})
	if sc.land != nil {
		l := *sc.land
		r.g.WithSurfaces(func() ground.Surfaces { return l })
	}
	if err := w.Carry(r.g); err != nil {
		t.Fatal(err)
	}
	shape := board.Shape{Size: 2, Height: 2}
	if sc.level != nil {
		shape.Height = 0
	}
	units := board.NewUnits(r.brd, shape,
		func(b body) geom.Vec { return geom.NewVec(b.x, b.y) })
	units.Define("body", unit.Mover{Domain: cell.Land},
		steering.Steering{MaxSpeed: 5},
		comp.Const(gravity.Body{Leap: sc.leap}),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Mass: 1}),
		comp.Load(func(b body) world.Velocity { return world.Velocity{Dir: b.facing} }))
	for _, b := range sc.bodies {
		w.Seed(units.Named("body").Entry(b))
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
	for _, p := range []plugin.Plugin{r.w, r.c, r.g, r.brd} {
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
		r.c.RunPlan(rc, d)
		r.g.RunPlan(rc, d)
		r.brd.RunPlan(rc, d)
		rc.Sync()
		r.w.Clock().Replay(rc, d)
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

// leap has the k-th body give itself a Leap.
func (r *rig) leap(k int) { r.w.Carrier().PutFrom(r.ids[k], gravity.Leap{}) }

// walk has the k-th body walk the way it faces at speed.
func (r *rig) walk(k int, speed float64) {
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

// state is the k-th body now: where its centre is, its Z, its drift and whether it is in the air.
type state struct {
	at       geom.Vec
	z        world.Z
	drift    world.Drift
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

// setPull turns the gravity's pull, as an effect's Alter would.
func (r *rig) setPull(p gravity.Pull) {
	for r.pulls.All(); r.pulls.Next(); {
		r.pull.Slice(r.pulls.Cursor())[0] = p
	}
}

// awayFrom is an Away from the entity from.
func awayFrom(from uid.UID64, speed, rise float64) gravity.Away {
	a := gravity.Away{Speed: speed, Rise: rise}
	a.Aim(from)
	return a
}

// landing is the tests' Landing rule: a landed command for each, Hard past hard.
func landing() rule.Rule {
	return rule.Then[gravity.Landing]("landed", rule.All, rule.OneOf(
		rule.If(
			func(l gravity.Landing) bool { return l.Speed > hard },
			rule.Order(landed{Hard: true}),
		),
		rule.Order(landed{}),
	))
}
