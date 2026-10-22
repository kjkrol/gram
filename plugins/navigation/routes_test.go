package navigation

import (
	"flag"
	"fmt"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"math"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/navigation/internal/pathfind"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var updateRoutes = flag.Bool("update-routes", false, "rewrite testdata/routes.golden")

const routesGolden = "testdata/routes.golden"

// a board the routes are pinned on: its grid, its world's edges, its ground and what stands on it
type routeBoard struct {
	name    string
	grid    grid.Grid
	edges   aabbworld.Edges
	span    uint32
	heights func(geom.Vec) float64
	lay     func(b testBoard, rng *rand.Rand)
	bridges []routeSpan
}

// a cell of a bridge running from its first head in direction ahead
type routeSpan struct {
	At           cell.ID
	Ahead        int
	First, Final bool
}

// the bridge of n+1 cells from (x, y) by (dx, dy) a step
func span(g grid.Grid, x, y uint32, dx, dy, n int) []routeSpan {
	ahead, _ := grid.Direction(g, dx, dy)
	var rows []routeSpan
	for k := range n + 1 {
		at := g.At(uint32(int(x)+k*dx), uint32(int(y)+k*dy))
		rows = append(rows, routeSpan{At: at, Ahead: ahead, First: k == 0, Final: k == n})
	}
	return rows
}

type routeRig struct {
	board  routeBoard
	finder *spacing.Finder
	brd    *board.BoardRead
	levels spacing.BoardLevels
	topo   *topography.Plugin
}

var (
	grass  = cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land}
	forest = cell.Kind{Name: cell.Named("forest"), Cost: 2.5, Allows: cell.Land}
	sand   = cell.Kind{Name: cell.Named("sand"), Cost: 1.5, Allows: cell.Land | cell.Air}.
		Costing(cell.Land, 1.25)
	rock = cell.Kind{Name: cell.Named("rock"), Cost: 1, Solid: true}
	ford = cell.Kind{Name: cell.Named("ford"), Cost: 3, Allows: cell.Land | cell.Water}
	road = cell.Kind{Name: cell.Named("road"), Cost: 0.8, Allows: cell.Land, Graded: true}
)

func scatter(b testBoard, rng *rand.Rand) {
	b.SetAll(grass)
	b.EachCell(func(c cell.ID) {
		switch r := rng.IntN(100); {
		case r < 14:
			b.Set(c, rock)
		case r < 26:
			b.Set(c, forest)
		case r < 32:
			b.Set(c, sand)
		}
	})
}

// a road of cells from a, n steps in direction i, linked cell to cell
func lay(b testBoard, a cell.ID, i, n int) {
	g := b.Grid
	back := g.Opposite(i)
	for k, c := 0, a; k <= n; k++ {
		var links cell.Links
		if k > 0 {
			links |= 1 << back
		}
		if k < n {
			links |= 1 << i
		}
		b.Set(c, grass)
		b.SetWay(c, cell.Way{Kind: road, Width: 6, Links: links})
		next, ok := g.Toward(c, i)
		if !ok {
			return
		}
		c = next
	}
}

// the valley of valleyBridge's across the north-west corner, a river down its middle, a ford, the
// bridge from (1,1) to (5,5) over it and one along the top row; rolling ground east of it
func squareRoutes() routeBoard {
	g := grid.DefaultGrids{}.Square(36, 28, 32)
	return routeBoard{
		name: "square",
		grid: g,
		span: 32,
		heights: func(p geom.Vec) float64 {
			if math.Abs((p.X+p.Y)/32-7) < 2 {
				return 0
			}
			hx, hy := 26*32.0, 18*32.0
			if d := math.Hypot(p.X-hx, p.Y-hy) / (5 * 32); d < 1 {
				return 10 + 24*(1-d)
			}
			if p.X > 12*32 {
				return 10 + 6*math.Sin((p.X-12*32)/50)*math.Cos(p.Y/45)
			}
			return 10
		},
		lay: func(b testBoard, rng *rand.Rand) {
			scatter(b, rng)
			for y := range uint32(28) {
				for x := range uint32(36) {
					c := g.At(x, y)
					switch {
					case x+y >= 5 && x+y <= 7:
						b.Set(c, grass)
					case x+y >= 4 && x+y <= 8 && x <= 6 && y <= 6:
						b.Set(c, grass)
					}
					if x+y == 6 {
						b.Set(c, water)
					}
				}
			}
			b.Set(g.At(0, 6), ford)
			lay(b, g.At(8, 14), 3, 22)
			lay(b, g.At(10, 2), 7, 11)
			lay(b, g.At(30, 3), 6, 9)
		},
		bridges: append(span(g, 1, 1, 1, 1, 4), span(g, 3, 0, 1, 0, 6)...),
	}
}

func torusRoutes() routeBoard {
	return routeBoard{
		name:  "torus",
		grid:  grid.DefaultGrids{}.Square(24, 20, 32),
		edges: aabbworld.Torus,
		span:  32,
		lay: func(b testBoard, rng *rand.Rand) {
			scatter(b, rng)
			lay(b, b.Grid.At(20, 4), 3, 10)
			lay(b, b.Grid.At(3, 17), 5, 7)
		},
	}
}

func hexRoutes() routeBoard {
	g := grid.DefaultGrids{}.Hex(30, 24, 16)
	return routeBoard{
		name: "hex",
		grid: g,
		span: 16,
		heights: func(p geom.Vec) float64 {
			return 24 * math.Sin(p.X/60) * math.Cos(p.Y/50)
		},
		lay: func(b testBoard, rng *rand.Rand) {
			scatter(b, rng)
			for r := uint32(0); r < 24; r++ {
				b.Set(g.At(14, r), water)
			}
			b.Set(g.At(14, 6), ford)
			b.Set(g.At(14, 17), ford)
			lay(b, g.At(2, 10), 0, 24)
			lay(b, g.At(5, 22), 1, 14)
		},
	}
}

func newRouteRig(t *testing.T, rb routeBoard) *routeRig {
	t.Helper()
	cw, ch := rb.grid.CellBounds()
	var right, bottom float64
	rb.grid.EachCell(func(c cell.ID) {
		m := rb.grid.CellCenter(c)
		right, bottom = max(right, m.X+cw/2), max(bottom, m.Y+ch/2)
	})
	width, height := uint32(math.Ceil(right)), uint32(math.Ceil(bottom))
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: width, Height: height, Edges: rb.edges},
		Entities: world.EntitiesCfg{MaxCount: 16, MinSize: 1, MaxSize: rb.span},
		Heights:  true,
	})
	m := motion.NewPlugin(w)
	brd := board.NewPlugin(rb.grid, &cell.MultipleOccupancy{}, w).WithMotion(m)
	var topo *topography.Plugin
	if rb.heights != nil {
		topo = topography.NewPlugin(w, brd, nil, topography.Config{})
	}
	sel := selection.NewPlugin(w)
	drive := driving.NewPlugin(w, m, sel).WithGround(brd)
	nav := NewPlugin(brd, w, m, sel, drive).WithSpacing(CellSpacing)
	rb.lay(boardOf(brd), rand.New(rand.NewPCG(11, 17)))
	if topo != nil {
		topo.Write().Relief().SetHeights(rb.heights)
	}
	if rb.bridges != nil {
		g := rb.grid
		spans := board.NewEntities(brd, board.Shape{Size: float64(rb.span)},
			func(r routeSpan) cell.ID { return r.At })
		spans.Define("bridge", comp.Load(func(r routeSpan) cell.Floor {
			back := g.Opposite(r.Ahead)
			links := cell.Links(1<<r.Ahead | 1<<back)
			switch {
			case r.First:
				links = 1 << r.Ahead
			case r.Final:
				links = 1 << back
			}
			return cell.Floor{
				Allows:    cell.Land,
				Height:    10 - topo.Write().Relief().Altitude(r.At),
				Thickness: 0.5,
				Cost:      1,
				Graded:    true,
				Links:     links,
				Width:     12,
			}
		}), comp.Const(cell.Rail{Keeps: cell.Land, Height: 1}))
		for _, r := range rb.bridges {
			w.Write().Seed(spans.Named("bridge").Entry(r))
		}
	}
	if err := w.Carry(nav); err != nil {
		t.Fatal(err)
	}
	ctx := &stubInstallCtx{ecs: goke.New()}
	installs := []func() error{
		func() error { return w.Install(ctx) },
		func() error { return m.Install(ctx) },
		func() error { return brd.Install(ctx) },
	}
	if topo != nil {
		installs = append(installs, func() error { return topo.Install(ctx) })
	}
	installs = append(installs,
		func() error { return sel.Install(ctx) },
		func() error { return drive.Install(ctx) },
		func() error { return nav.Install(ctx) },
	)
	for _, install := range installs {
		if err := install(); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		m.RunPlan(rc, d)
		brd.RunPlan(rc, d)
		if topo != nil {
			topo.RunPlan(rc, d)
		}
		nav.RunPlan(rc, d)
		drive.RunPlan(rc, d)
		rc.Sync()
		w.Write().Clock().Replay(rc, d)
	})
	for range 3 {
		ctx.ecs.Tick(time.Second / 60)
	}
	return &routeRig{board: rb, finder: nav.finder, brd: brd.Read(0).Board(), levels: spacing.BoardLevels{Read: brd.Read(0)},
		topo: topo}
}

func (r *routeRig) cells() []cell.ID {
	var all []cell.ID
	r.brd.EachCell(func(c cell.ID) { all = append(all, c) })
	return all
}

func routeBodies(feet float64) []pathfind.Body {
	return []pathfind.Body{
		{Feet: feet, Step: 1.5, Height: 3, Size: geom.NewVec(3, 3)},
		{Feet: feet, Step: 1.5, Height: 3, Size: geom.NewVec(14, 14)},
		{Feet: feet, Step: 1.5, Height: 12},
		{},
	}
}

func (r *routeRig) at(c cell.ID) string {
	x, y, _ := r.board.grid.Coords(c)
	return fmt.Sprintf("%d,%d", x, y)
}

func (r *routeRig) cellsOf(steps []cell.ID) string {
	var b strings.Builder
	for i, c := range steps {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(r.at(c))
	}
	return b.String()
}

func float(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

func yes(b bool) string {
	if b {
		return "y"
	}
	return "n"
}

// every query the planner answers, as it answers it, for random pairs of cells on the rig's board
func (r *routeRig) record(out *strings.Builder, pairs int) {
	rng := rand.New(rand.NewPCG(5, 23))
	all := r.cells()
	open := make([]cell.ID, 0, len(all))
	for _, c := range all {
		if r.brd.Kind(c).Admits(cell.Land) {
			open = append(open, c)
		}
	}
	g, f := r.board.grid, r.finder
	var near []cell.ID
	for _, c := range open {
		for _, s := range r.board.bridges {
			if g.Distance(c, s.At) <= 3 {
				near = append(near, c)
				break
			}
		}
	}
	far := func(from cell.ID) cell.ID {
		best := open[rng.IntN(len(open))]
		for range 8 {
			if c := open[rng.IntN(len(open))]; g.Distance(from, c) > g.Distance(from, best) {
				best = c
			}
		}
		return best
	}
	var dst [1024]cell.ID
	for i := range pairs {
		from, to := open[rng.IntN(len(open))], open[rng.IntN(len(open))]
		switch {
		case i%10 == 9:
			to = all[rng.IntN(len(all))]
		case i%3 == 1 && len(near) > 0:
			from, to = near[rng.IntN(len(near))], near[rng.IntN(len(near))]
		case i%4 == 3:
			to = far(from)
		}
		var feet float64
		if r.topo != nil {
			feet = r.topo.Write().Relief().Altitude(from)
		}
		if top, _, _, ok := r.levels.Deck(from); ok && rng.IntN(2) == 0 {
			feet = top
		}
		shapes := routeBodies(feet)
		body := shapes[rng.IntN(len(shapes))]
		end := pathfind.Level(rng.IntN(3))
		fmt.Fprintf(out, "%s %d %s->%s %v,%v,%v,%v end=%d\n", r.board.name, i, r.at(from),
			r.at(to), body.Feet, body.Step, body.Height, body.Size, end)

		n, ok := f.Find(uid.UID64(1), cell.Land, body, from, to, end, dst[:])
		fmt.Fprintf(out, " find %s %x [%s]\n", yes(ok), f.Decks(), r.cellsOf(dst[:n]))
		if ok && n > 2 {
			lo, hi := n/3, 2*n/3+1
			route := append([]cell.ID(nil), dst[lo:hi]...)
			blocked := func(c cell.ID) bool {
				for _, s := range route {
					if s == c && c != to {
						return true
					}
				}
				return false
			}
			n, ok := f.FindAround(uid.UID64(1), cell.Land, body, from, to, end, blocked, dst[:])
			fmt.Fprintf(out, " around %s %x [%s]\n", yes(ok), f.Decks(), r.cellsOf(dst[:n]))
		}
		taken := func(c cell.ID, deck bool) bool { return uint64(c)%5 == 0 || deck && c%3 == 0 }
		dest, n, ok := f.NearestFree(uid.UID64(1), cell.Land, body, from, to, end, taken, dst[:])
		fmt.Fprintf(out, " nearest %s %s [%s]\n", yes(ok), r.at(dest), r.cellsOf(dst[:n]))

		fmt.Fprintf(out, " level %d %s", f.Level(cell.Land, body, from),
			yes(f.On(cell.Land, body, from, end)))
		for d := range g.Directions() {
			nb, ok := g.Toward(from, d)
			if !ok {
				out.WriteString(" -")
				continue
			}
			onto, steps := f.Steps(cell.Land, body, from, nb)
			cost, priced := f.Price(from, nb, r.brd.KindAt(nb, onto), cell.Land)
			fmt.Fprintf(out, " %s%s%s:%s:%s", yes(onto), yes(steps), yes(priced), float(cost),
				float(f.Climb(from, nb, cell.Land)))
		}
		out.WriteByte('\n')
	}
}

func TestRoutes_AsPinned(t *testing.T) {
	var out strings.Builder
	for _, rb := range []routeBoard{squareRoutes(), torusRoutes(), hexRoutes()} {
		newRouteRig(t, rb).record(&out, 100)
	}
	got := out.String()
	if *updateRoutes {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(routesGolden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(routesGolden)
	if err != nil {
		t.Fatalf("%v; run with -update-routes on the planner to pin", err)
	}
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(string(want), "\n")
	for i := range max(len(gotLines), len(wantLines)) {
		var g, w string
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g != w {
			t.Fatalf("line %d differs from %s:\n got: %s\nwant: %s", i+1, routesGolden, g, w)
		}
	}
}
