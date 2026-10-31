package pathfind

import (
	"maps"
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/hpastar/graph"
)

// crossings is a board 32 cells by 16 of land, a river down column 15 crossed by a band 6 wide
// along row 3, a rope bearing ropeLoad in row 8 and a ford in row 14, and a wall down column 24
// but its last row with a gap gapWidth wide and gapHeight tall in row 4
type crossings struct {
	limitedTerrain
	g grid.Grid
}

func newCrossings() crossings {
	g := grid.DefaultGrids{}.Square(32, 16, 10)
	terrain := openTerrain()
	for y := range uint32(16) {
		terrain.Set(g.At(15, y), water)
		if y < 15 {
			terrain.Set(g.At(24, y), wall)
		}
	}
	for _, y := range []uint32{3, 8} {
		terrain.Set(g.At(15, y), cell.Kind{Pace: 1, Allows: cell.Land | cell.Water})
	}
	terrain.Set(g.At(15, 14), cell.Kind{Pace: 1.0 / 3, Allows: cell.Land | cell.Water})
	terrain.Set(g.At(24, 4), cell.Kind{Pace: 1, Allows: cell.Land, Solid: true})
	return crossings{limitedTerrain{TerrainMap: terrain,
		loads: map[cell.ID]float32{g.At(15, 8): ropeLoad},
		gaps:  map[cell.ID]cell.Gap{g.At(24, 4): {Width: gapWidth, Height: gapHeight}}}, g}
}

func (b crossings) Lane(from, to cell.ID, d cell.Domain, deck bool) float32 {
	x0, y0, _ := b.g.Coords(from)
	x1, y1, _ := b.g.Coords(to)
	if d&cell.Land == 0 || deck || y0 != 3 || y1 != 3 || max(x0, x1) < 15 || min(x0, x1) > 15 {
		return f32.Inf(1)
	}
	return 6
}

// the river of moated(x) with a deck 1 up over its cell in row 4 alone, its underside at 0.5,
// bearing ropeLoad
type footbridge struct {
	limitedTerrain
	g grid.Grid
	x uint32
}

func newFootbridge(x uint32) footbridge {
	g, river := moated(x)
	return footbridge{limitedTerrain{TerrainMap: river, raised: true,
		loads: map[cell.ID]float32{g.At(x, 4): ropeLoad}}, g, x}
}

func (b footbridge) Deck(c cell.ID) (top, under float32, carries cell.Domain, ok bool) {
	if x, y, _ := b.g.Coords(c); x == b.x && y == 4 {
		return 1, 0.5, cell.Land, true
	}
	return 0, 0, 0, false
}

func (b footbridge) Ground(geom.Vec) float32 { return 0 }

func (b footbridge) Over(p geom.Vec) (float32, bool) {
	if c, ok := b.g.CellAt(p); ok {
		if _, under, _, ok := b.Deck(c); ok {
			return under, true
		}
	}
	return 0, false
}

func (b footbridge) KindAt(c cell.ID, deck bool) cell.Kind {
	if _, _, _, ok := b.Deck(c); ok && deck {
		return cell.Kind{Pace: 1, Allows: cell.Land}
	}
	return b.Kind(c)
}

// a board each class's body meets its own way, the bodies its hierarchy serves
type classedBoard struct {
	f       *Finder
	classes []Body
}

// the boards of the classes: a band, a deck raised over a gorge and one over a river, a deck
// bearing a class and not another, a rope, a gap, a gap and a floor flanking slantwise steps, a
// deck slantwise; roads over hills limit none
func classedBoards() map[string]classedBoard {
	v := newGorge()
	rg, river := moated(11)
	cr := newCrossings()
	lane := newBridged()
	ropeG, rope := roped()
	gapG := grid.DefaultGrids{}.Square(7, 5, 10)
	gap := gapped(gapG)
	causewayG := grid.DefaultGrids{}.Square(5, 5, 10)
	cw := causeway{causewayG, cell.NewTerrainMap()}
	cw.terrain.SetAll(cell.Kind{Pace: 1, Allows: cell.Water})
	cw.terrain.Set(causewayG.At(0, 0), cell.Kind{Pace: 1, Allows: cell.Land})
	cw.terrain.Set(causewayG.At(4, 4), cell.Kind{Pace: 1, Allows: cell.Land})
	sq := grid.DefaultGrids{}.Square(24, 24, 10)
	roads, _ := scattered(sq, 3, 5)
	foot := newFootbridge(11)
	openG := grid.DefaultGrids{}.Square(7, 7, 10)
	open := limitedTerrain{TerrainMap: openTerrain(),
		loads: map[cell.ID]float32{openG.At(4, 4): ropeLoad},
		gaps:  map[cell.ID]cell.Gap{openG.At(2, 2): {Width: gapWidth}}}
	open.Set(openG.At(2, 2), cell.Kind{Pace: 1, Allows: cell.Land, Solid: true})
	sized := []Body{{}, {Size: box(4), Height: 1.5}, {Size: geom.NewVec(8, 4)},
		{Size: geom.NewVec(4, 8)}, {Size: box(8)}, {Size: box(4), Height: 3}}
	return map[string]classedBoard{
		"a band": {New(lane.g, lane, nil, &cell.MultipleOccupancy{}).WithLevels(flat{lane}),
			sized},
		"a gorge under a deck": {New(v.g, v.terrain, nil, &cell.MultipleOccupancy{}).
			WithLevels(v), []Body{{Height: 2}, {Step: 1, Height: 2}, {Step: 1, Height: 12},
			{Step: 1, Height: 2, Size: box(8)}}},
		"a river bridged": {New(rg, river, nil, &cell.MultipleOccupancy{}).
			WithLevels(bridgedRiver{rg, river}), []Body{{Height: 2}, {Step: 0.5, Height: 0.4},
			{Step: 1, Height: 2}}},
		"a deck bearing the light": {New(rg, weakDeck(rg, river), nil,
			&cell.MultipleOccupancy{}).WithLevels(bridgedRiver{rg, river}),
			[]Body{{Step: 1, Height: 2, Weight: 80}, {Step: 1, Height: 2, Weight: 1500}}},
		"a footbridge bearing the light": {New(foot.g, foot, nil, &cell.MultipleOccupancy{}).
			WithLevels(foot), []Body{{Step: 1, Height: 2, Weight: 80},
			{Step: 1, Height: 2, Weight: 1500}}},
		"a rope": {New(ropeG, rope, nil, &cell.MultipleOccupancy{}).WithLevels(flat{rope}),
			[]Body{{}, {Weight: 80}, {Weight: 1500}}},
		"a gap": {New(gapG, gap, nil, &cell.MultipleOccupancy{}).WithLevels(flat{gap}), sized},
		"a gap and a weak floor in the open, corners cut past them": {New(openG, open, nil,
			&cell.MultipleOccupancy{}).WithLevels(flat{open}), []Body{{}, {Size: box(4)},
			{Size: box(8)}, {Weight: 1500}, {Size: box(4), Weight: 80}}},
		"crossings": {New(cr.g, cr, nil, &cell.MultipleOccupancy{}).WithLevels(flat{cr}),
			[]Body{{Size: box(4), Height: 1.5, Weight: 80}, {Size: geom.NewVec(8, 4),
				Weight: 80}, {Size: box(8), Weight: 1500}, {Size: box(4), Height: 3}, {}}},
		"a causeway slantwise": {New(causewayG, cw, nil, &cell.MultipleOccupancy{}).
			WithLevels(cw), []Body{{Step: 1, Height: 2}, {Step: 0.5, Height: 2}}},
		"roads over hills": {New(sq, roads, hillsOver(sq), &cell.MultipleOccupancy{}).
			WithLevels(flat{roads}), []Body{{}, {Size: box(4)}, {Weight: 80},
			{Step: 1, Height: 2}}},
	}
}

// a step of the graph, its cost to the bit and the classes refusing it
type classedEdge struct {
	from, to node
	cost     uint64
	refuses  uint8
}

// each class takes out of a node of the graph what leveledFrom takes for its body where it
// stands on the node, at the same cost, and nothing where it does not; no step is refused by
// every class, and the steps into a node mirror those out of the others, refusals and all
func TestHierarchy_TheGraphRefusesAClassWhatItsBodyIsRefused(t *testing.T) {
	for name, b := range classedBoards() {
		t.Run(name, func(t *testing.T) {
			f := b.f
			h := builtWhole(t, f, land, 4, b.classes...)
			f.building(h)
			forward, backward := map[classedEdge]int{}, map[classedEdge]int{}
			var buf []graph.Transition[node]
			refused := 0
			for _, a := range nodesOf(f) {
				buf = f.classedFrom(h, a, a, buf)
				steps := slices.Clone(buf)
				for _, s := range steps {
					if s.Refuses == h.all {
						t.Fatalf("the step %v->%v refused by every class", a, s.To)
					}
					if s.Refuses != 0 {
						refused++
					}
					forward[classedEdge{a, s.To, math.Float64bits(s.Cost), s.Refuses}]++
				}
				for _, s := range f.classedInto(h, a, a, buf) {
					backward[classedEdge{s.To, a, math.Float64bits(s.Cost), s.Refuses}]++
				}
				for k, body := range h.Classes() {
					f.body = body
					r := f.at(a.c)
					want := map[node]uint64{}
					if f.stepAt(r, a.level).admits && f.bears(r, a.level) {
						for _, s := range f.leveledFrom(a, a, nil) {
							want[s.To] = math.Float64bits(s.Cost)
						}
					}
					got := map[node]uint64{}
					for _, s := range steps {
						if s.Refuses&(1<<k) == 0 {
							got[s.To] = math.Float64bits(s.Cost)
						}
					}
					if !maps.Equal(got, want) {
						t.Fatalf("class %d %v out of %v: steps %v, its body's %v", k, body, a, got,
							want)
					}
				}
			}
			if len(forward) == 0 || !maps.Equal(forward, backward) {
				t.Errorf("%d steps forward, %d backward; want each seen from both its ends alike",
					len(forward), len(backward))
			}
			if bodily := name != "roads over hills"; bodily != (refused > 0) {
				t.Errorf("%d steps refused some class; want some where a body is measured, none "+
					"where nothing is", refused)
			}
		})
	}
}

// a hierarchy of one class, whose Refuses hpastar never reads, is built over the steps
// leveledFrom takes for its body out of every node, those it stands on giving the classed graph's
// steps for it alone, the steps into a node mirroring them; one of several, over the classed graph
func TestHierarchy_OneClassIsBuiltOverTheStepsOfItsBody(t *testing.T) {
	for name, b := range classedBoards() {
		t.Run(name, func(t *testing.T) {
			f := b.f
			for _, body := range b.classes {
				h := builtWhole(t, f, land, 4, body)
				forward, backward := h.steps(f)
				f.building(h)
				out, in := map[edge]int{}, map[edge]int{}
				var buf, want []graph.Transition[node]
				for _, a := range nodesOf(f) {
					got := slices.Clone(forward(a, a, buf))
					if want = f.leveledFrom(a, a, want); !slices.Equal(got, want) {
						t.Fatalf("%v out of %v: %v, leveledFrom's %v", body, a, got, want)
					}
					if h.classesOn(a) == 0 {
						continue
					}
					if classed := f.classedFrom(h, a, a, nil); !slices.Equal(classed, got) {
						t.Fatalf("%v out of %v: %v, the classed graph's %v", body, a, got,
							classed)
					}
					for _, s := range got {
						out[edge{a, s.To, math.Float64bits(s.Cost)}]++
					}
					for _, s := range backward(a, a, buf) {
						in[edge{s.To, a, math.Float64bits(s.Cost)}]++
					}
				}
				if len(out) == 0 || !maps.Equal(out, in) {
					t.Errorf("%v: %d steps forward, %d backward; want each seen from both its "+
						"ends alike", body, len(out), len(in))
				}
			}
			h := builtWhole(t, f, land, 4, b.classes...)
			forward, _ := h.steps(f)
			f.building(h)
			for _, a := range nodesOf(f) {
				got := slices.Clone(forward(a, a, nil))
				if classed := f.classedFrom(h, a, a, nil); !slices.Equal(got, classed) {
					t.Fatalf("%d classes out of %v: %v, the classed graph's %v", len(b.classes),
						a, got, classed)
				}
			}
		})
	}
}

// a board with no band, no deck and no limit reads every step once for all its classes
func TestHierarchy_AStepMeetingNothingIsReadOnceForEveryClass(t *testing.T) {
	b := classedBoards()["roads over hills"]
	f := b.f
	h := builtWhole(t, f, land, 4, b.classes...)
	f.building(h)
	for _, a := range nodesOf(f) {
		rf := f.at(a.c)
		for i := range f.grid.Directions() {
			n, ok := f.grid.Toward(a.c, i)
			if !ok {
				continue
			}
			c := f.cornerOf(a.c, n, i)
			if f.bodily(a, rf, f.at(n), ground, c, f.laneOf(a.c, n, false)) {
				t.Fatalf("the step %v->%v measured against a body", a, n)
			}
		}
	}
}

// pairs are count pairs of cells g's terrain admits land on, at least apart by the grid's
// distance, a route between them or not
func pairs(g grid.Grid, terrain cell.Terrain, count int, apart float64, seed uint64) [][2]cell.ID {
	var cells []cell.ID
	g.EachCell(func(c cell.ID) {
		if terrain.Kind(c).Admits(cell.Land) {
			cells = append(cells, c)
		}
	})
	rng := rand.New(rand.NewPCG(seed, 3))
	var out [][2]cell.ID
	for tries := 0; len(out) < count && tries < 100*count; tries++ {
		a, b := cells[rng.IntN(len(cells))], cells[rng.IntN(len(cells))]
		if g.Distance(a, b) >= apart {
			out = append(out, [2]cell.ID{a, b})
		}
	}
	return out
}

// every class of a hierarchy climbs the routes a hierarchy of that class alone climbs and reaches
// what it reaches, to the step and the deck, on the boards wide enough for a climb
func TestHierarchy_AClassClimbsAsAHierarchyOfItsOwn(t *testing.T) {
	boards := classedBoards()
	sq := grid.DefaultGrids{}.Square(64, 64, 16)
	walls := walled(sq)
	boards["walls over a relief"] = classedBoard{New(sq, walls, terrainHills(sq),
		&cell.MultipleOccupancy{}).WithLevels(flat{walls}), []Body{{}, {Size: box(4)},
		{Weight: 80}}}
	const side = 4
	for name, b := range boards {
		prs := pairs(b.f.grid, b.f.terrain, 48, 2*side, 1)
		if len(prs) == 0 {
			continue
		}
		t.Run(name, func(t *testing.T) {
			f := b.f
			h := builtWhole(t, f, land, side, b.classes...)
			var dst, own [256]cell.ID
			climbed := 0
			for k, body := range h.Classes() {
				alone := builtWhole(t, f, land, side, body)
				for _, pr := range prs {
					from, to := pr[0], pr[1]
					n, ok := f.Climb(h, 1, cell.Land, body, from, to, Either, dst[:])
					decks := f.Decks()
					m, wok := f.Climb(alone, 1, cell.Land, body, from, to, Either, own[:])
					if ok != wok || !slices.Equal(dst[:n], own[:m]) || decks != f.Decks() {
						t.Fatalf("class %d %v from %v to %v: climbed %v %v on %b, alone %v %v "+
							"on %b", k, body, from, to, ok, dst[:n], decks, wok, own[:m],
							f.Decks())
					}
					if ok {
						climbed++
					}
					reached := f.Reaches(h, cell.Land, body, from, to)
					if alone := f.Reaches(alone, cell.Land, body, from, to); reached != alone {
						t.Fatalf("class %d %v from %v to %v: reached %v, alone %v", k, body, from,
							to, reached, alone)
					}
				}
			}
			if climbed == 0 {
				t.Error("no route climbed")
			}
		})
	}
}

// a one-cell deck over a one-cell river bearing the light alone, the river on a border of its
// clusters or within one: the steps off the deck join the banks for no class it does not bear,
// as the labels of a class join a step's ends either way
func TestHierarchy_ABridgeBearingOneClassJoinsTheBanksForItAlone(t *testing.T) {
	light, heavy := Body{Step: 1, Height: 2, Weight: 80}, Body{Step: 1, Height: 2, Weight: 1500}
	for name, x := range map[string]uint32{"on a border": 11, "within a cluster": 10} {
		t.Run(name, func(t *testing.T) {
			b := newFootbridge(x)
			f := New(b.g, b, nil, &cell.MultipleOccupancy{}).WithLevels(b)
			h := builtWhole(t, f, land, 4, light, heavy)
			bank, island := b.g.At(1, 4), b.g.At(14, 3)
			for _, pr := range [][2]cell.ID{{bank, island}, {island, bank}} {
				if !f.Reaches(h, cell.Land, light, pr[0], pr[1]) ||
					f.Reaches(h, cell.Land, heavy, pr[0], pr[1]) {
					t.Errorf("from %v to %v: reached by the light %v, by the heavy %v; want the "+
						"light alone", pr[0], pr[1], f.Reaches(h, cell.Land, light, pr[0], pr[1]),
						f.Reaches(h, cell.Land, heavy, pr[0], pr[1]))
				}
			}
			var dst [64]cell.ID
			n, ok := f.Climb(h, 1, cell.Land, light, bank, island, Either, dst[:])
			if k := slices.Index(dst[:n], b.g.At(x, 4)); !ok || k < 0 || f.Decks()>>k&1 == 0 {
				t.Errorf("the light climbed %v %v on decks %b; want over the river on the deck",
					ok, dst[:n], f.Decks())
			}
			if n, ok := f.Climb(h, 1, cell.Land, heavy, bank, island, Either, dst[:]); ok {
				t.Errorf("the heavy climbed %v over a deck that bears it not", dst[:n])
			}
			before := f.Searches()
			if c, n, ok := f.NearestFreeThrough(h, 1, cell.Land, heavy, bank, island, Either,
				nil, dst[:]); ok {
				t.Errorf("a free cell %v round the island for the heavy by %v", c, dst[:n])
			}
			if searched := f.Searches() - before; searched != 1 {
				t.Errorf("%d searches round the island for the heavy, want the first alone",
					searched)
			}
		})
	}
}

// climbs and reaches asked as one class and another in turn allocate nothing once built
func TestHierarchy_AClassAskedInTurnAllocatesNothing(t *testing.T) {
	g := grid.DefaultGrids{}.Square(terrainSide, terrainSide, terrainCell)
	terrain := openTerrain()
	for y := range uint32(100) {
		terrain.Set(g.At(64, y), wall)
	}
	f := New(g, terrain, terrainHills(g), &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	classes := []Body{{}, {Size: box(4)}, {Weight: 80}}
	h := builtWhole(t, f, land, 16, classes...)
	pr := farPairs(t, f, 1, 100, 5)[0]
	var dst [MaxPathLength]cell.ID
	ask := func() {
		for _, body := range classes {
			if _, ok := f.Climb(h, 1, cell.Land, body, pr[0], pr[1], Either, dst[:]); !ok {
				t.Fatal("no climb")
			}
			if !f.Reaches(h, cell.Land, body, pr[0], pr[1]) {
				t.Fatal("not reached")
			}
		}
	}
	ask()
	if allocs := testing.AllocsPerRun(10, ask); allocs != 0 {
		t.Errorf("climbs and reaches of three classes in turn allocate %v times; want none",
			allocs)
	}
}

func TestHierarchy_ServesOneToEightClasses(t *testing.T) {
	g := grid.DefaultGrids{}.Square(16, 16, 10)
	f := New(g, openTerrain(), nil, &cell.MultipleOccupancy{}).WithLevels(flat{openTerrain()})
	for _, n := range []int{0, MaxClasses + 1} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("a hierarchy of %d classes made", n)
				}
			}()
			f.NewHierarchy(land, 4, make([]Body, n))
		}()
	}
	classes := make([]Body, MaxClasses)
	for k := range classes {
		classes[k] = Body{Feet: 3, Weight: float32(k)}
	}
	h := builtWhole(t, f, land, 4, classes...)
	for k := range classes {
		if got, ok := h.classOf(Body{Feet: 9, Weight: float32(k)}); !ok || got != k ||
			h.Classes()[k].Feet != 0 {
			t.Errorf("class %d found as %d %v, kept on feet %v; want itself, no feet", k, got, ok,
				h.Classes()[k].Feet)
		}
	}
}
