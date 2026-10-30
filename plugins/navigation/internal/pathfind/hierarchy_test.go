package pathfind

import (
	"maps"
	"math"
	"math/rand/v2"
	"slices"
	"sync"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/hpastar/graph"
)

// a gorge between rims 10 up, its floor at 0 with a river two cells wide down its middle, a deck
// 10 up over row 6 from rim to rim, its underside at 9
type gorge struct {
	g       grid.Grid
	terrain *cell.TerrainMap
}

func newGorge() gorge {
	g := grid.DefaultGrids{}.Square(48, 16, 10)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	for y := range uint32(16) {
		terrain.Set(g.At(23, y), cell.Kind{Pace: 1, Allows: cell.Water})
		terrain.Set(g.At(24, y), cell.Kind{Pace: 1, Allows: cell.Water})
	}
	return gorge{g, terrain}
}

func (v gorge) Deck(c cell.ID) (top, under float32, carries cell.Domain, ok bool) {
	if x, y, _ := v.g.Coords(c); y == 6 && x >= 12 && x <= 35 {
		return 10, 9, cell.Land, true
	}
	return 0, 0, 0, false
}

func (v gorge) Ground(p geom.Vec) float32 {
	if p.X <= 120 || p.X >= 360 {
		return 10
	}
	return 0
}

func (v gorge) Over(p geom.Vec) (float32, bool) {
	if c, ok := v.g.CellAt(p); ok {
		if _, under, _, ok := v.Deck(c); ok {
			return under, true
		}
	}
	return 0, false
}

func (v gorge) KindAt(c cell.ID, deck bool) cell.Kind {
	if _, _, _, ok := v.Deck(c); ok && deck {
		return cell.Kind{Pace: 1, Allows: cell.Land, Graded: true}
	}
	return v.terrain.Kind(c)
}

func wrapped(g grid.Grid) grid.Grid {
	g.(interface{ SetWrap(x, y bool) }).SetWrap(true, true)
	return g
}

// the terrain rungs' board, a quarter of its cells walls
func walled(g grid.Grid) *board.BoardRead {
	brd, brdW := board.NewBoard(g)
	layWalls(brd, brdW, g.At(0, 0), g.At(0, 0))
	return brd
}

// builtWhole is a hierarchy f built whole for key, its clusters side cells a side
func builtWhole(t testing.TB, f *Finder, key Key, side int) *Hierarchy {
	t.Helper()
	h := f.NewHierarchy(key, side)
	if f.Build(h, math.MaxInt); !h.Ready() {
		t.Fatal("the hierarchy is not built")
	}
	return h
}

var land = Key{Domain: cell.Land}

// every node of f's grid, each level of each cell
func nodesOf(f *Finder) []node {
	var nodes []node
	f.grid.EachCell(func(c cell.ID) {
		for _, l := range f.levelsOf(f.at(c)) {
			nodes = append(nodes, node{c, l})
		}
	})
	return nodes
}

// a step of the graph and its cost to the bit
type edge struct {
	from, to node
	cost     uint64
}

// the steps forward from every node, and the steps backward into every node turned round
func edgesOf(f *Finder) (forward, backward map[edge]int) {
	forward, backward = map[edge]int{}, map[edge]int{}
	var buf []graph.Transition[node]
	for _, a := range nodesOf(f) {
		buf = f.leveledFrom(a, a, buf)
		for _, s := range buf {
			forward[edge{a, s.To, math.Float64bits(s.Cost)}]++
		}
		buf = f.leveledInto(a, a, buf)
		for _, s := range buf {
			backward[edge{s.To, a, math.Float64bits(s.Cost)}]++
		}
	}
	return forward, backward
}

func TestHierarchy_TheBackwardStepsMirrorTheForward(t *testing.T) {
	v := newGorge()
	sq := grid.DefaultGrids{}.Square(24, 24, 10)
	scatteredSq, _ := scattered(sq, 3, 5)
	hex := grid.DefaultGrids{}.Hex(24, 24, 10)
	scatteredHex, _ := scattered(hex, 0, 1)
	torus := wrapped(grid.DefaultGrids{}.Square(24, 24, 10))
	scatteredTorus, _ := scattered(torus, 3, 5)
	causewayG := grid.DefaultGrids{}.Square(5, 5, 10)
	cw := causeway{causewayG, cell.NewTerrainMap()}
	cw.terrain.SetAll(cell.Kind{Pace: 1, Allows: cell.Water})
	cw.terrain.Set(causewayG.At(0, 0), cell.Kind{Pace: 1, Allows: cell.Land})
	cw.terrain.Set(causewayG.At(4, 4), cell.Kind{Pace: 1, Allows: cell.Land})
	railed := fenced{openTerrain(), sq, geom.NewVec(95, 100), geom.NewVec(141, 100)}
	lane := newBridged()
	for name, tc := range map[string]struct {
		f    *Finder
		body Body
	}{
		"a gorge under a deck": {New(v.g, v.terrain, nil, &cell.MultipleOccupancy{}).
			WithLevels(v), Body{Step: 1, Height: 2}},
		"a tall body in the gorge": {New(v.g, v.terrain, nil, &cell.MultipleOccupancy{}).
			WithLevels(v), Body{Step: 1, Height: 12}},
		"roads over hills": {New(sq, scatteredSq, hillsOver(sq), &cell.MultipleOccupancy{}).
			WithLevels(flat{scatteredSq}), Body{}},
		"hex hills": {New(hex, scatteredHex, hillsOver(hex), &cell.MultipleOccupancy{}).
			WithLevels(flat{scatteredHex}), Body{}},
		"a torus": {New(torus, scatteredTorus, nil, &cell.MultipleOccupancy{}).
			WithLevels(flat{scatteredTorus}), Body{}},
		"a causeway slantwise": {New(causewayG, cw, nil, &cell.MultipleOccupancy{}).
			WithLevels(cw), Body{Step: 1, Height: 2}},
		"a railing": {New(sq, railed, nil, &cell.MultipleOccupancy{}).WithLevels(flat{railed}),
			Body{}},
		"a lane narrower than the body": {New(lane.g, lane, nil, &cell.MultipleOccupancy{}).
			WithLevels(flat{lane}), Body{Size: geom.NewVec(8, 8)}},
	} {
		t.Run(name, func(t *testing.T) {
			f := tc.f
			f.body, f.entity, f.blocked = tc.body, 0, nil
			f.goal, f.end = offGrid, Either
			f.begin(cell.Land)
			forward, backward := edgesOf(f)
			if len(forward) == 0 || !maps.Equal(forward, backward) {
				t.Errorf("%d steps forward, %d backward; want each seen from both its ends alike",
					len(forward), len(backward))
			}
		})
	}
}

// a goal on a deck is reached from the levels the route may end on, and seen so from the goal
func TestHierarchy_TheGoalIsReachedBackwardAsForward(t *testing.T) {
	v := newGorge()
	f := New(v.g, v.terrain, nil, &cell.MultipleOccupancy{}).WithLevels(v)
	f.body, f.entity, f.blocked = Body{Step: 1, Height: 2}, 0, nil
	goalCell := v.g.At(20, 6)
	for _, end := range []Level{Either, Ground, Deck} {
		f.goal, f.end = goalCell, end
		f.begin(cell.Land)
		var into []node
		for _, s := range f.leveledInto(node{goalCell, goal}, node{goalCell, goal}, nil) {
			into = append(into, s.To)
		}
		var from []node
		for _, l := range f.levelsOf(f.at(goalCell)) {
			a := node{goalCell, l}
			for _, s := range f.leveledFrom(a, a, nil) {
				if s.To.level == goal {
					from = append(from, a)
				}
			}
		}
		if len(into) == 0 || len(into) != len(from) || into[0] != from[0] ||
			len(into) == 2 && into[1] != from[1] {
			t.Errorf("to end on %v: the goal is reached from %v, seen from it from %v", end,
				from, into)
		}
	}
}

// fillOverInto fills the way to goal as FillField does, every step into a node read off
// leveledInto
func fillOverInto(f *Finder, goalCell cell.ID, body Body) ([]float32, []uint8) {
	n := f.grid.CellCount()
	cost, next, cells := make([]float32, 2*n), make([]uint8, 2*n), make([]cell.ID, n)
	f.grid.EachCell(func(c cell.ID) {
		o, _ := f.grid.Ordinal(c)
		cells[o] = c
	})
	for i := range cost {
		cost[i] = f32.Inf(1)
	}
	f.body, f.entity, f.blocked = body, 0, nil
	f.begin(cell.Land)
	var front frontier
	o, _ := f.grid.Ordinal(goalCell)
	for _, l := range f.levelsOf(f.at(goalCell)) {
		cost[2*o+int(l)], next[2*o+int(l)] = 0, fieldEnd
		front.push(int32(2*o+int(l)), 0)
	}
	var buf []graph.Transition[node]
	for {
		s, key, ok := front.pop()
		if !ok {
			break
		}
		if key > cost[s] {
			continue
		}
		t := node{cells[s>>1], uint8(s & 1)}
		buf = f.leveledInto(t, t, buf)
		for _, step := range buf {
			on, _ := f.grid.Ordinal(step.To.c)
			at := 2*on + int(step.To.level)
			if v := float32(float64(key) + step.Cost); v < cost[at] {
				way, _ := grid.Way(f.grid, step.To.c, t.c)
				cost[at], next[at] = v, uint8(way)<<1|t.level
				front.push(int32(at), v)
			}
		}
	}
	return cost, next
}

func TestField_IntoIsLeveledInto(t *testing.T) {
	v := newGorge()
	sq := grid.DefaultGrids{}.Square(24, 24, 10)
	roads, _ := scattered(sq, 3, 5)
	hex := grid.DefaultGrids{}.Hex(24, 24, 10)
	hexRoads, _ := scattered(hex, 0, 1)
	torus := wrapped(grid.DefaultGrids{}.Square(24, 24, 10))
	torusRoads, _ := scattered(torus, 3, 5)
	for name, tc := range map[string]struct {
		f    *Finder
		goal cell.ID
		body Body
	}{
		"a gorge under a deck": {New(v.g, v.terrain, nil, &cell.MultipleOccupancy{}).
			WithLevels(v), v.g.At(20, 6), Body{Step: 1, Height: 2}},
		"roads over hills": {New(sq, roads, hillsOver(sq), &cell.MultipleOccupancy{}).
			WithLevels(flat{roads}), sq.At(17, 15), Body{}},
		"hex hills": {New(hex, hexRoads, hillsOver(hex), &cell.MultipleOccupancy{}).
			WithLevels(flat{hexRoads}), hex.At(17, 15), Body{}},
		"a torus": {New(torus, torusRoads, nil, &cell.MultipleOccupancy{}).
			WithLevels(flat{torusRoads}), torus.At(0, 0), Body{}},
	} {
		t.Run(name, func(t *testing.T) {
			var seeds []cell.ID
			tc.f.grid.EachCell(func(c cell.ID) { seeds = append(seeds, c) })
			var field Field
			tc.f.FillField(&field, tc.goal, Either, cell.Land, tc.body, seeds, f32.Inf(1))
			cost, next := fillOverInto(tc.f, tc.goal, tc.body)
			reached := 0
			for s := range cost {
				if math.Float32bits(cost[s]) != math.Float32bits(field.cost[s]) ||
					!f32.IsInf(cost[s], 1) && next[s] != field.next[s] {
					t.Fatalf("node %d: the field %v by %d, over leveledInto %v by %d", s,
						field.cost[s], field.next[s], cost[s], next[s])
				}
				if !f32.IsInf(cost[s], 1) {
					reached++
				}
			}
			if reached < len(cost)/4 {
				t.Errorf("%d nodes of %d reached; want most", reached, len(cost))
			}
		})
	}
}

// a hierarchy is built over steps of which none leads to a goal, whatever its builder searched
// for last
func TestHierarchy_TheBuildLeadsNoStepToAGoal(t *testing.T) {
	g := grid.DefaultGrids{}.Square(64, 64, 16)
	brd := walled(g)
	f := New(g, brd, nil, &cell.MultipleOccupancy{}).WithLevels(flat{brd})
	farPairs(t, f, 1, 8, 1)
	if _, on := g.Ordinal(f.goal); !on {
		t.Fatalf("the goal %v searched for lies off the grid", f.goal)
	}
	h := f.NewHierarchy(land, 16)
	f.building(h)
	if _, on := g.Ordinal(f.goal); on {
		t.Fatalf("the builder's goal %v lies on the grid", f.goal)
	}
	var buf []graph.Transition[node]
	for _, a := range nodesOf(f) {
		for _, s := range f.leveledFrom(a, a, buf) {
			if s.To.level == goal {
				t.Fatalf("the step %v->%v leads to a goal", a, s.To)
			}
		}
	}
	builtWhole(t, f, land, 16)
}

func TestHierarchy_ClustersFollowTheGrid(t *testing.T) {
	v := newGorge()
	// built: the clusters of every level, four by four a cluster of the level over them
	for name, tc := range map[string]struct {
		f                     *Finder
		side, clusters, built int
	}{
		"square": {New(grid.DefaultGrids{}.Square(128, 128, 16), openTerrain(), nil,
			&cell.MultipleOccupancy{}).WithLevels(flat{openTerrain()}), 16, 64, 64 + 4},
		"torus": {New(wrapped(grid.DefaultGrids{}.Square(128, 128, 16)), openTerrain(), nil,
			&cell.MultipleOccupancy{}).WithLevels(flat{openTerrain()}), 16, 64, 64 + 4},
		"hex": {New(grid.DefaultGrids{}.Hex(48, 40, 10), openTerrain(), nil,
			&cell.MultipleOccupancy{}).WithLevels(flat{openTerrain()}), 16, 9, 9},
		"decks": {New(v.g, v.terrain, nil, &cell.MultipleOccupancy{}).WithLevels(v), 4, 48,
			48 + 3},
	} {
		t.Run(name, func(t *testing.T) {
			key := Key{Domain: cell.Land, Body: Body{Step: 1, Height: 2}}
			h := builtWhole(t, tc.f, key, tc.side)
			if len(h.counted) != tc.clusters || tc.f.Builds() != uint64(tc.built) {
				t.Fatalf("%d clusters on level 1, %d built on every level; want %d and %d",
					len(h.counted), tc.f.Builds(), tc.clusters, tc.built)
			}
			g := tc.f.grid
			cols := (h.cols + tc.side - 1) / tc.side
			tc.f.building(h)
			decks := 0
			g.EachCell(func(c cell.ID) {
				x, y, _ := g.Coords(c)
				want := int(y)/tc.side*cols + int(x)/tc.side
				for _, n := range []node{{c, ground}, {c, deck}, {c, goal}} {
					if k, ok := h.clusterOf(n); !ok || k != want {
						t.Fatalf("%v lies in cluster %d %v, want %d", n, k, ok, want)
					}
				}
				var stood []node
				r := tc.f.at(c)
				for _, l := range tc.f.levelsOf(r) {
					if tc.f.stepAt(r, l).admits {
						stood = append(stood, node{c, l})
					}
				}
				if laid := h.at(int(x), int(y), nil); !slices.Equal(laid, stood) {
					t.Fatalf("%v laid at %v, want the levels its class stands on %v", laid, c,
						stood)
				}
				if slices.Contains(stood, node{c, deck}) {
					decks++
				}
			})
			if name == "decks" && decks != 24 {
				t.Errorf("%d cells laid with a deck, want the 24 of the deck", decks)
			}
			// square past the last row, axial (0, CellCount) far below the last
			below := cell.ID(g.CellCount())
			if _, ok := h.clusterOf(node{below, ground}); ok {
				t.Errorf("a start off the grid lies in a cluster")
			}
		})
	}
}

// farPairs are count pairs of cells f's terrain admits land on, at least apart by the grid's
// distance, each with a route f finds between them
func farPairs(t *testing.T, f *Finder, count int, apart float64, seed uint64) [][2]cell.ID {
	t.Helper()
	var cells []cell.ID
	f.grid.EachCell(func(c cell.ID) {
		if f.terrain.Kind(c).Admits(cell.Land) {
			cells = append(cells, c)
		}
	})
	rng := rand.New(rand.NewPCG(seed, 7))
	var pairs [][2]cell.ID
	var dst [1024]cell.ID
	for tries := 0; len(pairs) < count && tries < 100*count; tries++ {
		a, b := cells[rng.IntN(len(cells))], cells[rng.IntN(len(cells))]
		if f.grid.Distance(a, b) < apart {
			continue
		}
		if _, ok := f.Find(1, cell.Land, Body{}, a, b, Either, dst[:]); ok {
			pairs = append(pairs, [2]cell.ID{a, b})
		}
	}
	if len(pairs) < count {
		t.Fatalf("%d pairs %v apart with a route, want %d", len(pairs), apart, count)
	}
	return pairs
}

// takenBy says whether every step of steps from from is one body takes, the last on to
func takenBy(f *Finder, body Body, from, to cell.ID, steps []cell.ID) bool {
	if len(steps) == 0 || steps[len(steps)-1] != to {
		return false
	}
	prev := from
	for _, s := range steps {
		if _, ok := grid.Way(f.grid, prev, s); !ok {
			return false
		}
		if _, ok := f.Steps(cell.Land, body, prev, s); !ok {
			return false
		}
		prev = s
	}
	return true
}

// climbsWithin climbs each pair through h and searches it: each climb a route of steps taken
// to its goal, costing at most a tenth more than the search's; it reports the mean and the
// worst of the climbs' costs over the searches'
func climbsWithin(t *testing.T, f *Finder, h *Hierarchy, pairs [][2]cell.ID) (mean,
	worst float64) {
	t.Helper()
	var climbed, found [1024]cell.ID
	for _, pr := range pairs {
		from, to := pr[0], pr[1]
		m, ok := f.Find(1, cell.Land, Body{}, from, to, Either, found[:])
		if !ok {
			t.Fatalf("no route from %v to %v", from, to)
		}
		search := priceOf(t, f, from, found[:m], f.Decks())
		n, ok := f.Climb(h, 1, cell.Land, Body{}, from, to, Either, climbed[:])
		if !ok {
			t.Errorf("no climb from %v to %v, %v apart", from, to, f.grid.Distance(from, to))
			continue
		}
		decks := f.Decks()
		if !takenBy(f, Body{}, from, to, climbed[:n]) {
			t.Errorf("the climb from %v to %v takes a step refused or ends off it: %v", from, to,
				climbed[:n])
			continue
		}
		ratio := priceOf(t, f, from, climbed[:n], decks) / search
		if ratio > 1.10 {
			t.Errorf("the climb from %v to %v costs %.3f of the search's", from, to, ratio)
		}
		mean, worst = mean+ratio/float64(len(pairs)), max(worst, ratio)
	}
	return mean, worst
}

func TestHierarchy_AClimbCostsWithinATenthOfASearch(t *testing.T) {
	sq := grid.DefaultGrids{}.Square(terrainSide, terrainSide, terrainCell)
	walls := walled(sq)
	hex := grid.DefaultGrids{}.Hex(48, 40, 10)
	hexWalls := walled(hex)
	torus := wrapped(grid.DefaultGrids{}.Square(terrainSide, terrainSide, terrainCell))
	torusWalls := walled(torus)
	for name, tc := range map[string]struct {
		f     *Finder
		side  int
		apart float64
	}{
		"map": {New(sq, walls, nil, &cell.MultipleOccupancy{}).WithLevels(flat{walls}), 16,
			64},
		"relief": {New(sq, walls, terrainHills(sq), &cell.MultipleOccupancy{}).
			WithLevels(flat{walls}), 16, 64},
		"hex": {New(hex, hexWalls, hillsOver(hex), &cell.MultipleOccupancy{}).
			WithLevels(flat{hexWalls}), 16, 32},
		"hex, two levels": {New(hex, hexWalls, hillsOver(hex), &cell.MultipleOccupancy{}).
			WithLevels(flat{hexWalls}), 8, 24},
		"torus": {New(torus, torusWalls, nil, &cell.MultipleOccupancy{}).
			WithLevels(flat{torusWalls}), 16, 64},
	} {
		t.Run(name, func(t *testing.T) {
			h := builtWhole(t, tc.f, land, tc.side)
			mean, worst := climbsWithin(t, tc.f, h, farPairs(t, tc.f, 64, tc.apart, 11))
			t.Logf("%s: %d clusters; climbs cost %.4f of the searches on average, %.4f at worst",
				name, tc.f.Builds(), mean, worst)
		})
	}
}

func TestHierarchy_AClimbCrossesADeck(t *testing.T) {
	v := newGorge()
	at := func(x, y uint32) cell.ID { return v.g.At(x, y) }
	f := New(v.g, v.terrain, nil, &cell.MultipleOccupancy{}).WithLevels(v)
	h := builtWhole(t, f, Key{Domain: cell.Land, Body: Body{Step: 1, Height: 2}}, 8)
	rim := Body{Feet: 10, Step: 1, Height: 2}
	var climbed, found [64]cell.ID
	n, ok := f.Climb(h, 1, cell.Land, rim, at(2, 6), at(45, 6), Either, climbed[:])
	decks := f.Decks()
	if !ok || climbed[n-1] != at(45, 6) {
		t.Fatalf("over the deck: climbed %v %v", ok, climbed[:n])
	}
	prev, onDeck := at(2, 6), 0
	for k, s := range climbed[:n] {
		_, _, _, decked := v.Deck(s)
		onto, ok := f.Steps(cell.Land, rim, prev, s)
		if on := decks>>k&1 != 0; !ok || on != decked || onto != on {
			t.Errorf("step %d to %v: on the deck %v, onto it %v %v; want the deck where it lies",
				k, s, on, onto, ok)
		}
		if decked {
			onDeck++
		}
		prev = s
	}
	m, _ := f.Find(1, cell.Land, rim, at(2, 6), at(45, 6), Either, found[:])
	climb, search := priceOf(t, f, at(2, 6), climbed[:n], decks),
		priceOf(t, f, at(2, 6), found[:m], f.Decks())
	if onDeck != 24 || climb > 1.10*search {
		t.Errorf("%d steps on the deck, want its 24; the climb costs %v, the search %v", onDeck,
			climb, search)
	}
	n, ok = f.Climb(h, 1, cell.Land, rim, at(20, 6), at(45, 6), Either, climbed[:])
	if decks = f.Decks(); !ok || climbed[n-1] != at(45, 6) || decks&0b111 != 0b111 {
		t.Errorf("from the deck: climbed %v %v on decks %b; want its first steps on it", ok,
			climbed[:n], decks)
	}
	for _, end := range []Level{Deck, Ground} {
		n, ok := f.Climb(h, 1, cell.Land, rim, at(2, 6), at(20, 6), end, climbed[:])
		ended := f.Ended()
		m, fok := f.Find(1, cell.Land, rim, at(2, 6), at(20, 6), end, found[:])
		if !ok || !fok || ended != end || f.Ended() != end || climbed[n-1] != at(20, 6) {
			t.Errorf("to end on %v: climbed %v %v ending on %v, found %v %v", end, ok,
				climbed[:n], ended, fok, found[:m])
		}
	}
}

func TestHierarchy_AClimbFillsDstLazily(t *testing.T) {
	g := grid.DefaultGrids{}.Square(terrainSide, terrainSide, terrainCell)
	terrain := openTerrain()
	f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	h := builtWhole(t, f, land, 16)
	var dst [16]cell.ID
	var expanded []uint64
	for _, x := range []uint32{40, 80, 120} {
		before := f.Expansions()
		n, ok := f.Climb(h, 1, cell.Land, Body{}, g.At(2, 64), g.At(x, 64), Either, dst[:])
		if !ok || n != len(dst) {
			t.Fatalf("to x=%d: climbed %v, %d steps; want the %d dst holds", x, ok, n, len(dst))
		}
		expanded = append(expanded, f.Expansions()-before)
	}
	// the start's cluster filled forward and the goal's backward, a node a cell each
	if bound := uint64(2 * 16 * 16); slices.Max(expanded) > bound ||
		expanded[2] > expanded[0] {
		t.Errorf("%v nodes expanded as the goal goes 38, 78, 118 cells off; want no more than "+
			"%d, and no more the farther", expanded, bound)
	}
	t.Logf("expanded %v climbing 16 steps toward goals 38, 78 and 118 cells off", expanded)
}

func TestHierarchy_TheSameRouteWhicheverFinderBuilds(t *testing.T) {
	g := grid.DefaultGrids{}.Square(terrainSide, terrainSide, terrainCell)
	brd := walled(g)
	a := New(g, brd, terrainHills(g), &cell.MultipleOccupancy{}).WithLevels(flat{brd})
	b := New(g, brd, terrainHills(g), &cell.MultipleOccupancy{}).WithLevels(flat{brd})
	ha, hb := builtWhole(t, a, land, 16), builtWhole(t, b, land, 16)
	var want, got [1024]cell.ID
	for _, pr := range farPairs(t, a, 16, 64, 3) {
		n, ok := a.Climb(ha, 1, cell.Land, Body{}, pr[0], pr[1], Either, want[:])
		if !ok {
			t.Fatalf("no climb from %v to %v", pr[0], pr[1])
		}
		for name, tc := range map[string]struct {
			f *Finder
			h *Hierarchy
		}{"b through a's": {b, ha}, "a through b's": {a, hb}, "b through its own": {b, hb}} {
			m, ok := tc.f.Climb(tc.h, 1, cell.Land, Body{}, pr[0], pr[1], Either, got[:])
			if !ok || !slices.Equal(got[:m], want[:n]) {
				t.Errorf("%s from %v to %v: %v, want a's %v", name, pr[0], pr[1], got[:m],
					want[:n])
			}
		}
	}
}

// over a terrain reading nothing anew, as a board with its cells does
func TestHierarchy_ClimbAllocatesNothingOnceBuilt(t *testing.T) {
	g := grid.DefaultGrids{}.Square(terrainSide, terrainSide, terrainCell)
	terrain := openTerrain()
	for y := range uint32(100) {
		terrain.Set(g.At(64, y), cell.Kind{Pace: 1, Solid: true})
	}
	f := New(g, terrain, terrainHills(g), &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	h := builtWhole(t, f, land, 16)
	pr := farPairs(t, f, 1, 100, 5)[0]
	var dst [MaxPathLength]cell.ID
	climb := func() {
		if _, ok := f.Climb(h, 1, cell.Land, Body{}, pr[0], pr[1], Either, dst[:]); !ok {
			t.Fatal("no climb")
		}
	}
	climb()
	if allocs := testing.AllocsPerRun(10, climb); allocs != 0 {
		t.Errorf("a climb allocates %v times once built; want none", allocs)
	}
	reach := func() {
		if !f.Reaches(h, cell.Land, Body{}, pr[0], pr[1]) {
			t.Fatal("not reached")
		}
	}
	reach()
	if allocs := testing.AllocsPerRun(10, reach); allocs != 0 {
		t.Errorf("a reach allocates %v times once built; want none", allocs)
	}
}

// two finders, each over a read of the board of its own, climb one hierarchy at once as one
// alone does
func TestHierarchy_FindersClimbAtOnceAsOneAlone(t *testing.T) {
	g := grid.DefaultGrids{}.Square(terrainSide, terrainSide, terrainCell)
	brd := walled(g)
	builder := New(g, brd, terrainHills(g), &cell.MultipleOccupancy{}).WithLevels(flat{brd})
	h := builtWhole(t, builder, land, 16)
	pairs := farPairs(t, builder, 32, 64, 9)
	finders := [2]*Finder{}
	for i := range finders {
		read := brd.Fork()
		finders[i] = New(g, read, terrainHills(g), &cell.MultipleOccupancy{}).
			WithLevels(flat{read})
	}
	type route struct {
		steps []cell.ID
		decks uint64
	}
	climbAll := func(f *Finder) []route {
		var routes []route
		var dst [MaxPathLength]cell.ID
		for _, pr := range pairs {
			n, ok := f.Climb(h, 1, cell.Land, Body{}, pr[0], pr[1], Either, dst[:])
			if !ok {
				return nil
			}
			routes = append(routes, route{slices.Clone(dst[:n]), f.Decks()})
		}
		return routes
	}
	want := climbAll(builder)
	if len(want) != len(pairs) {
		t.Fatal("a pair not climbed")
	}
	var got [2][]route
	var wg sync.WaitGroup
	for i, f := range finders {
		wg.Go(func() { got[i] = climbAll(f) })
	}
	wg.Wait()
	for i := range got {
		if !slices.EqualFunc(got[i], want, func(a, b route) bool {
			return a.decks == b.decks && slices.Equal(a.steps, b.steps)
		}) {
			t.Errorf("finder %d climbed other routes at once than one alone", i)
		}
	}
}

var wall = cell.Kind{Pace: 1, Solid: true}

// flipped has c a wall where it was open and open where it was a wall
func flipped(brd *board.BoardRead, brdW *board.BoardWrite, c cell.ID) {
	if brd.Kind(c).Admits(cell.Land) {
		brdW.Set(c, wall)
	} else {
		brdW.Set(c, cell.Kind{Pace: 1, Allows: cell.Land})
	}
}

// sameClimbs says whether f climbs each pair through h, each step taken, as through want
func sameClimbs(t *testing.T, f *Finder, h, want *Hierarchy, pairs [][2]cell.ID) {
	t.Helper()
	var got, wanted [1024]cell.ID
	for _, pr := range pairs {
		n, ok := f.Climb(h, 1, cell.Land, Body{}, pr[0], pr[1], Either, got[:])
		m, wok := f.Climb(want, 1, cell.Land, Body{}, pr[0], pr[1], Either, wanted[:])
		if ok != wok || !slices.Equal(got[:n], wanted[:m]) ||
			ok && !takenBy(f, Body{}, pr[0], pr[1], got[:n]) {
			t.Errorf("from %v to %v: climbed %v %v, built afresh %v %v", pr[0], pr[1], ok,
				got[:n], wok, wanted[:m])
		}
	}
}

// across is pairs of open cells either side of c along x, off cells off it on a board side cells
// a side, their routes passing by it
func across(g grid.Grid, terrain cell.Terrain, c cell.ID, side, off int) [][2]cell.ID {
	cx, cy, _ := g.Coords(c)
	var pairs [][2]cell.ID
	for dy := -6; dy <= 6; dy += 2 {
		ax, bx := (int(cx)-off+side)%side, (int(cx)+off)%side
		ay, by := (int(cy)+dy+side)%side, (int(cy)-dy+side)%side
		a, b := g.At(uint32(ax), uint32(ay)), g.At(uint32(bx), uint32(by))
		if terrain.Kind(a).Admits(cell.Land) && terrain.Kind(b).Admits(cell.Land) {
			pairs = append(pairs, [2]cell.ID{a, b}, [2]cell.ID{b, a})
		}
	}
	return pairs
}

// on a board of two levels, its clusters 8 cells a side
func TestHierarchy_AChangedCellStalesItsClusterAndTheRing(t *testing.T) {
	const side = 64
	sq := func() grid.Grid { return grid.DefaultGrids{}.Square(side, side, 16) }
	torus := func() grid.Grid { return wrapped(sq()) }
	for name, tc := range map[string]struct {
		g      func() grid.Grid
		x, y   uint32
		marked int
	}{
		"inside a cluster":     {sq, 20, 20, 1},
		"on a border":          {sq, 23, 20, 2},
		"at a corner":          {sq, 23, 23, 4},
		"across the seam":      {torus, 0, 20, 2},
		"at the seam's corner": {torus, 0, 0, 4},
	} {
		t.Run(name, func(t *testing.T) {
			g := tc.g()
			brd, brdW := board.NewBoard(g)
			layWalls(brd, brdW, g.At(0, 0), g.At(0, 0))
			f := New(g, brd, nil, &cell.MultipleOccupancy{}).WithLevels(flat{brd})
			h := builtWhole(t, f, land, 8)
			if f.Builds() != 64+4 {
				t.Fatalf("%d clusters built, want the 64 and 4 of two levels", f.Builds())
			}
			c := g.At(tc.x, tc.y)
			since := brd.Passage()
			k := brd.Kind(c)
			brdW.Set(c, cell.Kind{Name: cell.Named("meadow"), Pace: k.Pace, Allows: k.Allows,
				Solid: k.Solid})
			if marked := f.Stale(h, since); marked != 0 || !h.Ready() {
				t.Errorf("a cell renamed: %d clusters marked, ready %v; want none", marked,
					h.Ready())
			}
			for range 2 {
				since := brd.Passage()
				flipped(brd, brdW, c)
				pairs := across(g, brd, c, side, 20)
				var dst [64]cell.ID
				if _, ok := f.Climb(h, 1, cell.Land, Body{}, pairs[0][0], pairs[0][1], Either,
					dst[:]); ok {
					t.Error("a climb through a hierarchy not staled as the passage moved")
				}
				if marked := f.Stale(h, since); marked != tc.marked || h.Ready() {
					t.Errorf("%d clusters marked of level 1, ready %v; want %d", marked,
						h.Ready(), tc.marked)
				}
				for !h.Ready() {
					f.Build(h, 1)
				}
				b := New(g, brd, nil, &cell.MultipleOccupancy{}).WithLevels(flat{brd})
				sameClimbs(t, f, h, builtWhole(t, b, land, 8), pairs)
			}
		})
	}
}

// a terrain counting each cell's passage, as a board does, over a plain one reading nothing
// anew
type cellsPassing struct {
	*cell.TerrainMap
	cells map[cell.ID]uint64
}

func passingTerrain() *cellsPassing { return &cellsPassing{openTerrain(), map[cell.ID]uint64{}} }

func (t *cellsPassing) Set(c cell.ID, k cell.Kind) {
	t.TerrainMap.Set(c, k)
	t.cells[c] = t.Passage()
}

func (t *cellsPassing) CellPassage(c cell.ID) uint64 { return t.cells[c] }

func TestHierarchy_BuildAllocatesNothingTwice(t *testing.T) {
	g := grid.DefaultGrids{}.Square(terrainSide, terrainSide, terrainCell)
	terrain := passingTerrain()
	for y := range uint32(100) {
		terrain.Set(g.At(64, y), wall)
	}
	f := New(g, terrain, terrainHills(g), &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	h := builtWhole(t, f, land, 16)
	c := g.At(40, 40)
	rebuild := func() {
		since := terrain.Passage()
		if terrain.Kind(c).Admits(cell.Land) {
			terrain.Set(c, wall)
		} else {
			terrain.Set(c, cell.Kind{Pace: 1, Allows: cell.Land})
		}
		if f.Stale(h, since); h.Ready() {
			t.Fatal("ready as a cell changed")
		}
		f.Build(h, math.MaxInt)
	}
	rebuild()
	rebuild()
	if allocs := testing.AllocsPerRun(4, rebuild); allocs != 0 {
		t.Errorf("a cluster built again allocates %v times; want none", allocs)
	}
}

func TestHierarchy_CountsItsBuildsAndClimbs(t *testing.T) {
	g := grid.DefaultGrids{}.Square(terrainSide, terrainSide, terrainCell)
	terrain := openTerrain()
	f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	h := builtWhole(t, f, land, 16)
	if f.Builds() != 64+4 || f.Expansions() != 0 {
		t.Errorf("%d clusters built expanding %d nodes; want the 64 and 4 of its two levels, "+
			"expanding none a search would", f.Builds(), f.Expansions())
	}
	var dst [64]cell.ID
	f.Climb(h, 1, cell.Land, Body{}, g.At(2, 64), g.At(120, 64), Either, dst[:])
	f.Climb(h, 1, cell.Land, Body{}, g.At(2, 64), g.At(20, 64), Either, dst[:])
	if f.Climbs() != 1 || f.Searches() != 0 || f.Expansions() == 0 {
		t.Errorf("%d climbs, %d searches, %d nodes expanded; want the far route climbed, "+
			"expanding the ends' clusters", f.Climbs(), f.Searches(), f.Expansions())
	}
}

// hpastar's Update of no budget builds everything stale: a Build of none builds nothing; one of a
// budget below a cluster's work finishes a cluster at most, a cluster's work spread over Builds
func TestHierarchy_ABuildBuildsItsBudgetAndNoMore(t *testing.T) {
	g := grid.DefaultGrids{}.Square(terrainSide, terrainSide, terrainCell)
	terrain := openTerrain()
	f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	h := f.NewHierarchy(land, 16)
	if built := f.Build(h, 0); built != 0 || f.Builds() != 0 || h.Ready() {
		t.Fatalf("a budget of none built %d, counted %d, ready %v", built, f.Builds(),
			h.Ready())
	}
	sum, most, none, calls := 0, 0, 0, 0
	for ; !h.Ready() && calls < 10000; calls++ {
		built := f.Build(h, 5000)
		sum, most = sum+built, max(most, built)
		if built == 0 {
			none++
		}
	}
	if sum != 64+4 || f.Builds() != 64+4 || most > 1 || none == 0 {
		t.Errorf("built %d over %d Builds, at most %d one, %d none, counted %d; want the 64 and 4 "+
			"of its two levels, at most one a Build, some over several", sum, calls, most, none,
			f.Builds())
	}
}

func TestHierarchy_AClimbRefusesWhatItCannotAnswer(t *testing.T) {
	g := grid.DefaultGrids{}.Square(terrainSide, terrainSide, terrainCell)
	terrain := passingTerrain()
	f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	h := f.NewHierarchy(land, 16)
	from, to := g.At(2, 64), g.At(120, 64)
	var dst [64]cell.ID
	climbs := func(f *Finder, domain cell.Domain, body Body) bool {
		_, ok := f.Climb(h, 1, domain, body, from, to, Either, dst[:])
		return ok
	}
	if climbs(f, cell.Land, Body{}) || !f.Reaches(h, cell.Land, Body{}, from, from) {
		t.Error("before it is built: climbed, or a reach denied")
	}
	f.Build(h, math.MaxInt)
	if !climbs(f, cell.Land, Body{}) || !climbs(f, cell.Land, Body{Feet: 5}) {
		t.Error("built: no climb for its class, its feet anywhere")
	}
	if climbs(f, cell.Land, Body{Height: 2}) || climbs(f, cell.Air, Body{}) {
		t.Error("built for another class or domain: climbed")
	}
	since := terrain.Passage()
	terrain.Set(g.At(60, 10), wall)
	if climbs(f, cell.Land, Body{}) || !f.Reaches(h, cell.Land, Body{}, from, g.At(60, 10)) {
		t.Error("the passage moved, not staled: climbed, or a reach denied")
	}
	f.Stale(h, since)
	f.Build(h, math.MaxInt)
	if !climbs(f, cell.Land, Body{}) {
		t.Error("staled and built again: no climb")
	}
	plain := New(g, terrain, nil, &cell.MultipleOccupancy{})
	if climbs(plain, cell.Land, Body{}) || !plain.Reaches(h, cell.Land, Body{}, from, to) {
		t.Error("a finder without levels: climbed, or a reach denied")
	}
	if h := plain.NewHierarchy(land, 16); h.Ready() || plain.Stale(h, 0) != 0 {
		t.Error("a hierarchy over a finder without levels is ready or staled")
	} else if built := plain.Build(h, math.MaxInt); built != 0 || h.Ready() {
		t.Errorf("a hierarchy over a finder without levels built %d, ready %v", built,
			h.Ready())
	}
}

// pairs nearer than a level-1 cluster's two sides are refused for a search, which routes them
// exactly
func TestHierarchy_ANearPairAcrossABorderIsSearched(t *testing.T) {
	type pair [2][2]uint32
	for name, tc := range map[string]struct {
		g    grid.Grid
		near []pair
		far  pair
	}{
		"two levels": {grid.DefaultGrids{}.Square(terrainSide, terrainSide, terrainCell),
			[]pair{{{15, 40}, {16, 40}}, {{15, 15}, {16, 16}}, {{63, 40}, {64, 40}},
				{{63, 63}, {64, 64}}, {{2, 40}, {33, 40}}, {{60, 70}, {66, 60}}},
			pair{{2, 40}, {34, 40}}},
		"a torus": {wrapped(grid.DefaultGrids{}.Square(terrainSide, terrainSide, terrainCell)),
			[]pair{{{0, 40}, {127, 40}}, {{0, 0}, {127, 127}}, {{5, 40}, {120, 40}},
				{{40, 0}, {40, 127}}, {{3, 3}, {125, 124}}},
			pair{{16, 40}, {112, 40}}},
	} {
		t.Run(name, func(t *testing.T) {
			g := tc.g
			terrain := openTerrain()
			f := New(g, terrain, terrainHills(g), &cell.MultipleOccupancy{}).
				WithLevels(flat{terrain})
			h := builtWhole(t, f, land, 16)
			if f.Builds() != 64+4 {
				t.Fatalf("%d clusters built, want the 64 and 4 of two levels", f.Builds())
			}
			var dst [256]cell.ID
			for _, pr := range tc.near {
				from, to := g.At(pr[0][0], pr[0][1]), g.At(pr[1][0], pr[1][1])
				if n, ok := f.Climb(h, 1, cell.Land, Body{}, from, to, Either, dst[:]); ok {
					t.Errorf("%v, %v cells off: climbed %v", pr, g.Distance(from, to), dst[:n])
				}
				n, ok := f.Find(1, cell.Land, Body{}, from, to, Either, dst[:])
				if !ok || !takenBy(f, Body{}, from, to, dst[:n]) {
					t.Fatalf("%v: found %v %v", pr, ok, dst[:n])
				}
				var field Field
				f.FillField(&field, to, Either, cell.Land, Body{}, []cell.ID{from}, 2)
				least, _ := field.Cost(from, Either)
				if cost := priceOf(t, f, from, dst[:n], f.Decks()); !near(cost, least) {
					t.Errorf("%v: the search's route costs %v, the cheapest %v", pr, cost, least)
				}
			}
			from, to := g.At(tc.far[0][0], tc.far[0][1]), g.At(tc.far[1][0], tc.far[1][1])
			n, ok := f.Climb(h, 1, cell.Land, Body{}, from, to, Either, dst[:])
			if !ok || !takenBy(f, Body{}, from, to, dst[:n]) {
				t.Errorf("%v, %v cells off: climbed %v %v; want the far pair climbed", tc.far,
					g.Distance(from, to), ok, dst[:n])
			}
		})
	}
}

// land with an island in a ring of water, a wall bent round a pocket of one cluster open to
// the cluster to the north, a wall cutting another cluster in two, and a box shut all round
func islands(g grid.Grid) *cell.TerrainMap {
	terrain := openTerrain()
	g.EachCell(func(c cell.ID) {
		x, y, _ := g.Coords(c)
		dx, dy := int(x)-12, int(y)-44
		if max(abs(float64(dx)), abs(float64(dy))) == 6 {
			terrain.Set(c, cell.Kind{Pace: 1, Allows: cell.Water})
		}
		u := (x == 35 || x == 44) && y >= 32 && y <= 44 || y == 44 && x >= 35 && x <= 44
		split := x == 52 && y >= 32 && y <= 47
		box := (x == 36 || x == 44) && y >= 4 && y <= 12 || (y == 4 || y == 12) && x >= 36 &&
			x <= 44
		if u || split || box {
			terrain.Set(c, wall)
		}
	})
	return terrain
}

func TestHierarchy_ReachesAgreesWithASearch(t *testing.T) {
	g := grid.DefaultGrids{}.Square(64, 64, 16)
	terrain := islands(g)
	f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	h := builtWhole(t, f, land, 16)
	var dst [1024]cell.ID
	agree := func(t *testing.T, from, to cell.ID) bool {
		_, found := f.Find(1, cell.Land, Body{}, from, to, Either, dst[:])
		if reached := f.Reaches(h, cell.Land, Body{}, from, to); reached != found {
			t.Errorf("from %v to %v: reached %v, found %v", from, to, reached, found)
		}
		return found
	}
	at := g.At
	for name, tc := range map[string]struct {
		from, to cell.ID
		reached  bool
	}{
		"onto the island":            {at(30, 30), at(12, 44), false},
		"off the island":             {at(10, 42), at(60, 60), false},
		"about the island":           {at(8, 40), at(16, 48), true},
		"out of the pocket":          {at(40, 40), at(40, 46), true},
		"into the pocket":            {at(33, 46), at(39, 42), true},
		"across the cut cluster":     {at(50, 40), at(56, 40), true},
		"into the box":               {at(30, 8), at(40, 8), false},
		"out of the box":             {at(40, 8), at(52, 20), false},
		"across the land":            {at(2, 2), at(60, 62), true},
		"within one cluster, unseen": {at(17, 17), at(30, 30), true},
	} {
		if found := agree(t, tc.from, tc.to); found != tc.reached {
			t.Errorf("%s: found %v, want %v", name, found, tc.reached)
		}
	}
	sq := grid.DefaultGrids{}.Square(terrainSide, terrainSide, terrainCell)
	walls := walled(sq)
	f = New(sq, walls, nil, &cell.MultipleOccupancy{}).WithLevels(flat{walls})
	h = builtWhole(t, f, land, 16)
	var cells []cell.ID
	sq.EachCell(func(c cell.ID) {
		if walls.Kind(c).Admits(cell.Land) {
			cells = append(cells, c)
		}
	})
	rng := rand.New(rand.NewPCG(21, 8))
	unreached := 0
	for range 100 {
		if !agree(t, cells[rng.IntN(len(cells))], cells[rng.IntN(len(cells))]) {
			unreached++
		}
	}
	if unreached == 0 {
		t.Error("every pair of the walled board reached; want some walled in")
	}
}

// a target in a room walled all round but for a corner, which the rings round it step across
// slantwise and a route cannot
func TestHierarchy_NearestFreeSearchesOnceAfterItFails(t *testing.T) {
	g := grid.DefaultGrids{}.Square(64, 64, 16)
	terrain := openTerrain()
	g.EachCell(func(c cell.ID) {
		x, y, _ := g.Coords(c)
		if (x == 29 || x == 35) && y >= 29 && y <= 35 || (y == 29 || y == 35) && x >= 29 &&
			x <= 35 {
			terrain.Set(c, wall)
		}
	})
	gap := g.At(29, 29)
	terrain.Set(gap, cell.Kind{Pace: 1, Allows: cell.Land})
	f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	h := builtWhole(t, f, land, 16)
	from, target := g.At(5, 5), g.At(32, 32)
	var dst [MaxPathLength]cell.ID
	before := f.Searches()
	plain, n, ok := f.NearestFree(1, cell.Land, Body{}, from, target, Either, nil, dst[:])
	searched := f.Searches() - before
	want := slices.Clone(dst[:n])
	before = f.Searches()
	through, n, ok2 := f.NearestFreeThrough(h, 1, cell.Land, Body{}, from, target, Either, nil,
		dst[:])
	if !ok || !ok2 || plain != gap || through != gap || !slices.Equal(dst[:n], want) {
		t.Fatalf("nearest free: %v %v and through the hierarchy %v %v %v; want %v by %v", plain,
			ok, through, ok2, dst[:n], gap, want)
	}
	if climbed := f.Searches() - before; climbed > 2 || searched < 20 {
		t.Errorf("%d searches through the hierarchy, %d without; want the one failing and the "+
			"one finding the gap, and one a room's cell without", climbed, searched)
	}
}

var water = cell.Kind{Pace: 1, Allows: cell.Water}

// moated is a board 16 cells by 8 of land, a river one cell wide down column x and round the
// island of the cells past it in rows 2 to 5
func moated(x uint32) (grid.Grid, *cell.TerrainMap) {
	g := grid.DefaultGrids{}.Square(16, 8, 16)
	terrain := openTerrain()
	g.EachCell(func(c cell.ID) {
		cx, cy, _ := g.Coords(c)
		if cx == x || cx > x && (cy <= 1 || cy >= 6) {
			terrain.Set(c, water)
		}
	})
	return g, terrain
}

// the river of moated(11) with a deck 1 up over row 4 from bank to island, its underside at 0.5
type bridgedRiver struct {
	g       grid.Grid
	terrain *cell.TerrainMap
}

func (b bridgedRiver) Deck(c cell.ID) (top, under float32, carries cell.Domain, ok bool) {
	if x, y, _ := b.g.Coords(c); y == 4 && x >= 10 && x <= 12 {
		return 1, 0.5, cell.Land, true
	}
	return 0, 0, 0, false
}

func (b bridgedRiver) Ground(geom.Vec) float32 { return 0 }

func (b bridgedRiver) Over(p geom.Vec) (float32, bool) {
	if c, ok := b.g.CellAt(p); ok {
		if _, under, _, ok := b.Deck(c); ok {
			return under, true
		}
	}
	return 0, false
}

func (b bridgedRiver) KindAt(c cell.ID, deck bool) cell.Kind {
	if _, _, _, ok := b.Deck(c); ok && deck {
		return cell.Kind{Pace: 1, Allows: cell.Land}
	}
	return b.terrain.Kind(c)
}

// the clusters 4 cells a side: a step out of the river across a border, were the water a node,
// would join the banks
func TestHierarchy_AOneCellRiverOnABorderParts(t *testing.T) {
	for name, x := range map[string]uint32{"on a border": 11, "within a cluster": 10} {
		t.Run(name, func(t *testing.T) {
			g, terrain := moated(x)
			f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
			h := builtWhole(t, f, land, 4)
			from, island := g.At(1, 4), g.At(13, 3)
			if f.Reaches(h, cell.Land, Body{}, from, island) ||
				f.Reaches(h, cell.Land, Body{}, island, from) {
				t.Error("across the river reached")
			}
			if !f.Reaches(h, cell.Land, Body{}, from, g.At(9, 7)) ||
				!f.Reaches(h, cell.Land, Body{}, island, g.At(15, 2)) {
				t.Error("along a bank not reached")
			}
			var dst [64]cell.ID
			before := f.Searches()
			if c, n, ok := f.NearestFreeThrough(h, 1, cell.Land, Body{}, from, island, Either,
				nil, dst[:]); ok {
				t.Errorf("a free cell %v round the island by %v", c, dst[:n])
			}
			if searched := f.Searches() - before; searched != 1 {
				t.Errorf("%d searches round the island, want the first alone", searched)
			}
		})
	}
}

func TestHierarchy_ADeckOverWaterStillJoinsTheBanks(t *testing.T) {
	g, terrain := moated(11)
	class := Body{Step: 1, Height: 2}
	for name, tc := range map[string]struct {
		levels Levels
		joined bool
	}{
		"bridged":   {bridgedRiver{g, terrain}, true},
		"unbridged": {flat{terrain}, false},
	} {
		t.Run(name, func(t *testing.T) {
			f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(tc.levels)
			h := builtWhole(t, f, Key{Domain: cell.Land, Body: class}, 4)
			from, to := g.At(1, 4), g.At(14, 4)
			if reached := f.Reaches(h, cell.Land, class, from, to); reached != tc.joined {
				t.Errorf("across the river reached %v, want %v", reached, tc.joined)
			}
			var climbed, found [64]cell.ID
			n, ok := f.Climb(h, 1, cell.Land, class, from, to, Either, climbed[:])
			decks := f.Decks()
			m, fok := f.Find(1, cell.Land, class, from, to, Either, found[:])
			if ok != tc.joined || fok != tc.joined || !slices.Equal(climbed[:n], found[:m]) ||
				decks != f.Decks() {
				t.Fatalf("climbed %v %v on decks %b, found %v %v on %b", ok, climbed[:n],
					decks, fok, found[:m], f.Decks())
			}
			if k := slices.Index(climbed[:n], g.At(11, 4)); tc.joined && (k < 0 ||
				decks>>k&1 == 0) {
				t.Errorf("climbed %v on decks %b, want over the river on the deck",
					climbed[:n], decks)
			}
		})
	}
}

// no route ends in the water: a climb to it is refused and a reach denied off the insertion of
// the goal, as a search finds none
func TestHierarchy_AGoalInTheWaterIsRefusedOrAnswered(t *testing.T) {
	for name, x := range map[string]uint32{"on a border": 11, "within a cluster": 10} {
		t.Run(name, func(t *testing.T) {
			g, terrain := moated(x)
			f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
			h := builtWhole(t, f, land, 4)
			from := g.At(1, 4)
			var dst [64]cell.ID
			for _, wet := range []cell.ID{g.At(x, 4), g.At(x, 0), g.At(14, 7)} {
				_, found := f.Find(1, cell.Land, Body{}, from, wet, Either, dst[:])
				_, climbed := f.Climb(h, 1, cell.Land, Body{}, from, wet, Either, dst[:])
				reached := f.Reaches(h, cell.Land, Body{}, from, wet)
				if found || climbed || reached {
					t.Errorf("into the water at %v: found %v, climbed %v, reached %v", wet,
						found, climbed, reached)
				}
			}
		})
	}
}

// a unit standing in the river on a border of its cluster, where its class stands on no node:
// hpastar sets a start into its own cluster alone, blind to its step across, so a search answers
func TestHierarchy_AStartInTheWaterIsSearched(t *testing.T) {
	g, terrain := moated(11)
	f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	h := builtWhole(t, f, land, 4)
	wet, bank, island := g.At(11, 3), g.At(1, 3), g.At(15, 3)
	var dst [64]cell.ID
	for _, to := range []cell.ID{bank, island} {
		if _, ok := f.Climb(h, 1, cell.Land, Body{}, wet, to, Either, dst[:]); ok {
			t.Errorf("to %v: climbed out of the water", to)
		}
		if !f.Reaches(h, cell.Land, Body{}, wet, to) {
			t.Errorf("to %v: not reached out of the water", to)
		}
		if n, ok := f.Find(1, cell.Land, Body{}, wet, to, Either, dst[:]); !ok ||
			!takenBy(f, Body{}, wet, to, dst[:n]) {
			t.Errorf("to %v: found %v %v", to, ok, dst[:n])
		}
	}
}

// h's graph holds the levels of cells its class stands on and no other: the steps out of one
// lead to another, and the steps into one seen from it are those out of the others, to the bit
func TestHierarchy_ItsGraphHoldsTheNodesItsClassStandsOn(t *testing.T) {
	v := newGorge()
	rg, river := moated(11)
	sq := grid.DefaultGrids{}.Square(24, 24, 10)
	roads, _ := scattered(sq, 3, 5)
	for name, tc := range map[string]struct {
		f    *Finder
		body Body
	}{
		"a gorge under a deck": {New(v.g, v.terrain, nil, &cell.MultipleOccupancy{}).
			WithLevels(v), Body{Step: 1, Height: 2}},
		"a river bridged": {New(rg, river, nil, &cell.MultipleOccupancy{}).
			WithLevels(bridgedRiver{rg, river}), Body{Step: 1, Height: 2}},
		"rocks among roads over hills": {New(sq, roads, hillsOver(sq),
			&cell.MultipleOccupancy{}).WithLevels(flat{roads}), Body{}},
	} {
		t.Run(name, func(t *testing.T) {
			f := tc.f
			h := builtWhole(t, f, Key{Domain: cell.Land, Body: tc.body}, 4)
			f.building(h)
			into := backward{f, h}.steps
			forward, backward := map[edge]int{}, map[edge]int{}
			var buf []graph.Transition[node]
			held, outside := 0, 0
			for _, a := range nodesOf(f) {
				if stands := f.stepAt(f.at(a.c), a.level).admits; stands != h.stands(a) {
					t.Fatalf("%v held %v, its class standing on it %v", a, h.stands(a), stands)
				}
				if !h.stands(a) {
					outside++
					continue
				}
				held++
				for _, s := range f.leveledFrom(a, a, buf) {
					if !h.stands(s.To) {
						t.Fatalf("the step %v->%v leads out of the graph", a, s.To)
					}
					forward[edge{a, s.To, math.Float64bits(s.Cost)}]++
				}
				for _, s := range into(a, a, buf) {
					backward[edge{s.To, a, math.Float64bits(s.Cost)}]++
				}
			}
			if outside == 0 || held == 0 || !maps.Equal(forward, backward) {
				t.Errorf("%d nodes held, %d not; %d steps forward, %d backward", held, outside,
					len(forward), len(backward))
			}
		})
	}
}
