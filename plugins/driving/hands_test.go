package driving_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

type installCtx struct {
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
func (c *installCtx) RegSys(factory func() goke.System) goke.Runnable { return c.ecs.RegSys(factory()) }
func (c *installCtx) ECS() *goke.ECS                                  { return c.ecs }
func (c *installCtx) Hosts(...host.Host)                              {}
func (c *installCtx) Draws(...render.Source)                          {}
func (c *installCtx) Looks(func())                                    {}
func (*installCtx) Pool() *parallel.Pool                              { return nil }

type walker struct {
	start    cell.ID
	owner    command.Issuer
	selected bool
	collides bool
}

type field struct {
	t       *testing.T
	grid    grid.Grid
	w       *world.Plugin
	ecs     *goke.ECS
	cams    [3]camera.Camera // by player, 1 and 2
	ids     []uid.UID64
	base    goke.Comp[world.Base]
	driven  goke.OptComp[steering.Driven]
	states  goke.OptComp[tag.Tags[driving.States]]
	course  goke.OptComp[steering.Course]
	walkers *goke.Query
}

func newField(t *testing.T, walkers ...walker) *field {
	t.Helper()
	return newFencedField(t, nil, walkers...)
}

// a fence runs north to south through the middle of each cell of fences
func newFencedField(t *testing.T, fences []cell.ID, walkers ...walker) *field {
	t.Helper()
	f := &field{t: t, grid: grid.DefaultGrids{}.Square(10, 3, 32)}
	f.w = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 320, Height: 96},
		Entities: world.EntitiesCfg{MaxCount: len(walkers) + len(fences), MinSize: 10, MaxSize: 22},
		Heights:  true,
	})
	m := motion.NewPlugin(f.w)
	col := collision.NewPlugin(f.w, m)
	brd := board.NewPlugin(f.grid, &cell.SingleOccupancy{}, f.w).WithMotion(m).WithCollision(col)
	brd.Write().Board().SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	sel := selection.NewPlugin(f.w)
	drv := driving.NewPlugin(f.w, m, sel).WithGround(brd).WithTap(rigTap)
	if err := f.w.Carry(drv); err != nil { // as the engine does with Use
		t.Fatal(err)
	}
	for p := range f.cams {
		f.cams[p] = cameratest.NewFromSpace(320, 96, 0)
	}
	ctx := &installCtx{ecs: goke.New()}
	for _, p := range []plugin.Plugin{f.w, m, col, brd, sel, drv} {
		if err := p.Install(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if len(fences) > 0 {
		north, _ := grid.Direction(f.grid, 0, -1)
		south, _ := grid.Direction(f.grid, 0, 1)
		kinds := board.NewEntities(brd, board.Shape{Size: 10}, func(c cell.ID) cell.ID { return c })
		kinds.Define("fence",
			comp.Const(cell.Floor{Allows: cell.Land, Links: 1<<north | 1<<south, Width: 10}),
			comp.Const(cell.Rail{Keeps: cell.Land}))
		for _, c := range fences {
			f.w.Write().Seed(kinds.Named("fence").Entry(c))
		}
	}
	kinds := make([]kind.ID, len(walkers))
	for i, wk := range walkers {
		spec := kind.Spec{
			comp.Load(func(wk walker) world.Position {
				return world.Position{AABB: plane.NewAABB(geom.NewVec(f.grid.CellCenter(wk.start).X-11, f.grid.CellCenter(wk.start).Y-11), 22, 22)}
			}),
			comp.Const(world.Velocity{}),
			comp.Const(steering.Steering{MaxSpeed: 96, Accel: 192, Brake: 384, V0: 48, TurnRate: 0.15}),
			comp.Load(func(wk walker) unit.At { return unit.At{Cell: wk.start} }),
			comp.Const(unit.Mover{Domain: cell.Land}),
		}
		tags := []tag.Tag[selection.Family]{sel.Tags().Selectable}
		if wk.selected {
			tags = append(tags, sel.Tags().Selected)
		}
		spec = append(spec, comp.Tagged(tags...))
		if wk.owner != command.Nobody {
			spec = append(spec, comp.Tagged(owner.Of(wk.owner)))
		}
		if wk.collides {
			spec = append(spec,
				comp.Const(collision.Collider{}), comp.Const(collision.Physics{Mass: 1}))
		}
		name := string(rune('a' + i))
		kind.Define[walker](f.w.Kinds(), name, spec)
		k := kind.Named[walker](f.w.Kinds(), name)
		kinds[i] = k.ID()
		f.w.Write().Seed(k.Entry(wk))
	}
	if err := f.w.Populate(); err != nil {
		t.Fatal(err)
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	f.ids = make([]uid.UID64, len(walkers))
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f.walkers = si.NewQueryBuilder(&f.base).Optional(&f.driven).Optional(&f.states).Optional(&f.course).Build()
		for f.walkers.All(); f.walkers.Next(); {
			cur := f.walkers.Cursor()
			for i, id := range cur.IDs {
				for row, k := range kinds {
					if f.base.Slice(cur)[i].TypeID == k {
						f.ids[row] = id
					}
				}
			}
		}
	}})
	f.ecs = ctx.ecs
	f.ecs.Setup(systems...)
	f.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		f.w.RunPlan(rc, d)
		m.RunPlan(rc, d)
		col.RunPlan(rc, d)
		brd.RunPlan(rc, d)
		drv.RunPlan(rc, d)
		rc.Sync()
		f.w.Write().Clock().Replay(rc, d)
	})
	return f
}

func (f *field) give(by command.Issuer, cmd command.Command) {
	if !f.w.Write().Carrier().Put(by, cmd) {
		f.t.Fatalf("the world carries no %T", cmd)
	}
}

func (f *field) tick(n int) {
	for range n {
		f.ecs.Tick(time.Second / 60)
	}
}

type seen struct {
	driven  *steering.Driven
	driving bool
	speed   float32
	side    float32
	at      geom.Vec
	dir     geom.Vec
}

func (f *field) read() map[uid.UID64]seen {
	out := map[uid.UID64]seen{}
	for f.walkers.All(); f.walkers.Next(); {
		cur := f.walkers.Cursor()
		drivens, marks, courses := f.driven.Slice(cur), f.states.Slice(cur), f.course.Slice(cur)
		for i, id := range cur.IDs {
			s := seen{at: f.base.Slice(cur)[i].Pos.Center(), dir: f.base.Slice(cur)[i].Vel.Dir}
			if drivens != nil {
				d := drivens[i]
				s.driven = &d
			}
			if marks != nil {
				s.driving = marks[i].Has(driving.Driving)
			}
			if courses != nil {
				s.speed, s.side = courses[i].Speed, courses[i].Side
			}
			out[id] = s
		}
	}
	return out
}

func TestHand_WalksTheSelectedUnitsOfThePlayerAlone(t *testing.T) {
	f := newField(t, walker{start: 12, owner: 1, selected: true}, walker{start: 16, owner: 2, selected: true})
	mine, theirs := f.ids[0], f.ids[1]
	before := f.read()
	for range 3 {
		f.give(1, driving.Ahead{Camera: f.cams[1]})
		f.tick(1)
	}
	after := f.read()
	if after[mine].at == before[mine].at {
		t.Errorf("the player's unit stands at %v still, want it driven on", after[mine].at)
	}
	if after[theirs].at != before[theirs].at || after[theirs].driven != nil {
		t.Errorf("the other player's unit moved to %v or got a Driven %v, want it left alone", after[theirs].at, after[theirs].driven)
	}
	if s := after[mine]; s.driven == nil || s.driven.Ahead != 1 || !s.driving {
		t.Errorf("the unit driven carries %+v, Driving %v; want Ahead 1 and Driving", s.driven, s.driving)
	}
}

func TestHand_TwoKeysInOneTickAddUp(t *testing.T) {
	f := newField(t, walker{start: 12, owner: 1, selected: true})
	f.give(1, driving.Ahead{Camera: f.cams[1]})
	f.give(1, driving.Turn{Camera: f.cams[1], Way: 1})
	f.give(1, driving.Turn{Camera: f.cams[1], Way: 1})
	f.tick(1)
	tick := time.Second / 60
	if s := f.read()[f.ids[0]]; s.driven == nil ||
		*s.driven != (steering.Driven{Ahead: 1, Turn: 1, Held: tick}) {
		t.Errorf("the unit carries %+v, want Ahead 1 and Turn 1", s.driven)
	}
}

func TestHand_StrafeKeysAddUpAndNeverPastOne(t *testing.T) {
	f := newField(t, walker{start: 12, owner: 1, selected: true})
	f.give(1, driving.Strafe{Camera: f.cams[1], Way: 1})
	f.give(1, driving.Strafe{Camera: f.cams[1], Way: 1})
	f.tick(1)
	tick := time.Second / 60
	if s := f.read()[f.ids[0]]; s.driven == nil ||
		*s.driven != (steering.Driven{Side: 1, Held: tick}) {
		t.Errorf("the unit carries %+v, want Side 1", s.driven)
	}
}

func TestHand_AStrafeStopsWithTheKey(t *testing.T) {
	f := newField(t, walker{start: 12, owner: 1, selected: true})
	id := f.ids[0]
	f.give(1, driving.Strafe{Camera: f.cams[1], Way: 1})
	f.tick(1)
	if s := f.read()[id]; s.side == 0 {
		t.Fatalf("D held: the unit strafes at %v, want its top speed", s.side)
	}
	f.tick(2)
	if s := f.read()[id]; s.side != 0 || s.driving {
		t.Errorf("D let go, standing: strafes at %v, driving %v; want 0 and let go",
			s.side, s.driving)
	}
}

func TestHand_LetGoAtAFenceItStandsWhereTheFenceHeldIt(t *testing.T) {
	fence := grid.DefaultGrids{}.Square(10, 3, 32).At(3, 1)
	f := newFencedField(t, []cell.ID{fence},
		walker{start: 12, owner: 1, selected: true, collides: true})
	id := f.ids[0]
	// a hair off square to the fence: what the wall leaves of the speed must not turn it
	f.give(1, driving.Toward{Camera: f.cams[1], Way: geom.NewVec(1, 1e-6)})
	f.tick(1)
	for range 30 { // east into the fence, held against it
		f.give(1, driving.Ahead{Camera: f.cams[1]})
		f.tick(1)
	}
	held := f.read()[id]
	if held.at.X < 94 || held.at.X > 97 { // the fence's west wall at 107, the unit 22 wide
		t.Fatalf("driven east into the fence the unit stands at %v, want held at its wall", held.at)
	}
	f.tick(30)
	s := f.read()[id]
	if f32.Hypot(s.at.X-held.at.X, s.at.Y-held.at.Y) > 1e-3 || s.driving || s.speed != 0 {
		t.Errorf("let go at the fence: at %v, Driving %v, speed %v; want standing at %v, let go",
			s.at, s.driving, s.speed, held.at)
	}
}

func TestHand_TurnedAboutItBrakesTurnsAndWalksOn(t *testing.T) {
	f := newField(t, walker{start: 12, owner: 1, selected: true})
	id := f.ids[0]
	for range 10 {
		f.give(1, driving.Toward{Camera: f.cams[1], Way: geom.NewVec(1, 0)})
		f.tick(1)
	}
	if s := f.read()[id]; s.speed == 0 {
		t.Fatal("driven east for ten ticks the unit stands still")
	}
	stopped, faced := false, false
	for range 40 {
		f.give(1, driving.Toward{Camera: f.cams[1], Way: geom.NewVec(-1, 0)})
		f.tick(1)
		s := f.read()[id]
		if s.speed == 0 && !faced {
			stopped = true
		}
		if s.dir.X < -0.99 {
			faced = true
		}
	}
	s := f.read()[id]
	if !stopped || !faced || s.speed == 0 || s.dir.X > -0.99 {
		t.Errorf("turned about: stopped first %v, faced west %v, now at %v facing %v; want a "+
			"brake, the turn, then walking west", stopped, faced, s.speed, s.dir)
	}
	// a turn within square carries on at speed
	before := f.read()[id].speed
	f.give(1, driving.Toward{Camera: f.cams[1], Way: geom.NewVec(-1, -1)})
	f.tick(1)
	if s := f.read()[id]; s.speed < before {
		t.Errorf("turned 45 degrees the unit slowed from %v to %v, want no brake", before, s.speed)
	}
}

func TestHand_ATickWithoutAHandBrakesAndThenLetsGo(t *testing.T) {
	f := newField(t, walker{start: 12, owner: 1, selected: true})
	id := f.ids[0]
	for range 10 {
		f.give(1, driving.Ahead{Camera: f.cams[1]})
		f.tick(1)
	}
	if s := f.read()[id]; s.speed == 0 {
		t.Fatal("after ten ticks driven the unit stands still")
	}
	f.tick(1)
	if s := f.read()[id]; s.driven == nil || *s.driven != (steering.Driven{}) || !s.driving {
		t.Fatalf("a tick without a hand: the unit carries %+v, Driving %v; want a zero Driven, still Driving", s.driven, s.driving)
	}
	f.tick(30)
	if s := f.read()[id]; s.driven != nil || s.driving || s.speed != 0 {
		t.Errorf("thirty ticks later the unit carries %+v, Driving %v, speed %v; want let go, standing", s.driven, s.driving, s.speed)
	}
}

func TestHand_TowardDrivesTheUnitThatWay(t *testing.T) {
	f := newField(t, walker{start: 2, owner: 1, selected: true})
	id := f.ids[0]
	before := f.read()[id].at
	for range 8 {
		f.give(1, driving.Toward{Camera: f.cams[1], Way: geom.NewVec(0, 1)})
		f.tick(1)
	}
	after := f.read()[id]
	if after.at.Y <= before.Y+4 || f32.Abs(after.at.X-before.X) > 4 {
		t.Errorf("driven south from %v the unit is at %v, want it gone down the screen", before, after.at)
	}
	if after.driven == nil || after.driven.Face != geom.NewVec(0, 1) {
		t.Errorf("the unit carries %+v, want its Face the way driven", after.driven)
	}
}

func TestHand_IsOnTheUnitTheCameraIsFastenedTo(t *testing.T) {
	f := newField(t, walker{start: 12, owner: 1, selected: true}, walker{start: 16, owner: 1})
	selected, fastened := f.ids[0], f.ids[1]
	f.cams[1].(camera.Fastenable).Fasten(camera.Fastening{Entity: fastened, How: camera.Centred})
	before := f.read()
	for range 3 {
		f.give(1, driving.Ahead{Camera: f.cams[1]})
		f.tick(1)
	}
	after := f.read()
	if after[fastened].at == before[fastened].at || !after[fastened].driving {
		t.Errorf("the unit the camera is fastened to stands at %v still, Driving %v; want it driven on", after[fastened].at, after[fastened].driving)
	}
	if after[selected].at != before[selected].at || after[selected].driven != nil {
		t.Errorf("the selected unit moved to %v or got a Driven %v, want it left alone while the camera is fastened to another", after[selected].at, after[selected].driven)
	}
}

func TestHand_AnEntitysOwnHandDrivesIt(t *testing.T) {
	f := newField(t, walker{start: 12})
	id := f.ids[0]
	before := f.read()[id].at
	for range 3 {
		f.w.Write().Carrier().PutFrom(id, driving.Ahead{})
		f.tick(1)
	}
	if after := f.read()[id]; after.at == before || !after.driving {
		t.Errorf("driving itself the unit stands at %v, Driving %v; want it walked on", after.at, after.driving)
	}
}

// the rig's tap, set by a test and put back
var rigTap time.Duration

func TestHand_ATapOnAStandingUnitOnlyTurnsIt(t *testing.T) {
	rigTap = 150 * time.Millisecond
	defer func() { rigTap = 0 }()
	f := newField(t, walker{start: 2, owner: 1, selected: true})
	id := f.ids[0]
	before := f.read()[id]
	south := geom.NewVec(0, 1)
	for range 3 { // 50 ms, a tap
		f.give(1, driving.Toward{Camera: f.cams[1], Way: south})
		f.tick(1)
	}
	tapped := f.read()[id]
	if tapped.dir != south || tapped.at != before.at || tapped.speed != 0 {
		t.Fatalf("tapped south: dir %v at %v speed %v, want turned south, not moved", tapped.dir,
			tapped.at, tapped.speed)
	}
	if tapped.driven == nil || tapped.driven.Held != 3*(time.Second/60) {
		t.Errorf("the hand held %v, want three ticks", tapped.driven)
	}
	f.tick(2)
	if letGo := f.read()[id]; letGo.driven != nil && letGo.driven.Held != 0 {
		t.Errorf("let go, the hand is still held %v", letGo.driven.Held)
	}
	for range 12 { // 200 ms, held past the tap
		f.give(1, driving.Toward{Camera: f.cams[1], Way: south})
		f.tick(1)
	}
	if held := f.read()[id]; held.at.Y <= before.at.Y || held.speed == 0 {
		t.Errorf("held south: at %v speed %v, want it walking down the screen", held.at, held.speed)
	}
}
