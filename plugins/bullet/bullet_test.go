package bullet_test

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/internal/hosts"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/bullet"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// installCtx is the plugin.Installer a Stage would hand over, minus the engine.
type installCtx struct {
	hosts   []plugin.Host // of the rules of the moments the plugins installed catch
	ecs     *goke.ECS
	pending []func() []goke.System
	tracked []any
}

func (c *installCtx) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.tracked = append(c.tracked, m)
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}
func (c *installCtx) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.tracked = append(c.tracked, p)
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *installCtx) RegSys(factory func() goke.System) goke.Runnable { return c.ecs.RegSys(factory()) }
func (c *installCtx) ECS() *goke.ECS                                  { return c.ecs }
func (c *installCtx) systems() []goke.System {
	var systems []goke.System
	for _, produce := range c.pending {
		systems = append(systems, produce()...)
	}
	return systems
}

// family is the tag family the tests mark their shots by.
type family struct{}

// piece is one unit of a test: where it stands, how big it is, the way it faces and how fast it
// walks, whether a player may select it and has, whose it is, and with heights its Z and Eye.
type piece struct {
	x, y, size float64
	facing     geom.Vec
	speed      float64
	selectable bool
	selected   bool
	owner      control.PlayerID
	z          world.Z
	eye        *world.Eye
}

// scene is what a rig is built with: a closed world of 400 by 400 unless said otherwise.
type scene struct {
	width, height uint32
	edges         aabbworld.Edges
	heights, eyes bool
	field         collision.Field
	pieces        []piece
}

// rig is a world of units with collision and bullet, ticked by hand: bullet before the world,
// as a Stage's Update runs them.
type rig struct {
	rules  []rule.Rule // for start to deliver
	t      *testing.T
	pieces []piece
	w      *world.Plugin
	c      *collision.Plugin
	sel    *selection.Plugin
	b      *bullet.Plugin
	arms   *bullet.Shots
	fx     *effect.Effects
	ecs    *goke.ECS
	units  kind.Of[piece]
	ids    []uid.UID64 // by piece, once started

	q      *goke.Query
	base   goke.Comp[world.Base]
	coll   goke.OptComp[collision.Collider]
	flight goke.OptComp[bullet.Flight]
	z      goke.OptComp[world.Z]
	marks  goke.OptComp[tag.Tags[effect.States]]
	owned  goke.OptComp[tag.Tags[owner.Family]]
}

const tps = 60

func newRig(t *testing.T, sc scene) *rig {
	t.Helper()
	if sc.width == 0 {
		sc.width, sc.height = 400, 400
	}
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: sc.width, Height: sc.height, Edges: sc.edges},
		Entities: world.EntitiesCfg{MaxCount: len(sc.pieces) + 8, MinSize: 4, MaxSize: 40},
		Heights:  sc.heights,
	})
	c := collision.NewPlugin(w)
	if sc.field != nil {
		c.WithField(sc.field)
	}
	sel := selection.NewPlugin(w)
	r := &rig{t: t, pieces: sc.pieces, w: w, c: c, sel: sel, b: bullet.NewPlugin(w, sel), arms: bullet.NewShots(w), fx: w.Effects()}
	spec := kind.Spec{
		comp.Load(func(p piece) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(p.x, p.y), p.size, p.size)}
		}),
		comp.Load(func(p piece) world.Velocity { return world.Velocity{Dir: p.facing, Value: p.speed} }),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Mass: 1}),
		comp.Load(func(p piece) tag.Tags[selection.Family] {
			var m tag.Tags[selection.Family]
			if p.selectable {
				m = m.With(sel.Tags().Selectable)
			}
			if p.selected {
				m = m.With(sel.Tags().Selected)
			}
			return m
		}),
		comp.Load(func(p piece) tag.Tags[owner.Family] {
			var o tag.Tags[owner.Family]
			if p.owner != control.Nobody {
				o = o.With(owner.Of(p.owner))
			}
			return o
		}),
	}
	if sc.heights {
		spec = append(spec, comp.Load(func(p piece) world.Z { return p.z }))
	}
	if sc.eyes {
		spec = append(spec, comp.Load(func(p piece) world.Eye {
			if p.eye != nil {
				return *p.eye
			}
			return world.Eye{}
		}))
	}
	kind.Define[piece](w.Kinds(), "unit", spec)
	r.units = kind.Named[piece](w.Kinds(), "unit")
	for _, p := range sc.pieces {
		w.Seed(r.units.Entry(p))
	}
	return r
}

// obey keeps rules for start to hand to the hosts of their moments.
func (r *rig) obey(rules ...rule.Rule) error {
	r.rules = append(r.rules, rules...)
	return nil
}

// start installs the plugins and sets the world up; the ammo and the rules are defined before.
func (r *rig) start() {
	t := r.t
	t.Helper()
	if err := r.w.Carry(r.b); err != nil {
		t.Fatal(err)
	}
	if err := r.w.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx := &installCtx{ecs: goke.New()}
	for _, p := range []plugin.Plugin{r.w, r.c, r.b} {
		if err := p.Install(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := ctx.Deliver(r.rules...); err != nil {
		t.Fatal(err)
	}
	systems := append(ctx.systems(), goke.SystemFn{OnInit: func(si *goke.SysInit) {
		r.q = si.NewQueryBuilder(&r.base).Optional(&r.coll).Optional(&r.flight).Optional(&r.z).Optional(&r.marks).Optional(&r.owned).Build()
	}})
	r.ecs = ctx.ecs
	r.ecs.Setup(systems...)
	r.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		r.b.RunPlan(rc, d)
		r.w.RunPlan(rc, d)
		r.c.RunPlan(rc, d)
		rc.Sync()
		r.w.Clock().Replay(rc, d)
	})
	r.ids = make([]uid.UID64, len(r.pieces))
	for r.q.All(); r.q.Next(); {
		cur := r.q.Cursor()
		bases := r.base.Slice(cur)
		for i, id := range cur.IDs {
			for k, p := range r.pieces {
				if bases[i].Pos.TopLeft.X == p.x && bases[i].Pos.TopLeft.Y == p.y {
					r.ids[k] = id
				}
			}
		}
	}
}

func (r *rig) tick(n int) {
	for range n {
		r.ecs.Tick(time.Second / tps)
	}
}

// found is one entity as the rig reads it.
type found struct {
	id       uid.UID64
	base     world.Base
	flight   *bullet.Flight
	z        *world.Z
	contacts []collision.Contact
	marks    tag.Tags[effect.States]
	owned    tag.Tags[owner.Family]
}

// read walks every entity, keeping those keep lets through.
func (r *rig) read(keep func(f found) bool) []found {
	var out []found
	for r.q.All(); r.q.Next(); {
		cur := r.q.Cursor()
		bases, colls, flights, zs := r.base.Slice(cur), r.coll.Slice(cur), r.flight.Slice(cur), r.z.Slice(cur)
		marks, owned := r.marks.Slice(cur), r.owned.Slice(cur)
		for i, id := range cur.IDs {
			f := found{id: id, base: bases[i]}
			if colls != nil {
				f.contacts = append([]collision.Contact(nil), colls[i].Contacts()...)
			}
			if flights != nil {
				fl := flights[i]
				f.flight = &fl
			}
			if zs != nil {
				z := zs[i]
				f.z = &z
			}
			if marks != nil {
				f.marks = marks[i]
			}
			if owned != nil {
				f.owned = owned[i]
			}
			if keep(f) {
				out = append(out, f)
			}
		}
	}
	return out
}

// shots are the entities in flight or landed.
func (r *rig) shots() []found { return r.read(func(f found) bool { return f.flight != nil }) }

// unit is the k-th piece as it stands.
func (r *rig) unit(k int) found {
	got := r.read(func(f found) bool { return f.id == r.ids[k] })
	if len(got) != 1 {
		r.t.Fatalf("piece %d is gone", k)
	}
	return got[0]
}

// theShot is the one shot there is.
func (r *rig) theShot() found {
	shots := r.shots()
	if len(shots) != 1 {
		r.t.Fatalf("%d shots, want one", len(shots))
	}
	return shots[0]
}

var east = geom.NewVec(1, 0)

// round is the tests' plain shot: 4 across, 600 a second — ten a step, five times its own cap.
func round(r *rig, lands bool, tags ...tag.Tag[family]) bullet.Ammo {
	r.arms.Define("round", bullet.Body{Size: 4, Speed: 600, Range: 320, Lands: lands}, comp.Tagged(tags...))
	return r.arms.Named("round")
}

const allSides = collide.Left | collide.Right | collide.Top | collide.Bottom

// wallField is one solid wall, its band from below up to top, open all round.
type wallField struct {
	box  geom.AABB
	top  float64
	cell uint64
}

func (f *wallField) Solid(_ world.Layers, band collision.Band, box geom.AABB, visit func(collision.FieldBox) bool) {
	if box.Intersects(f.box) && band.Meets(collision.Band{Bottom: math.Inf(-1), Top: f.top}) {
		visit(collision.FieldBox{Box: f.box, Cell: f.cell, Open: allSides})
	}
}

func (f *wallField) Overhang(world.Layers, geom.AABB) float64 { return 0 }

// A Shoot an entity gives itself fires a shot at its muzzle, just outside its own box, the way it
// faces — slantwise too; walking on into the shot it never meets it: the shot ignores its shooter.
func TestShoot_AnEntityFiresFromItsMuzzleTheWayItFaces(t *testing.T) {
	for _, tc := range []struct {
		name   string
		facing geom.Vec
	}{{"east", east}, {"slantwise", geom.NewVec(math.Sqrt2/2, math.Sqrt2/2)}} {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig(t, scene{pieces: []piece{{x: 100, y: 100, size: 20, facing: tc.facing, speed: 600}}})
			ammo := round(r, true)
			r.start()
			if !r.w.Commands().PutFrom(r.ids[0], bullet.Shoot{Ammo: ammo}) {
				t.Fatal("the world does not carry a Shoot")
			}
			r.tick(1)
			shot, who := r.theShot(), r.unit(0)
			if d := shot.flight.Dir; math.Hypot(d.X-tc.facing.X, d.Y-tc.facing.Y) > 1e-9 || shot.flight.Shooter != who.id {
				t.Errorf("the shot flies %v shot by %v, want %v by %v", shot.flight.Dir, shot.flight.Shooter, tc.facing, who.id)
			}
			if muzzle := geom.NewAABBAt(geom.NewVec(100, 100), 20, 20); shot.base.Pos.AABB.AABB.Intersects(muzzle) {
				t.Errorf("the shot's box %v overlaps the shooter's as it fired, %v; want it outside", shot.base.Pos.AABB, muzzle)
			}
			r.tick(3)
			if who = r.unit(0); len(who.contacts) != 0 {
				t.Errorf("the shooter walking into its shot struck it: %+v, want nothing", who.contacts)
			}
			if shot = r.theShot(); len(shot.contacts) != 0 || shot.flight.Landed {
				t.Errorf("the shot struck its shooter: %+v, landed %v; want it flying on", shot.contacts, shot.flight.Landed)
			}
		})
	}
}

// A shot flying ten a step strikes a box of ten on its path: the Meeting is the game's rule's,
// the Landing says whom it struck — its ForOther acts on that one — and the shot lies just short
// of the box.
func TestShoot_AFastShotStrikesWhatLiesOnItsPath(t *testing.T) {
	r := newRig(t, scene{pieces: []piece{
		{x: 100, y: 100, size: 20, facing: east},
		{x: 200, y: 105, size: 10},
	}})
	roundTag := r.w.Kinds().DefineTag[family]("round")
	ammo := round(r, true, roundTag)
	r.fx.Define("hit", effect.Spec{effect.Lasts(time.Minute)})
	hit := r.fx.Named("hit")
	r.fx.Define("told", effect.Spec{effect.Lasts(time.Minute)})
	told := r.fx.Named("told")
	if err := r.obey(rule.Then[collision.Meeting]("hit", rule.Between(roundTag, tag.Any), rule.ForOther(rule.Apply(hit)))); err != nil {
		t.Fatal(err)
	}
	if err := r.obey(rule.Then[bullet.Landing]("told", rule.Self(roundTag), rule.If(func(l bullet.Landing) bool { return l.Struck }, rule.ForOther(rule.Apply(told))))); err != nil {
		t.Fatal(err)
	}
	r.start()
	r.w.Commands().PutFrom(r.ids[0], bullet.Shoot{Ammo: ammo})
	for i := 0; i < 60 && !r.unit(1).marks.Has(told.Mark()); i++ {
		r.tick(1)
	}
	target, shot := r.unit(1), r.theShot()
	if !target.marks.Has(hit.Mark()) || !target.marks.Has(told.Mark()) {
		t.Fatalf("the target carries %v, want hit (the Meeting, %v) and told (the Landing, %v)", target.marks, hit.Mark(), told.Mark())
	}
	if f := shot.flight; !f.Landed || f.Ending != bullet.Struck || f.Other != target.id {
		t.Errorf("the shot's flight is %+v, want landed, Struck, on the target %v", *f, target.id)
	}
	if gap := target.base.Pos.TopLeft.X - (shot.base.Pos.TopLeft.X + shot.base.Pos.Size.X); gap < 0 || gap > 0.1 {
		t.Errorf("the shot lies %v short of the target, want just short of touching it", gap)
	}
}

// A shot that does not Land is gone a step after it lands, the step its landing's effects took.
func TestLanding_ASpentShotIsGoneAStepAfterItLands(t *testing.T) {
	r := newRig(t, scene{pieces: []piece{
		{x: 100, y: 100, size: 20, facing: east},
		{x: 200, y: 105, size: 10},
	}})
	roundTag := r.w.Kinds().DefineTag[family]("round")
	ammo := round(r, false, roundTag)
	r.fx.Define("scored", effect.Spec{effect.Lasts(time.Minute)})
	scored := r.fx.Named("scored")
	if err := r.obey(rule.Then[bullet.Landing]("scored", rule.Self(roundTag), rule.Apply(scored))); err != nil { // on the shot itself: harmless, it lies a step
		t.Fatal(err)
	}
	r.start()
	r.w.Commands().PutFrom(r.ids[0], bullet.Shoot{Ammo: ammo})
	landed := -1
	for i := 0; i < 60; i++ {
		r.tick(1)
		if shots := r.shots(); len(shots) == 1 && shots[0].flight.Landed {
			landed = i
			break
		}
	}
	if landed < 0 {
		t.Fatal("the shot never landed")
	}
	r.tick(1)
	if shots := r.shots(); len(shots) != 0 {
		t.Errorf("%d shots a step after the landing, want the spent one gone", len(shots))
	}
	if target := r.unit(1); len(target.contacts) != 0 {
		t.Errorf("the target still records %+v, want nothing: the shot is gone", target.contacts)
	}
}

// A target within the last stretch of the shot's range is struck: the last step, shorter than
// the others, is swept like any.
func TestShoot_ATargetInTheLastStretchOfTheRangeIsStruck(t *testing.T) {
	r := newRig(t, scene{pieces: []piece{
		{x: 100, y: 100, size: 20, facing: east},
		{x: 219, y: 105, size: 10}, // the muzzle is at 123, the range of 95 ends at 218: the shot's last step, a half one, reaches it
	}})
	r.arms.Define("short", bullet.Body{Size: 4, Speed: 600, Range: 95, Lands: true})
	ammo := r.arms.Named("short")
	r.start()
	r.w.Commands().PutFrom(r.ids[0], bullet.Shoot{Ammo: ammo})
	r.tick(13)
	shot := r.theShot()
	if f := shot.flight; !f.Landed || f.Ending != bullet.Struck || f.Other != r.ids[1] {
		t.Errorf("the shot's flight is %+v, want landed, Struck, on the target %v", *f, r.ids[1])
	}
}

// A shot with nothing on its path flies its Range, lands a step later and, Landing, lies there.
func TestFlight_TheRangeFlownLandsAStepLater(t *testing.T) {
	r := newRig(t, scene{pieces: []piece{{x: 100, y: 100, size: 20, facing: east}}})
	r.arms.Define("short", bullet.Body{Size: 4, Speed: 600, Range: 100, Lands: true})
	ammo := r.arms.Named("short")
	r.start()
	r.w.Commands().PutFrom(r.ids[0], bullet.Shoot{Ammo: ammo})
	r.tick(11) // spawned, then ten steps of ten
	shot := r.theShot()
	if f := shot.flight; f.Landed || f.Ending != bullet.Spent || math.Abs(f.Flown-100) > 1e-3 {
		t.Fatalf("after the range the flight is %+v, want its range flown, Spent, not yet landed", *f)
	}
	r.tick(1)
	shot = r.theShot()
	if f := shot.flight; !f.Landed || f.Ending != bullet.Spent || math.Abs(f.At.X-223) > 1e-3 {
		t.Errorf("a step later the flight is %+v, want landed where the range ended, at 223", *f)
	}
	if shot.contacts != nil {
		t.Errorf("the landed shot still carries a Collider, want it gone")
	}
	r.tick(5)
	if len(r.shots()) != 1 {
		t.Error("the landed shot is gone, want it lying there")
	}
}

// A landed shot rests: told to Burst, every entity within the radius is a Blast for the rules,
// the one beyond it not, and the shot is gone.
func TestLanding_ALandedShotRestsAndBurstsOnThoseWithinItsRadius(t *testing.T) {
	r := newRig(t, scene{pieces: []piece{
		{x: 100, y: 100, size: 20, facing: east},
		{x: 235, y: 105, size: 10}, // 19 beyond where the grenade comes down, off its path
		{x: 216, y: 135, size: 10}, // 30 beside it
		{x: 295, y: 105, size: 10}, // 79 beyond
	}})
	grenadeTag := r.w.Kinds().DefineTag[family]("grenade")
	r.arms.Define("grenade", bullet.Body{Size: 8, Speed: 160, Range: 96, Lands: true}, comp.Tagged(grenadeTag))
	grenade := r.arms.Named("grenade")
	r.fx.Define("hurt", effect.Spec{effect.Lasts(time.Minute)})
	hurt := r.fx.Named("hurt")
	if err := r.obey(
		rule.Then[bullet.Resting]("burst", rule.Self(grenadeTag), rule.Order(bullet.Burst{Radius: 40})),
		rule.Then[bullet.Blast]("blast", rule.Between(grenadeTag, tag.Any), rule.ForOther(rule.Apply(hurt))),
	); err != nil {
		t.Fatal(err)
	}
	r.start()
	r.w.Commands().PutFrom(r.ids[0], bullet.Shoot{Ammo: grenade})
	r.tick(60)
	for k, want := range []bool{false, true, true, false} {
		if got := r.unit(k).marks.Has(hurt.Mark()); got != want {
			t.Errorf("piece %d hurt: %v, want %v", k, got, want)
		}
	}
	if n := len(r.shots()); n != 0 {
		t.Errorf("%d shots after the burst, want the grenade gone", n)
	}
}

// A thrown shot in a world with heights rises from the shooter's eye and comes down where it was
// aimed, within a step; a low wall on its way is flown over, a high one struck.
func TestFlight_AThrownShotArcsAndComesDownWhereAimed(t *testing.T) {
	for _, tc := range []struct {
		name string
		top  float64
		want bullet.End
	}{{"over a low wall", 18, bullet.Grounded}, {"into a high wall", 30, bullet.Wall}} {
		t.Run(tc.name, func(t *testing.T) {
			wall := &wallField{box: geom.NewAABBAt(geom.NewVec(150, 0), 10, 400), top: tc.top, cell: 7}
			r := newRig(t, scene{heights: true, eyes: true, field: wall, pieces: []piece{
				{x: 100, y: 100, size: 20, facing: east, z: world.Z{Height: 20}, eye: &world.Eye{Height: 16}},
			}})
			r.arms.Define("grenade", bullet.Body{Size: 8, Speed: 160, Range: 200, Gravity: 240, Lands: true})
			grenade := r.arms.Named("grenade")
			r.start()
			r.w.Commands().PutFrom(r.ids[0], bullet.Shoot{Ammo: grenade, At: geom.NewVec(220, 110), Targeted: true})
			peak := 0.0
			for i := 0; i < 90; i++ {
				r.tick(1)
				shot := r.theShot()
				peak = max(peak, shot.z.Altitude)
				if shot.flight.Landed {
					break
				}
			}
			shot := r.theShot()
			if peak <= 16 {
				t.Errorf("the shot rose to %v, want above the eye it left at 16", peak)
			}
			if f := shot.flight; !f.Landed || f.Ending != tc.want {
				t.Fatalf("the flight is %+v, want landed, %v", *f, tc.want)
			}
			switch tc.want {
			case bullet.Grounded:
				if math.Abs(shot.flight.At.X-220) > 4 || shot.z.Altitude != 0 {
					t.Errorf("came down at %v, %v up; want within a step of 220, on the ground", shot.flight.At, shot.z.Altitude)
				}
			case bullet.Wall:
				if shot.flight.Cell != 7 || shot.flight.At.X+4 > 150 {
					t.Errorf("struck cell %v at %v, want the wall's cell 7, just short of 150", shot.flight.Cell, shot.flight.At)
				}
			}
		})
	}
}

// A shot flying out by an open edge lands Left and is Outside, the world's leaving rules told;
// at a closed edge it stops and lands there.
func TestFlight_AnOpenEdgeIsLeftAClosedOneStopsTheShot(t *testing.T) {
	t.Run("open", func(t *testing.T) {
		r := newRig(t, scene{edges: aabbworld.OpenX, pieces: []piece{{x: 300, y: 100, size: 20, facing: east}}})
		ammo := round(r, true)
		r.fx.Define("gone", effect.Spec{effect.Lasts(time.Minute)})
		gone := r.fx.Named("gone")
		if err := r.obey(rule.Then[world.Leaving]("leaving", rule.All, rule.Apply(gone))); err != nil {
			t.Fatal(err)
		}
		r.start()
		r.w.Commands().PutFrom(r.ids[0], bullet.Shoot{Ammo: ammo})
		r.tick(14)
		shot := r.theShot()
		if f := shot.flight; !f.Landed || f.Ending != bullet.Left || !shot.marks.Has(gone.Mark()) {
			t.Errorf("the flight is %+v carrying %v, want landed, Left, and the leaving rule's mark", *f, shot.marks)
		}
	})
	t.Run("closed", func(t *testing.T) {
		r := newRig(t, scene{pieces: []piece{{x: 300, y: 100, size: 20, facing: east}}})
		ammo := round(r, true)
		r.start()
		r.w.Commands().PutFrom(r.ids[0], bullet.Shoot{Ammo: ammo})
		r.tick(14)
		shot := r.theShot()
		if f := shot.flight; !f.Landed || f.Ending != bullet.Edge || f.At.X+2 != 400 {
			t.Errorf("the flight is %+v, want landed at the edge, its box against 400", *f)
		}
	})
}

// A player's Shoot fires from the units it has selected and owns alone — not another's, not
// one it has not selected, not nobody's — each shot carrying its shooter's owners.
func TestShoot_APlayerFiresFromItsSelectedUnitsAlone(t *testing.T) {
	r := newRig(t, scene{pieces: []piece{
		{x: 50, y: 50, size: 20, facing: east, selectable: true, selected: true, owner: 1},
		{x: 50, y: 150, size: 20, facing: east, selectable: true, selected: true, owner: 2},
		{x: 50, y: 250, size: 20, facing: east, selectable: true, owner: 1},
		{x: 50, y: 350, size: 20, facing: east, selectable: true, selected: true},
	}})
	ammo := round(r, true)
	r.start()
	if !r.w.Commands().Put(1, bullet.Shoot{Ammo: ammo}) {
		t.Fatal("the world does not carry a Shoot")
	}
	r.tick(1)
	shot := r.theShot()
	if shot.flight.Shooter != r.ids[0] || !shot.owned.Has(owner.Of(1)) {
		t.Errorf("the shot is %v's carrying %v, want the first unit's (%v), owned by player 1", shot.flight.Shooter, shot.owned, r.ids[0])
	}
}

// A Shoot aimed at someone — the subject of the moment a rule orders it on — goes at that one,
// whichever way the shooter faces.
func TestShoot_AimedGoesAtTheSubject(t *testing.T) {
	r := newRig(t, scene{pieces: []piece{
		{x: 100, y: 100, size: 20, facing: east},
		{x: 100, y: 300, size: 20},
	}})
	ammo := round(r, true)
	r.start()
	cmd := bullet.Shoot{Ammo: ammo}
	cmd.Aim(r.ids[1])
	r.w.Commands().PutFrom(r.ids[0], cmd)
	r.tick(1)
	if shot := r.theShot(); shot.flight.Dir != geom.NewVec(0, 1) {
		t.Errorf("the shot flies %v, want south, at the one aimed at", shot.flight.Dir)
	}
}

// A wrapping world is refused at Install.
func TestInstall_AWrappingWorldIsRefused(t *testing.T) {
	r := newRig(t, scene{edges: aabbworld.Torus})
	if err := r.b.Install(&installCtx{ecs: goke.New()}); err == nil {
		t.Error("a wrapping world was not refused")
	}
}

// eachOnce drops repeated tokens, as the engine does.
func eachOnce(tokens []goke.CompToken) []goke.CompToken {
	listed := map[string]bool{}
	var once []goke.CompToken
	for _, token := range tokens {
		if !listed[token.Name] {
			listed[token.Name] = true
			once = append(once, token)
		}
	}
	return once
}

// A shot in flight is saved with its Body and Flight and flies on after a load.
func TestSaveLoad_AFlightGoesOnAfterALoad(t *testing.T) {
	sc := scene{pieces: []piece{{x: 100, y: 100, size: 20, facing: east}}}
	r := newRig(t, sc)
	r.arms.Define("short", bullet.Body{Size: 4, Speed: 600, Range: 100, Lands: true})
	ammo := r.arms.Named("short")
	r.start()
	r.w.Commands().PutFrom(r.ids[0], bullet.Shoot{Ammo: ammo})
	r.tick(4)
	before := r.theShot()
	path := t.TempDir() + "/save.bin"
	r.ecs.Pause()
	if err := r.ecs.Save(path); err != nil {
		t.Fatal(err)
	}

	r2 := newRig(t, sc)
	r2.arms.Define("short", ammo.Body())
	if err := r2.w.Carry(r2.b); err != nil {
		t.Fatal(err)
	}
	ctx := &installCtx{ecs: goke.New()}
	for _, p := range []plugin.Plugin{r2.w, r2.c, r2.b} {
		if err := p.Install(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := ctx.ecs.Load(path, eachOnce(goke.ProvidedComps(ctx.tracked...))...); err != nil {
		t.Fatal(err)
	}
	systems := ctx.systems()
	for _, v := range ctx.tracked {
		if pl, ok := v.(plugin.PostLoader); ok {
			systems = append(systems, pl.PostLoad())
		}
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		r2.q = si.NewQueryBuilder(&r2.base).Optional(&r2.coll).Optional(&r2.flight).Optional(&r2.z).Optional(&r2.marks).Optional(&r2.owned).Build()
	}})
	r2.ecs = ctx.ecs
	r2.ecs.Setup(systems...)
	r2.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		r2.b.RunPlan(rc, d)
		r2.w.RunPlan(rc, d)
		r2.c.RunPlan(rc, d)
		rc.Sync()
		r2.w.Clock().Replay(rc, d)
	})
	loaded := r2.theShot()
	if *loaded.flight != *before.flight {
		t.Fatalf("the flight loaded is %+v, want the one saved, %+v", *loaded.flight, *before.flight)
	}
	r2.tick(10)
	if f := r2.theShot().flight; !f.Landed || f.Ending != bullet.Spent {
		t.Errorf("after the load the flight is %+v, want flown on to its range and landed", *f)
	}
}

// Hosts keeps the hosts of the rules of the moments a plugin catches.
func (c *installCtx) Hosts(h ...plugin.Host) { c.hosts = append(c.hosts, h...) }

// Deliver hands rules — a role's, each of its own — to the hosts of their moments, as the engine
// does with the roles played once a Stage's Init returns.
func (c *installCtx) Deliver(rules ...rule.Rule) error { return hosts.Deliver(c.hosts, rules...) }
