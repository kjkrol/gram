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
	"strings"
	"testing"

	"github.com/kjkrol/gram/plugins/board/ground"
)

// isle is a recipe of an island 18 km by 8 km, high along its north coast, a valley cut through it
// north to south, a road from one end to the other across it, and its heights.
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
  bridge: 0.5
  high: 200
  span: 6
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
		h := (400-50*y/1000)*(1-d*d) - 250*math.Exp(-x*x/(2*900*900))
		return max(h, -20), nil
	}
	return r, height
}

// The roads of a map in relief lie where Transport Tycoon would lay them, its bridges each run at
// one deck from end to end, and a board lays the map.
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
		var units [4]float64
		for i, k := range c.cornersOf(cl) {
			units[i] = c.corners[k] / c.perUnit
		}
		if !ground.RoadFits(units, c.roads.net.Links(cl), c.size) {
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
		for _, e := range s.edges {
			for _, k := range e {
				if c.corners[k] != s.deck {
					t.Errorf("a bridge's end stands at %v, its deck at %v", c.corners[k], s.deck)
				}
			}
		}
	}
	if bridges == 0 {
		t.Error("no bridge carries the road over the valley's water")
	}
	if err := c.check(c.text("isle.yaml", "")); err != nil {
		t.Error(err)
	}
}

// With -flat a bridge is the one cell of running water it crosses.
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
	if err := c.check(data); err != nil {
		t.Error(err)
	}
}

// The units of a map made before stay as they were, up to the next section.
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

// The tiles are read as the terrarium encodes heights, fetched once into the cache.
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
