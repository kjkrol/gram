package main

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"maps"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/network"
	"github.com/kjkrol/gram/plugins/board/water"
	roadsite "github.com/kjkrol/gram/plugins/roads/site"
)

// isle is an island 18 km by 8 km, high along its north coast, a valley cut through it north to
// south.
func isle(t *testing.T) (recipe, elevation) {
	t.Helper()
	r, err := parseRecipe([]byte(`
area: {centre: [10, 40], cells: [48, 24], cell: 500, units: 32}
ground:
  rock: {slope: 0.9}
  sand: {within: 1, below: 15, share: 0.5}
rivers:
  brook: 2
  stream: 6
  river: 15
  ford: {every: 3, slope: 0.2}
roads:
  cut: 200
  fill: 200
  # the way round the valley by its dry divide climbs for nothing within 200 m of cut and fill:
  # a bridge beats it a tenth dearer than the ground it spans, its heads' earth free
  cubic: 0
  bridge: 0.1
  high: 200
  longest: 3
  places:
    west: [9.94, 39.982]
    east: [10.06, 39.982]
  routes:
    - [west, east]
`), "isle.yaml")
	if err != nil {
		t.Fatal(err)
	}
	height := func(lon, lat float64) (float64, error) {
		x, y := metresOf(r.Area.Centre, lon, lat)
		d := math.Hypot(x/9000, y/4000)
		if d >= 1 {
			return -20, nil
		}
		h := (400-50*y/1000)*(1-d*d) - 250*math.Exp(-x*x/(2*900*900))
		return max(h, -20), nil
	}
	return r, height
}

func TestBuild_RoadsLieAsTransportTycoonLaysThem(t *testing.T) {
	r, height := isle(t)
	c, err := build(r, height, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.roads.laid) == 0 {
		t.Fatalf("no road laid; left out %v", c.roads.skipped)
	}
	for _, cl := range c.roads.net.Cells() {
		units := c.outline(cl)
		if !ground.RoadFits(c.grid, cl, units, c.roads.net.Links(cl)) {
			x, y := c.xy(cl)
			t.Errorf("the road at [%d, %d] lies on ground no road may: %v", x, y, units)
		}
	}
	overWater := 0
	for _, s := range c.roads.live() {
		if s.ford {
			continue
		}
		if len(s.heads) != 2 {
			t.Errorf("a bridge with %d heads, want 2", len(s.heads))
		}
		for _, h := range s.heads {
			for _, k := range c.cornersOf(h) {
				if c.corners[k] != s.deck {
					t.Errorf("a bridge's head stands at %v, its deck at %v", c.corners[k], s.deck)
				}
			}
		}
		if len(s.cells) > r.Roads.Longest {
			t.Errorf("a bridge spans %d cells, %d at most", len(s.cells), r.Roads.Longest)
		}
		low, wet := math.Inf(1), false
		for _, cl := range s.cells {
			wet = wet || c.wet(cl)
			for _, k := range c.cornersOf(cl) {
				low = min(low, c.corners[k])
				if c.corners[k] > s.deck {
					t.Errorf("the ground under a bridge stands at %v, over its deck at %v",
						c.corners[k], s.deck)
				}
			}
			for i := range c.grid.Directions() {
				n, ok := c.grid.Toward(cl, i)
				if o := c.roads.spanOf[n]; ok && o != nil && o != s && !o.ford &&
					!slices.Contains(s.cells, n) && slices.Contains(o.cells, n) {
					t.Errorf("a bridge's span runs beside another's")
				}
			}
		}
		if wet {
			overWater++
		} else if s.deck-low < c.roads.underpass() {
			t.Errorf("a bridge over dry ground %v under its deck, want %v: neither water nor a "+
				"chasm crossed", s.deck-low, c.roads.underpass())
		}
	}
	if overWater == 0 {
		t.Error("no bridge carries the road over the valley's water")
	}
	if err := c.check(c.text("isle.yaml", "")); err != nil {
		t.Error(err)
	}
}

func TestBuild_EveryLegRunsFromPlaceToPlace(t *testing.T) {
	r, height := isle(t)
	c, err := build(r, height, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	legs := 0
	for _, route := range r.Roads.Routes {
		legs += len(route) - 1
	}
	if got := len(c.roads.laid) + len(c.roads.skipped); got != legs {
		t.Errorf("%d legs laid or left out, want the recipe's %d", got, legs)
	}
	if len(c.roads.laid) == 0 {
		t.Fatalf("no leg laid; left out %v", c.roads.skipped)
	}
	places := slices.Collect(maps.Values(c.roads.places))
	for _, rd := range c.roads.laid {
		ends := [2]int{0, len(rd.cells) - 1}
		for _, i := range ends {
			if x, y := c.xy(rd.cells[i]); !slices.Contains(places, rd.cells[i]) || rd.over[i] {
				t.Errorf("a leg ends at [%d, %d], over %v, no place", x, y, rd.over[i])
			}
		}
		for i := 1; i < len(rd.cells); i++ {
			if _, ok := grid.Way(c.grid, rd.cells[i-1], rd.cells[i]); !ok {
				x, y := c.xy(rd.cells[i])
				t.Errorf("a leg breaks off before [%d, %d]", x, y)
			}
		}
	}
}

func TestBuild_AFlatBridgeIsACellLong(t *testing.T) {
	r, height := isle(t)
	r.Flat = flatCfg{Longest: 1}
	r.Roads.Fords = false
	c, err := build(r.asFlat(), height, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	spans := c.roads.live()
	if len(spans) == 0 {
		t.Fatal("no crossing on the flat map")
	}
	for _, s := range spans {
		if len(s.cells) != 1 || !c.wet(s.cells[0]) {
			t.Errorf("a crossing of %d cells, wet %v, want one cell of water", len(s.cells),
				c.wet(s.cells[0]))
		}
	}
	data := c.text("isle.yaml", "")
	if bytes.Contains(data, []byte("relief:")) {
		t.Error("a flat map has a relief")
	}
	if bridges := bytes.Count(data, []byte("from: [")); bridges == 0 {
		t.Error("a flat map's bridges are not written from head to head")
	}
	if err := c.check(data); err != nil {
		t.Error(err)
	}
}

func TestUnitsOf_KeepsTheMapsUnits(t *testing.T) {
	old := "board:\n  default: water\nunits:\n  - {kind: walker, at: [1, 2]}\n\n" +
		"  # the rival's\n  - {kind: rival, at: [3, 4]}\nnext: {}\n"
	want := "units:\n  - {kind: walker, at: [1, 2]}\n\n  # the rival's\n" +
		"  - {kind: rival, at: [3, 4]}\n"
	if got := unitsOf([]byte(old)); got != want {
		t.Errorf("units: %q, want %q", got, want)
	}
	if got := unitsOf([]byte("board: {}\n")); got != "" {
		t.Errorf("a map without units has units %q", got)
	}
}

func TestTerrarium_ReadsTheTilesHeights(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, tilePx, tilePx))
	for y := range tilePx {
		for x := range tilePx {
			h := 1000 + 2*x // metres, rising eastwards
			v := h + 32768
			img.Set(x, y, color.NRGBA{R: uint8(v >> 8), G: uint8(v), A: 255})
		}
	}
	var tile bytes.Buffer
	if err := png.Encode(&tile, img); err != nil {
		t.Fatal(err)
	}
	fetched := 0
	tr := newTerrarium(0, t.TempDir())
	tr.fetch = func(url string) (io.ReadCloser, error) {
		fetched++
		if !strings.HasSuffix(url, "/terrarium/0/0/0.png") {
			t.Errorf("fetched %s, want the one tile of zoom 0", url)
		}
		return io.NopCloser(bytes.NewReader(tile.Bytes())), nil
	}
	for _, lon := range []float64{-90, 0, 90} {
		h, err := tr.at(lon, 0)
		if err != nil {
			t.Fatal(err)
		}
		px := (lon+180)/360*float64(tilePx) - 0.5
		if want := 1000 + 2*px; math.Abs(h-want) > 1e-6 {
			t.Errorf("at %v° east: %v m, want %v", lon, h, want)
		}
	}
	if fetched != 1 {
		t.Errorf("fetched %d tiles, want the one, once", fetched)
	}
	if _, err := os.Stat(filepath.Join(tr.cache, "0", "0", "0.png")); err != nil {
		t.Errorf("the tile is not kept: %v", err)
	}
}

func TestBuild_TheAreaGrowsForSeaRoundTheLand(t *testing.T) {
	r, height := isle(t)
	plain, err := build(r, height, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	r.Area.Sea = 12
	c, err := build(r, height, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if c.origin == [2]int{} || c.cols <= plain.cols || c.rows <= plain.rows {
		t.Fatalf("grown from %d x %d to %d x %d, from %v", plain.cols, plain.rows, c.cols, c.rows,
			c.origin)
	}
	x0, y0, x1, y1 := c.cols, c.rows, -1, -1
	for y := range c.rows {
		for x := range c.cols {
			if c.land[y*c.cols+x] {
				x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
			}
		}
	}
	if x0 < 12 || y0 < 12 || c.cols-1-x1 < 12 || c.rows-1-y1 < 12 {
		t.Errorf("land from [%d, %d] to [%d, %d] on %d x %d, want 12 cells of sea round it",
			x0, y0, x1, y1, c.cols, c.rows)
	}
	for y := range plain.rows {
		for x := range plain.cols {
			if plain.land[y*plain.cols+x] != c.land[(y+c.origin[1])*c.cols+x+c.origin[0]] {
				t.Fatalf("the land at [%d, %d] moved as the area grew", x, y)
			}
		}
	}
	if x, y := originOf(c.text("isle.yaml", "")); x != c.origin[0] || y != c.origin[1] {
		t.Errorf("the map says its area begins at [%d, %d], want %v", x, y, c.origin)
	}
}

func TestShiftUnits_MovesEveryCell(t *testing.T) {
	units := "units:\n  - {kind: walker, start: [1, 2], target: [10, 20]} # to [3, 4]\n"
	want := "units:\n  - {kind: walker, start: [4, 7], target: [13, 25]} # to [6, 9]\n"
	if got := shiftUnits(units, 3, 5); got != want {
		t.Errorf("shifted: %q, want %q", got, want)
	}
}

func TestRecipe_AFlatMapTakesItsOwnValues(t *testing.T) {
	r, _ := isle(t)
	r.Flat = flatCfg{Units: 64, Road: 30, Deck: 34}
	f := r.asFlat()
	if f.Area.Units != 64 || f.Roads.Width != 30 || f.Roads.Deck != 34 ||
		f.Rivers.Width != 2*r.Rivers.Width {
		t.Errorf("flat: units %d, road %v, deck %v, rivers %v; want 64, 30, 34 and twice %v",
			f.Area.Units, f.Roads.Width, f.Roads.Deck, f.Rivers.Width, r.Rivers.Width)
	}
}

func TestRecipe_TheRoadsSectionIsTheSitesRules(t *testing.T) {
	r, err := parseRecipe([]byte(`
area: {centre: [0, 0], cells: [4, 4], cell: 100, units: 25}
roads:
  cut: 30
  fill: 20
  earth: 1000000
  step: 8
  spread: 2
  slope: 8
  fords: false
  metre: 2
  cubic: 0.5
  climb: 3
  route: {cut: 12, fill: 8}
`), "rules.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if r.Roads.Step != 8 {
		t.Errorf("step %v, want 8", r.Roads.Step)
	}
	got := r.Roads.rules(4, 16)
	if want := 8 * math.Pi / 180; math.Abs(got.Slope-want) > 1e-12 {
		t.Errorf("slope %v rad, want 8° in radians, %v", got.Slope, want)
	}
	got.Slope = 0
	want := roadsite.Rules{
		Cut:       7.5,
		Fill:      5,
		Earth:     15625,
		Spread:    2,
		Crossings: true,
		Longest:   16,
		Ends:      true,
		Metre:     8,
		Cubic:     32,
		Climb:     3,
	}
	if got != want {
		t.Errorf("rules at 4 m a world unit %+v, want %+v", got, want)
	}
	route := r.Roads.routing(4, 16, 0.5)
	route.Slope = 0
	want.Cut, want.Fill = 1.5, 1
	if route != want {
		t.Errorf("a route planned with half its cut and fill under %+v, want %+v", route, want)
	}
}

func TestRecipe_WithoutARoadsSectionTheDefaultsServe(t *testing.T) {
	r, err := parseRecipe([]byte("area: {centre: [0, 0], cells: [4, 4], cell: 100, units: 25}\n"),
		"plain.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Roads, defaultRecipe().Roads) {
		t.Errorf("roads %+v, want the defaults %+v", r.Roads, defaultRecipe().Roads)
	}
	got := r.Roads.rules(4, 16)
	if got.Cut != 15 || got.Fill != 15 || !math.IsInf(float64(got.Earth), 1) || !got.Fords ||
		!got.Crossings || !got.Ends || got.Decks || got.Longest != 16 ||
		math.Abs(got.Slope-ground.RoadSlope) > 1e-12 {
		t.Errorf("the default rules %+v: 60 m cut and filled, earth without end, fords, roads "+
			"crossed, ends loose, no deck, any length, a road's slope", got)
	}
	if route := r.Roads.routing(4, 16, 0.25); route.Cut != 3.75 || route.Fill != 3.75 {
		t.Errorf("a route planned with a quarter of the levelling's cut and fill under %v and %v, "+
			"want 3.75", route.Cut, route.Fill)
	}
}

// strip is a chart cols by rows of earth, cells of 100 m and 32 units, its corners at heights[x]
// by their column, a river down each column of wet
func strip(t *testing.T, cols, rows int, heights []float64, wet ...int) *chart {
	t.Helper()
	r := defaultRecipe()
	r.Area = area{Cells: [2]int{cols, rows}, Cell: 100, Units: 32}
	r.Roads.High = 200
	w := &chart{
		r:       r,
		cols:    cols,
		rows:    rows,
		grid:    grid.DefaultGrids{}.Square(uint32(cols), uint32(rows), 32),
		size:    32,
		perUnit: 100.0 / 32,
		land:    make([]bool, cols*rows),
		corners: make([]float64, (cols+1)*(rows+1)),
		soils:   make([]string, cols*rows),
		rivers:  &water.Network{Courses: map[cell.ID]water.Course{}},
	}
	for i := range w.land {
		w.land[i], w.soils[i] = true, r.Kinds.Earth
	}
	for k := range w.corners {
		w.corners[k] = heights[k%(cols+1)]
	}
	for _, x := range wet {
		for y := range rows {
			w.rivers.Courses[w.cell(x, y)] = water.River
		}
	}
	w.roads = newRoads(w)
	return w
}

func TestDeckFor_StandsBetweenItsHeadsOverTheGround(t *testing.T) {
	w := strip(t, 9, 5, []float64{50, 50, 50, 50, -40, -40, 70, 70, 70, 70})
	a, b := w.cell(2, 2), w.cell(6, 2)
	east, _ := grid.Way(w.grid, a, w.cell(3, 2))
	if deck, ok := w.roads.deckFor(a, b, east, 3); !ok || deck != 60 {
		t.Errorf("a deck at %v, %v; want 60, between heads at 50 and 70", deck, ok)
	}
	w.roads.cfg.Cut, w.roads.cfg.Fill = 5, 5
	if deck, ok := w.roads.deckFor(a, b, east, 3); ok {
		t.Errorf("a deck at %v, its heads 20 apart and 5 to cut or fill each", deck)
	}
	w = strip(t, 9, 5, []float64{50, 50, 50, 50, 49, 49, 50, 50, 50, 50})
	// a span's top folds from a head's corner at the deck down to one at 49: its middle clears
	// the clearance under the deck only with the deck twice the clearance over 49
	if deck, ok := w.roads.deckFor(a, b, east, 3); !ok || math.Abs(deck-55.05) > 1e-9 {
		t.Errorf("a deck at %v, %v; want 55.05, twice the clearance and its margin over 49",
			deck, ok)
	}
}

func TestLeaps_BridgeTheRiverFromBankToBank(t *testing.T) {
	w := strip(t, 9, 5, []float64{50, 50, 50, 50, 20, 20, 50, 50, 50, 50}, 4)
	a := w.cell(2, 2)
	east, _ := grid.Way(w.grid, a, w.cell(3, 2))
	got := w.roads.leaps(a, east, nil)
	// four cells of 100 m at Bridge 4 a metre, the heads flat at 50 already
	want := []roadsite.Leap{{To: w.cell(6, 2), Spans: 3, Cost: 1600}}
	if !slices.Equal(got, want) {
		t.Errorf("leaps %v, want %v: from bank to bank, the banks by the water taking none", got,
			want)
	}
	w.roads.barred[w.cell(6, 2)] = true
	if got := w.roads.leaps(a, east, nil); len(got) != 0 {
		t.Errorf("leaps %v onto a cell barred", got)
	}
}

func TestUnfold_PutsTheSpansBackBetweenTheHeads(t *testing.T) {
	w := strip(t, 9, 5, []float64{50, 50, 50, 50, 20, 20, 50, 50, 50, 50}, 4)
	rd := w.roads.unfold([]cell.ID{w.cell(1, 2), w.cell(2, 2), w.cell(6, 2), w.cell(7, 2)})
	var cells []cell.ID
	for x := 1; x <= 7; x++ {
		cells = append(cells, w.cell(x, 2))
	}
	over := []bool{false, false, true, true, true, false, false}
	if !slices.Equal(rd.cells, cells) || !slices.Equal(rd.over, over) {
		t.Errorf("unfolded %v, over %v; want %v, over %v", rd.cells, rd.over, cells, over)
	}
	for i, at := range rd.at {
		if at != 50 {
			t.Errorf("the road at %d stands at %v, want 50: the ground and the deck", i, at)
		}
	}
}

// row is a straight road east along row 2 from column x0 to x1, over the columns over
func row(w *chart, x0, x1 int, over ...int) road {
	var rd road
	for x := x0; x <= x1; x++ {
		rd.cells = append(rd.cells, w.cell(x, 2))
		rd.over = append(rd.over, slices.Contains(over, x))
		rd.at = append(rd.at, 10)
	}
	return rd
}

func TestClash_NoBridgeBeginsWhereOneEnds(t *testing.T) {
	w := strip(t, 13, 5, slices.Repeat([]float64{10}, 14))
	rd := row(w, 1, 11, 3, 4, 5, 7, 8, 9)
	if !w.roads.clash(rd, rd.cells[0], rd.cells[len(rd.cells)-1]) {
		t.Fatal("two bridges head to head clash with nothing")
	}
	second := [2]cell.ID{w.cell(6, 2), w.cell(10, 2)}
	if !w.roads.banned[second] || len(w.roads.banned) != 1 || len(w.roads.barred) != 0 {
		t.Errorf("banned %v, barred %v; want the second bridge banned alone", w.roads.banned,
			w.roads.barred)
	}
	if w.roads.clash(rd, rd.cells[0], rd.cells[len(rd.cells)-1]) {
		t.Error("the clash found again, its bridge banned already")
	}
}

// path is the road through the cells at xys, over those at over
func path(w *chart, over [][2]int, xys ...[2]int) road {
	var rd road
	for _, xy := range xys {
		rd.cells = append(rd.cells, w.cell(xy[0], xy[1]))
		rd.over = append(rd.over, slices.Contains(over, xy))
		rd.at = append(rd.at, 10)
	}
	return rd
}

func TestClash_NoRoadFlanksABridgeAcrossACorner(t *testing.T) {
	w := strip(t, 12, 10, slices.Repeat([]float64{10}, 13))
	// up to the head of a bridge running south-east, from the side its span's corner touches
	rd := path(w, [][2]int{{5, 5}, {6, 6}}, [2]int{4, 8}, [2]int{4, 7}, [2]int{4, 6},
		[2]int{4, 5}, [2]int{4, 4}, [2]int{5, 5}, [2]int{6, 6}, [2]int{7, 7}, [2]int{8, 7},
		[2]int{9, 7})
	if !w.roads.clash(rd, rd.cells[0], rd.cells[len(rd.cells)-1]) ||
		!w.roads.barred[w.cell(4, 5)] || len(w.roads.barred) != 1 {
		t.Errorf("barred %v, want the cell beside the head and the span", w.roads.barred)
	}
	way, _ := grid.Way(w.grid, w.cell(4, 4), w.cell(5, 5))
	w.roads.marks[w.cell(4, 4)] = mark{paved: true, head: true, headWay: way}
	w.roads.marks[w.cell(5, 5)] = mark{bridged: true, axis: way}
	past := path(w, nil, [2]int{2, 4}, [2]int{3, 4}, [2]int{4, 3}, [2]int{5, 4}, [2]int{6, 4},
		[2]int{7, 4})
	if !w.roads.clash(past, past.cells[0], past.cells[len(past.cells)-1]) ||
		!w.roads.barred[w.cell(5, 4)] {
		t.Errorf("barred %v, want the cell beside a laid bridge's head and its span",
			w.roads.barred)
	}
}

func TestAway_MakesWhatWasRefusedDearer(t *testing.T) {
	w := strip(t, 13, 5, slices.Repeat([]float64{10}, 14))
	rd := row(w, 1, 9, 3, 4, 5)
	from, to := rd.cells[0], rd.cells[len(rd.cells)-1]
	if !w.roads.away(rd, roadsite.Refuse(w.grid, w.cell(6, 2), "lies askew"), from, to) {
		t.Fatal("a bridge's head refused, nothing kept off")
	}
	if !w.roads.banned[[2]cell.ID{w.cell(2, 2), w.cell(6, 2)}] || len(w.roads.dear) != 0 {
		t.Errorf("banned %v, dearer %v; want the bridge whose head was refused banned alone",
			w.roads.banned, w.roads.dear)
	}
	dearAround := func(x0, y0, reach int) map[cell.ID]float32 {
		want := map[cell.ID]float32{}
		for y := max(y0-reach, 0); y <= min(y0+reach, w.rows-1); y++ {
			for x := x0 - reach; x <= x0+reach; x++ {
				if c := w.cell(x, y); c != from && c != to {
					want[c] = dearer
				}
			}
		}
		return want
	}
	if !w.roads.away(rd, roadsite.Refuse(w.grid, w.cell(8, 2), "lies askew"), from, to) {
		t.Fatal("a cell of the road refused, nothing kept off")
	}
	if want := dearAround(8, 2, 1); !maps.Equal(w.roads.dear, want) {
		t.Errorf("dearer %v, want the cell refused and those round it but the leg's end, %v",
			w.roads.dear, want)
	}
	w.roads.away(rd, roadsite.Refuse(w.grid, w.cell(8, 2), "lies askew"), from, to)
	if got := w.roads.dear[w.cell(8, 2)]; got != dearer*dearer {
		t.Errorf("a cell refused twice %v times dearer, want %v", got, dearer*dearer)
	}
	clear(w.roads.dear)
	w.roads.marks[w.cell(8, 2)] = mark{paved: true}
	w.roads.away(rd, roadsite.Refuse(w.grid, w.cell(8, 2), "lies askew"), from, to)
	if want := dearAround(8, 2, 2); !maps.Equal(w.roads.dear, want) {
		t.Errorf("dearer %v, want two rings round a road standing refused, %v", w.roads.dear,
			want)
	}
	if w.roads.away(rd, errors.New("no planner"), from, to) {
		t.Error("an error naming no cell kept the road off one")
	}
}

func TestPlannerGround_LaysACellFlatWhereTheLevellingMay(t *testing.T) {
	w := strip(t, 9, 5, []float64{0, 0, 10, 30, 30, 230, 0, 0, 0, 0})
	g := plannerGround{chartGround{r: w.roads}}
	heights := g.Heights(w.cell(2, 2), nil)
	mid := 20 / w.perUnit
	for _, h := range heights {
		if h != heights[0] || math.Abs(float64(h)-mid) > 1e-5 {
			t.Errorf("a cell 20 m uneven planned at %v, want flat at its middle, %v", heights, mid)
			break
		}
	}
	heights = g.Heights(w.cell(4, 2), nil)
	if want := (chartGround{r: w.roads}).Heights(w.cell(4, 2), nil); !slices.Equal(heights, want) {
		t.Errorf("a cell 200 m uneven planned at %v, want its ground, %v: the levelling's 60 m "+
			"each way cannot lay it flat", heights, want)
	}
	w.roads.dear[w.cell(2, 2)] = 9
	if got := g.Price(w.cell(2, 2)); got != 9 {
		t.Errorf("a cell made 9 times dearer priced %v", got)
	}
}

func TestChartGround_HoldsTheWaterAndPricesTheRock(t *testing.T) {
	w := strip(t, 9, 5, slices.Repeat([]float64{10}, 10), 4)
	g := chartGround{r: w.roads}
	w.soils[2*w.cols+2] = w.r.Kinds.Rock
	if got := g.Price(w.cell(2, 2)); got != 4 {
		t.Errorf("rock at %v, want the recipe's 4", got)
	}
	if got := g.Price(w.cell(6, 2)); got != 1 {
		t.Errorf("earth at %v, want 1", got)
	}
	if !g.Fixed(w.cell(4, 2)) || g.Fixed(w.cell(6, 2)) {
		t.Error("want the water's bed fixed, the dry ground free")
	}
	if g.Takes(w.cell(3, 2)) || !g.Takes(w.cell(6, 2)) || g.Held(w.cell(6, 2)) {
		t.Error("want the river's bank taking no road, the dry ground taking one, nothing held")
	}
	ford := w.cell(4, 2)
	w.rivers.Courses[ford] = water.Ford
	if !g.Ford(ford) || g.Takes(ford) {
		t.Error("want the ford a ford, through which alone a road runs into water")
	}
	w.roads.barred[ford] = true
	if g.Ford(ford) {
		t.Error("a ford barred is still a way through")
	}
	way, _ := grid.Way(w.grid, w.cell(4, 2), w.cell(5, 2))
	w.roads.marks[w.cell(5, 2)] = mark{bridged: true, axis: way}
	along := cell.Links(1<<way | 1<<w.grid.Opposite(way))
	if links, ok := g.Decked(w.cell(5, 2)); !ok || links != along {
		t.Errorf("a bridge laid over the cell decks it %v, %v; want %v", links, ok, along)
	}
	if _, ok := (chartGround{r: w.roads, l: &layout{}}).Decked(w.cell(5, 2)); ok {
		t.Error("a levelling's road runs on a deck")
	}
}

// knoll is an island 10 km by 4 km of whole metres, rising by steps of half a cell from its
// sandy shore through earth and forest to its rocky top, with no river and no road: its map is
// the same whatever the machine rounds.
func knoll(t *testing.T) (recipe, elevation) {
	t.Helper()
	r, err := parseRecipe([]byte(`
area: {centre: [10, 40], cells: [20, 12], cell: 500}
ground:
  rock: {slope: 0.5, above: 290}
  sand: {within: 1, below: 100, share: 1}
  forest: {from: 200, to: 260, share: 1}
rivers: {brook: 1000, stream: 2000, river: 3000}
`), "knoll.yaml")
	if err != nil {
		t.Fatal(err)
	}
	height := func(lon, lat float64) (float64, error) {
		x, y := metresOf(r.Area.Centre, lon, lat)
		// half cells from the area's corner: even at a cell's corner, odd at its middle
		kx := math.Floor(2*(x/r.Area.Cell+10) + 0.5)
		ky := math.Floor(2*(y/r.Area.Cell+6) + 0.5)
		return 320 - 20*max(math.Abs(kx-20), 2*math.Abs(ky-12)), nil
	}
	return r, height
}

// knollMap is the knoll's map as gramap wrote it before fields and settlements.
const knollMap = `# Made by cmd/gramap. Heights: Terrain Tiles on AWS (Mapzen's Tilezen
# terrarium) — SRTM, EU-DEM, GMTED2010, ETOPO1 and others; see
# https://github.com/tilezen/joerd/blob/master/docs/attribution.md
# From knoll.yaml.
# 20 x 12 cells of 500 m; a world unit 15.625 m.
board:
  default: water
  legend:
    "~": water
    ".": earth
    ":": sand
    "^": rock
    "*": forest
  cells: |
    ~~~~~~~~~~~~~~~~~~~~
    ~~~~~~~~~~~~~~~~~~~~
    ~~::::::::::::::::~~
    ~~:..............:~~
    ~~:.....****.....:~~
    ~~:....**^^**....:~~
    ~~:....**^^**....:~~
    ~~:.....****.....:~~
    ~~:..............:~~
    ~~::::::::::::::::~~
    ~~~~~~~~~~~~~~~~~~~~
    ~~~~~~~~~~~~~~~~~~~~
relief: # metres
  corners: |
    0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
    0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
    0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
    0 0 0 40 80 80 80 80 80 80 80 80 80 80 80 80 80 40 0 0 0
    0 0 0 40 80 120 160 160 160 160 160 160 160 160 160 120 80 40 0 0 0
    0 0 0 40 80 120 160 200 240 240 240 240 240 200 160 120 80 40 0 0 0
    0 0 0 40 80 120 160 200 240 280 320 280 240 200 160 120 80 40 0 0 0
    0 0 0 40 80 120 160 200 240 240 240 240 240 200 160 120 80 40 0 0 0
    0 0 0 40 80 120 160 160 160 160 160 160 160 160 160 120 80 40 0 0 0
    0 0 0 40 80 80 80 80 80 80 80 80 80 80 80 80 80 40 0 0 0
    0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
    0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
    0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
ways:
`

func TestText_WithoutFieldsOrSettlementsTheMapIsAsBefore(t *testing.T) {
	r, height := knoll(t)
	c, err := build(r, height, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(c.text("knoll.yaml", "")); got != knollMap {
		t.Errorf("the map\n%s\nwant\n%s", got, knollMap)
	}
}

func TestRecipe_APlaceIsAPairOrItsPlaceAndSize(t *testing.T) {
	r, err := parseRecipe([]byte(`
area: {centre: [0, 0], cells: [4, 4], cell: 100}
settlements:
  cells: built
  sizes: {hamlet: 3, village: 8}
  default: village
roads:
  places:
    bare: [1, 2]
    hamlet: {at: [3, 4], size: hamlet}
    pass: {at: [5, 6], size: none}
    counted: {at: [7, 8], size: 5}
`), "places.yaml")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct {
		at    [2]float64
		cells int
	}{
		"bare":    {[2]float64{1, 2}, 8},
		"hamlet":  {[2]float64{3, 4}, 3},
		"pass":    {[2]float64{5, 6}, 0},
		"counted": {[2]float64{7, 8}, 5},
	}
	for name, w := range want {
		p := r.Roads.Places[name]
		n, err := r.Settlements.cells(p.Size)
		if p.At != w.at || err != nil || n != w.cells {
			t.Errorf("%s at %v on %d cells, %v; want at %v on %d", name, p.At, n, err, w.at,
				w.cells)
		}
	}
}

func TestRecipe_RefusesFieldsAndSettlementsItCannotLay(t *testing.T) {
	for _, tc := range []struct{ name, yaml string }{
		{"a field of no cells", "fields: [{kind: forest, grain: 16, least: 4}]"},
		{"a field of no kind", "fields: [{cells: forest, grain: 16, least: 4}]"},
		{"a field of no grain", "fields: [{cells: forest, kind: forest, least: 4}]"},
		{"a field of no least", "fields: [{cells: forest, kind: forest, grain: 16}]"},
		{"a field over no kind's cells", "fields: [{cells: woods, kind: forest, grain: 4, " +
			"least: 1}]"},
		{"a size of no word", "roads: {places: {a: {at: [1, 2], size: city}}}"},
		{"a default of no word", "settlements: {cells: built, default: city}"},
		{"a place of no at", "roads: {places: {a: {size: none}}}"},
		{"a place of another key", "roads: {places: {a: {at: [1, 2], people: 40}}}"},
		{"a place of three numbers", "roads: {places: {a: [1, 2, 3]}}"},
		{"settled on no land's kind", "settlements: {cells: water, sizes: {town: 4}, " +
			"default: town}\nroads: {places: {a: [1, 2]}}"},
	} {
		data := "area: {centre: [0, 0], cells: [4, 4], cell: 100}\n" + tc.yaml + "\n"
		if _, err := parseRecipe([]byte(data), "bad.yaml"); err == nil {
			t.Errorf("%s: no error", tc.name)
		}
	}
}

func TestFields_ABlockOfLeastCellsIsAFieldOverThem(t *testing.T) {
	w := strip(t, 20, 9, slices.Repeat([]float64{10}, 21))
	w.r.Fields = []fieldCfg{
		{Cells: cellKinds{w.r.Kinds.Forest}, Kind: "forest", Grain: 8, Least: 3},
		{Cells: cellKinds{w.r.Kinds.Sand}, Kind: "dunes", Grain: 4, Least: 1},
	}
	lay := func(kind string, xys ...[2]int) {
		for _, xy := range xys {
			w.soils[xy[1]*w.cols+xy[0]] = kind
		}
	}
	// two short of least in the first block, least in the second, more in the narrow last one
	// and in the short last row
	lay(w.r.Kinds.Forest, [2]int{1, 1}, [2]int{6, 2})
	lay(w.r.Kinds.Forest, [2]int{9, 1}, [2]int{12, 5}, [2]int{10, 3})
	lay(w.r.Kinds.Forest, [2]int{17, 6}, [2]int{19, 7}, [2]int{16, 2}, [2]int{18, 4})
	lay(w.r.Kinds.Forest, [2]int{0, 8}, [2]int{3, 8}, [2]int{7, 8})
	lay(w.r.Kinds.Sand, [2]int{2, 5})
	want := []field{
		{kind: "forest", from: w.cell(9, 1), to: w.cell(12, 5), cells: 3},
		{kind: "forest", from: w.cell(16, 2), to: w.cell(19, 7), cells: 4},
		{kind: "dunes", from: w.cell(2, 5), to: w.cell(2, 5), cells: 1},
		{kind: "forest", from: w.cell(0, 8), to: w.cell(7, 8), cells: 3},
	}
	if got := w.fields(); !slices.Equal(got, want) {
		t.Errorf("fields %v, want %v", got, want)
	}
	var b bytes.Buffer
	w.writeBoard(&b)
	rows := "  entities:\n" +
		"    - {kind: forest, from: [9, 1], to: [12, 5], cells: 3}\n" +
		"    - {kind: forest, from: [16, 2], to: [19, 7], cells: 4}\n" +
		"    - {kind: dunes, from: [2, 5], to: [2, 5], cells: 1}\n" +
		"    - {kind: forest, from: [0, 8], to: [7, 8], cells: 3}\n"
	if !strings.HasSuffix(b.String(), rows) {
		t.Errorf("the board\n%s\nwant its entities\n%s", b.String(), rows)
	}
}

// settled is a strip with a road along row 4 from column 1 to 11, rock either side of column
// 4, a bridge's span beside it at [6, 3] and a river down column 12
func settled(t *testing.T) *chart {
	t.Helper()
	w := strip(t, 16, 9, slices.Repeat([]float64{10}, 17), 12)
	node := network.Node{Kind: w.r.Kinds.Road, Width: 7}
	for x := 1; x <= 11; x++ {
		w.roads.net.Set(w.cell(x, 4), node)
		if x > 1 {
			w.roads.net.Link(w.cell(x-1, 4), w.cell(x, 4))
		}
	}
	w.soils[3*w.cols+4] = w.r.Kinds.Rock
	w.soils[5*w.cols+4] = w.r.Kinds.Rock
	w.roads.spanOf[w.cell(6, 3)] = &span{cells: []cell.ID{w.cell(6, 3)}}
	w.r.Settlements = settlementsCfg{Cells: w.r.Kinds.Built,
		Sizes: map[string]int{"village": 8, "hamlet": 3}}
	return w
}

func TestSettle_LaysASizeOfCellsBesideTheRoadOnDryFreeLand(t *testing.T) {
	w := settled(t)
	w.r.Roads.Places = map[string]place{
		"village": {Size: "village"},
		"pass":    {Size: "none"},
		"hamlet":  {Size: "hamlet"},
	}
	w.roads.places = map[string]cell.ID{
		"village": w.cell(5, 4),
		"pass":    w.cell(9, 4),
		"hamlet":  w.cell(14, 8),
	}
	w.settle()
	if got := w.settled["pass"]; len(got) != 0 {
		t.Errorf("a place of no size settled on %d cells", len(got))
	}
	built := map[cell.ID]bool{}
	for name, n := range map[string]int{"village": 8, "hamlet": 3} {
		cells := w.settled[name]
		if len(cells) != n {
			t.Errorf("%s settled on %d cells, want %d", name, len(cells), n)
		}
		for _, c := range cells {
			x, y := w.xy(c)
			_, road := w.roads.net.Node(c)
			_, bridged := w.roads.spanOf[c]
			soil := w.soils[y*w.cols+x]
			if soil != w.r.Kinds.Built || w.wet(c) || road || bridged || built[c] {
				t.Errorf("%s settled on [%d, %d]: %s, wet %v, a road %v, a bridge %v, twice %v",
					name, x, y, soil, w.wet(c), road, bridged, built[c])
			}
			// the village's road has free cells beside it to spare
			if name == "village" && !w.byRoad(c) {
				t.Errorf("the village settled on [%d, %d], off its road", x, y)
			}
			built[c] = true
		}
	}
	for _, xy := range [][2]int{{4, 3}, {4, 5}} {
		if soil := w.soils[xy[1]*w.cols+xy[0]]; soil != w.r.Kinds.Rock {
			t.Errorf("the rock at %v settled: %s", xy, soil)
		}
	}
}

func TestSettle_APlaceWithoutACellIsLeftUnsettled(t *testing.T) {
	w := settled(t)
	w.r.Roads.Places = map[string]place{"lost": {Size: "village"}}
	w.settle()
	if len(w.settled["lost"]) != 0 || !slices.Equal(w.unsettled, []string{"lost"}) {
		t.Errorf("settled %v, unsettled %v; want lost unsettled", w.settled, w.unsettled)
	}
}

func TestCheck_TakesAMapWithFieldsAndSettlements(t *testing.T) {
	r, height := knoll(t)
	r.Fields = []fieldCfg{
		{Cells: cellKinds{r.Kinds.Forest}, Kind: "forest", Grain: 4, Least: 2},
		{Cells: cellKinds{r.Kinds.Built}, Kind: "settlement", Grain: 4, Least: 1},
	}
	r.Settlements = settlementsCfg{Cells: r.Kinds.Built, Sizes: map[string]int{"town": 6}}
	r.Roads.Places = map[string]place{"town": {At: lonLatAt(r, 5, 3), Size: "town"}}
	c, err := build(r, height, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(c.settled["town"]); got != 6 {
		t.Errorf("the town settled on %d cells, want 6", got)
	}
	data := c.text("knoll.yaml", "")
	for _, want := range []string{
		`"@": built`,
		"{kind: forest, from: ",
		"{kind: settlement, from: ",
	} {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("the map has no %s", want)
		}
	}
	if err := c.check(data); err != nil {
		t.Error(err)
	}
}

// lonLatAt is the longitude and latitude of the middle of the cell x, y of r's area
func lonLatAt(r recipe, x, y int) [2]float64 {
	a := r.Area
	east := (float64(x) + 0.5 - float64(a.Cells[0])/2) * a.Cell
	south := (float64(y) + 0.5 - float64(a.Cells[1])/2) * a.Cell
	lon, lat := lonLatOf(a.Centre, east, south)
	return [2]float64{lon, lat}
}
