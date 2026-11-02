package site_test

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/roads/site"
)

var boards = []struct {
	name string
	grid func(w, h uint32) grid.Grid
}{
	{"square", square},
	{"hex", hex},
}

func square(w, h uint32) grid.Grid { return grid.DefaultGrids{}.Square(w, h, 10) }

func hex(w, h uint32) grid.Grid { return grid.DefaultGrids{}.Hex(w, h, 10) }

func TestRoute_RunsStraightOverLevelGround(t *testing.T) {
	slants := map[string][]int{"square": {1, 1, 1, 1, 6}, "hex": {3, 0, 0, 1, 7}}
	for _, b := range boards {
		g := b.grid(10, 7)
		f := newFakeGround(g)
		s := slants[b.name]
		lines := [][]cell.ID{run(g, 1, 3, 1, 0, 8), run(g, s[0], s[1], s[2], s[3], s[4])}
		for _, want := range lines {
			from, to := want[0], want[len(want)-1]
			for _, graded := range []bool{true, false} {
				route, err := site.Route(f, site.Rules{}, from, to, graded, false, nil, nil)
				switch {
				case err != nil:
					t.Errorf("%s: %v", b.name, err)
				case !slices.Equal(route, want):
					t.Errorf("%s: the road runs %v, want %v", b.name, where(g, route),
						where(g, want))
				case math.Abs(length(g, route)-g.Distance(from, to)) > 1e-9:
					t.Errorf("%s: the road %v runs %v, the cells lie %v apart", b.name,
						where(g, route), length(g, route), g.Distance(from, to))
				}
			}
		}
	}
}

func TestRoute_GoesRoundAHillItMayNotLevel(t *testing.T) {
	ramp := ground.RoadSlope
	cases := []struct {
		name      string
		rules     site.Rules
		graded    bool
		levelling bool
		over      bool
	}{
		{"levelled flat", site.Rules{Cut: 3, Fill: 3, Slope: ramp, Metre: 1}, true, true, true},
		{"levelled to ramps", site.Rules{Cut: 2, Fill: 2, Slope: ramp, Metre: 1}, true, true, true},
		{"levelled too little", site.Rules{Cut: 0.5, Fill: 0.5, Slope: ramp, Metre: 1}, true, true,
			false},
		{"not levelling", site.Rules{Cut: 3, Fill: 3, Slope: ramp, Metre: 1}, true, false, false},
		{"no ramp allowed", site.Rules{Cut: 2, Fill: 2, Metre: 1}, true, true, false},
		{"a track up it", site.Rules{Slope: 0.5, Metre: 1}, false, false, true},
		{"a track too steep", site.Rules{Slope: 0.2, Metre: 1}, false, false, false},
	}
	hill := func(x, y uint32) bool { return x >= 4 && x <= 7 && y <= 6 }
	for _, b := range boards {
		g := b.grid(12, 9)
		f := newFakeGround(g)
		g.EachCell(func(c cell.ID) {
			if x, y, _ := g.Coords(c); hill(x, y) {
				level(f, c, 5)
			}
		})
		from, to := g.At(1, 3), g.At(10, 3)
		for _, c := range cases {
			route, err := site.Route(f, c.rules, from, to, c.graded, c.levelling, nil, nil)
			if err != nil {
				t.Errorf("%s, %s: %v", b.name, c.name, err)
				continue
			}
			if c.over {
				if want := run(g, 1, 3, 1, 0, 10); !slices.Equal(route, want) {
					t.Errorf("%s, %s: the road runs %v, want it over the hill", b.name, c.name,
						where(g, route))
				}
				continue
			}
			walks(t, g, route, from, to)
			for _, rc := range route {
				if x, y, _ := g.Coords(rc); hill(x, y) {
					t.Errorf("%s, %s: the road climbs the hill at [%d, %d]", b.name, c.name, x, y)
				}
			}
		}
	}
}

func TestRoute_TurnsAGradedRoadThroughACornerOnlyLevel(t *testing.T) {
	g := square(7, 7)
	f := newFakeGround(g)
	g.EachCell(func(c cell.ID) {
		x, y, _ := g.Coords(c)
		level(f, c, 0.5*float32(x+y))
	})
	from, to := g.At(0, 0), g.At(6, 6)
	ramp := ground.RoadSlope
	route, err := site.Route(f, site.Rules{Slope: ramp, Metre: 1}, from, to, true, false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if walks(t, g, route, from, to) && len(route) != 13 {
		t.Errorf("the road runs %v, want 12 steps across edges", where(g, route))
	}
	for k := 1; k < len(route); k++ {
		if i, _ := grid.Way(g, route[k-1], route[k]); g.Corner(i) {
			t.Errorf("the road %v runs through a corner, not level, from step %d", where(g, route),
				k-1)
		}
	}
	diagonal := run(g, 0, 0, 1, 1, 7)
	levelled := site.Rules{Cut: 0.5, Fill: 0.5, Slope: ramp, Metre: 1}
	route, err = site.Route(f, levelled, from, to, true, true, nil, nil)
	if err != nil || !slices.Equal(route, diagonal) {
		t.Errorf("levelled, the road runs %v (%v), want the diagonal", where(g, route), err)
	}
	route, err = site.Route(f, site.Rules{Slope: 0.2, Metre: 1}, from, to, false, false, nil, nil)
	if err != nil || !slices.Equal(route, diagonal) {
		t.Errorf("a track runs %v (%v), want the diagonal", where(g, route), err)
	}
}

func TestRoute_BendsAGradedRoadOnlyWhereItLevels(t *testing.T) {
	g := square(8, 8)
	f := newFakeGround(g)
	g.EachCell(func(c cell.ID) { tilt(f, c, 1.5) })
	rules := site.Rules{Cut: 0.5, Fill: 0.5, Slope: ground.RoadSlope, Metre: 1}
	from := g.At(1, 1)
	for _, want := range [][]cell.ID{run(g, 1, 1, 1, 0, 6), run(g, 1, 1, 0, 1, 6)} {
		route, err := site.Route(f, rules, from, want[len(want)-1], true, true, nil, nil)
		if err != nil || !slices.Equal(route, want) {
			t.Errorf("the road runs %v (%v), want %v", where(g, route), err, where(g, want))
		}
	}
	to := g.At(6, 6)
	_, err := site.Route(f, rules, from, to, true, true, nil, nil)
	noWay(t, err, from)
	level(f, g.At(6, 1), 9.75)
	route, err := site.Route(f, rules, from, to, true, true, nil, nil)
	want := append(run(g, 1, 1, 1, 0, 6), run(g, 6, 2, 0, 1, 5)...)
	if err != nil || !slices.Equal(route, want) {
		t.Errorf("the road runs %v (%v), want it to bend on the cell levelled, %v",
			where(g, route), err, where(g, want))
	}
}

func TestRoute_ClimbsARampItReachedFirstFromTheSide(t *testing.T) {
	g := square(9, 7)
	f := newFakeGround(g)
	g.EachCell(func(c cell.ID) {
		tilt(f, c, 1.5)
		if x, y, _ := g.Coords(c); x == 1 {
			level(f, c, 2.25)
			if y > 0 {
				f.price[c] = 2
			}
		}
	})
	level(f, g.At(4, 0), 6.75)
	rules := site.Rules{Cut: 0.5, Fill: 0.5, Slope: ground.RoadSlope, Metre: 1, Climb: 1}
	route, err := site.Route(f, rules, g.At(1, 0), g.At(7, 3), true, true, nil, nil)
	want := append(run(g, 1, 0, 0, 1, 4), run(g, 2, 3, 1, 0, 6)...)
	if err != nil || !slices.Equal(route, want) {
		t.Errorf("the road runs %v (%v), want %v", where(g, route), err, where(g, want))
	}
}

func TestRoute_CrossesWaterOverADeckOrThroughAFord(t *testing.T) {
	g := square(11, 9)
	from, to := g.At(2, 4), g.At(8, 4)
	river := func() *fakeGround {
		f := newFakeGround(g)
		for y := range uint32(9) {
			f.wet[g.At(5, y)] = true
		}
		return f
	}
	rules := site.Rules{Slope: ground.RoadSlope, Metre: 1}
	both := rules
	both.Decks = true
	both.Fords = true
	_, err := site.Route(river(), both, from, to, true, true, nil, nil)
	noWay(t, err, from)

	f := river()
	f.decks[g.At(4, 4)] = links(g, g.At(4, 4), g.At(5, 4))
	f.decks[g.At(5, 4)] = links(g, g.At(5, 4), g.At(4, 4), g.At(6, 4))
	f.decks[g.At(6, 4)] = links(g, g.At(6, 4), g.At(5, 4))
	route, err := site.Route(f, both, from, to, true, true, nil, nil)
	if want := run(g, 2, 4, 1, 0, 7); err != nil || !slices.Equal(route, want) {
		t.Errorf("over a deck the road runs %v (%v), want %v", where(g, route), err, where(g, want))
	}
	_, err = site.Route(f, rules, from, to, true, true, nil, nil)
	noWay(t, err, from)

	// a ford water runs over, and one a kind of both water and land
	for _, wet := range []bool{true, false} {
		f = river()
		f.fords[g.At(5, 7)] = true
		f.wet[g.At(5, 7)] = wet
		route, err = site.Route(f, both, from, to, true, true, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		walks(t, g, route, from, to)
		for _, c := range []cell.ID{g.At(4, 7), g.At(5, 7), g.At(6, 7)} {
			if !slices.Contains(route, c) {
				t.Errorf("wet %v, the road runs %v, want it straight through the ford at [5, 7]",
					wet, where(g, route))
			}
		}
		_, err = site.Route(f, rules, from, to, true, true, nil, nil)
		noWay(t, err, from)
	}
}

func TestRoute_TakesADeckOnlyAlongIt(t *testing.T) {
	g := square(9, 7)
	f := newFakeGround(g)
	for x := uint32(1); x <= 7; x++ {
		c := g.At(x, 3)
		switch x {
		case 1:
			f.decks[c] = links(g, c, g.At(2, 3))
		case 7:
			f.decks[c] = links(g, c, g.At(6, 3))
		default:
			f.decks[c] = links(g, c, g.At(x-1, 3), g.At(x+1, 3))
		}
	}
	span := func(c cell.ID) bool {
		x, y, _ := g.Coords(c)
		return y == 3 && x >= 2 && x <= 6
	}
	decked := func(c cell.ID) bool {
		_, ok := f.decks[c]
		return ok
	}
	rules := site.Rules{Slope: ground.RoadSlope, Metre: 1, Decks: true}
	from, to := g.At(0, 3), g.At(8, 3)
	route, err := site.Route(f, rules, from, to, true, true, nil, nil)
	if want := run(g, 0, 3, 1, 0, 9); err != nil || !slices.Equal(route, want) {
		t.Errorf("along the bridge the road runs %v (%v), want %v", where(g, route), err,
			where(g, want))
	}
	route, err = site.Route(f, rules, g.At(3, 0), g.At(3, 6), true, true, nil, nil)
	if err != nil || !walks(t, g, route, g.At(3, 0), g.At(3, 6)) ||
		slices.ContainsFunc(route, span) {
		t.Errorf("across the bridge the road runs %v (%v), want it off the spans",
			where(g, route), err)
	}
	none := rules
	none.Decks = false
	route, err = site.Route(f, none, from, to, true, true, nil, nil)
	if err != nil || !walks(t, g, route, from, to) || slices.ContainsFunc(route, decked) {
		t.Errorf("without decks the road runs %v (%v), want it off the bridge",
			where(g, route), err)
	}
}

func TestRoute_CutsNoCornerOfWater(t *testing.T) {
	g := square(6, 6)
	f := newFakeGround(g)
	for k := range uint32(6) {
		f.wet[g.At(k, 5-k)] = true
	}
	from, to := g.At(2, 2), g.At(3, 3)
	rules := site.Rules{Slope: ground.RoadSlope, Metre: 1, Fords: true}
	_, err := site.Route(f, rules, from, to, false, false, nil, nil)
	noWay(t, err, from)
	f.fords[g.At(2, 3)] = true
	route, err := site.Route(f, rules, from, to, false, false, nil, nil)
	if want := []cell.ID{from, g.At(2, 3), to}; err != nil || !slices.Equal(route, want) {
		t.Errorf("the road runs %v (%v), want %v", where(g, route), err, where(g, want))
	}
}

func TestRoute_RunsAlongARoadStandingOnlyWithCrossings(t *testing.T) {
	g := square(12, 9)
	f := newFakeGround(g)
	g.EachCell(func(c cell.ID) { f.price[c] = 3 })
	for x := range 12 {
		c := g.At(uint32(x), 6)
		for _, n := range []int{x - 1, x + 1} {
			if n >= 0 && n < 12 {
				f.roads[c] |= links(g, c, g.At(uint32(n), 6))
			}
		}
	}
	onRoad := func(c cell.ID) bool {
		_, y, _ := g.Coords(c)
		return y == 6
	}
	from, to := g.At(1, 4), g.At(10, 4)
	rules := site.Rules{Slope: ground.RoadSlope, Metre: 1}
	crossing := rules
	crossing.Crossings = true
	route, err := site.Route(f, crossing, from, to, true, true, nil, nil)
	if err != nil || !walks(t, g, route, from, to) || !slices.ContainsFunc(route, onRoad) {
		t.Errorf("with crossings the road runs %v (%v), want it along the road standing",
			where(g, route), err)
	}
	route, err = site.Route(f, rules, from, to, true, true, nil, nil)
	if want := run(g, 1, 4, 1, 0, 10); err != nil || !slices.Equal(route, want) {
		t.Errorf("without crossings the road runs %v (%v), want %v", where(g, route), err,
			where(g, want))
	}
	_, err = site.Route(f, rules, from, g.At(1, 8), true, true, nil, nil)
	noWay(t, err, from)
}

func TestRoute_MeetsARoadStandingInACellNeverAtACorner(t *testing.T) {
	g := square(6, 6)
	f := newFakeGround(g)
	a, b := g.At(2, 3), g.At(3, 2)
	f.roads[a] = links(g, a, b)
	f.roads[b] = links(g, b, a)
	rules := site.Rules{Slope: ground.RoadSlope, Metre: 1, Crossings: true}
	route, err := site.Route(f, rules, g.At(2, 2), g.At(3, 3), true, true, nil, nil)
	if err != nil || len(route) != 3 || (route[1] != a && route[1] != b) {
		t.Errorf("the road runs %v (%v), want it through [2, 3] or [3, 2]", where(g, route), err)
	}
}

func TestRoute_LevelsNoFixedCorner(t *testing.T) {
	g := square(8, 5)
	f := newFakeGround(g)
	for y := range uint32(5) {
		level(f, g.At(4, y), 3)
	}
	from, to := g.At(1, 2), g.At(6, 2)
	rules := site.Rules{Cut: 1, Fill: 1, Slope: ground.RoadSlope, Metre: 1}
	route, err := site.Route(f, rules, from, to, true, true, nil, nil)
	if want := run(g, 1, 2, 1, 0, 6); err != nil || !slices.Equal(route, want) {
		t.Errorf("the road runs %v (%v), want %v", where(g, route), err, where(g, want))
	}
	for y := range uint32(5) {
		f.fixed[g.At(4, y)] = true
	}
	_, err = site.Route(f, rules, from, to, true, true, nil, nil)
	noWay(t, err, from)
}

func TestRoute_LeapsWhereALeapIsCheaper(t *testing.T) {
	g := square(12, 9)
	from, to := g.At(2, 4), g.At(9, 4)
	rules := site.Rules{Slope: ground.RoadSlope, Metre: 1, Fords: true}
	river := func(ford bool) *fakeGround {
		f := newFakeGround(g)
		for y := range uint32(9) {
			f.wet[g.At(5, y)] = true
			f.wet[g.At(6, y)] = true
		}
		if ford {
			f.fords[g.At(5, 7)] = true
			f.fords[g.At(6, 7)] = true
		}
		return f
	}
	leapt := []cell.ID{g.At(2, 4), g.At(3, 4), g.At(4, 4), g.At(7, 4), g.At(8, 4), g.At(9, 4)}
	for _, ford := range []bool{false, true} {
		f := river(ford)
		route, err := site.Route(f, rules, from, to, true, true, leapsOver(f, 3, 5), nil)
		if err != nil || !slices.Equal(route, leapt) {
			t.Errorf("a ford %v, the road runs %v (%v), want %v", ford, where(g, route), err,
				where(g, leapt))
		}
	}
	f := river(true)
	route, err := site.Route(f, rules, from, to, true, true, leapsOver(f, 3, 1000), nil)
	if err != nil || !walks(t, g, route, from, to) ||
		!slices.Contains(route, g.At(5, 7)) || !slices.Contains(route, g.At(6, 7)) {
		t.Errorf("the leaps dear, the road runs %v (%v), want it through the ford",
			where(g, route), err)
	}
}

func TestPlanner_FindsWhatDijkstraFinds(t *testing.T) {
	for _, b := range boards {
		for _, graded := range []bool{true, false} {
			for _, sloped := range []bool{false, true} {
				name := fmt.Sprintf("%s, graded %v, sloped %v", b.name, graded, sloped)
				g := b.grid(9, 7)
				f := randomGround(g, sloped)
				rules := site.Rules{Cut: 0.4, Fill: 0.3, Slope: 0.15, Metre: 2, Climb: 4,
					Fords: true}
				if graded {
					rules.Slope = ground.RoadSlope
				}
				p := site.NewPlanner(f, rules, graded, true, leapsOver(f, 3, 3))
				for _, ends := range [][2]cell.ID{
					{g.At(0, 0), g.At(8, 6)},
					{g.At(1, 6), g.At(7, 0)},
					{g.At(8, 1), g.At(0, 4)},
					{g.At(2, 3), g.At(6, 3)},
				} {
					agrees(t, name, g, p, ends[0], ends[1])
				}
			}
		}
	}
}

func TestRoute_RefusesAWayOfMoreCellsThanTheLongest(t *testing.T) {
	g := square(10, 7)
	f := newFakeGround(g)
	from, to := g.At(1, 3), g.At(8, 3)
	route, err := site.Route(f, site.Rules{Longest: 5}, from, to, true, false, nil, nil)
	var r *site.Refusal
	if !errors.As(err, &r) || r.At != g.At(6, 3) ||
		!strings.Contains(r.Why(), "8 cells, 5 at most") {
		t.Errorf("got %v, want the sixth cell refused for a road of 8 cells, 5 at most", err)
	}
	if want := run(g, 1, 3, 1, 0, 8); !slices.Equal(route, want) {
		t.Errorf("the road refused runs %v, want its cells kept, %v", where(g, route),
			where(g, want))
	}
	_, err = site.Route(f, site.Rules{Longest: 8}, from, to, true, false, nil, nil)
	if err != nil {
		t.Errorf("a road of the longest refused: %v", err)
	}
}

func TestRoute_SaysWhyAnEndTakesNoRoad(t *testing.T) {
	g := square(10, 7)
	from, to := g.At(1, 3), g.At(8, 3)
	off := cell.ID(1 << 40)
	cases := []struct {
		name     string
		lay      func(f *fakeGround)
		from, at cell.ID
		why      string
	}{
		{"water", func(f *fakeGround) { f.wet[to] = true }, from, to, "into water"},
		{"ground", func(f *fakeGround) { f.takes[from] = false }, from, from, "takes no road"},
		{"road", func(f *fakeGround) { f.roads[to] = 1 }, from, to, "road standing"},
		{"deck", func(f *fakeGround) { f.decks[to] = 1 }, from, to, "bridge's deck"},
		{"ford", func(f *fakeGround) { f.fords[to] = true }, from, to, "through a ford"},
		{"off the board", func(*fakeGround) {}, off, off, "off the board"},
	}
	for _, c := range cases {
		f := newFakeGround(g)
		c.lay(f)
		_, err := site.Route(f, site.Rules{}, c.from, to, true, false, nil, nil)
		var r *site.Refusal
		if !errors.As(err, &r) || r.At != c.at || !strings.Contains(r.Why(), c.why) {
			t.Errorf("%s: got %v, want a refusal of the end saying %q", c.name, err, c.why)
		}
	}
}

func TestPlanner_RoutesAgainAllocatingNothing(t *testing.T) {
	for _, b := range boards {
		g := b.grid(16, 12)
		f := newFakeGround(g)
		for y := range uint32(12) {
			f.wet[g.At(8, y)] = true
		}
		for x := uint32(2); x < 6; x++ {
			level(f, g.At(x, 4), 3)
		}
		rules := site.Rules{Cut: 1, Fill: 1, Slope: ground.RoadSlope, Metre: 1, Climb: 2}
		p := site.NewPlanner(f, rules, true, true, leapsOver(f, 2, 4))
		from, to := g.At(1, 5), g.At(14, 6)
		dst, err := p.Route(from, to, make([]cell.ID, 0, 64))
		if err != nil {
			t.Fatalf("%s: %v", b.name, err)
		}
		allocs := testing.AllocsPerRun(10, func() { dst, err = p.Route(from, to, dst) })
		if allocs != 0 || err != nil {
			t.Errorf("%s: a route asked again allocates %v times (%v)", b.name, allocs, err)
		}
	}
}

func TestRoute_WritesIntoDst(t *testing.T) {
	g := square(10, 7)
	dst := make([]cell.ID, 0, 32)
	route, err := site.Route(newFakeGround(g), site.Rules{}, g.At(1, 3), g.At(8, 3), true, false,
		nil, dst)
	if err != nil || len(route) != 8 || &route[0] != &dst[:1][0] {
		t.Errorf("the road %v (%v) is not written into dst", where(g, route), err)
	}
}

func BenchmarkPlanner_Route(b *testing.B) {
	for _, bd := range boards {
		b.Run(bd.name, func(b *testing.B) {
			g := bd.grid(256, 256)
			f := newFakeGround(g)
			rnd := rand.New(rand.NewPCG(1, 2))
			g.EachCell(func(c cell.ID) {
				x, y, _ := g.Coords(c)
				level(f, c, float32(80*math.Sin(float64(x)/15)*math.Cos(float64(y)/20)))
				f.price[c] = 1 + rnd.Float32()
			})
			rules := site.Rules{Cut: 2, Fill: 2, Slope: ground.RoadSlope, Metre: 1, Climb: 2}
			p := site.NewPlanner(f, rules, true, true, nil)
			from, to := g.At(8, 8), g.At(247, 247)
			dst := make([]cell.ID, 0, 1024)
			b.ReportAllocs()
			for b.Loop() {
				dst, _ = p.Route(from, to, dst)
			}
		})
	}
}

// randomGround is g with a river down its fifth column, a ford in it, two cells fixed, every
// cell at a height and price of its own, its corners apart by up to one where sloped
func randomGround(g grid.Grid, sloped bool) *fakeGround {
	f := newFakeGround(g)
	rnd := rand.New(rand.NewPCG(7, 11))
	g.EachCell(func(c cell.ID) {
		level(f, c, 2*rnd.Float32())
		if sloped {
			for k := range f.heights[c] {
				f.heights[c][k] += rnd.Float32()
			}
		}
		f.price[c] = 1 + 2*rnd.Float32()
	})
	for y := range uint32(7) {
		f.wet[g.At(4, y)] = true
	}
	f.fords[g.At(4, 5)] = true
	f.fixed[g.At(2, 2)] = true
	f.fixed[g.At(6, 3)] = true
	return f
}

// agrees fails t unless p's route from from to to is Dijkstra's over every step p takes, its cost
// and its cells
func agrees(t *testing.T, name string, g grid.Grid, p *site.Planner, from, to cell.ID) {
	t.Helper()
	route, err := p.Route(from, to, nil)
	cost, want, ok := dijkstra(p, from, to)
	switch {
	case !ok && err == nil:
		t.Errorf("%s: the road runs %v where Dijkstra finds no way", name, where(g, route))
	case !ok:
	case err != nil:
		t.Errorf("%s: %v, where Dijkstra finds %v", name, err, where(g, want))
	case math.Abs(priced(t, p, route)-cost) > 1e-9*cost:
		t.Errorf("%s: the road %v costs %v, Dijkstra's %v costs %v", name, where(g, route),
			priced(t, p, route), where(g, want), cost)
	case !slices.Equal(route, want):
		t.Errorf("%s: the road runs %v, Dijkstra's %v", name, where(g, route), where(g, want))
	}
}

// state is a cell a road came into and the way it came, -1 where it may leave it any way
type state struct {
	c  cell.ID
	in int
}

// dijkstra is the cheapest way from from to to over every step p takes, what it costs and its
// cells; false where none runs
func dijkstra(p *site.Planner, from, to cell.ID) (float64, []cell.ID, bool) {
	start := state{c: from, in: -1}
	dist := map[state]float64{start: 0}
	prev := map[state]state{}
	done := map[state]bool{}
	var end state
	for {
		at, found := state{}, false
		for s, d := range dist {
			if !done[s] && (!found || d < dist[at]) {
				at, found = s, true
			}
		}
		if !found {
			return 0, nil, false
		}
		if at.c == to {
			end = at
			break
		}
		done[at] = true
		back := at.c
		if pr, ok := prev[at]; ok {
			back = pr.c
		}
		p.Steps(at.c, at.in, back, func(n cell.ID, in int, cost float64) {
			s := state{c: n, in: in}
			if d, ok := dist[s]; !done[s] && (!ok || dist[at]+cost < d) {
				dist[s] = dist[at] + cost
				prev[s] = at
			}
		})
	}
	route := []cell.ID{end.c}
	for s := end; s != start; {
		s = prev[s]
		route = append(route, s.c)
	}
	slices.Reverse(route)
	return dist[end], route, true
}

// priced is what route costs, step by step, as p prices its steps
func priced(t *testing.T, p *site.Planner, route []cell.ID) float64 {
	t.Helper()
	sum := 0.0
	in := -1
	back := route[0]
	for k := 1; k < len(route); k++ {
		cost, next := math.Inf(1), -1
		p.Steps(route[k-1], in, back, func(n cell.ID, i int, c float64) {
			if n == route[k] && c < cost {
				cost, next = c, i
			}
		})
		if math.IsInf(cost, 1) {
			t.Fatalf("the road steps from %v to %v, where none may", route[k-1], route[k])
		}
		sum += cost
		back = route[k-1]
		in = next
	}
	return sum
}

// leapsOver leaps from dry ground over water, longest cells of it at most, onto the dry ground
// beyond, at cost
func leapsOver(f *fakeGround, longest int, cost float32) site.Leaps {
	return func(c cell.ID, way int, dst []site.Leap) []site.Leap {
		if f.wet[c] {
			return dst
		}
		at := c
		for spans := 0; spans <= longest; spans++ {
			n, ok := f.grid.Toward(at, way)
			switch {
			case !ok:
				return dst
			case !f.wet[n]:
				if spans > 0 {
					dst = append(dst, site.Leap{To: n, Spans: spans, Cost: cost})
				}
				return dst
			}
			at = n
		}
		return dst
	}
}

// run is n cells from x, y on, dx, dy a step
func run(g grid.Grid, x, y, dx, dy, n int) []cell.ID {
	out := make([]cell.ID, n)
	for k := range n {
		out[k] = g.At(uint32(x+k*dx), uint32(y+k*dy))
	}
	return out
}

// links are the links of c toward each of its neighbours ns
func links(g grid.Grid, c cell.ID, ns ...cell.ID) cell.Links {
	var l cell.Links
	for _, n := range ns {
		b, _ := grid.Link(g, c, n)
		l |= b
	}
	return l
}

func level(f *fakeGround, c cell.ID, h float32) {
	f.heights[c] = slices.Repeat([]float32{h}, len(f.grid.CellOutline(c, nil)))
}

// tilt lays the square c as a ramp rising k a cell eastward from k times its column
func tilt(f *fakeGround, c cell.ID, k float32) {
	x, _, _ := f.grid.Coords(c)
	w := k * float32(x)
	e := k * float32(x+1)
	f.heights[c] = []float32{w, e, e, w}
}

// length is how far route runs in steps of the grid's Distance
func length(g grid.Grid, route []cell.ID) float64 {
	sum := 0.0
	for k := 1; k < len(route); k++ {
		sum += g.NeighborCost(route[k-1], route[k])
	}
	return sum
}

// walks reports whether route runs from from to to a neighbour a step, failing t where not
func walks(t *testing.T, g grid.Grid, route []cell.ID, from, to cell.ID) bool {
	t.Helper()
	if len(route) == 0 || route[0] != from || route[len(route)-1] != to {
		t.Errorf("the road runs %v, not from %v to %v", where(g, route),
			where(g, []cell.ID{from}), where(g, []cell.ID{to}))
		return false
	}
	for k := 1; k < len(route); k++ {
		if _, ok := grid.Way(g, route[k-1], route[k]); !ok {
			t.Errorf("the road %v leaps from step %d", where(g, route), k-1)
			return false
		}
	}
	return true
}

// noWay fails t unless err refuses a road from from for finding no way
func noWay(t *testing.T, err error, from cell.ID) {
	t.Helper()
	var r *site.Refusal
	if !errors.As(err, &r) || r.At != from || !strings.Contains(r.Why(), "finds no way") {
		t.Errorf("got %v, want no way found from the start", err)
	}
}

// where is cells by their grid coordinates, for a message
func where(g grid.Grid, cells []cell.ID) [][2]uint32 {
	out := make([][2]uint32, len(cells))
	for k, c := range cells {
		x, y, _ := g.Coords(c)
		out[k] = [2]uint32{x, y}
	}
	return out
}
