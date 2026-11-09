package collision_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/internal/collisiontest"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// a world of houses standing for good, plates standing for good a walker is told of, and
// walkers; seeded and stepped as a game steps them
type standing struct {
	w      *world.Plugin
	m      *motion.Plugin
	c      *collision.Plugin
	stats  collision.ContactStats
	houses kind.Of[placed]
	plates kind.Of[placed]
	walker kind.Of[placed]

	ecs      *goke.ECS
	all      *goke.Query
	base     goke.Comp[world.Base]
	collider goke.OptComp[collision.Collider]
	restills *int
}

func newStanding(cfg world.Config) *standing {
	s := &standing{w: world.NewPlugin(cfg)}
	s.m = motion.NewPlugin(s.w)
	s.c = collision.NewPlugin(s.w, s.m).WithStats(&s.stats)
	sized := func(side float32) comp.Comp {
		return comp.Load(func(p placed) world.Position { return posAt(p.x, p.y, side, side) })
	}
	kind.Define[placed](s.w.Kinds(), "house", kind.Spec{
		sized(40),
		comp.Const(world.Velocity{}),
		comp.Const(world.Fixed{}),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Mass: f32.Inf(1)}),
	})
	kind.Define[placed](s.w.Kinds(), "plate", kind.Spec{
		sized(40),
		comp.Const(world.Velocity{}),
		comp.Const(world.Fixed{}),
		comp.Const(collision.Collider{}),
	})
	kind.Define[placed](s.w.Kinds(), "walker", kind.Spec{
		sized(10),
		comp.Load(func(p placed) world.Velocity { return p.vel }),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{}),
	})
	s.houses = kind.Named[placed](s.w.Kinds(), "house")
	s.plates = kind.Named[placed](s.w.Kinds(), "plate")
	s.walker = kind.Named[placed](s.w.Kinds(), "walker")
	return s
}

func standingWorld() world.Config {
	return world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 10, MaxSize: 40},
	}
}

func (s *standing) start(t *testing.T, seeds ...kind.Entry) {
	t.Helper()
	s.w.Write().Seed(seeds...)
	if err := s.w.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx := collisiontest.NewInstallCtx(goke.New())
	for _, p := range []interface {
		Install(plugin.Installer) error
	}{s.w, s.m, s.c} {
		if err := p.Install(ctx); err != nil {
			t.Fatal(err)
		}
	}
	s.restills = s.c.CountRestills()
	s.ecs = ctx.ECS()
	s.ecs.Setup(append(ctx.Systems(), goke.SystemFn{OnInit: func(si *goke.SysInit) {
		s.all = si.NewQueryBuilder(&s.base).Optional(&s.collider).Build()
	}})...)
	s.ecs.SetPlan(collisiontest.Step(s.w, s.m, s.c))
}

// ticks steps n times and returns the contacts counted over them and the times the collision
// asked the world to take what stands for good anew
func (s *standing) ticks(n int) (contacts, restills int) {
	s.stats.Counter, *s.restills = 0, 0
	for range n {
		s.ecs.Tick(time.Second / 60)
	}
	return s.stats.Counter, *s.restills
}

// the base and the contacts of the entity whose box is side wide; the test fails with none
func (s *standing) of(t *testing.T, side float32) (world.Base, []collision.Contact) {
	t.Helper()
	for s.all.All(); s.all.Next(); {
		cur := s.all.Cursor()
		for i, b := range s.base.Slice(cur) {
			if b.Pos.Size.X != side {
				continue
			}
			var contacts []collision.Contact
			if c := s.collider.Slice(cur); c != nil {
				contacts = c[i].Contacts()
			}
			return b, contacts
		}
	}
	t.Fatalf("no entity %v wide", side)
	return world.Base{}, nil
}

func (s *standing) finds(box geom.AABB, want aabbworld.Capability) []uid.UID64 {
	var found []uid.UID64
	s.w.Read(0).Space().Query(box, want, func(id uid.UID64) { found = append(found, id) })
	return found
}

func overlaps(a, b world.Base) bool {
	return a.Pos.AABB.AABB.Intersects(b.Pos.AABB.AABB) && !touching(a, b)
}

// touching within a hair, as a pair pushed apart rests
func touching(a, b world.Base) bool {
	ba, bb := a.Pos.AABB.AABB, b.Pos.AABB.AABB
	gapX := max(ba.TopLeft.X-bb.BottomRight.X, bb.TopLeft.X-ba.BottomRight.X)
	gapY := max(ba.TopLeft.Y-bb.BottomRight.Y, bb.TopLeft.Y-ba.BottomRight.Y)
	return max(gapX, gapY) > -1e-3
}

func TestStill_AHouseStandingForGoodPushesAWalkerOutAndStays(t *testing.T) {
	s := newStanding(standingWorld())
	s.start(t, s.houses.Entry(placed{x: 100, y: 100}), s.walker.Entry(placed{x: 132, y: 110}))

	if n, _ := s.ticks(1); n == 0 {
		t.Fatal("no contact on the first tick between a walker and the house it stands in")
	}
	if _, restills := s.ticks(5); restills != 0 {
		t.Errorf("the world asked %d times to take the house anew, its Caps kept; want none",
			restills)
	}
	house, _ := s.of(t, 40)
	walker, _ := s.of(t, 10)
	if house.Pos.TopLeft != geom.NewVec(100, 100) {
		t.Errorf("the house stands at %v, want (100, 100): what stands is never pushed",
			house.Pos.TopLeft)
	}
	if overlaps(house, walker) {
		t.Errorf("the walker at %v is still inside the house at %v", walker.Pos.AABB,
			house.Pos.AABB)
	}
	if house.Caps != aabbworld.CanCollide|aabbworld.Static {
		t.Errorf("the house's Caps %v, want CanCollide|Static", house.Caps)
	}
	if found := s.finds(geom.NewAABBAt(geom.NewVec(110, 110), 5, 5),
		aabbworld.AnyCapability); len(found) != 1 {
		t.Errorf("the space finds %v in the house, want it", found)
	}
}

func TestStill_APlateStandingForGoodIsTouchedAndPushesNothing(t *testing.T) {
	s := newStanding(standingWorld())
	s.start(t, s.plates.Entry(placed{x: 100, y: 100}), s.walker.Entry(placed{x: 110, y: 110}))

	if n, _ := s.ticks(3); n == 0 {
		t.Fatal("no contact between a walker and the plate it stands on")
	}
	walker, contacts := s.of(t, 10)
	if walker.Pos.TopLeft != geom.NewVec(110, 110) {
		t.Errorf("the walker on the plate went to %v, want it left where it stands",
			walker.Pos.TopLeft)
	}
	if len(contacts) != 1 || !contacts[0].Sensed {
		t.Errorf("the walker's contacts %+v, want the plate's, sensed", contacts)
	}
}

func TestStill_AHouseSpawnedMidGameCollidesTheStepItComes(t *testing.T) {
	s := newStanding(standingWorld())
	s.start(t, s.walker.Entry(placed{x: 132, y: 110}))
	s.ticks(2)

	s.w.Write().Spawn(s.houses.Entry(placed{x: 100, y: 100}))
	n, restills := s.ticks(1)
	if n == 0 {
		t.Fatal("no contact the step the house came, a walker standing in it")
	}
	// its Caps set as it came, after the world gathered it
	if restills != 1 {
		t.Errorf("the step the house came the world was asked %d times to take it anew, want 1",
			restills)
	}
	if _, restills := s.ticks(5); restills != 0 {
		t.Errorf("the world was asked %d times later, want none", restills)
	}
	house, _ := s.of(t, 40)
	walker, _ := s.of(t, 10)
	if house.Pos.TopLeft != geom.NewVec(100, 100) || overlaps(house, walker) {
		t.Errorf("the house at %v, the walker at %v; want the house where it came and the "+
			"walker out of it", house.Pos.AABB, walker.Pos.AABB)
	}
}

func TestStill_ColliderOffAndOnAThingStandingForGood(t *testing.T) {
	s := newStanding(standingWorld())
	s.start(t, s.plates.Entry(placed{x: 100, y: 100}), s.walker.Entry(placed{x: 110, y: 110}))
	var plate uid.UID64
	for s.all.All(); s.all.Next(); {
		cur := s.all.Cursor()
		for i, b := range s.base.Slice(cur) {
			if b.Pos.Size.X == 40 {
				plate = cur.IDs[i]
			}
		}
	}
	var edit func(cb *goke.CmdBuf)
	editor := s.ecs.RegSys(goke.SystemFn{OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
		if edit != nil {
			edit(cb)
			edit = nil
		}
	}})
	step := collisiontest.Step(s.w, s.m, s.c)
	s.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		rc.Run(editor, d)
		rc.Sync()
		step(rc, d)
	})
	inPlate := geom.NewAABBAt(geom.NewVec(100, 100), 40, 40)

	if n, _ := s.ticks(1); n != 1 {
		t.Fatalf("%d contacts with the plate on, want 1", n)
	}
	collider := s.ecs.RegComp[collision.Collider]()
	edit = func(cb *goke.CmdBuf) { cb.RemoveCompOne(plate, collider) }
	// the world gathers it with the Caps it collided by; met, they go Plain
	if n, restills := s.ticks(1); n != 0 || restills != 1 {
		t.Errorf("the tick the plate's Collider came off: %d contacts, the world asked %d "+
			"times to take it anew; want 0 and 1", n, restills)
	}
	if found := s.finds(inPlate, aabbworld.CanCollide); len(found) != 1 || found[0] == plate {
		t.Errorf("the space lists %v collidable on the plate, want the walker alone", found)
	}
	if n, restills := s.ticks(1); n != 0 || restills != 0 {
		t.Errorf("a tick later: %d contacts, %d asked; want none", n, restills)
	}
	edit = func(cb *goke.CmdBuf) { cb.AddOne(plate, collider, collision.Collider{}) }
	if n, _ := s.ticks(1); n != 1 {
		t.Errorf("%d contacts the tick the plate's Collider went back on, want 1", n)
	}
}

// far off a house keeps the Caps it collides by, the walker beside it Plain and pushed by
// nothing; looked at, the walker is pushed out of it, the house's Caps as they were
func TestStill_AHouseFarOffKeepsItsCapsAndPushesOnceLookedAt(t *testing.T) {
	cfg := world.Config{
		Space:    world.SpaceCfg{Width: 4096, Height: 512},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 10, MaxSize: 40},
		Sectors:  world.SectorsCfg{Buckets: 2},
		LOD:      lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 0.5, 0.4, 0.3}, Sight: 600},
	}
	at := newObservers(4096, 512, 100, 256)
	s := newStanding(cfg)
	s.w.Write().Observe(at)
	s.start(t, s.houses.Entry(placed{x: 2200, y: 240}), s.walker.Entry(placed{x: 2232, y: 250}))

	if n, _ := s.ticks(2); n != 0 {
		t.Errorf("%d contacts far off, want none", n)
	}
	house, _ := s.of(t, 40)
	walker, _ := s.of(t, 10)
	if house.Caps != aabbworld.CanCollide|aabbworld.Static || walker.Caps != aabbworld.Plain {
		t.Errorf("far off the house's Caps %v, the walker's %v; want CanCollide|Static and Plain",
			house.Caps, walker.Caps)
	}
	if walker.Pos.TopLeft != geom.NewVec(2232, 250) {
		t.Errorf("far off the walker went to %v, want it pushed by nothing", walker.Pos.TopLeft)
	}

	at.look(2200, 256)
	n, restills := s.ticks(1)
	if n == 0 {
		t.Error("no contact once the house is looked at, the walker standing in it")
	}
	if restills != 0 {
		t.Errorf("looked at, the world was asked %d times to take the house anew, want none: "+
			"the level's move has it gathered", restills)
	}
	s.ticks(5)
	house, _ = s.of(t, 40)
	walker, _ = s.of(t, 10)
	if house.Pos.TopLeft != geom.NewVec(2200, 240) || overlaps(house, walker) {
		t.Errorf("looked at, the house at %v, the walker at %v; want the house where it stands "+
			"and the walker out of it", house.Pos.AABB, walker.Pos.AABB)
	}
}
