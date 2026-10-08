package driving_test

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

// installCtx is the plugin.Installer a Stage would hand over, minus the engine.
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
func (c *installCtx) Hosts(...plugin.Host)                            {}

// walker is one unit of a field: the cell it starts in, whose it is, whether it is selected.
type walker struct {
	start    cell.ID
	owner    control.PlayerID
	selected bool
}

// field is a world, a board of open land 10 cells by 3 of 32 and the driving over it — no
// navigation — with two players each looking through a camera of its own.
type field struct {
	t       *testing.T
	grid    grid.Grid
	w       *world.Plugin
	ecs     *goke.ECS
	cams    [3]camera.Camera // by player, 1 and 2
	ids     []uid.UID64      // the walkers', in the order given
	base    goke.Comp[world.Base]
	driven  goke.OptComp[steering.Driven]
	states  goke.OptComp[tag.Tags[driving.States]]
	course  goke.OptComp[steering.Course]
	walkers *goke.Query
}

func newField(t *testing.T, walkers ...walker) *field {
	t.Helper()
	f := &field{t: t, grid: grid.DefaultGrids{}.Square(10, 3, 32)}
	f.w = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 320, Height: 96},
		Entities: world.EntitiesCfg{MaxCount: len(walkers), MinSize: 22, MaxSize: 22},
	})
	brd := board.NewPlugin(f.grid, &cell.SingleOccupancy{}, f.w)
	brd.Res.Logic.Board.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	sel := selection.NewPlugin(f.w)
	drv := driving.NewPlugin(f.w, sel).WithGround(brd)
	if err := f.w.Carry(drv); err != nil { // as the engine does with Use
		t.Fatal(err)
	}
	for p := range f.cams {
		f.cams[p] = cameras.TopDown()(320, 96, 0, camera.Config{})
	}
	ctx := &installCtx{ecs: goke.New()}
	for _, p := range []plugin.Plugin{f.w, brd, sel, drv} {
		if err := p.Install(ctx); err != nil {
			t.Fatal(err)
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
		if wk.owner != control.Nobody {
			spec = append(spec, comp.Tagged(owner.Of(wk.owner)))
		}
		name := string(rune('a' + i))
		kind.Define[walker](f.w.Kinds(), name, spec)
		k := kind.Named[walker](f.w.Kinds(), name)
		kinds[i] = k.ID()
		f.w.Seed(k.Entry(wk))
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
		brd.RunPlan(rc, d)
		drv.RunPlan(rc, d)
		rc.Sync()
		f.w.Clock().Replay(rc, d)
	})
	return f
}

// give gives cmd as player by.
func (f *field) give(by control.PlayerID, cmd any) {
	if !f.w.Carrier().Put(by, cmd) {
		f.t.Fatalf("the world carries no %T", cmd)
	}
}

func (f *field) tick(n int) {
	for range n {
		f.ecs.Tick(time.Second / 60)
	}
}

// seen is what a test reads off a walker: its Driven, if it carries one, whether it is Driving,
// its speed and where it stands.
type seen struct {
	driven  *steering.Driven
	driving bool
	speed   float64
	at      geom.Vec
}

func (f *field) read() map[uid.UID64]seen {
	out := map[uid.UID64]seen{}
	for f.walkers.All(); f.walkers.Next(); {
		cur := f.walkers.Cursor()
		drivens, marks, courses := f.driven.Slice(cur), f.states.Slice(cur), f.course.Slice(cur)
		for i, id := range cur.IDs {
			s := seen{at: f.base.Slice(cur)[i].Pos.Center()}
			if drivens != nil {
				d := drivens[i]
				s.driven = &d
			}
			if marks != nil {
				s.driving = marks[i].Has(driving.Driving)
			}
			if courses != nil {
				s.speed = courses[i].Speed
			}
			out[id] = s
		}
	}
	return out
}

// A hand walks the selected units of the player who gave it and no one else's, with no navigation
// in the game; the unit driven carries a Driven and is Driving.
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

// Several commands in one tick add up: on and turning at once, each way held within one.
func TestHand_TwoKeysInOneTickAddUp(t *testing.T) {
	f := newField(t, walker{start: 12, owner: 1, selected: true})
	f.give(1, driving.Ahead{Camera: f.cams[1]})
	f.give(1, driving.Turn{Camera: f.cams[1], Way: 1})
	f.give(1, driving.Turn{Camera: f.cams[1], Way: 1})
	f.tick(1)
	if s := f.read()[f.ids[0]]; s.driven == nil || *s.driven != (steering.Driven{Ahead: 1, Turn: 1}) {
		t.Errorf("the unit carries %+v, want Ahead 1 and Turn 1", s.driven)
	}
}

// A tick without a hand brakes the unit; once it stands its Driven is taken off and it is not
// Driving any more.
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

// Toward drives the unit the way of the screen: it turns to it and walks on.
func TestHand_TowardDrivesTheUnitThatWay(t *testing.T) {
	f := newField(t, walker{start: 2, owner: 1, selected: true})
	id := f.ids[0]
	before := f.read()[id].at
	for range 8 {
		f.give(1, driving.Toward{Camera: f.cams[1], Way: geom.NewVec(0, 1)})
		f.tick(1)
	}
	after := f.read()[id]
	if after.at.Y <= before.Y+4 || math.Abs(after.at.X-before.X) > 4 {
		t.Errorf("driven south from %v the unit is at %v, want it gone down the screen", before, after.at)
	}
	if after.driven == nil || after.driven.Face != geom.NewVec(0, 1) {
		t.Errorf("the unit carries %+v, want its Face the way driven", after.driven)
	}
}

// A player's hand is on the unit the camera it was given through is fastened to, selected or not,
// and not on the units it has selected then.
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

// An entity drives itself by the commands it gives itself, nobody's selection asked.
func TestHand_AnEntitysOwnHandDrivesIt(t *testing.T) {
	f := newField(t, walker{start: 12})
	id := f.ids[0]
	before := f.read()[id].at
	for range 3 {
		f.w.Carrier().PutFrom(id, driving.Ahead{})
		f.tick(1)
	}
	if after := f.read()[id]; after.at == before || !after.driving {
		t.Errorf("driving itself the unit stands at %v, Driving %v; want it walked on", after.at, after.driving)
	}
}
