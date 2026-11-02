package roads_test

import (
	"bytes"
	"encoding/gob"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/roads"
	"github.com/kjkrol/gram/plugins/roads/site"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const roadCell = "road"

type installer struct {
	ecs     *goke.ECS
	pending []func() []goke.System
	hosts   []host.Host
	drawn   []render.Source
}

func (c *installer) UseModule(m goke.Module) {
	reg := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), reg) })
}
func (c *installer) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *installer) RegSys(f func() goke.System) goke.Runnable { return c.ecs.RegSys(f()) }
func (c *installer) ECS() *goke.ECS                            { return c.ecs }
func (c *installer) Hosts(h ...host.Host)                      { c.hosts = append(c.hosts, h...) }
func (c *installer) Draws(s ...render.Source)                  { c.drawn = append(c.drawn, s...) }
func (c *installer) Looks(func())                              {}
func (*installer) Pool() *parallel.Pool                        { return nil }

// fakeSite routes a leg along the grid's columns, then its rows, noting the levelling of every
// router made, and plans the cells it is given with their links and ways, shape adding what a
// test asks of every plan
type fakeSite struct {
	g         grid.Grid
	routers   []bool
	legs      [][2]cell.ID
	levelling []bool
	shape     func(e *site.Estimate)
}

func (s *fakeSite) router(levelling bool) roads.Router {
	s.routers = append(s.routers, levelling)
	return s
}

func (s *fakeSite) Route(from, to cell.ID, dst []cell.ID) ([]cell.ID, error) {
	s.legs = append(s.legs, [2]cell.ID{from, to})
	x, y, _ := s.g.Coords(from)
	tx, ty, _ := s.g.Coords(to)
	dst = append(dst[:0], from)
	for x != tx || y != ty {
		switch {
		case x < tx:
			x++
		case x > tx:
			x--
		case y < ty:
			y++
		default:
			y--
		}
		dst = append(dst, s.g.At(x, y))
	}
	return dst, nil
}

func (s *fakeSite) plan(g site.Ground, _ site.Rules, cells []cell.ID, kind string, width float32,
	_, levelling bool, _ float32, _ []cell.Links) (site.Estimate, error) {
	s.levelling = append(s.levelling, levelling)
	gr := g.Grid()
	e := site.Estimate{Cells: slices.Clone(cells), Links: make([]cell.Links, len(cells))}
	for k, c := range cells {
		if k > 0 {
			l, _ := grid.Link(gr, c, cells[k-1])
			e.Links[k] |= l
		}
		if k+1 < len(cells) {
			l, _ := grid.Link(gr, c, cells[k+1])
			e.Links[k] |= l
		}
		e.Ways = append(e.Ways, cell.WayEntry{Kind: kind, Cell: c, Width: width, Links: e.Links[k]})
	}
	e.Length = float32(len(cells)-1) * gr.CellSpan()
	if s.shape != nil {
		s.shape(&e)
	}
	return e, nil
}

// shaper notes the corners every cell is graded to
type shaper struct{ graded map[cell.ID][]float32 }

var _ ground.Shaper = (*shaper)(nil)

func (*shaper) Flatten(cell.ID, float32) bool { return false }

func (s *shaper) Grade(c cell.ID, corners []float32) bool {
	s.graded[c] = slices.Clone(corners)
	return true
}

// field is a board 8 x 6 of cells 10 wide, of earth, with the roads plugin laying a graded road
// 4 wide: as fakeSite plans it and noting its grades in shaper, or, over a relief, as package site
// plans it and levelling it with the topography
type field struct {
	t      *testing.T
	g      grid.Grid
	w      *world.Plugin
	m      *motion.Plugin
	col    *collision.Plugin
	board  *board.Plugin
	topo   *topography.Plugin // nil without a relief
	roads  *roads.Plugin
	site   *fakeSite
	shaper *shaper
	ecs    *goke.ECS
	drawn  []render.Source
}

func newField(t *testing.T, cfg roads.Config) *field {
	t.Helper()
	f := newBoard(t)
	f.site = &fakeSite{g: f.g}
	f.shaper = &shaper{graded: map[cell.ID][]float32{}}
	f.make(cfg, f.shaper)
	f.roads.PlanWith(f.site.router, f.site.plan)
	return f
}

// newRelief is a field whose ground heights raises, a topography's
func newRelief(t *testing.T, cfg roads.Config, heights func(geom.Vec) float32) *field {
	t.Helper()
	f := newBoard(t)
	f.topo = topography.NewPlugin(f.w, f.board, nil, topography.Config{})
	f.topo.Write().Seed(heights)
	f.make(cfg, f.topo.Write())
	return f
}

func newBoard(t *testing.T) *field {
	f := &field{t: t, g: grid.DefaultGrids{}.Square(8, 6, 10)}
	f.w = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 80, Height: 60},
		Entities: world.EntitiesCfg{MaxCount: 16, MinSize: 1, MaxSize: 10},
		Heights:  true,
	})
	f.m = motion.NewPlugin(f.w)
	f.col = collision.NewPlugin(f.w, f.m)
	f.board = board.NewPlugin(f.g, &cell.MultipleOccupancy{}, f.w).
		WithMotion(f.m, cell.DefaultHiking).
		WithCollision(f.col)
	f.board.Write().Board().SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	f.board.Read(0).CellKinds().Define(roadCell,
		cell.Kind{Pace: 1, Allows: cell.Land, Graded: true})
	return f
}

func (f *field) make(cfg roads.Config, s ground.Shaper) {
	f.t.Helper()
	if cfg.Kind == "" {
		cfg.Kind = roadCell
	}
	if cfg.Width == 0 {
		cfg.Width = 4
	}
	f.roads = roads.NewPlugin(f.w, f.board, cfg).WithShaper(s)
	if err := f.w.Carry(f.roads); err != nil {
		f.t.Fatal(err)
	}
}

func (f *field) start() {
	f.t.Helper()
	ctx := &installer{ecs: goke.New()}
	made := []plugin.Plugin{f.w, f.m, f.col, f.board}
	if f.topo != nil {
		made = append(made, f.topo)
	}
	made = append(made, f.roads)
	for _, p := range made {
		if err := p.Install(ctx); err != nil {
			f.t.Fatal(err)
		}
	}
	if err := ceremony.Deliver(ctx.hosts, f.w.Kinds().Played()...); err != nil {
		f.t.Fatal(err)
	}
	for _, p := range made {
		if populated, ok := p.(plugin.Populator); ok {
			if err := populated.Populate(); err != nil {
				f.t.Fatal(err)
			}
		}
	}
	f.drawn = ctx.drawn
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		settling := f.w.Write().Clock().Settling(rc)
		for _, p := range made {
			p.RunPlan(settling, d)
		}
		settling.Sync()
		f.w.Write().Clock().Replay(rc, d)
	})
	f.ecs = ctx.ecs
}

func (f *field) give(cmd command.Command) {
	f.t.Helper()
	if !f.w.Write().Carrier().Put(1, cmd) {
		f.t.Fatalf("nobody takes %T", cmd)
	}
	f.ecs.Tick(time.Second / 60)
}

func (f *field) ticks(n int) {
	for range n {
		f.ecs.Tick(time.Second / 60)
	}
}

func (f *field) cell(x, y uint32) cell.ID { return f.g.At(x, y) }

func (f *field) at(x, y uint32) geom.Vec { return f.g.CellCenter(f.g.At(x, y)) }

func (f *field) way(x, y uint32) cell.Way { return f.board.Read(0).Board().Way(f.cell(x, y)) }

func (f *field) link(from, to cell.ID) cell.Links {
	l, _ := grid.Link(f.g, from, to)
	return l
}

func (f *field) paused() bool { return f.w.Read(0).Clock().State().Paused }

// levelled has every plan move c's corners from 0 to h
func (f *field) levelled(c cell.ID, h float32, e *site.Estimate) {
	e.Graded = append(e.Graded, c)
	for _, at := range f.g.CellOutline(c, nil) {
		e.Corners = append(e.Corners, site.CornerMove{At: at, To: h})
	}
}

func TestTool_MarksAndAimsPlanTheRoadAndTheSecondMarkAsks(t *testing.T) {
	f := newField(t, roads.Config{Levelling: true})
	f.start()
	f.give(roads.Mark{At: f.at(1, 2)})
	if _, _, ok := f.roads.Read(0).Estimate(1); ok {
		t.Fatal("one point marked holds a plan, want none")
	}
	f.give(roads.Aim{At: f.at(4, 2)})
	want := []cell.ID{f.cell(1, 2), f.cell(2, 2), f.cell(3, 2), f.cell(4, 2)}
	if e, _, ok := f.roads.Read(0).Estimate(1); !ok || !slices.Equal(e.Cells, want) {
		t.Fatalf("aimed at [4, 2], the plan runs %v (%v), want %v", e.Cells, ok, want)
	}
	if ask, _, _ := f.roads.Says(); ask != "" {
		t.Errorf("asks %q while the road is aimed, want nothing till it is marked", ask)
	}
	f.give(roads.Mark{At: f.at(4, 2)})
	ask, estimate, reason := f.roads.Says()
	if ask != "Build the road?" || !strings.Contains(estimate, "It runs 4 cells") || reason != "" {
		t.Errorf("marked at [4, 2], says %q, %q, %q; want it asking about a road of 4 cells",
			ask, estimate, reason)
	}
	leg := [2]cell.ID{f.cell(1, 2), f.cell(4, 2)}
	if len(f.site.legs) != 2 || f.site.legs[0] != leg || f.site.legs[1] != leg {
		t.Errorf("routed %v, want the leg from [1, 2] to [4, 2] aimed, then marked", f.site.legs)
	}
	if l := f.site.levelling; len(l) == 0 || !l[len(l)-1] {
		t.Error("planned without levelling, want with it: the Config's start and a shaper given")
	}
}

func TestTool_BuildGradesTheGroundThenLaysTheRoad(t *testing.T) {
	f := newField(t, roads.Config{
		Levelling: true,
		Tariff:    func(e site.Estimate) float32 { return e.Length + e.Fill },
	})
	f.site.shape = func(e *site.Estimate) {
		f.levelled(f.cell(2, 2), 5, e)
		e.Fill = 125
	}
	f.start()
	f.give(roads.Mark{At: f.at(1, 2)})
	f.give(roads.Mark{At: f.at(3, 2)})
	f.give(roads.Build{})
	f.ticks(2)
	// every cell of the road graded, [2, 2]'s corners at 5 and those its neighbours share
	for x, want := range map[uint32][]float32{1: {0, 5, 5, 0}, 2: {5, 5, 5, 5}, 3: {5, 0, 0, 5}} {
		if got := f.shaper.graded[f.cell(x, 2)]; !slices.Equal(got, want) {
			t.Errorf("[%d, 2] graded to %v, want %v", x, got, want)
		}
	}
	if len(f.shaper.graded) != 3 {
		t.Errorf("graded %v, want the road's three cells", f.shaper.graded)
	}
	west, east := f.link(f.cell(2, 2), f.cell(1, 2)), f.link(f.cell(2, 2), f.cell(3, 2))
	for x, links := range map[uint32]cell.Links{1: east, 2: west | east, 3: west} {
		way := f.way(x, 2)
		if !way.Runs() || !way.Kind.Graded || way.Width != 4 || way.Links != links {
			t.Errorf("[%d, 2] carries %+v, want a graded road 4 wide running %08b", x, way, links)
		}
	}
	if _, _, ok := f.roads.Read(0).Estimate(1); ok {
		t.Error("the hand still holds a plan after the road was built, want it empty")
	}
}

// corners is the heights of the ground at the corners of the cell [x, y], in CellOutline's order
func (f *field) corners(x, y uint32) []float32 {
	var hs []float32
	for _, at := range f.g.CellOutline(f.cell(x, y), nil) {
		hs = append(hs, f.board.Read(0).Heights().At(at))
	}
	return hs
}

func TestTool_ARoadsEndExtendedUpARiseTurnsIntoARampAndIsBuilt(t *testing.T) {
	// level ground to x 40, 3 higher from x 50: more than a road climbs across one cell
	f := newRelief(t, roads.Config{
		Levelling: true,
		Rules: site.Rules{
			Cut:       6,
			Fill:      6,
			Earth:     1000,
			Slope:     ground.RoadSlope,
			Crossings: true,
			Ends:      true,
			Metre:     1,
		},
	}, func(p geom.Vec) float32 {
		if p.X > 45 {
			return 3
		}
		return 0
	})
	f.start()
	road, _ := f.board.Read(0).CellKinds().Get(roadCell)
	west, east := f.link(f.cell(2, 2), f.cell(1, 2)), f.link(f.cell(2, 2), f.cell(3, 2))
	write := f.board.Write().Board()
	for x, links := range map[uint32]cell.Links{1: east, 2: west | east, 3: west} {
		write.SetWay(f.cell(x, 2), cell.Way{Kind: road, Width: 4, Links: links})
	}
	f.give(roads.Mark{At: f.at(3, 2)})
	f.give(roads.Mark{At: f.at(6, 2)})
	if _, _, reason := f.roads.Says(); reason != "" {
		t.Fatalf("the road from [3, 2], the end, up the rise to [6, 2] refused: %s", reason)
	}
	f.give(roads.Build{})
	f.ticks(2)
	if built := f.roads.Read(0).Built(); len(built) != 1 {
		t.Fatalf("built %d roads, want the one up the rise", len(built))
	}
	for x, links := range map[uint32]cell.Links{
		2: west | east,
		3: west | east,
		4: west | east,
		5: west | east,
		6: west,
	} {
		way, hs := f.way(x, 2), f.corners(x, 2)
		if !way.Runs() || way.Links != links {
			t.Errorf("[%d, 2] carries %+v, want a road running %08b", x, way, links)
		}
		if !ground.RoadFits(f.g, f.cell(x, 2), hs, links) {
			t.Errorf("[%d, 2] lies at %v, as no road may run %08b", x, hs, links)
		}
	}
	// TL and TR of the end, its west edge staying with the road beside it
	if hs := f.corners(3, 2); hs[0] != 0 || hs[1] <= 0.1 {
		t.Errorf("the end at [3, 2] lies at %v, want it a ramp up east from 0", hs)
	}
}

type deckRow struct {
	At    cell.ID
	Links cell.Links
}

// bridged stands a bridge over the water of row 2 from x 3 to x 6, its heads at the ends, each
// piece a Graded floor carrying land along the bridge
func (f *field) bridged() {
	f.t.Helper()
	water := cell.Kind{Pace: 1, Allows: cell.Water}
	west, east := f.link(f.cell(4, 2), f.cell(3, 2)), f.link(f.cell(4, 2), f.cell(5, 2))
	decks := board.NewEntities(f.board, board.Shape{Size: 10},
		func(r deckRow) cell.ID { return r.At })
	decks.Define("deck", comp.Load(func(r deckRow) cell.Floor {
		return cell.Floor{Allows: cell.Land, Thickness: 0.5, Pace: 1, Graded: true,
			Links: r.Links, Width: 6}
	}))
	f.w.Write().Seed(
		decks.Named("deck").Entry(deckRow{At: f.cell(3, 2), Links: east}),
		decks.Named("deck").Entry(deckRow{At: f.cell(4, 2), Links: west | east}),
		decks.Named("deck").Entry(deckRow{At: f.cell(5, 2), Links: west | east}),
		decks.Named("deck").Entry(deckRow{At: f.cell(6, 2), Links: west}),
	)
	f.start()
	for x := uint32(4); x <= 5; x++ {
		f.board.Write().Board().Set(f.cell(x, 2), water)
	}
}

func TestTool_ARoadFromABridgesHeadReachesTheHeadAndLeadsOntoTheSpan(t *testing.T) {
	f := newRelief(t, roads.Config{
		Levelling: true,
		Rules: site.Rules{
			Earth: 1000, Slope: ground.RoadSlope, Crossings: true, Decks: true, Ends: true,
			Metre: 1,
		},
	}, func(geom.Vec) float32 { return 0 })
	f.bridged()
	west, east := f.link(f.cell(2, 2), f.cell(1, 2)), f.link(f.cell(2, 2), f.cell(3, 2))
	f.give(roads.Mark{At: f.at(3, 2)})
	f.give(roads.Mark{At: f.at(1, 2)})
	if _, _, reason := f.roads.Says(); reason != "" {
		t.Fatalf("the road from the head [3, 2] to [1, 2] refused: %s", reason)
	}
	f.give(roads.Build{})
	f.ticks(2)
	if built := f.roads.Read(0).Built(); len(built) != 1 || built[0].Estimate.Laid != 3 {
		t.Fatalf("built %+v, want one road of 3 cells laid anew", built)
	}
	for x, links := range map[uint32]cell.Links{1: east, 2: west | east, 3: west | east} {
		if way := f.way(x, 2); !way.Runs() || way.Links != links {
			t.Errorf("[%d, 2] carries %+v, want a road running %08b", x, way, links)
		}
	}
	// over the bridge to the far bank: the heads take the road, the spans carry it
	f.give(roads.Mark{At: f.at(1, 2)})
	f.give(roads.Mark{At: f.at(7, 2)})
	if _, _, reason := f.roads.Says(); reason != "" {
		t.Fatalf("the road from [1, 2] over the bridge to [7, 2] refused: %s", reason)
	}
	f.give(roads.Build{})
	f.ticks(2)
	if built := f.roads.Read(0).Built(); len(built) != 1 || built[0].Estimate.Laid != 2 {
		t.Fatalf("built %+v, want one road laying the far head and [7, 2] anew", built)
	}
	for x, links := range map[uint32]cell.Links{3: west | east, 6: west | east, 7: west} {
		if way := f.way(x, 2); !way.Runs() || way.Links != links {
			t.Errorf("[%d, 2] carries %+v, want a road running %08b", x, way, links)
		}
	}
	for x := uint32(4); x <= 5; x++ {
		if way := f.way(x, 2); way.Runs() {
			t.Errorf("[%d, 2], a span, carries %+v, want no way on it", x, way)
		}
	}
}

func TestRead_BuiltHandsEveryReceiptOnce(t *testing.T) {
	f := newField(t, roads.Config{Tariff: func(e site.Estimate) float32 { return e.Length }})
	f.start()
	f.give(roads.Mark{At: f.at(1, 2)})
	f.give(roads.Mark{At: f.at(3, 2)})
	f.give(roads.Build{})
	f.ticks(1)
	built := f.roads.Read(0).Built()
	want := []cell.ID{f.cell(1, 2), f.cell(2, 2), f.cell(3, 2)}
	if len(built) != 1 || built[0].Issuer != 1 || built[0].Price != 20 ||
		!slices.Equal(built[0].Estimate.Cells, want) {
		t.Fatalf("built %+v, want one receipt of player 1's road %v at 20", built, want)
	}
	if again := f.roads.Read(0).Built(); len(again) != 0 {
		t.Errorf("read again, built %+v, want nothing new", again)
	}
}

func TestTool_BuildRefusesARoadWhoseGroundChangedSinceItWasPlanned(t *testing.T) {
	f := newField(t, roads.Config{})
	f.start()
	f.give(roads.Mark{At: f.at(1, 2)})
	f.give(roads.Mark{At: f.at(3, 2)})
	f.site.shape = func(e *site.Estimate) { e.Cut = 40 }
	f.give(roads.Build{})
	f.ticks(2)
	if f.way(2, 2).Runs() {
		t.Fatal("built a road whose ground changed since it was planned")
	}
	ask, estimate, reason := f.roads.Says()
	if !strings.Contains(reason, "the ground changed since") {
		t.Errorf("says %q of the refusal, want the ground changed", reason)
	}
	if ask == "" || !strings.Contains(estimate, "digging 40") {
		t.Errorf("asks %q of %q, want the plan made anew still asked about", ask, estimate)
	}
	f.give(roads.Build{})
	f.ticks(2)
	if !f.way(2, 2).Runs() {
		t.Error("built nothing once the plan made anew was confirmed, want the road")
	}
}

func TestTool_BuildWaitsWhileSomethingStandsWhereTheRoadLevels(t *testing.T) {
	f := newField(t, roads.Config{Levelling: true})
	f.site.shape = func(e *site.Estimate) {
		f.levelled(f.cell(2, 2), 3, e)
		e.Blocked = []cell.ID{f.cell(2, 2)}
	}
	f.start()
	f.give(roads.Mark{At: f.at(1, 2)})
	f.give(roads.Mark{At: f.at(3, 2)})
	f.give(roads.Build{})
	f.ticks(2)
	if f.way(2, 2).Runs() || len(f.shaper.graded) != 0 {
		t.Fatalf("built %v, graded %v, while a unit stands where the road levels; want nothing",
			f.way(2, 2), f.shaper.graded)
	}
	if _, _, reason := f.roads.Says(); !strings.Contains(reason, "under a unit") {
		t.Errorf("says %q of the refusal, want the unit standing in the way", reason)
	}
}

func TestTool_CancelBuildsNothingAndKeepsThePointsTheLastOfWhichAMarkTakesBack(t *testing.T) {
	f := newField(t, roads.Config{})
	f.start()
	f.give(roads.Mark{At: f.at(1, 2)})
	f.give(roads.Mark{At: f.at(3, 2)})
	f.give(roads.Cancel{})
	f.give(roads.Build{})
	f.ticks(2)
	if f.way(2, 2).Runs() {
		t.Fatal("built a road cancelled")
	}
	if ask, _, _ := f.roads.Says(); ask != "" {
		t.Errorf("asks %q after Cancel, want nothing", ask)
	}
	if _, _, ok := f.roads.Read(0).Estimate(1); !ok {
		t.Fatal("the plan is gone after Cancel, want the points still marked")
	}
	f.give(roads.Mark{At: f.at(3, 2)})
	if _, _, ok := f.roads.Read(0).Estimate(1); ok {
		t.Fatal("a plan stands after the last of two points was taken back, want none")
	}
	f.give(roads.Mark{At: f.at(3, 4)})
	f.give(roads.Build{})
	f.ticks(2)
	bend := f.link(f.cell(3, 2), f.cell(2, 2)) | f.link(f.cell(3, 2), f.cell(3, 3))
	if way := f.way(3, 2); way.Links != bend || !f.way(3, 4).Runs() {
		t.Errorf("[3, 2] runs %08b, [3, 4] %v; want the road from [1, 2] bending there to [3, 4]",
			way.Links, f.way(3, 4).Runs())
	}
}

func TestTool_APieceJoinsTheRoadsEndingAtItsCell(t *testing.T) {
	for _, crossings := range []bool{true, false} {
		f := newField(t, roads.Config{Rules: site.Rules{Crossings: crossings}})
		f.start()
		road, _ := f.board.Read(0).CellKinds().Get(roadCell)
		c := f.cell(3, 2)
		write := f.board.Write().Board()
		for _, w := range []struct{ at, toward cell.ID }{
			{f.cell(2, 2), c},            // ends toward c
			{f.cell(3, 1), c},            // ends toward c
			{f.cell(4, 2), f.cell(5, 2)}, // runs past c
		} {
			write.SetWay(w.at, cell.Way{Kind: road, Width: 4, Links: f.link(w.at, w.toward)})
		}
		f.give(roads.Piece{At: f.at(3, 2), Toward: f.at(3, 3)})
		f.give(roads.Build{})
		f.ticks(2)
		want := f.link(c, f.cell(3, 3))
		if crossings {
			want |= f.link(c, f.cell(2, 2)) | f.link(c, f.cell(3, 1))
		}
		if got := f.way(3, 2).Links; got != want {
			t.Errorf("crossings %v: the piece runs %08b, want %08b", crossings, got, want)
		}
		if f.way(3, 3).Runs() {
			t.Errorf("crossings %v: a road laid in the cell the piece runs toward, want in its own",
				crossings)
		}
	}
}

type villageRow struct{ At cell.ID }

func TestGround_TellsWaterFromAFordAndHoldsWhatStandsFixed(t *testing.T) {
	f := newField(t, roads.Config{})
	kinds := f.board.Read(0).CellKinds()
	kinds.Define("water", cell.Kind{Pace: 1, Allows: cell.Water})
	kinds.Define("ford", cell.Kind{Pace: 0.5, Allows: cell.Land | cell.Water})
	kinds.Define("track", cell.Kind{Pace: 1, Allows: cell.Land})
	villages := board.NewEntities(f.board, board.Shape{Size: 8},
		func(r villageRow) cell.ID { return r.At })
	villages.Define("village")
	f.w.Write().Seed(villages.Named("village").Entry(villageRow{At: f.cell(5, 3)}))
	f.start()
	write := f.board.Write().Board()
	for name, at := range map[string]cell.ID{"water": f.cell(1, 1), "ford": f.cell(2, 1)} {
		k, _ := kinds.Get(name)
		write.Set(at, k)
	}
	east := f.link(f.cell(1, 4), f.cell(2, 4))
	for name, at := range map[string]cell.ID{roadCell: f.cell(1, 4), "track": f.cell(2, 4)} {
		k, _ := kinds.Get(name)
		write.SetWay(at, cell.Way{Kind: k, Width: 4, Links: east})
	}
	g := f.roads.Ground()
	for _, c := range []struct {
		what                    string
		at                      cell.ID
		takes, wet, ford, fixed bool
		road                    cell.Links
	}{
		{"earth", f.cell(6, 1), true, false, false, false, 0},
		{"water", f.cell(1, 1), false, true, false, true, 0},
		{"a ford", f.cell(2, 1), true, false, true, true, 0},
		{"a village", f.cell(5, 3), true, false, false, true, 0},
		{"a graded road", f.cell(1, 4), true, false, false, false, east},
		{"a track", f.cell(2, 4), true, false, false, false, east},
	} {
		takes, wet, ford := g.Takes(c.at), g.Wet(c.at), g.Ford(c.at)
		fixed, road := g.Fixed(c.at), g.Road(c.at)
		if takes != c.takes || wet != c.wet || ford != c.ford || fixed != c.fixed ||
			road != c.road {
			t.Errorf("%s: takes a road %v, wet %v, a ford %v, fixed %v, a road %08b; "+
				"want %v, %v, %v, %v, %08b", c.what, takes, wet, ford, fixed, road,
				c.takes, c.wet, c.ford, c.fixed, c.road)
		}
	}
}

func TestTool_TakenInHandHoldsTheGameInTheTacticalPause(t *testing.T) {
	f := newField(t, roads.Config{InPause: true})
	f.start()
	f.give(roads.Aim{At: f.at(1, 2)})
	f.ticks(1)
	if !f.paused() {
		t.Fatal("the game runs with a road in hand, want it held in the tactical pause")
	}
	f.give(roads.Aim{At: f.at(2, 2)})
	f.ticks(1)
	if !f.paused() {
		t.Error("the game runs again as the road in hand is aimed, want it held still")
	}
}

func TestRules_ARoleThePluginPlaysRefusesTheRoadInThePause(t *testing.T) {
	const why = "the treasury is empty"
	for _, plays := range []bool{true, false} {
		f := newField(t, roads.Config{})
		f.w.Roles().Define("builder", act.Branch[roads.Estimated]{
			Name: why,
			When: cond.Self.If(cond.Always),
			Then: act.Self.Do(roads.Refuse{Why: why}),
		})
		if plays {
			f.roads.Plays(f.w.Roles().Named("builder"))
		}
		f.start()
		f.give(world.Pause{})
		if !f.paused() {
			t.Fatal("the game runs, want it in the tactical pause")
		}
		f.give(roads.Mark{At: f.at(1, 2)})
		f.give(roads.Mark{At: f.at(3, 2)})
		if _, estimate, _ := f.roads.Says(); strings.Contains(estimate, why) != plays {
			t.Errorf("played by the plugin %v: the estimate says %q", plays, estimate)
		}
		f.give(roads.Build{})
		f.give(world.Pause{})
		f.ticks(2)
		if got := f.way(2, 2).Runs(); got == plays {
			t.Errorf("played by the plugin %v: built %v", plays, got)
		}
	}
}

func TestRules_ARoleThePluginPlaysHearsOfTheRoadsBuilt(t *testing.T) {
	for _, levels := range []bool{true, false} {
		f := newField(t, roads.Config{Levelling: true})
		fx := f.w.Write().Effects()
		fx.Define("roadworks", effect.Spec{effect.Lasts(time.Hour)})
		roadworks := fx.Named("roadworks")
		f.w.Roles().Define("builder", act.Branch[roads.Built]{
			Name: "roadworks",
			When: cond.Self.If(roads.Levelled()),
			Then: act.Self.Put(roadworks),
		})
		f.roads.Plays(f.w.Roles().Named("builder"))
		if levels {
			f.site.shape = func(e *site.Estimate) { f.levelled(f.cell(2, 2), 5, e) }
		}
		f.start()
		f.give(roads.Mark{At: f.at(1, 2)})
		f.give(roads.Mark{At: f.at(3, 2)})
		f.give(roads.Build{})
		f.ticks(2)
		if !f.way(2, 2).Runs() {
			t.Fatalf("levelling %v: the road was not built", levels)
		}
		if got := f.w.Read(0).Effects().Has(f.roads.Entity(), roadworks); got != levels {
			t.Errorf("a road levelling %v built: the plugin under roadworks %v", levels, got)
		}
	}
}

func TestSettings_LevellingSwitchedPlansAnewAndIsSavedWithTheGame(t *testing.T) {
	saved := newField(t, roads.Config{Levelling: true})
	saved.start()
	saved.give(roads.Mark{At: saved.at(1, 2)})
	saved.give(roads.Aim{At: saved.at(3, 2)})
	saved.give(roads.Levelling{On: false})
	if saved.roads.Read(0).Levelling() {
		t.Fatal("levelling after Levelling{On: false}, want not")
	}
	if l := saved.site.levelling; len(l) < 2 || !l[0] || l[len(l)-1] {
		t.Errorf("planned levelling %v, want with it, then anew without as it was switched off", l)
	}
	if r := saved.site.routers; !slices.Equal(r, []bool{true, false}) {
		t.Errorf("routers made levelling %v, want one levelling, then one not", r)
	}
	loaded := newField(t, roads.Config{Levelling: true})
	loaded.load(saved)
	if loaded.roads.Read(0).Levelling() {
		t.Error("levelling after a load of a game saved without it, want not")
	}
}

// load decodes into f's roads what from's give to be saved, as the engine does with every
// Serializable, and has f's take it (Restore)
func (f *field) load(from *field) {
	f.t.Helper()
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	for _, v := range from.roads.Serializable().Persisted() {
		if err := enc.Encode(v); err != nil {
			f.t.Fatal(err)
		}
	}
	dec := gob.NewDecoder(&buf)
	for _, v := range f.roads.Serializable().Persisted() {
		if err := dec.Decode(v); err != nil {
			f.t.Fatal(err)
		}
	}
	f.roads.Restore()
}

func TestSave_ARoadOrderedInThePauseIsDrawnWaitingAndBuiltAtTheFirstStepAfterALoad(t *testing.T) {
	saved := newField(t, roads.Config{
		Levelling: true,
		Tariff:    func(e site.Estimate) float32 { return e.Length + e.Fill },
	})
	saved.site.shape = func(e *site.Estimate) {
		saved.levelled(saved.cell(2, 2), 5, e)
		e.Fill = 125
	}
	saved.start()
	saved.give(world.Pause{})
	saved.give(roads.Mark{At: saved.at(1, 2)})
	saved.give(roads.Mark{At: saved.at(3, 2)})
	saved.give(roads.Build{})
	saved.ticks(2)
	if saved.way(2, 2).Runs() || len(saved.roads.Res.Orders) != 1 {
		t.Fatalf("in the tactical pause: a road at [2, 2] %v, %d ordered; want one ordered and "+
			"waiting for a step", saved.way(2, 2).Runs(), len(saved.roads.Res.Orders))
	}
	// no Tariff and no levelling of its own: the price and the corners are the save's
	loaded := newField(t, roads.Config{})
	loaded.load(saved)
	if n := len(loaded.roads.Res.Orders); n != 1 {
		t.Fatalf("loaded %d roads ordered, want the one confirmed in the pause", n)
	}
	loaded.start()
	loaded.give(world.Pause{})
	if loaded.way(2, 2).Runs() {
		t.Fatal("built in the tactical pause after the load, want the road waiting for a step")
	}
	n, band := 0, false
	loaded.compose(func(at render.At) {
		n++
		band = band || f32.Abs(at.Z()-5.9) < 1e-4
	})
	if n == 0 || !band {
		t.Errorf("the road ordered drawn in %d pieces, its band at 5.9 %v; want it waiting over "+
			"[2, 2] levelled to 5", n, band)
	}
	loaded.give(world.Pause{})
	for x, want := range map[uint32][]float32{1: {0, 5, 5, 0}, 2: {5, 5, 5, 5}, 3: {5, 0, 0, 5}} {
		if got := loaded.shaper.graded[loaded.cell(x, 2)]; !slices.Equal(got, want) {
			t.Errorf("[%d, 2] graded to %v at the first step after the pause, want %v", x, got,
				want)
		}
	}
	if !loaded.way(2, 2).Runs() || len(loaded.roads.Res.Orders) != 0 {
		t.Fatalf("at the first step after the pause: a road at [2, 2] %v, %d still ordered; want "+
			"it built", loaded.way(2, 2).Runs(), len(loaded.roads.Res.Orders))
	}
	built := loaded.roads.Read(0).Built()
	want := []cell.ID{loaded.cell(1, 2), loaded.cell(2, 2), loaded.cell(3, 2)}
	if len(built) != 1 || built[0].Issuer != 1 || built[0].Price != 145 ||
		!slices.Equal(built[0].Estimate.Cells, want) {
		t.Errorf("built %+v, want one receipt of player 1's road %v at 145", built, want)
	}
}

func TestSave_AReceiptNotReadYetComesBackWithTheGameLoaded(t *testing.T) {
	saved := newField(t, roads.Config{Tariff: func(e site.Estimate) float32 { return e.Length }})
	saved.start()
	saved.give(roads.Mark{At: saved.at(1, 2)})
	saved.give(roads.Mark{At: saved.at(3, 2)})
	saved.give(roads.Build{})
	saved.ticks(1)
	if !saved.way(2, 2).Runs() {
		t.Fatal("the road was not built")
	}
	loaded := newField(t, roads.Config{})
	loaded.load(saved)
	built := loaded.roads.Read(0).Built()
	want := []cell.ID{loaded.cell(1, 2), loaded.cell(2, 2), loaded.cell(3, 2)}
	if len(built) != 1 || built[0].Issuer != 1 || built[0].Price != 20 ||
		!slices.Equal(built[0].Estimate.Cells, want) {
		t.Errorf("read after the load, built %+v; want the receipt of player 1's road %v at 20",
			built, want)
	}
}

// compose has the roads' own renderer, installed last, publish and compose a frame, every piece
// it lays handed to each
func (f *field) compose(each func(at render.At)) {
	f.t.Helper()
	src := f.drawn[len(f.drawn)-1]
	cam := cameratest.NewFromSpace(80, 60, 0)
	src.(plugin.Publisher).Publish()
	src.(plugin.Publisher).Swap()
	var fr render.Frame
	fr.Reset(cam)
	src.Compose(&fr, cam)
	fr.Each(func(at render.At, _ []render.Vertex) { each(at) })
}

func TestRenderer_TheRoadInHandLiesOverTheGroundAsItWillBeLevelled(t *testing.T) {
	f := newField(t, roads.Config{Levelling: true})
	f.site.shape = func(e *site.Estimate) { f.levelled(f.cell(2, 2), 5, e) }
	f.start()
	f.give(roads.Mark{At: f.at(1, 2)})
	f.give(roads.Mark{At: f.at(3, 2)})
	band, outline, n := false, false, 0
	f.compose(func(at render.At) {
		n++
		if at.Top() || at.Z() < 0.9 || at.Z() > 6 {
			t.Errorf("a piece of the road in hand lies at %v, want over the ground, under 6", at)
		}
		band = band || f32.Abs(at.Z()-5.9) < 1e-4
		outline = outline || f32.Abs(at.Z()-5.92) < 1e-4
	})
	if n == 0 {
		t.Fatal("the road in hand is not drawn")
	}
	if !band || !outline {
		t.Errorf("the band at 5.9 %v, the levelled cell's outline at 5.92 %v; want both over "+
			"[2, 2] levelled to 5", band, outline)
	}
}
