package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kjkrol/gram/plugins/board/ground"
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
  deep: 200
  grade: 3 # cheap enough for a bridge's flat heads to beat the way round the valley
  bridge: 0.5
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
	c, err := build(r, height, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.roads.laid) == 0 {
		t.Fatalf("no road laid; skipped %v", c.roads.skipped)
	}
	for _, cl := range c.roads.net.Cells() {
		units := c.outline(cl)
		if !ground.RoadFits(c.grid, cl, units, c.roads.net.Links(cl)) {
			x, y := c.xy(cl)
			t.Errorf("the road at [%d, %d] lies on ground no road may: %v", x, y, units)
		}
	}
	bridges := 0
	for _, s := range c.roads.live() {
		if s.ford {
			continue
		}
		bridges++
		if len(s.heads) != 2 {
			t.Errorf("a bridge with %d heads, want 2", len(s.heads))
		}
		for _, h := range s.heads {
			for _, k := range c.cornersOf(h.at) {
				if c.corners[k] != s.deck {
					t.Errorf("a bridge's head stands at %v, its deck at %v", c.corners[k], s.deck)
				}
			}
		}
		if len(s.cells) > 3 {
			t.Errorf("a bridge spans %d cells, 3 at most", len(s.cells))
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
		if !wet && s.deck-low < c.roads.underpass() {
			t.Errorf("a bridge over dry ground %v under its deck, want %v", s.deck-low,
				c.roads.underpass())
		}
	}
	if bridges == 0 {
		t.Error("no bridge carries the road over the valley's water")
	}
	if err := c.check(c.text("isle.yaml", "")); err != nil {
		t.Error(err)
	}
}

func TestBuild_AFlatBridgeIsACellLong(t *testing.T) {
	r, height := isle(t)
	c, err := build(r, height, true)
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
	plain, err := build(r, height, true)
	if err != nil {
		t.Fatal(err)
	}
	r.Area.Sea = 12
	c, err := build(r, height, true)
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
