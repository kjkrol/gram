package board

import (
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/world"
	"go.yaml.in/yaml/v3"
)

type mark struct{}

func (mark) Command() {}

type recruit struct {
	Start   cell.ID              `yaml:"start"`
	Route   []cell.ID            `yaml:"route"`
	Talks   string               `yaml:"talks"`
	Rests   time.Duration        `yaml:"rests"`
	Muster  struct{ At cell.ID } `yaml:"muster"`
	Ignored cell.ID
}

func newLevelBoard(t *testing.T, g grid.Grid, metres float32) *Plugin {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 400, Height: 400},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 1, MaxSize: 32},
		Heights:  true,
		Scale:    world.Scale{Metres: metres},
	})
	p := NewPlugin(g, &cell.MultipleOccupancy{}, w)
	kinds := p.Read(0).CellKinds()
	kinds.Define("grass", cell.Kind{Pace: 1, Allows: cell.Land})
	kinds.Define("water", cell.Kind{Pace: 1, Allows: cell.Water})
	kinds.Define("wall", cell.Kind{Pace: 1, Solid: true})
	kinds.Define("road", cell.Kind{Pace: 1, Allows: cell.Land, Graded: true})
	kinds.Define("bridge", cell.Kind{Pace: 1, Allows: cell.Land})
	kinds.Define("river", cell.Kind{Pace: 1, Allows: cell.Water})
	kinds.Define("brook", cell.Kind{Pace: 1, Allows: cell.Land | cell.Water})
	w.Roles().Define("lake")
	kind.Define[recruit](w.Kinds(), "scout", kind.Spec{
		comp.Const(world.Position{}),
		comp.Const(world.Velocity{}),
	})
	return p
}

func lay(t *testing.T, p *Plugin, file string) (*Level, error) {
	t.Helper()
	fsys := fstest.MapFS{"level.yaml": {Data: []byte(file)}}
	return p.Lay(fsys, "level.yaml")
}

func TestLay_TheDrawingLaysTheLegendsKinds(t *testing.T) {
	g := grid.DefaultGrids{}.Square(4, 2, 10)
	p := newLevelBoard(t, g, 0)
	_, err := lay(t, p, `
board:
  default: grass
  legend:
    ".": grass
    "~": {kind: water, plays: [lake], group: lake}
    "#": {kind: wall, name: gate}
  cells: |
    .~~.
    #..~
`)
	if err != nil {
		t.Fatal(err)
	}
	laid := map[cell.ID]cell.Entry{}
	for _, e := range p.seeded.Cells {
		laid[e.Cell] = e
	}
	lake := p.worldPlugin.Roles().Named("lake")
	water := cell.Entry{Kind: "water", Group: "lake"}.Plays(lake)
	want := map[cell.ID]cell.Entry{
		g.At(1, 0): water,
		g.At(2, 0): water,
		g.At(0, 1): {Kind: "wall", Name: "gate"},
		g.At(3, 1): water,
	}
	if len(laid) != len(want) {
		t.Errorf("%d cells laid, want the %d not of the default", len(laid), len(want))
	}
	for c, w := range want {
		w.Cell = c
		if laid[c] != w {
			t.Errorf("cell %d laid as %+v, want %+v", c, laid[c], w)
		}
	}
	if err := p.Populate(); err != nil {
		t.Fatal(err)
	}
	if got := p.Read(0).Board().Kind(g.At(3, 0)).Name.String(); got != "grass" {
		t.Errorf("a cell of the default is %q, want grass", got)
	}
}

func TestLay_AHexRowIsAnR(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(3, 2, 10)
	p := newLevelBoard(t, g, 0)
	if _, err := lay(t, p, `
board:
  legend: {".": grass, "~": water}
  cells: |
    . ~ .
     ~ . .
`); err != nil {
		t.Fatal(err)
	}
	if err := p.Populate(); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		q, r uint32
		want string
	}{{1, 0, "water"}, {0, 1, "water"}, {0, 0, "grass"}, {1, 1, "grass"}} {
		if got := p.Read(0).Board().Kind(g.At(c.q, c.r)).Name.String(); got != c.want {
			t.Errorf("(q %d, r %d) is %q, want %q", c.q, c.r, got, c.want)
		}
	}
}

func TestLay_TheReliefIsInMetres(t *testing.T) {
	g := grid.DefaultGrids{}.Square(2, 1, 10)
	p := newLevelBoard(t, g, 2)
	level, err := lay(t, p, "relief:\n  cells: |\n    4 8\n")
	if err != nil {
		t.Fatal(err)
	}
	h := level.Heights()
	if got := h(geom.NewVec(5, 5)); got != 2 {
		t.Errorf("in the first cell: %v, want 4 m at 2 m a unit, 2", got)
	}
	if got := h(geom.NewVec(10, 5)); got != 3 {
		t.Errorf("where the cells meet: %v, want the mean, 3", got)
	}
	p = newLevelBoard(t, g, 0)
	level, err = lay(t, p, "relief:\n  corners: |\n    0 1 2\n    3 4 5\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := level.Heights()(geom.NewVec(10, 10)); got != 4 {
		t.Errorf("at the corner (1, 1): %v, want 4", got)
	}
}

func TestLay_WaysRunAlongTheirPaths(t *testing.T) {
	g := grid.DefaultGrids{}.Square(6, 6, 10)
	p := newLevelBoard(t, g, 0)
	if _, err := lay(t, p, `
ways:
  roads:
    road:
      width: 4
      paths: [[[0, 2], {onto: [3, 2]}], [{onto: [3, 2]}, [5, 2]], [[1, 2], [1, 5]]]
  rivers:
    - [{at: [3, 0], kind: river, width: 2}, {at: [3, 4], width: 6}, [3, 5]]
`); err != nil {
		t.Fatal(err)
	}
	if err := p.Populate(); err != nil {
		t.Fatal(err)
	}
	brd := p.Read(0).Board()
	if w := brd.Way(g.At(3, 2)); w.Kind.Name.String() != "river" {
		t.Errorf("under the bridge: a way of %q, want the river", w.Kind.Name)
	}
	toBridge, _ := grid.Link(g, g.At(2, 2), g.At(3, 2))
	fromBridge, _ := grid.Link(g, g.At(4, 2), g.At(3, 2))
	if brd.Way(g.At(2, 2)).Links&toBridge == 0 || brd.Way(g.At(4, 2)).Links&fromBridge == 0 {
		t.Error("the road does not lead onto the bridge from either side")
	}
	south, _ := grid.Link(g, g.At(1, 2), g.At(1, 3))
	east, _ := grid.Link(g, g.At(1, 2), g.At(2, 2))
	if l := brd.Way(g.At(1, 2)).Links; l&south == 0 || l&east == 0 {
		t.Errorf("where the branch leaves the road: links %08b, want south and east", l)
	}
	head, mouth := brd.Way(g.At(3, 0)).Width, brd.Way(g.At(3, 4)).Width
	if head != 2 || mouth != 6 {
		t.Errorf("the river is %v wide at its head and %v at its mouth, want 2 and 6", head, mouth)
	}
	out, _ := grid.Link(g, g.At(3, 4), g.At(3, 5))
	if l := brd.Way(g.At(3, 4)).Links; l&out == 0 || brd.Way(g.At(3, 5)).Runs() {
		t.Errorf("the mouth: links %08b, want it flowing out south into what is no part of it", l)
	}
	if w := brd.Way(g.At(0, 0)); w.Runs() {
		t.Errorf("a way runs off every path: %+v", w)
	}
}

func TestLay_ARiverTurnsAtItsPoints(t *testing.T) {
	g := grid.DefaultGrids{}.Square(6, 6, 10)
	p := newLevelBoard(t, g, 0)
	if _, err := lay(t, p, `
ways:
  rivers:
    - [{at: [0, 4], kind: brook, width: 1}, {at: [2, 4], kind: river, width: 3}, [5, 4]]
    - [{at: [3, 0], kind: brook, width: 1}, [3, 4]]
`); err != nil {
		t.Fatal(err)
	}
	if err := p.Populate(); err != nil {
		t.Fatal(err)
	}
	brd := p.Read(0).Board()
	for x, want := range []struct {
		kind  string
		width float32
	}{{"brook", 1}, {"brook", 2}, {"river", 3}, {"river", 3}, {"river", 3}} {
		w := brd.Way(g.At(uint32(x), 4))
		if w.Kind.Name.String() != want.kind || w.Width != want.width {
			t.Errorf("(%d, 4): a %s %v wide, want a %s %v wide", x, w.Kind.Name, w.Width, want.kind,
				want.width)
		}
	}
	north, _ := grid.Link(g, g.At(3, 4), g.At(3, 3))
	if l := brd.Way(g.At(3, 4)).Links; l&north == 0 {
		t.Errorf("where the tributary flows in: links %08b, want north among them", l)
	}
}

func TestSpawn_AnEntrysRowIsItsTaggedKeys(t *testing.T) {
	g := grid.DefaultGrids{}.Square(6, 6, 10)
	p := newLevelBoard(t, g, 0)
	level, err := lay(t, p, `units:
  - {kind: scout, side: red, name: ann, group: scouts, start: [1, 2], route: [[2, 2], [3, 4]],
     talks: miller, rests: 2s, muster: {at: [5, 5]}}`)
	if err != nil {
		t.Fatal(err)
	}
	e, _, err := level.l.entry(&level.waiting[0], Sides{"red": {mark{}}})
	if err != nil {
		t.Fatal(err)
	}
	r, ok := e.Row().(recruit)
	if !ok {
		t.Fatalf("the row is a %T, want a recruit", e.Row())
	}
	if r.Start != g.At(1, 2) || len(r.Route) != 2 || r.Route[1] != g.At(3, 4) ||
		r.Talks != "miller" || r.Rests != 2*time.Second || r.Muster.At != g.At(5, 5) {
		t.Errorf("the row: %+v", r)
	}
	if e.Kind() != "scout" || e.Name() != "ann" || e.Group() != "scouts" ||
		len(e.Commands()) != 1 {
		t.Errorf("the entry: %q %q %q %v", e.Kind(), e.Name(), e.Group(), e.Commands())
	}
}

func node(t *testing.T, s string) *yaml.Node {
	t.Helper()
	var n yaml.Node
	if err := yaml.Unmarshal([]byte(s), &n); err != nil {
		t.Fatal(err)
	}
	return n.Content[0]
}

func TestLay_AWrongLevelSaysWhere(t *testing.T) {
	for name, tc := range map[string]struct{ file, want string }{
		"a character not in the legend": {
			"board:\n  legend: {\".\": grass}\n  cells: |\n    ....\n    ..x.\n",
			`level.yaml:5: 'x' is not in the legend`,
		},
		"a row too short": {
			"board:\n  legend: {\".\": grass}\n  cells: |\n    ....\n    ...\n",
			`level.yaml:5: the row has 3 cells, the board 4`,
		},
		"a row too many": {
			"board:\n  legend: {\".\": grass}\n  cells: |\n    ....\n    ....\n    ....\n",
			`level.yaml:3: the drawing has 3 rows, the board 2`,
		},
		"a kind of cell nobody defined": {
			"board:\n  legend: {\".\": lava}\n",
			`level.yaml:2: "lava" is no kind of cell`,
		},
		"a role nobody defined": {
			"board:\n  legend: {\".\": {kind: grass, plays: [sea]}}\n",
			`no role is defined as "sea"`,
		},
		"a kind of unit nobody defined": {
			"units:\n  - {kind: knight, start: [0, 0]}\n",
			`level.yaml:2: no kind is defined as "knight"`,
		},
		"a field the row has not": {
			"units:\n  - {kind: scout, horse: grey}\n",
			`level.yaml:2: recruit has no field tagged "horse"`,
		},
		"a field the row does not tag": {
			"units:\n  - {kind: scout, ignored: [0, 0]}\n",
			`level.yaml:2: recruit has no field tagged "ignored"`,
		},
		"a side the game gives not": {
			"units:\n  - {kind: scout, side: blue}\n",
			`level.yaml:2: the game gives no side "blue"`,
		},
		"a point off the board": {
			"units:\n  - {kind: scout, start: [4, 0]}\n",
			`level.yaml:2: [4, 0] is off the board of 4 x 2`,
		},
		"a name given twice": {
			"units:\n  - {kind: scout, name: ann}\n  - {kind: scout, name: ann}\n",
			`level.yaml:3: the name "ann" is given on line 2 already`,
		},
		"a road over a river": {
			"ways:\n  roads: {road: {width: 4, paths: [[[0, 0], [3, 0]]]}}\n" +
				"  rivers: [[{at: [1, 0], kind: river, width: 2}, [1, 1]]]\n",
			`level.yaml:2: the road runs over a river at [1, 0]: lead it onto`,
		},
		"a road onto a bridge in its middle": {
			"ways:\n  roads: {road: {width: 4, paths: [[[0, 0], {onto: [1, 0]}, [3, 0]]]}}\n",
			`level.yaml:2: a road leads onto a bridge or a ford at its ends alone`,
		},
		"a river not beginning with its kind and width": {
			"ways:\n  rivers:\n    - [[0, 0], {at: [3, 0], kind: river, width: 2}]\n",
			`level.yaml:3: a river begins with its kind and width`,
		},
		"a river of a kind of cell nobody defined": {
			"ways:\n  rivers:\n    - [{at: [0, 0], kind: lava, width: 2}, [3, 0]]\n",
			`level.yaml:3: "lava" is no kind of cell`,
		},
		"a key the file has not": {
			"bord: {}\n",
			`field bord not found`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			p := newLevelBoard(t, grid.DefaultGrids{}.Square(4, 2, 10), 0)
			level, err := lay(t, p, tc.file)
			if err != nil && p.seeded != nil {
				t.Error("the board was seeded by a level it could not lay")
			}
			if err == nil {
				err = Spawn[recruit](level, Sides{"red": {mark{}}})
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want an error saying %s", err, tc.want)
			}
		})
	}
}

func TestSpawn_ARowMayNotTagAnEntrysKey(t *testing.T) {
	type named struct {
		Name string `yaml:"name"`
	}
	p := newLevelBoard(t, grid.DefaultGrids{}.Square(4, 2, 10), 0)
	kind.Define[named](p.worldPlugin.Kinds(), "herald", kind.Spec{
		comp.Const(world.Position{}),
		comp.Const(world.Velocity{}),
	})
	level, err := lay(t, p, "units:\n  - {kind: herald}\n")
	if err != nil {
		t.Fatal(err)
	}
	err = Spawn[named](level, nil)
	if err == nil || !strings.Contains(err.Error(), "a key every entity keeps to itself") {
		t.Fatalf("got %v, want the field Name refused", err)
	}
}

var _ command.Command = mark{}

func TestSpawn_AnEntityNobodySpawnsFailsPopulate(t *testing.T) {
	p := newLevelBoard(t, grid.DefaultGrids{}.Square(4, 2, 10), 0)
	if _, err := lay(t, p, "board:\n  default: grass\nunits:\n  - {kind: scout}\n"); err != nil {
		t.Fatal(err)
	}
	err := p.Populate()
	want := `level.yaml:4: nobody spawns the rows of "scout": board.Spawn[recruit]`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("got %v, want %s", err, want)
	}
}

func TestLay_ARoadLiesAsTransportTycoonLaysIt(t *testing.T) {
	road := "ways:\n  roads: {road: {width: 4, paths: [[[0, 0], [3, 0]]]}}\n"
	for name, c := range map[string]struct{ corners, want string }{
		"level ends, a ramp between": {"0 0 1 1.5 1.5\n    0 0 1 1.5 1.5", ""},
		"a ramp too steep":           {"0 0 3 3 3\n    0 0 3 3 3", "road at [1, 0] lies on ground"},
		"tilted across the road":     {"0 0 0 0 0\n    2 2 2 2 2", "road at [0, 0] lies on ground"},
	} {
		p := newLevelBoard(t, grid.DefaultGrids{}.Square(4, 2, 10), 0)
		file := "relief:\n  corners: |\n    " + c.corners + "\n    0 0 0 0 0\n" + road
		_, err := lay(t, p, file)
		if c.want == "" && err != nil || c.want != "" && (err == nil ||
			!strings.Contains(err.Error(), "level.yaml:7: the "+c.want)) {
			t.Errorf("%s: %v, want %q", name, err, c.want)
		}
	}
	hex := newLevelBoard(t, grid.DefaultGrids{}.Hex(3, 1, 10), 0)
	hexRoad := "ways:\n  roads: {road: {width: 4, paths: [[[0, 0], [2, 0]]]}}\n"
	_, err := lay(t, hex, "relief:\n  cells: |\n    0 0 5\n"+hexRoad)
	if err == nil || !strings.Contains(err.Error(), "joins cells at different levels") {
		t.Errorf("a road over a step between hexes: %v, want it refused", err)
	}
}
