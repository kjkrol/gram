package board

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/mapfile"
	"github.com/kjkrol/gram/plugins/world"
)

// heightsOf is an image of cols by rows pixels, each the height in metres at its place, in
// millimetres over 100 m under zero
func heightsOf(t *testing.T, cols, rows int, at func(i, j int) float64) []byte {
	t.Helper()
	img := image.NewGray16(image.Rect(0, 0, cols, rows))
	for j := range rows {
		for i := range cols {
			img.SetGray16(i, j, color.Gray16{Y: uint16(math.Round((at(i, j) + 10) * 1000))})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func lineOf(id, tags string, points ...[2]float32) string {
	var at []string
	for _, p := range points {
		at = append(at, fmt.Sprintf("[%g,%g]", p[0], p[1]))
	}
	return `{"type":"Feature","id":"` + id + `","properties":{` + tags + `},"geometry":{"type":` +
		`"LineString","coordinates":[` + strings.Join(at, ",") + `]}}`
}

// through is a line of the map through the middles of cells of g, a map high metres north
// to south, a unit of the world units a metre
func through(g grid.Grid, id, tags string, high, units float32, cells ...[2]uint32) string {
	var at [][2]float32
	for _, c := range cells {
		mid := g.CellCenter(g.At(c[0], c[1]))
		at = append(at, [2]float32{mid.X / units, high - mid.Y/units})
	}
	return lineOf(id, tags, at...)
}

// landFiles is a level beside a map in files in its folder land
func landFiles(level, manifest string, heights []byte, features ...string) fstest.MapFS {
	files := fstest.MapFS{
		"level.yaml":      {Data: []byte(level)},
		"land/level.yaml": {Data: []byte(manifest)},
		"land/features.geojson": {Data: []byte(
			`{"type":"FeatureCollection","features":[` + strings.Join(features, ",") + "]}")},
	}
	if heights != nil {
		files["land/heights.png"] = &fstest.MapFile{Data: heights}
	}
	return files
}

const heightsKey = "heights: {file: heights.png, pixel: %g, lowest: -10, step: 0.001}\n"

func TestLay_AReliefFromAnImageIsTheReliefInNumbers(t *testing.T) {
	// 2 m a unit too: a cell of 10 units is 20 m
	for _, metres := range []float32{0, 2} {
		unit := float32(1)
		if metres > 0 {
			unit = metres
		}
		g := grid.DefaultGrids{}.Square(4, 2, 10)
		high := func(i, j int) float64 { return float64(i)*1.5 + float64(j*j)*0.75 }
		var rows []string
		for j := range 3 {
			var row []string
			for i := range 5 {
				row = append(row, fmt.Sprint(high(i, j)))
			}
			rows = append(rows, "    "+strings.Join(row, " "))
		}
		told, err := lay(t, newLevelBoard(t, g, metres),
			"relief:\n  corners: |\n"+strings.Join(rows, "\n")+"\n")
		if err != nil {
			t.Fatal(err)
		}
		manifest := fmt.Sprintf("size: [%g, %g]\n"+heightsKey, 40*unit, 20*unit, 10*unit)
		files := landFiles("geography: {file: land/level.yaml}\n", manifest,
			heightsOf(t, 5, 3, high))
		drawn, err := newLevelBoard(t, g, metres).Lay(files, "level.yaml")
		if err != nil {
			t.Fatal(err)
		}
		for j := range 3 {
			for i := range 5 {
				p := geom.NewVec(float32(i)*10, float32(j)*10)
				a, b := told.Heights()(p), drawn.Heights()(p)
				if d := a - b; d < -1e-3 || d > 1e-3 {
					t.Errorf("%g m a unit, the corner (%d, %d): %g in numbers, %g from the image",
						metres, i, j, a, b)
				}
			}
		}
	}
}

func TestLay_AHexReliefFromAnImageIsItsCellsMiddles(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(4, 3, 10)
	slope := func(p geom.Vec) float64 { return float64(p.X)*0.05 + float64(p.Y)*0.02 }
	var rows []string
	for y := range uint32(3) {
		var row []string
		for x := range uint32(4) {
			row = append(row, fmt.Sprintf("%.4f", slope(g.CellCenter(g.At(x, y)))))
		}
		rows = append(rows, "    "+strings.Join(row, " "))
	}
	told, err := lay(t, newLevelBoard(t, g, 0),
		"relief:\n  cells: |\n"+strings.Join(rows, "\n")+"\n")
	if err != nil {
		t.Fatal(err)
	}
	// a pixel every 5 m over 100 m by 100 m: an even slope is the same between them
	manifest := fmt.Sprintf("size: [100, 100]\n"+heightsKey, 5.0)
	heights := heightsOf(t, 21, 21, func(i, j int) float64 {
		return slope(geom.NewVec(float32(i)*5, float32(j)*5))
	})
	drawn, err := newLevelBoard(t, g, 0).Lay(
		landFiles("geography: {file: land/level.yaml}\n", manifest, heights), "level.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for y := range uint32(3) {
		for x := range uint32(4) {
			c := g.At(x, y)
			for _, p := range append(g.CellOutline(c, nil), g.CellCenter(c)) {
				a, b := told.Heights()(p), drawn.Heights()(p)
				if d := a - b; d < -2e-3 || d > 2e-3 {
					t.Errorf("the hex (%d, %d) at %v: %g by cells, %g from the image", x, y, p,
						a, b)
				}
			}
		}
	}
}

const landRoads = `
geography:
  file: land/level.yaml
  roads: {residential: road}
  rivers: {stream: {kind: brook, width: 3}, river: {kind: river, width: 6}}
`

func TestLay_WaysFromLinesAreTheWaysOfPaths(t *testing.T) {
	for name, g := range map[string]grid.Grid{
		"square": grid.DefaultGrids{}.Square(6, 6, 10),
		"hex":    grid.DefaultGrids{}.Hex(6, 6, 10),
	} {
		told, err := lay(t, newLevelBoard(t, g, 0), `
ways:
  roads:
    road:
      width: 4
      paths: [[[0, 1], [5, 1]], [[1, 1], [1, 5]]]
  rivers:
    - [{at: [3, 2], kind: river, width: 6}, [5, 5]]
    - [{at: [3, 4], kind: brook, width: 3}, [4, 4]]
`)
		if err != nil {
			t.Fatal(name, err)
		}
		// six cells of 10 a row, every other row half a cell east: 114 by 95 on the hex board
		const high = 95
		manifest := "size: [114, 95]\nfeatures: features.geojson\n" +
			"roads: {residential: {lanes: 1, lane: 4}}\n"
		files := landFiles(landRoads, manifest, nil,
			through(g, "main", `"highway":"residential"`, high, 1, [2]uint32{0, 1},
				[2]uint32{5, 1}),
			through(g, "side", `"highway":"residential"`, high, 1, [2]uint32{1, 1},
				[2]uint32{1, 5}),
			through(g, "water", `"waterway":"river"`, high, 1, [2]uint32{3, 2}, [2]uint32{5, 5}),
			through(g, "beck", `"waterway":"stream"`, high, 1, [2]uint32{3, 4}, [2]uint32{4, 4}),
			lineOf("fence", `"barrier":"fence"`, [2]float32{0, 0}, [2]float32{50, 50}),
		)
		drawn, err := newLevelBoard(t, g, 0).Lay(files, "level.yaml")
		if err != nil {
			t.Fatal(name, err)
		}
		if len(told.laid.Ways) == 0 || !reflect.DeepEqual(told.laid.Ways, drawn.laid.Ways) {
			t.Errorf("%s: the ways from lines are\n%+v\nfrom paths\n%+v", name, drawn.laid.Ways,
				told.laid.Ways)
		}
	}
}

func TestLay_AMapsLinesLieInTheWorldsUnits(t *testing.T) {
	// 2 m a unit: the cell (5, 1) of 10 units is 100 m to 120 m east
	g := grid.DefaultGrids{}.Square(6, 6, 10)
	manifest := "size: [160, 160]\nfeatures: features.geojson\n" +
		"roads: {residential: {lanes: 2, lane: 4}}\n"
	files := landFiles(landRoads, manifest, nil,
		through(g, "main", `"highway":"residential"`, 160, 0.5, [2]uint32{0, 1}, [2]uint32{5, 1}))
	drawn, err := newLevelBoard(t, g, 2).Lay(files, "level.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(drawn.laid.Ways) != 6 {
		t.Fatalf("%d cells of road, want the six of the row", len(drawn.laid.Ways))
	}
	for _, w := range drawn.laid.Ways {
		if _, y, _ := g.Coords(w.Cell); y != 1 || w.Width != 4 {
			t.Errorf("a road %g wide in the row %d, want 8 m as 4 units in the row 1", w.Width, y)
		}
	}
}

func kindsLaid(l *Level) map[cell.ID]string {
	out := map[cell.ID]string{}
	for _, e := range l.laid.Cells {
		out[e.Cell] = e.Kind
	}
	return out
}

func TestLay_AnAreaLaysTheCellsWhoseMiddlesItHolds(t *testing.T) {
	g := grid.DefaultGrids{}.Square(6, 4, 10)
	told, err := lay(t, newLevelBoard(t, g, 0), `
board:
  default: grass
  legend: {".": grass, "~": water, "#": wall}
  cells: |
    ......
    .~~~~.
    .~.#~.
    .~~~~.
`)
	if err != nil {
		t.Fatal(err)
	}
	// a lake over the cells 1 to 4 across and 1 to 3 down, an island on (2, 2) and (3, 2);
	// the wall drawn on the island, grass drawn over the lake's corner
	lake := `{"type":"Feature","id":"lake","properties":{"natural":"water"},"geometry":{"type":` +
		`"Polygon","coordinates":[[[8,2],[52,2],[52,32],[8,32],[8,2]],` +
		`[[21,11],[39,11],[39,19],[21,19],[21,11]]]}}`
	park := `{"type":"Feature","id":"park","properties":{"leisure":"park"},"geometry":{"type":` +
		`"Polygon","coordinates":[[[0,0],[60,0],[60,40],[0,40],[0,0]]]}}`
	level := `
board:
  default: grass
  legend: {".": grass, "#": wall, "?": water}
geography:
  file: land/level.yaml
  areas: {natural=water: water}
`
	manifest := "size: [60, 40]\nfeatures: features.geojson\n"
	drawn, err := newLevelBoard(t, g, 0).Lay(landFiles(level, manifest, nil, park, lake),
		"level.yaml")
	if err != nil {
		t.Fatal(err)
	}
	want := kindsLaid(told)
	delete(want, g.At(3, 2))
	if got := kindsLaid(drawn); !reflect.DeepEqual(got, want) {
		t.Errorf("the area lays %v, the drawing %v", got, want)
	}
	// and the drawing lies over the area
	over := strings.Replace(level, `"?": water}`, `"?": water}
  cells: |
    ......
    ??.??.
    ???#??
    ??????`, 1)
	drawn, err = newLevelBoard(t, g, 0).Lay(landFiles(over, manifest, nil, park, lake),
		"level.yaml")
	if err != nil {
		t.Fatal(err)
	}
	got := kindsLaid(drawn)
	// grass drawn on the lake at (2, 1), the wall on its island, water beside it
	if got[g.At(3, 2)] != "wall" || got[g.At(0, 1)] != "water" || got[g.At(2, 1)] != "" ||
		got[g.At(2, 2)] != "water" {
		t.Errorf("under the drawing the cells are %v", got)
	}
}

func TestLay_AMapInTilesLaysTheSameBoard(t *testing.T) {
	g := grid.DefaultGrids{}.Square(6, 6, 10)
	slope := func(i, j int) float64 { return float64(i)*0.5 + float64(j)*0.25 }
	manifest := fmt.Sprintf("size: [64, 64]\nfeatures: features.geojson\n"+
		"roads: {residential: {lanes: 1, lane: 4}}\n"+heightsKey, 4.0)
	level := "geography:\n  file: land/level.yaml\n  roads: {residential: bridge}\n" +
		"  areas: {natural=water: water}\n"
	whole := landFiles(level, manifest, heightsOf(t, 17, 17, slope),
		through(g, "main", `"highway":"residential"`, 64, 1, [2]uint32{0, 1}, [2]uint32{5, 1}),
		through(g, "side", `"highway":"residential"`, 64, 1, [2]uint32{1, 1}, [2]uint32{1, 5}),
		`{"type":"Feature","id":"lake","properties":{"natural":"water"},"geometry":{"type":`+
			`"Polygon","coordinates":[[[22,2],[58,2],[58,38],[22,38],[22,2]]]}}`,
	)
	dir := t.TempDir()
	for name, f := range whole {
		file := filepath.Join(dir, "source", name)
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, f.Data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "level.yaml"), []byte(level), 0o644); err != nil {
		t.Fatal(err)
	}
	c := mapfile.Cutting{Tile: 32, Margin: 8}
	err := mapfile.Cut(os.DirFS(filepath.Join(dir, "source", "land")), "level.yaml",
		filepath.Join(dir, "land"), c)
	if err != nil {
		t.Fatal(err)
	}
	var laid [2]*Level
	for i, fsys := range []fs.FS{whole, os.DirFS(dir)} {
		if laid[i], err = newLevelBoard(t, g, 0).Lay(fsys, "level.yaml"); err != nil {
			t.Fatal(err)
		}
	}
	if len(laid[0].laid.Ways) == 0 || !reflect.DeepEqual(laid[0].laid.Ways, laid[1].laid.Ways) {
		t.Errorf("the ways of the map in tiles are\n%+v\nwhole\n%+v", laid[1].laid.Ways,
			laid[0].laid.Ways)
	}
	if a, b := kindsLaid(laid[0]), kindsLaid(laid[1]); len(a) == 0 || !reflect.DeepEqual(a, b) {
		t.Errorf("the cells of the map in tiles are %v, whole %v", b, a)
	}
	for _, p := range []geom.Vec{geom.NewVec(0, 0), geom.NewVec(31, 33), geom.NewVec(60, 60)} {
		if a, b := laid[0].Heights()(p), laid[1].Heights()(p); a != b {
			t.Errorf("the height at %v is %g in tiles, %g whole", p, b, a)
		}
	}
}

func TestLay_AWrongGeographySaysWhere(t *testing.T) {
	g := grid.DefaultGrids{}.Square(6, 6, 10)
	road := through(g, "main", `"highway":"residential"`, 80, 1, [2]uint32{0, 1}, [2]uint32{5, 1})
	manifest := "size: [80, 80]\nfeatures: features.geojson\n" +
		"roads: {residential: {lanes: 1, lane: 4}}\n"
	steep := fmt.Sprintf(heightsKey, 10.0)
	for name, c := range map[string]struct {
		level, manifest, want string
		heights               bool
	}{
		"a map smaller than the board": {
			level:    "geography: {file: land/level.yaml}\n",
			manifest: "size: [50, 80]\nfeatures: features.geojson\n",
			want:     "land/level.yaml: a map of 50 x 80 m does not cover the board's 60 x 60",
		},
		"a highway the level lays no road for": {
			level:    "geography:\n  file: land/level.yaml\n  roads: {track: road}\n",
			manifest: manifest,
			want:     "land/level.yaml: the feature main: the level's geography lays no road",
		},
		"a road of no kind of cell": {
			level:    "geography:\n  file: land/level.yaml\n  roads: {residential: lane}\n",
			manifest: manifest,
			want:     "level.yaml:3: \"lane\" is no kind of cell",
		},
		"two reliefs": {
			level:    "geography: {file: land/level.yaml}\nrelief:\n  cells: |\n    0\n",
			manifest: manifest + steep,
			heights:  true,
			want:     "the relief is the geography's heights or the level's own, not both",
		},
		"a road on ground no road may lie on": {
			level:    "geography:\n  file: land/level.yaml\n  roads: {residential: road}\n",
			manifest: manifest + steep,
			heights:  true,
			want:     "land/level.yaml: the feature main: the road at [",
		},
		"no file": {
			level:    "geography: {roads: {residential: road}}\n",
			manifest: manifest,
			want:     "the geography names no file",
		},
		"a file that is not there": {
			level:    "geography: {file: sea/level.yaml}\n",
			manifest: manifest,
			want:     "sea/level.yaml",
		},
	} {
		var heights []byte
		if c.heights {
			heights = heightsOf(t, 9, 9, func(i, j int) float64 { return float64(i*i) * 0.6 })
		}
		_, err := newLevelBoard(t, g, 0).Lay(landFiles(c.level, c.manifest, heights, road),
			"level.yaml")
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", name, err, c.want)
		}
	}
}

type spanRow struct {
	From cell.ID `yaml:"from"`
	To   cell.ID `yaml:"to"`
}

type boreRow struct {
	From cell.ID   `yaml:"from"`
	To   cell.ID   `yaml:"to"`
	Via  []cell.ID `yaml:"via"`
}

type fordRow struct {
	At  cell.ID `yaml:"at"`
	Way [2]int  `yaml:"way"`
}

func newCrossingBoard(t *testing.T, g grid.Grid) *Plugin {
	t.Helper()
	p := newLevelBoard(t, g, 0)
	kinds := p.worldPlugin.Kinds()
	spec := kind.Spec{comp.Const(world.Position{}), comp.Const(world.Velocity{})}
	kind.Define[spanRow](kinds, "bridge", spec)
	kind.Define[boreRow](kinds, "tunnel", spec)
	kind.Define[fordRow](kinds, "ford", spec)
	return p
}

// rowsOf is the level's rows of bridges, tunnels and fords, each as it reads, in order
func rowsOf(t *testing.T, l *Level) []string {
	t.Helper()
	var rows []string
	note := func(kind string, row any) { rows = append(rows, fmt.Sprintf("%s %+v", kind, row)) }
	for _, err := range []error{
		Each(l, func(k string, r spanRow, _ string) error { note(k, r); return nil }),
		Each(l, func(k string, r boreRow, _ string) error { note(k, r); return nil }),
		Each(l, func(k string, r fordRow, _ string) error { note(k, r); return nil }),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	slices.Sort(rows)
	return rows
}

const landCrossings = `
geography:
  file: land/level.yaml
  roads: {residential: road}
  rivers: {river: {kind: river, width: 6}}
  bridges: {residential: bridge}
  tunnels: {residential: tunnel}
  fords: ford
`

func TestLay_ALinesBridgeTunnelAndFordAreTheRowsAndTheLeadsOfAFile(t *testing.T) {
	g := grid.DefaultGrids{}.Square(12, 8, 10)
	told, err := lay(t, newCrossingBoard(t, g), `
board:
  entities:
    - {kind: bridge, from: [2, 1], to: [4, 1]}
    - {kind: ford, at: [3, 3], way: [1, 0]}
    - {kind: tunnel, from: [6, 1], to: [10, 3], via: [[8, 1], [8, 3]]}
    - {kind: ford, at: [3, 5], way: [-1, 0]}
ways:
  roads:
    road:
      width: 4
      paths:
        - [[0, 1], {onto: [3, 1]}]
        - [{onto: [3, 1]}, [6, 1], {onto: [7, 1]}]
        - [{onto: [9, 3]}, [10, 3], [11, 3]]
        - [[0, 3], {onto: [3, 3]}]
        - [{onto: [3, 3]}, [5, 3]]
  rivers:
    - [{at: [3, 0], kind: river, width: 6}, [3, 7]]
`)
	if err != nil {
		t.Fatal(err)
	}
	const high = 80
	on := func(id, tags string, cells ...[2]uint32) string {
		return through(g, id, `"highway":"residential"`+tags, high, 1, cells...)
	}
	manifest := "size: [120, 80]\nfeatures: features.geojson\n" +
		"roads: {residential: {lanes: 1, lane: 4}}\n"
	mid := g.CellCenter(g.At(3, 5))
	files := landFiles(landCrossings, manifest, nil,
		on("west", "", [2]uint32{0, 1}, [2]uint32{2, 1}),
		on("over", `,"bridge":"yes"`, [2]uint32{2, 1}, [2]uint32{4, 1}),
		on("on", "", [2]uint32{4, 1}, [2]uint32{6, 1}),
		on("under", `,"tunnel":"yes"`, [2]uint32{6, 1}, [2]uint32{8, 1}, [2]uint32{8, 3},
			[2]uint32{10, 3}),
		on("east", "", [2]uint32{10, 3}, [2]uint32{11, 3}),
		on("lane", "", [2]uint32{0, 3}, [2]uint32{2, 3}),
		on("wade", `,"ford":"yes"`, [2]uint32{2, 3}, [2]uint32{4, 3}),
		on("lane on", "", [2]uint32{4, 3}, [2]uint32{5, 3}),
		through(g, "water", `"waterway":"river"`, high, 1, [2]uint32{3, 0}, [2]uint32{3, 7}),
		`{"type":"Feature","id":"shallows","properties":{"ford":"yes"},"geometry":{"type":`+
			fmt.Sprintf(`"Point","coordinates":[%g,%g]}}`, mid.X, high-mid.Y),
		`{"type":"Feature","id":"wade again","properties":{"ford":"yes"},"geometry":{"type":`+
			fmt.Sprintf(`"Point","coordinates":[%g,%g]}}`, mid.X, high-mid.Y+20),
	)
	drawn, err := newCrossingBoard(t, g).Lay(files, "level.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(told.laid.Ways, drawn.laid.Ways) {
		t.Errorf("the ways from lines are\n%+v\nfrom paths\n%+v", drawn.laid.Ways,
			told.laid.Ways)
	}
	want, got := rowsOf(t, told), rowsOf(t, drawn)
	if len(want) != 4 || !slices.Equal(want, got) {
		t.Errorf("the rows from lines are\n%v\nfrom the file\n%v", got, want)
	}
}

func TestLay_ACrossingTheGeographyCannotLaySaysWhy(t *testing.T) {
	g := grid.DefaultGrids{}.Square(12, 8, 10)
	const high = 80
	manifest := "size: [120, 80]\nfeatures: features.geojson\n" +
		"roads: {residential: {lanes: 1, lane: 4}}\n"
	road := func(tags string, cells ...[2]uint32) string {
		return through(g, "way", `"highway":"residential"`+tags, high, 1, cells...)
	}
	river := through(g, "water", `"waterway":"river"`, high, 1, [2]uint32{3, 0}, [2]uint32{3, 7})
	point := func(x, y uint32) string {
		mid := g.CellCenter(g.At(x, y))
		return `{"type":"Feature","id":"way","properties":{"ford":"yes"},"geometry":{"type":` +
			fmt.Sprintf(`"Point","coordinates":[%g,%g]}}`, mid.X, high-mid.Y)
	}
	for name, c := range map[string]struct {
		level, feature, want string
	}{
		"no table": {
			strings.Replace(landCrossings, "  bridges: {residential: bridge}\n", "", 1),
			road(`,"bridge":"yes"`, [2]uint32{2, 1}, [2]uint32{4, 1}),
			"the feature way: the level's geography lays no bridge for highway=residential",
		},
		"too short": {
			landCrossings,
			road(`,"bridge":"yes"`, [2]uint32{2, 1}, [2]uint32{3, 1}),
			"the feature way: a bridge runs from the cell before it to the cell after",
		},
		"off the board": {
			landCrossings,
			lineOf("way", `"highway":"residential","tunnel":"yes"`, [2]float32{-30, 65},
				[2]float32{45, 65}),
			"the feature way: a tunnel runs from the cell before it to the cell after",
		},
		"no river": {
			landCrossings,
			point(8, 5),
			"the feature way: the ford lies on no river",
		},
		"no kind": {
			strings.Replace(landCrossings, "fords: ford", "fords: wade", 1),
			point(3, 5),
			`the geography lays a ford as "wade", no kind of entity`,
		},
	} {
		_, err := newCrossingBoard(t, g).Lay(landFiles(c.level, manifest, nil, c.feature, river),
			"level.yaml")
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: the level is refused with %v, want %q", name, err, c.want)
		}
	}
}

func TestLay_ARiversWidthIsItsLinesOwnPointByPoint(t *testing.T) {
	for name, g := range map[string]grid.Grid{
		"square": grid.DefaultGrids{}.Square(6, 8, 10),
		"hex":    grid.DefaultGrids{}.Hex(6, 8, 10),
	} {
		// 2 m a unit: the widths told in units are the line's in metres halved
		told, err := lay(t, newLevelBoard(t, g, 2), `
ways:
  rivers:
    - [{at: [3, 0], kind: river, width: 2}, {at: [3, 4], width: 6}, {at: [3, 7], width: 3}]
    - [{at: [0, 2], kind: brook, width: 4}, [2, 2]]
`)
		if err != nil {
			t.Fatal(name, err)
		}
		const high = 260
		files := landFiles(`
geography:
  file: land/level.yaml
  rivers: {river: river, stream: {kind: brook, width: 9}}
`, "size: [260, 260]\nfeatures: features.geojson\n", nil,
			through(g, "water", `"waterway":"river","width":"4; 12;6"`, high, 0.5,
				[2]uint32{3, 0}, [2]uint32{3, 4}, [2]uint32{3, 7}),
			through(g, "beck", `"waterway":"stream","width":8`, high, 0.5,
				[2]uint32{0, 2}, [2]uint32{2, 2}),
		)
		drawn, err := newLevelBoard(t, g, 2).Lay(files, "level.yaml")
		if err != nil {
			t.Fatal(name, err)
		}
		if len(told.laid.Ways) == 0 || !reflect.DeepEqual(told.laid.Ways, drawn.laid.Ways) {
			t.Errorf("%s: the rivers from lines are\n%+v\nfrom paths\n%+v", name,
				drawn.laid.Ways, told.laid.Ways)
		}
	}
}

func TestLay_ARiverWithoutAWidthOrWithAWrongOneSaysWhy(t *testing.T) {
	g := grid.DefaultGrids{}.Square(6, 6, 10)
	for _, c := range []struct {
		tags, says string
	}{
		{`"waterway":"river"`, "needs a width"},
		{`"waterway":"river","width":"4;5"`, "2 widths for its 3 points"},
		{`"waterway":"river","width":"4;wide;5"`, `the feature water: the width "wide"`},
		{`"waterway":"river","width":0`, "no number over zero"},
	} {
		files := landFiles(`
geography:
  file: land/level.yaml
  rivers: {river: river}
`, "size: [60, 60]\nfeatures: features.geojson\n", nil,
			through(g, "water", c.tags, 60, 1, [2]uint32{3, 0}, [2]uint32{3, 3},
				[2]uint32{3, 5}))
		_, err := newLevelBoard(t, g, 0).Lay(files, "level.yaml")
		if err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%s: the error is %v, want one saying %q", c.tags, err, c.says)
		}
	}
}
