package interiors_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/parallel"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/interiors"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

type installCtx struct {
	hosts   []host.Host
	ecs     *goke.ECS
	pending []func() []goke.System
	// what the plugins installed, as the engine lists what a game saves and loads
	tracked []any
}

func (c *installCtx) UseModule(m goke.Module) {
	c.tracked = append(c.tracked, m)
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}
func (c *installCtx) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.tracked = append(c.tracked, p)
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

// what the rules said: in and out
type came struct{ Room uid.UID64 }
type went struct{ Room uid.UID64 }

func (came) Command() {}
func (went) Command() {}

type heard struct {
	in  command.Queue[came]
	out command.Queue[went]
}

func (h *heard) Queues() []command.CommandQueue     { return []command.CommandQueue{&h.in, &h.out} }
func (h *heard) DefaultBindings() []control.Binding { return nil }

// a house of two cells, (3, 3) and (4, 3), on ground falling 4 a cell southward: its floor 16, as
// high as the ground in the middle of its door, in the first cell's southern edge; a window in
// its eastern wall, from 2 to 8 over the floor
const (
	floor float32 = 16
	clear float32 = 10
)

var window = light.Opening{A: geom.NewVec(160, 100), B: geom.NewVec(160, 124), Bottom: 2, Top: 8}

type pieceRow struct {
	At    cell.ID
	Links cell.Links
}

type wholeRow struct{}

type unitRow struct {
	At   geom.Vec
	Face geom.Vec // the way it is walked, none standing
}

type house struct {
	ecs     *goke.ECS
	tracked []any
	heard   *heard
	q       *goke.Query
	base    goke.Comp[world.Base]
	at      goke.Comp[unit.At]
	in      goke.OptComp[interiors.Inside]
	shade   goke.OptComp[light.Sheltered]
}

func housed(t *testing.T, units ...unitRow) *house {
	t.Helper()
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 256, Height: 256},
		Entities: world.EntitiesCfg{MaxCount: 16, MinSize: 2, MaxSize: 64},
		Heights:  true,
	})
	m := motion.NewPlugin(w)
	c := collision.NewPlugin(w, m)
	lit := light.NewPlugin(w, light.Config{})
	brd := board.NewPlugin(g, &cell.MultipleOccupancy{}, w).WithMotion(m, cell.DefaultHiking).
		WithCollision(c)
	topo := topography.NewPlugin(w, brd, lit, topography.Config{})
	sel := selection.NewPlugin(w)
	drive := driving.NewPlugin(w, m, sel).WithGround(brd)
	rooms := interiors.NewPlugin(w, brd)
	brd.Write().Board().SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	topo.Write().Relief().SetHeights(func(p geom.Vec) float32 { return (256 - p.Y) / 8 })
	east, _ := grid.Direction(g, 1, 0)
	west, _ := grid.Direction(g, -1, 0)
	south, _ := grid.Direction(g, 0, 1)
	pieces := board.NewEntities(brd, board.Shape{Size: 32},
		func(r pieceRow) cell.ID { return r.At })
	pieces.Define("room", comp.Load(func(r pieceRow) cell.Floor {
		return cell.Floor{Allows: cell.Land, Height: floor - topo.Write().Relief().Altitude(r.At),
			Pace: 1, Graded: true, Links: r.Links, Storey: 1, Clear: clear, Standing: true,
			Door: cell.Gap{Width: 6, Height: 8}}
	}))
	shelter := light.Shelter{Sky: 0.5, Lamp: [3]float32{0.5, 0.4, 0.2}}
	shelter.Opens(window)
	kind.Define[wholeRow](w.Kinds(), "house", kind.Spec{
		comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(96, 96), 64, 32)}),
		comp.Const(world.Velocity{}),
		comp.Const(world.Z{Altitude: floor, Height: clear}),
		comp.Const(interiors.Room{}),
		comp.Const(shelter),
		comp.Const(world.Whole{}),
	})
	w.Write().SeedWhole(kind.EntryOf("house", wholeRow{}),
		pieces.Named("room").Entry(pieceRow{At: g.At(3, 3), Links: 1<<east | 1<<south}),
		pieces.Named("room").Entry(pieceRow{At: g.At(4, 3), Links: 1 << west}))
	walkers := board.NewUnits(brd, board.Shape{Size: 3, Height: 6},
		func(r unitRow) geom.Vec { return r.At })
	walkers.Define("walker", unit.Mover{Domain: cell.Land},
		steering.Steering{MaxSpeed: 24, Accel: 48, Brake: 96, V0: 12, TurnRate: 0.15},
		comp.Load(func(r unitRow) steering.Driven {
			if r.Face == (geom.Vec{}) {
				return steering.Driven{}
			}
			return steering.Driven{Ahead: 1, Face: r.Face, Held: time.Second}
		}))
	for _, u := range units {
		w.Write().Seed(walkers.Named("walker").Entry(u))
	}
	h := &house{heard: &heard{}}
	if err := w.Carry(h.heard); err != nil {
		t.Fatal(err)
	}
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx := &installCtx{ecs: goke.New()}
	for _, p := range []plugin.Plugin{w, m, c, lit, brd, topo, sel, drive, rooms} {
		if err := p.Install(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := ctx.Deliver(
		act.Branch[interiors.Entered]{
			Name: "came in", When: cond.Self.If(cond.Always), Then: act.Self.Do(came{}),
		},
		act.Branch[interiors.Left]{
			Name: "went out", When: cond.Self.If(cond.Always), Then: act.Self.Do(went{}),
		},
	); err != nil {
		t.Fatal(err)
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		h.q = si.NewQueryBuilder(&h.base, &h.at).Optional(&h.in).Optional(&h.shade).Build()
	}})
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		lit.RunPlan(rc, d)
		m.RunPlan(rc, d)
		c.RunPlan(rc, d)
		brd.RunPlan(rc, d)
		topo.RunPlan(rc, d)
		drive.RunPlan(rc, d)
		rooms.RunPlan(rc, d)
		rc.Sync()
		w.Write().Clock().Replay(rc, d)
	})
	h.ecs, h.tracked = ctx.ecs, ctx.tracked
	return h
}

func (h *house) tick(seconds float32) {
	for range int(seconds * 60) {
		h.ecs.Tick(time.Second / 60)
	}
}

// what each unit is as the last step left it, by where it stands
type stood struct {
	at      geom.Vec
	inside  bool
	shelter *light.Sheltered
}

func (h *house) units() []stood {
	var out []stood
	for h.q.All(); h.q.Next(); {
		cur := h.q.Cursor()
		ins, shades := h.in.Slice(cur), h.shade.Slice(cur)
		for i, b := range h.base.Slice(cur) {
			s := stood{at: b.Pos.Center(), inside: ins != nil && ins[i].Room != 0}
			if shades != nil {
				sh := shades[i]
				s.shelter = &sh
			}
			out = append(out, s)
		}
	}
	return out
}

func TestInside_TellsTheRulesOnceAsAUnitComesInAndOnceAsItGoesOut(t *testing.T) {
	for _, c := range []struct {
		name    string
		walker  unitRow
		inside  bool
		in, out int
	}{
		{"walking in by the door", unitRow{At: geom.NewVec(112, 150),
			Face: geom.NewVec(0, -1)}, true, 1, 0},
		// found in the room as the world starts, so come in then
		{"walking out by it", unitRow{At: geom.NewVec(112, 112), Face: geom.NewVec(0, 1)},
			false, 1, 1},
	} {
		h := housed(t, c.walker)
		h.tick(3)
		if u := h.units()[0]; u.inside != c.inside || u.at.Y < 128 != c.inside {
			t.Errorf("%s: the unit stands at %v, inside %v; want inside %v", c.name, u.at,
				u.inside, c.inside)
		}
		var in, out int
		h.heard.in.Drain(func(command.Issued[came]) { in++ })
		h.heard.out.Drain(func(command.Issued[went]) { out++ })
		if in != c.in || out != c.out {
			t.Errorf("%s: the rules were told %d times in and %d out, want %d and %d", c.name,
				in, out, c.in, c.out)
		}
	}
}

func TestSheltered_UnderTheRoofOfTheRoomItStandsInAlone(t *testing.T) {
	h := housed(t, unitRow{At: geom.NewVec(140, 112)}, unitRow{At: geom.NewVec(140, 170)})
	h.tick(0.5)
	got := map[bool]stood{}
	for _, u := range h.units() {
		got[u.at.Y < 128] = u
	}
	in, out := got[true], got[false]
	if !in.inside || in.shelter == nil || in.shelter.In == 0 {
		t.Errorf("the unit in the house is inside %v, sheltered %v; want under the house's roof",
			in.inside, in.shelter)
	}
	if out.inside || out.shelter != nil {
		t.Errorf("the unit before the house is inside %v, sheltered %v; want in the open",
			out.inside, out.shelter)
	}
}

func TestRoom_SavedWithTheGameAndWhoeverStandsInIt(t *testing.T) {
	h := housed(t, unitRow{At: geom.NewVec(112, 112)})
	h.tick(0.5)
	path := t.TempDir() + "/save.bin"
	h.ecs.Pause()
	if err := h.ecs.Save(path); err != nil {
		t.Fatal(err)
	}
	// each component once, as the engine loads them
	var comps []goke.CompToken
	listed := map[string]bool{}
	for _, c := range goke.ProvidedComps(h.tracked...) {
		if !listed[c.Name] {
			listed[c.Name] = true
			comps = append(comps, c)
		}
	}
	// selection reads the players' owner family, which the players plugin lists in a game
	comps = append(comps, goke.LoadComp[tag.Tags[owner.Family]]())
	loaded := goke.New()
	if err := loaded.Load(path, comps...); err != nil {
		t.Fatal(err)
	}
	var roof goke.Comp[light.Shelter]
	var inside goke.Comp[interiors.Inside]
	var under goke.Comp[light.Sheltered]
	rooms := map[uid.UID64]light.Shelter{}
	var in []interiors.Inside
	var sheltered []light.Sheltered
	loaded.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q := si.NewQueryBuilder(&roof).With(goke.Include[interiors.Room]()).Build()
		for q.All(); q.Next(); {
			cur := q.Cursor()
			for i, r := range roof.Slice(cur) {
				rooms[cur.IDs[i]] = r
			}
		}
		qi := si.NewQueryBuilder(&inside, &under).Build()
		for qi.All(); qi.Next(); {
			in = append(in, inside.Slice(qi.Cursor())...)
			sheltered = append(sheltered, under.Slice(qi.Cursor())...)
		}
	}})
	if len(rooms) != 1 || len(in) != 1 {
		t.Fatalf("loaded %d rooms under a roof and %d inside one, want 1 and 1", len(rooms),
			len(in))
	}
	r, ok := rooms[in[0].Room]
	if !ok || r.Open != 1 || r.Openings[0] != window || r.Sky != 0.5 ||
		sheltered[0].In != in[0].Room {
		t.Errorf("the unit loaded inside %v under %v, the room %+v; want the room saved, its "+
			"window and its sky, the unit under it", in[0].Room, sheltered[0].In, r)
	}
}
