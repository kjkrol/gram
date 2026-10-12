package board_test

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

var water = cell.Kind{Name: cell.Named("water"), Cost: 1, Allows: cell.Water}

type deckRow struct {
	At    cell.ID
	Floor cell.Floor
	Rail  cell.Rail
}

type decked struct {
	w   *world.Plugin
	p   *board.Plugin
	ecs *goke.ECS
	sag effect.Effect
}

func newDecked(t *testing.T, g grid.Grid, w, h uint32, heights bool, bridges board.Bridges,
	rows ...deckRow) *decked {
	t.Helper()
	d := &decked{}
	d.w = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: w, Height: h},
		Entities: world.EntitiesCfg{MaxCount: 16, MinSize: 1, MaxSize: 10},
		Heights:  heights,
	})
	d.p = board.NewPlugin(g, &cell.MultipleOccupancy{}, d.w).WithBridges(bridges)
	d.p.Board().SetAll(water)
	d.w.Effects().Define("sag", effect.Spec{effect.Alter(func(f *cell.Floor) { f.Height += 5 })})
	d.sag = d.w.Effects().Named("sag")
	decks := board.NewEntities(d.p, board.Shape{Size: 1}, func(r deckRow) cell.ID { return r.At })
	decks.Define("deck",
		comp.Load(func(r deckRow) cell.Floor { return r.Floor }),
		comp.Load(func(r deckRow) cell.Rail { return r.Rail }))
	for i, r := range rows {
		e := decks.Named("deck").Entry(r)
		if i == 0 {
			e = e.Named("first")
		}
		d.w.Seed(e)
	}
	ctx := boardtest.NewInstallCtx()
	if err := d.w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := d.p.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := d.w.Populate(); err != nil {
		t.Fatal(err)
	}
	d.ecs = ctx.ECS()
	d.ecs.Setup(ctx.Systems()...)
	d.ecs.SetPlan(func(rc goke.RunCtx, dt time.Duration) {
		d.w.RunPlan(rc, dt)
		d.p.RunPlan(rc, dt)
		rc.Sync()
		d.w.Clock().Replay(rc, dt)
	})
	return d
}

func (d *decked) tick() { d.ecs.Tick(time.Second / 60) }

func (d *decked) over(p geom.Vec) []cell.ID {
	var out []cell.ID
	d.p.Board().DecksOver(p, func(c cell.ID, _ cell.Floor) { out = append(out, c) })
	return out
}

func band(g grid.Grid, way [2]int, width float64, height float64, cells ...cell.ID) []deckRow {
	ahead, _ := grid.Direction(g, way[0], way[1])
	back, _ := grid.Direction(g, -way[0], -way[1])
	var rows []deckRow
	for _, c := range cells {
		rows = append(rows, deckRow{At: c,
			Floor: cell.Floor{Allows: cell.Land, Height: height, Links: 1<<ahead | 1<<back,
				Width: width},
			Rail: cell.Rail{Keeps: cell.Land}})
	}
	return rows
}

func bridgeRows(g grid.Grid, way [2]int, width, height float64, cells ...cell.ID) []deckRow {
	rows := band(g, way, width, height, cells...)
	ahead, _ := grid.Direction(g, way[0], way[1])
	back, _ := grid.Direction(g, -way[0], -way[1])
	for i := range rows {
		rows[i].Floor.Graded = true
	}
	rows[0].Floor.Links, rows[0].Floor.Height = 1<<ahead, 0
	last := len(rows) - 1
	rows[last].Floor.Links, rows[last].Floor.Height = 1<<back, 0
	return rows
}

func TestDecks_ABandSlantwiseCutsTheCorners(t *testing.T) {
	g := grid.DefaultGrids{}.Square(5, 5, 10)
	d := newDecked(t, g, 50, 50, false, board.Bridges{},
		band(g, [2]int{1, 1}, 4, 0, g.At(1, 1), g.At(2, 2), g.At(3, 3))...)
	d.tick()
	brd := d.p.Board()
	for _, c := range []struct {
		name string
		at   geom.Vec
		want []cell.ID
	}{
		{"its middle", geom.NewVec(25, 25), []cell.ID{g.At(2, 2)}},
		{"by the corner, in the cell beside", geom.NewVec(20.5, 19), []cell.ID{g.At(1, 1)}},
		{"in the middle of the cell beside", geom.NewVec(25, 15), nil},
		{"beside it in its cell", geom.NewVec(28, 22), nil},
	} {
		if got := d.over(c.at); !slices.Equal(got, c.want) {
			t.Errorf("%s: decks of %v, want %v", c.name, got, c.want)
		}
	}
	feet := math.Inf(-1)
	if k := brd.KindFor(g.At(2, 1), geom.NewVec(20.5, 19), feet, 0); !k.Admits(cell.Land) {
		t.Error("by the corner on the band, in the cell beside, a walker has fallen in")
	}
	if k := brd.KindFor(g.At(2, 2), geom.NewVec(28, 22), feet, 0); k.Admits(cell.Land) {
		t.Error("beside the band in its cell a walker has not fallen in")
	}
	for _, c := range []struct {
		from, to cell.ID
		reaches  bool
	}{
		{g.At(1, 1), g.At(2, 2), true},
		{g.At(2, 2), g.At(1, 1), true},
		{g.At(1, 1), g.At(2, 1), false},
		{g.At(2, 1), g.At(1, 1), false},
	} {
		if got := brd.Reaches(c.from, c.to); got != c.reaches {
			t.Errorf("from %v to %v it reaches %v, want %v", c.from, c.to, got, c.reaches)
		}
	}
	if !brd.Along(g.At(1, 1), g.At(2, 2)) {
		t.Error("a step along the band is not along it")
	}
	for _, c := range []struct {
		name     string
		from, to cell.ID
		fenced   bool
	}{
		{"along it", g.At(1, 1), g.At(2, 2), false},
		{"across it through the corner", g.At(2, 1), g.At(1, 2), true},
		{"onto it from the side", g.At(2, 1), g.At(2, 2), true},
		{"beside it", g.At(1, 0), g.At(2, 1), false},
	} {
		if got := brd.Fenced(c.from, c.to, cell.Land, false); got != c.fenced {
			t.Errorf("a step %s: fenced %v, want %v", c.name, got, c.fenced)
		}
	}
}

func (d *decked) lies() ([]world.Lie, []geom.Vec) {
	var base goke.Comp[world.Base]
	var lie goke.Comp[world.Lie]
	var q *goke.Query
	d.ecs.RegSys(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&base, &lie).Build()
	}})
	var lies []world.Lie
	var dirs []geom.Vec
	for q.All(); q.Next(); {
		cur := q.Cursor()
		for i := range cur.IDs {
			lies = append(lies, lie.Slice(cur)[i])
			dirs = append(dirs, base.Slice(cur)[i].Vel.Dir)
		}
	}
	return lies, dirs
}

func TestDecks_ABandLiesAlongItself(t *testing.T) {
	g := grid.DefaultGrids{}.Square(5, 5, 10)
	d := newDecked(t, g, 50, 50, false, board.Bridges{},
		band(g, [2]int{1, 1}, 4, 0, g.At(1, 1), g.At(2, 2))...)
	d.tick()
	lies, dirs := d.lies()
	if len(lies) != 2 {
		t.Fatalf("%d entities lying, want the two decks", len(lies))
	}
	for i, lie := range lies {
		want := world.Lie{Length: 10 * math.Sqrt2, Width: 4, Edged: true}
		if math.Abs(lie.Length-want.Length) > 1e-9 || lie.Width != 4 || !lie.Edged {
			t.Errorf("a deck lies %+v, want %+v", lie, want)
		}
		if d := dirs[i]; math.Abs(math.Abs(d.X)-math.Sqrt2/2) > 1e-9 || d.X != d.Y {
			t.Errorf("a deck is headed %v, want along the band slantwise", d)
		}
	}
}

func TestDecks_ABandOnAHexRunsFromEdgeToEdge(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(5, 5, 10)
	d := newDecked(t, g, 130, 90, false, board.Bridges{},
		band(g, [2]int{0, 1}, 6, 0, g.At(2, 1), g.At(2, 2), g.At(2, 3))...)
	d.tick()
	brd := d.p.Board()
	se, _ := grid.Direction(g, 0, 1)
	s := g.Step(se)
	l := math.Hypot(s.X, s.Y)
	m := g.CellCenter(g.At(2, 2))
	beside := geom.NewVec(m.X-s.Y/l*5, m.Y+s.X/l*5)
	if got := d.over(m); !slices.Equal(got, []cell.ID{g.At(2, 2)}) {
		t.Errorf("its middle: decks of %v, want its own", got)
	}
	if got := d.over(beside); got != nil {
		t.Errorf("beside it: decks of %v, want none", got)
	}
	if !brd.Reaches(g.At(2, 2), g.At(2, 3)) || brd.Reaches(g.At(2, 2), g.At(3, 2)) {
		t.Error("the band reaches the wrong neighbours")
	}
	if brd.Fenced(g.At(2, 2), g.At(2, 3), cell.Land, false) {
		t.Error("a step along the band is fenced")
	}
	if !brd.Fenced(g.At(2, 2), g.At(3, 2), cell.Land, false) {
		t.Error("a step off the band's side is not fenced")
	}
}

func TestRailings_OnAHexRunAlongTheBand(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(5, 5, 10)
	d := newDecked(t, g, 130, 90, false, board.Bridges{},
		band(g, [2]int{0, 1}, 6, 0, g.At(2, 1), g.At(2, 2), g.At(2, 3))...)
	d.tick()
	se, _ := grid.Direction(g, 0, 1)
	s := g.Step(se)
	l := math.Hypot(s.X, s.Y)
	m := g.CellCenter(g.At(2, 2))
	var walls []collide.FieldWall
	d.p.Cover().(collision.Field).Walls(world.Layers(cell.Land), collision.Everywhere,
		geom.NewAABB(geom.NewVec(m.X-2, m.Y-2), geom.NewVec(m.X+2, m.Y+2)),
		func(w collide.FieldWall) bool {
			walls = append(walls, w)
			return true
		})
	if len(walls) != 2 {
		t.Fatalf("%d walls by the band's middle, want its two railings", len(walls))
	}
	for _, w := range walls {
		dx, dy := w.B.X-w.A.X, w.B.Y-w.A.Y
		if math.Abs(dx*s.Y-dy*s.X) > 1e-9 || math.Abs(math.Hypot(dx, dy)-l) > 1e-9 {
			t.Errorf("a railing from %v to %v, want one link long along %v", w.A, w.B, s)
		}
		off := math.Abs((w.A.X-m.X)*s.Y-(w.A.Y-m.Y)*s.X) / l
		if math.Abs(off-3) > 1e-9 {
			t.Errorf("a railing %v off the band's middle line, want 3", off)
		}
	}
}

func TestBridges_BrokenPanicsSayingWhy(t *testing.T) {
	g := grid.DefaultGrids{}.Square(6, 5, 10)
	row := func() []deckRow {
		return bridgeRows(g, [2]int{1, 0}, 4, 0, g.At(1, 2), g.At(2, 2), g.At(3, 2), g.At(4, 2))
	}
	turning := row()
	south, _ := grid.Direction(g, 0, 1)
	turning[2].Floor.Links |= 1 << south
	uneven := row()
	uneven[2].Floor.Height = 2
	headless := band(g, [2]int{1, 0}, 4, 0, g.At(1, 2), g.At(2, 2), g.At(3, 2))
	for i := range headless {
		headless[i].Floor.Graded = true
	}
	outward := row()
	west, _ := grid.Direction(g, -1, 0)
	outward[0].Floor.Links = 1 << west
	raised := row()
	for i := range raised {
		raised[i].Floor.Height = 10
	}
	for _, c := range []struct {
		name    string
		bridges board.Bridges
		rows    []deckRow
		want    string
	}{
		{"turning", board.Bridges{}, turning, "turns"},
		{"uneven", board.Bridges{}, uneven, "a bridge is level"},
		{"without heads", board.Bridges{}, headless, "has 0 heads"},
		{"a head running out", board.Bridges{}, outward, "runs out of it"},
		{"raised over its heads", board.Bridges{}, raised, "stands 10.00 off its deck"},
		{"too low", board.Bridges{Clearance: 3}, row(), "clears it by 3.00"},
	} {
		t.Run(c.name, func(t *testing.T) {
			d := newDecked(t, g, 60, 50, true, c.bridges, c.rows...)
			panicsWith(t, d.tick, c.want)
		})
	}
	d := newDecked(t, g, 60, 50, true, board.Bridges{}, row()...)
	d.tick()
	if got := d.over(g.CellCenter(g.At(2, 2))); len(got) != 1 {
		t.Errorf("a bridge keeping to the rules: decks of %v, want its own", got)
	}
	fence := band(g, [2]int{0, 1}, 4, 0, g.At(2, 1))
	d = newDecked(t, g, 60, 50, true, board.Bridges{Clearance: 3}, fence...)
	d.tick()
	if got := d.over(g.CellCenter(g.At(2, 1))); len(got) != 1 {
		t.Errorf("a band carrying no road: decks of %v, want its own, kept to no rule", got)
	}
}

func TestDecks_AHeadIsAHalfBand(t *testing.T) {
	g := grid.DefaultGrids{}.Square(6, 5, 10)
	d := newDecked(t, g, 60, 50, false, board.Bridges{},
		bridgeRows(g, [2]int{1, 0}, 4, 0, g.At(1, 2), g.At(2, 2), g.At(3, 2))...)
	d.tick()
	brd := d.p.Board()
	for _, c := range []struct {
		name string
		at   geom.Vec
		want []cell.ID
	}{
		{"its middle", geom.NewVec(15, 25), []cell.ID{g.At(1, 2)}},
		{"towards the bridge", geom.NewVec(19, 25), []cell.ID{g.At(1, 2)}},
		{"behind its middle", geom.NewVec(11, 25), nil},
	} {
		if got := d.over(c.at); !slices.Equal(got, c.want) {
			t.Errorf("%s: decks of %v, want %v", c.name, got, c.want)
		}
	}
	for _, from := range []cell.ID{g.At(0, 2), g.At(1, 1), g.At(1, 3), g.At(0, 1)} {
		if brd.Fenced(from, g.At(1, 2), cell.Land, false) {
			t.Errorf("onto the head from %v: fenced, want open from any side", from)
		}
	}
	if !brd.Fenced(g.At(2, 1), g.At(2, 2), cell.Land, false) {
		t.Error("onto the bridge between its heads from the side: open, want fenced")
	}
	lies, dirs := d.lies()
	heads := 0
	for i, lie := range lies {
		if lie.Length == 5 {
			heads++
			if lie.Ahead != 2.5 || lie.Edged || dirs[i].Y != 0 {
				t.Errorf("a head lies %+v headed %v, want half a link a quarter ahead, unedged",
					lie, dirs[i])
			}
		}
	}
	if heads != 2 {
		t.Errorf("%d heads lie half a link long, want 2", heads)
	}
}

type spanRow struct {
	At  cell.ID `yaml:"at"`
	Way [2]int  `yaml:"way"`
}

func TestBridges_BrokenInALevelSaysItsLine(t *testing.T) {
	g := grid.DefaultGrids{}.Square(5, 1, boardtest.CellSize)
	level := fstest.MapFS{"level.yaml": {Data: []byte(`
board:
  default: water
  legend: {"~": water}
  cells: |
    ~~~~~
  entities:
    - {kind: bridge, at: [1, 0], way: [1, 0]}
    - {kind: bridge, at: [2, 0], way: [1, 1]}
`)}}
	prepare := func(w *world.Plugin, brd *board.Plugin) []rule.Rule {
		brd.CellKinds().Define("water", water)
		spans := board.NewEntities(brd, board.Shape{Size: boardtest.UnitSize},
			func(r spanRow) cell.ID { return r.At })
		spans.Define("bridge", comp.Load(func(r spanRow) cell.Floor {
			ahead, _ := grid.Direction(g, r.Way[0], r.Way[1])
			back, _ := grid.Direction(g, -r.Way[0], -r.Way[1])
			return cell.Floor{Allows: cell.Land, Links: 1<<ahead | 1<<back, Width: 4,
				Graded: true}
		}))
		laid, err := brd.Lay(level, "level.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if err := board.Spawn[spanRow](laid, nil); err != nil {
			t.Fatal(err)
		}
		if err := brd.Populate(); err != nil {
			t.Fatal(err)
		}
		return nil
	}
	bw := boardtest.NewWorldWith(t, g, 5*boardtest.CellSize, boardtest.CellSize, prepare, nil)
	panicsWith(t, bw.Tick, "level.yaml:8: ", "turns")
}

func TestBridges_BrokenInTheGameIsNotBuilt(t *testing.T) {
	g := grid.DefaultGrids{}.Square(5, 5, 10)
	rows := bridgeRows(g, [2]int{1, 0}, 4, 0, g.At(1, 2), g.At(2, 2), g.At(3, 2))
	rows[0], rows[1] = rows[1], rows[0] // the one called "first" between the heads
	d := newDecked(t, g, 50, 50, true, board.Bridges{}, rows...)
	var out bytes.Buffer
	d.p.WithLog(log.New(&out, "", 0))
	d.tick()
	middle := g.CellCenter(g.At(2, 2))
	if got := d.over(middle); len(got) != 1 {
		t.Fatalf("before: decks of %v over the bridge's middle, want one", got)
	}
	carrier := d.w.Carrier()
	if !carrier.Put(control.Nobody, rule.Cast(d.sag).On(entity.Named("first"))) {
		t.Fatal("the world carries no command about an effect")
	}
	d.tick()
	d.tick()
	if got := d.over(middle); got != nil {
		t.Errorf("broken: decks of %v over the bridge's middle, want none", got)
	}
	if !strings.Contains(out.String(), "a bridge is level") {
		t.Errorf("the log: %q, want the bridge refused for being uneven", out.String())
	}
	carrier.Put(control.Nobody, rule.Lift(d.sag).On(entity.Named("first")))
	d.tick()
	d.tick()
	if got := d.over(middle); len(got) != 1 {
		t.Errorf("mended: decks of %v over the bridge's middle, want one", got)
	}
}

func TestRailings_AWalkerSlidesAlongASlantwiseOne(t *testing.T) {
	g := grid.DefaultGrids{}.Square(6, 6, 48)
	start := g.At(2, 2)
	prepare := func(w *world.Plugin, brd *board.Plugin) []rule.Rule {
		brd.Board().SetAll(grass)
		decks := board.NewEntities(brd, board.Shape{Size: boardtest.UnitSize},
			func(r deckRow) cell.ID { return r.At })
		decks.Define("deck",
			comp.Load(func(r deckRow) cell.Floor { return r.Floor }),
			comp.Load(func(r deckRow) cell.Rail { return r.Rail }))
		for _, r := range band(g, [2]int{1, 1}, 44, 0,
			g.At(1, 1), g.At(2, 2), g.At(3, 3), g.At(4, 4)) {
			w.Seed(decks.Named("deck").Entry(r))
		}
		return nil
	}
	bw := boardtest.NewWorldWith(t, g, 6*48, 6*48, prepare,
		[]boardtest.Mover{{Here: start, Heading: geom.NewVec(1, 0)}})
	m := g.CellCenter(start)
	// the box's corner keeps the railing's half thickness off its middle line, 22 off the axis
	room := 22 - 48.0/32 - boardtest.UnitSize/2*math.Sqrt2
	for tick := range 60 {
		bw.Tick()
		c := middleOf(bw.Snapshot()[0])
		if off := (c.X - c.Y) / math.Sqrt2; off > room+0.5 {
			t.Fatalf("tick %d: the walker at %v is %.2f off the band's axis, past the railing",
				tick, c, off)
		}
	}
	if c := middleOf(bw.Snapshot()[0]); c.Y-m.Y < 10 {
		t.Errorf("the walker at %v, want it slid along the railing, down from %v", c, m)
	}
}

func middleOf(b geom.AABB) geom.Vec {
	return geom.NewVec((b.TopLeft.X+b.BottomRight.X)/2, (b.TopLeft.Y+b.BottomRight.Y)/2)
}

func panicsWith(t *testing.T, f func(), want ...string) {
	t.Helper()
	defer func() {
		t.Helper()
		r := recover()
		if r == nil {
			t.Errorf("no panic, want one mentioning %q", want)
			return
		}
		msg := fmt.Sprint(r)
		for _, w := range want {
			if !strings.Contains(msg, w) {
				t.Errorf("panic %q, want one mentioning %q", msg, w)
			}
		}
	}()
	f()
}
