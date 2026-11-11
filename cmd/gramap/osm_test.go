package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

func TestOverpass_ReadsTheRoadsGreatestFirstAndAsksOnce(t *testing.T) {
	asked := 0
	o := &overpass{cache: t.TempDir(), post: func(q string) (io.ReadCloser, error) {
		asked++
		if !strings.Contains(q, "(trunk|tertiary)") || !strings.Contains(q, "1.0000,2.0000") {
			t.Errorf("asked %q, want the classes and the box", q)
		}
		return io.NopCloser(strings.NewReader(`{"elements": [
			{"id": 3, "tags": {"highway": "tertiary_link", "bridge": "viaduct"},
			 "geometry": [{"lat": 1, "lon": 2}, {"lat": 1.5, "lon": 2.5}]},
			{"id": 2, "tags": {"highway": "trunk", "tunnel": "yes", "ref": "VR1",
			 "oneway": "yes"}, "geometry": [{"lat": 1, "lon": 2}, {"lat": 2, "lon": 3}]},
			{"id": 1, "tags": {"highway": "footway"},
			 "geometry": [{"lat": 1, "lon": 2}, {"lat": 2, "lon": 3}]}]}`)), nil
	}}
	for range 2 {
		ways, err := o.roads([4]float64{1, 2, 3, 4}, []string{"trunk", "tertiary"})
		if err != nil {
			t.Fatal(err)
		}
		if len(ways) != 2 || ways[0].id != 2 || !ways[0].tunnel ||
			ways[0].line[1] != [2]float64{3, 2} {
			t.Fatalf("read %+v, want the trunk road's tunnel first, no footway", ways)
		}
		if w := ways[1]; w.class <= ways[0].class || !w.bridge || w.tunnel {
			t.Errorf("the link %+v, want a tertiary road's, a bridge", w)
		}
	}
	if asked != 1 {
		t.Errorf("asked %d times, want the answer kept after the first", asked)
	}
}

// a chart of bare cells of 100 m, for what reads no ground
func bare(cols, rows int) *chart {
	r := defaultRecipe()
	r.Area = area{Cells: [2]int{cols, rows}, Cell: 100, Units: 32}
	return &chart{r: r, cols: cols, rows: rows, size: 32, perUnit: 100.0 / 32,
		grid: grid.DefaultGrids{}.Square(uint32(cols), uint32(rows), 32)}
}

// a way of class through the points, in cells
func (w *chart) way(id int64, class int, pts ...[2]float64) osmWay {
	way := osmWay{id: id, class: class}
	for _, p := range pts {
		lon, lat := w.lonLat(p[0], p[1])
		way.line = append(way.line, [2]float64{lon, lat})
	}
	return way
}

func TestGuide_ThinsARoadOfTwoCarriagewaysToOneLeg(t *testing.T) {
	w := bare(16, 12)
	g := w.newGuide([]osmWay{
		w.way(1, 1, [2]float64{2.5, 5.5}, [2]float64{12.5, 5.5}),
		w.way(2, 1, [2]float64{12.5, 6.5}, [2]float64{2.5, 6.5}),
	}, nil, nil)
	if len(g.legs) != 1 {
		t.Fatalf("%d legs, want the two carriageways as one", len(g.legs))
	}
	l := g.legs[0]
	if n := len(l.trace); n < 8 || n > 11 || l.class != 1 {
		t.Errorf("a leg of %d cells of class %d, want the road's ten or so, a trunk road", n,
			l.class)
	}
	for i := 1; i < len(l.trace); i++ {
		if _, beside := grid.Way(w.grid, l.trace[i-1], l.trace[i]); !beside {
			t.Fatalf("the trace leaps from %v to %v", l.trace[i-1], l.trace[i])
		}
	}
}

func TestGuide_CutsTheRoadsAtTheirJunctionsTheGreatestFirst(t *testing.T) {
	w := bare(16, 12)
	g := w.newGuide([]osmWay{
		w.way(1, 4, [2]float64{2.5, 5.5}, [2]float64{12.5, 5.5}),
		w.way(2, 1, [2]float64{7.5, 5.5}, [2]float64{7.5, 10.5}),
		// too short a spur to be a road
		w.way(3, 4, [2]float64{4.5, 5.5}, [2]float64{4.5, 4.5}),
	}, nil, nil)
	if len(g.legs) != 3 {
		t.Fatalf("%d legs, want three meeting at the junction: %+v", len(g.legs), g.legs)
	}
	if g.legs[0].class != 1 {
		t.Errorf("the first leg of class %d, want the trunk road's", g.legs[0].class)
	}
	junction := w.cell(7, 5)
	for _, l := range g.legs {
		if l.from != junction && l.to != junction {
			t.Errorf("the leg from %v to %v ends off the junction", l.from, l.to)
		}
	}
}

func TestStraighten_LeadsATunnelAlongTheGridsWays(t *testing.T) {
	var run [][2]int
	for x := 0; x <= 9; x++ {
		run = append(run, [2]int{x, x / 3})
	}
	turns := append(append([][2]int{run[0]}, straighten(run, bend)...), run[len(run)-1])
	if len(turns) < 3 {
		t.Fatalf("turns at %v, want a turn between ends lying on no way of the grid", turns)
	}
	for i := 1; i < len(turns); i++ {
		dx, dy := turns[i][0]-turns[i-1][0], turns[i][1]-turns[i-1][1]
		if dx != 0 && dy != 0 && abs(dx) != abs(dy) || dx == 0 && dy == 0 {
			t.Errorf("from %v to %v runs along no way of the grid", turns[i-1], turns[i])
		}
	}
}

func TestSquared_LeadsATunnelOutOfItsHeadsAcrossTheirEdges(t *testing.T) {
	for name, c := range map[string]struct {
		from, to [2]int
		via      [][2]int
	}{
		"slanting":             {from: [2]int{1, 1}, to: [2]int{6, 6}},
		"across one corner":    {from: [2]int{3, 3}, to: [2]int{4, 2}},
		"slanting from a turn": {from: [2]int{0, 4}, to: [2]int{9, 0}, via: [][2]int{{5, 4}}},
		"slanting to a turn":   {from: [2]int{9, 0}, to: [2]int{0, 4}, via: [][2]int{{5, 4}}},
		"along a row":          {from: [2]int{1, 3}, to: [2]int{6, 3}},
	} {
		turns := append(append([][2]int{c.from}, squared(c.from, c.to, c.via)...), c.to)
		for i := 1; i < len(turns); i++ {
			dx, dy := turns[i][0]-turns[i-1][0], turns[i][1]-turns[i-1][1]
			if dx != 0 && dy != 0 && abs(dx) != abs(dy) || dx == 0 && dy == 0 {
				t.Errorf("%s: %v: from %v to %v runs along no way of the grid", name, turns,
					turns[i-1], turns[i])
			}
			if (i == 1 || i == len(turns)-1) && dx != 0 && dy != 0 {
				t.Errorf("%s: %v: a head's stretch crosses a corner", name, turns)
			}
		}
	}
}

// ridged is an island 9 km by 3 km with a ridge 300 m high across its middle, and
// OpenStreetMap's road along it from west to east, through the ridge by a tunnel of 2.4 km
func ridged(t *testing.T) (recipe, elevation, source) {
	t.Helper()
	r, err := parseRecipe([]byte(`
area: {centre: [10, 40], cells: [100, 40], cell: 100, units: 32}
ground: {rock: {slope: 2}}
rivers: {brook: 1000, stream: 2000, river: 3000}
roads:
  source: osm
  classes: [trunk]
tunnels: {clear: 3, cover: 5, grade: 0.1, longest: 40, reshape: 60}
settlements: {source: osm, least: 3, cells: built}
`), "ridged.yaml")
	if err != nil {
		t.Fatal(err)
	}
	height := func(lon, lat float64) (float64, error) {
		x, y := metresOf(r.Area.Centre, lon, lat)
		if math.Abs(x) > 4500 || math.Abs(y) > 1500 {
			return -20, nil
		}
		return 40 + 300*math.Exp(-x*x/(2*500*500)), nil
	}
	at := func(x, y float64) [2]float64 {
		lon, lat := lonLatOf(r.Area.Centre, x, y)
		return [2]float64{lon, lat}
	}
	osm := func([4]float64, []string, bool) (*osmData, error) {
		data := &osmData{ways: []osmWay{
			{id: 1, class: 1, line: [][2]float64{at(-4000, 50), at(-1200, 50)}},
			{id: 2, class: 1, tunnel: true, line: [][2]float64{at(-1200, 50), at(1200, 50)}},
			{id: 3, class: 1, line: [][2]float64{at(1200, 50), at(4000, 50)}},
		}}
		for i := range 5 {
			data.buildings = append(data.buildings, at(-3050+float64(i), 450))
		}
		data.buildings = append(data.buildings, at(-2050, 450))
		return data, nil
	}
	return r, height, osm
}

func TestBuild_FollowsOpenStreetMapsRoadThroughItsTunnel(t *testing.T) {
	r, height, osm := ridged(t)
	c, err := build(r, height, osm, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.roads.tunnels) != 1 || len(c.roads.lost) != 0 {
		t.Fatalf("%d tunnels and %d stretches left out, want the tunnel and every leg laid",
			len(c.roads.tunnels), len(c.roads.lost))
	}
	tn := c.roads.tunnels[0]
	if n := len(tn.plan.Cells); n < 16 || n > 24 {
		t.Errorf("the tunnel runs under %d cells, want the 24 of its line or a few fewer", n)
	}
	for _, h := range [2]cell.ID{tn.from, tn.to} {
		if _, road := c.roads.net.Node(h); !road {
			t.Errorf("no road reaches the tunnel's head at %s", c.point(h))
		}
	}
	for _, under := range tn.plan.Cells {
		if _, road := c.roads.net.Node(under); road {
			t.Errorf("a road lies over the tunnel at %s: the ridge is crossed twice",
				c.point(under))
		}
	}
	data := c.text("ridged.yaml", "")
	if !strings.Contains(string(data), "["+c.point(tn.from)+", {onto: "+
		c.point(tn.plan.Cells[0])+"}]") {
		t.Error("no road runs on from the head into the tunnel")
	}
	if !strings.Contains(string(data), "kind: tunnel, from: "+c.point(tn.from)) ||
		!strings.Contains(string(data), "OpenStreetMap contributors") {
		t.Error("the map names no tunnel or not whose its roads are")
	}
	if err := c.check(data); err != nil {
		t.Errorf("a board refuses the map: %v", err)
	}
}

func TestSettle_BuildsWhereOpenStreetMapHasTheBuildings(t *testing.T) {
	r, height, osm := ridged(t)
	c, err := build(r, height, osm, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if c.built != 1 {
		t.Fatalf("%d cells built, want the one of five buildings, not the one of one", c.built)
	}
	fx, fy := c.place(lonLatOf(r.Area.Centre, -3050, 450))
	if got := c.kindAt(int(fx), int(fy)); got != "built" {
		t.Errorf("the cell of five buildings is %q, want built", got)
	}
}

// the maps of Madeira as the recipe makes them, by the SHA-256 of their text
var madeira = map[bool]string{
	false: "c8e18aa456dadb5eac695aeb4ab7356bbafcd2c3825dd0557c644115ad926f80",
	true:  "123d909c3286f484a3c7a68e10bd458cb815644f2f04273889f24f32950255ff",
}

// With GRAMAP_MADEIRA set and the tiles and OpenStreetMap's answers in the user's cache, the
// recipe's maps are made whole (two minutes) and held to what they were: a change to gramap
// that should move no cell moves none.
func TestMadeira_IsTheMapItWas(t *testing.T) {
	if os.Getenv("GRAMAP_MADEIRA") == "" {
		t.Skip("GRAMAP_MADEIRA is not set: Madeira's maps are not made")
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		t.Skip(err)
	}
	r, err := readRecipe("madeira.yaml")
	if err != nil {
		t.Fatal(err)
	}
	tiles := newTerrarium(zoomFor(r.Area.Cell, r.Area.Centre[1]),
		filepath.Join(dir, "gramap", "terrarium"))
	over := newOverpass(filepath.Join(dir, "gramap", "osm"))
	osm := func(box [4]float64, classes []string, buildings bool) (*osmData, error) {
		ways, err := over.roads(box, classes)
		if err != nil {
			return nil, err
		}
		data := &osmData{ways: ways}
		if buildings {
			data.buildings, err = over.buildings(box)
		}
		return data, err
	}
	land := newLandcover(filepath.Join(dir, "gramap", "cover"))
	for flat, want := range madeira {
		rc := r
		if flat {
			rc = r.asFlat()
		}
		c, err := build(rc, tiles.at, osm, land, flat)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(c.text("cmd/gramap/madeira.yaml", ""))
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("flat %v: the map is %s, was %s", flat, got, want)
		}
	}
}
