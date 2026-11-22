package board

import (
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/mapfile"
	"github.com/kjkrol/gram/plugins/board/network"
	"go.yaml.in/yaml/v3"
)

type levelGeography struct {
	File   string               `yaml:"file"`
	Roads  map[string]yaml.Node `yaml:"roads"`
	Rivers map[string]yaml.Node `yaml:"rivers"`
	Areas  map[string]string    `yaml:"areas"`
}

// geography is a map in files a level lays its ground by, and how: its metres as the
// world's units
type geography struct {
	file *mapfile.File
	name string
	// units is the world's units a metre
	units float32
	cfg   *levelGeography
}

func (l *leveler) geography(fsys fs.FS, cfg *levelGeography) (*geography, error) {
	if cfg == nil {
		return nil, nil
	}
	if cfg.File == "" {
		return nil, fmt.Errorf("board: %s: the geography names no file", l.path)
	}
	g := &geography{
		name:  path.Join(path.Dir(l.path), cfg.File),
		units: l.p.worldPlugin.Read(0).Scale().Units(1),
		cfg:   cfg,
	}
	var err error
	if g.file, err = mapfile.Load(fsys, g.name); err != nil {
		return nil, err
	}
	if err := g.file.Whole(); err != nil {
		return nil, err
	}
	brd := &l.p.board.read
	cols, rows := l.shape.Cols, l.shape.Rows
	var wide, tall float32
	// the far corner, and a hex board's second row, half a cell east of the others' ends
	for _, at := range [][2]uint32{{cols - 1, rows - 1}, {cols - 1, min(1, rows-1)}} {
		for _, p := range brd.CellOutline(brd.At(at[0], at[1]), nil) {
			wide, tall = max(wide, p.X), max(tall, p.Y)
		}
	}
	if w, h := g.file.Width*g.units, g.file.Height*g.units; w < wide-1e-3 || h < tall-1e-3 {
		return nil, fmt.Errorf("board: %s: a map of %g x %g m does not cover the board's "+
			"%g x %g", g.name, g.file.Width, g.file.Height, wide/g.units, tall/g.units)
	}
	return g, nil
}

// at is a place of the map, metres, as the world's
func (g *geography) at(v geom.Vec) geom.Vec { return geom.NewVec(v.X*g.units, v.Y*g.units) }

// land is the map's heights as the level's relief: as they are under a square board, a
// cell's the height at its middle under a hex one
func (l *leveler) land(g *geography) func(p geom.Vec) float32 {
	at := func(p geom.Vec) float32 {
		return g.file.HeightAt(p.X/g.units, p.Y/g.units) * g.units
	}
	if !l.shape.Hex {
		return at
	}
	brd := &l.p.board.read
	return ground.MeanOfCells(brd, func(c cell.ID) float32 { return at(brd.CellCenter(c)) })
}

// cells is a line of the map as the cells it runs over, stretch by stretch that lies on the
// board: its points' cells joined as a level's path joins its own
func (l *leveler) cells(g *geography, line []geom.Vec) [][]cell.ID {
	brd := &l.p.board.read
	var all [][]cell.ID
	var run []cell.ID
	end := func() {
		if len(run) > 1 {
			all = append(all, run)
		}
		run = nil
	}
	for _, v := range line {
		c, ok := brd.CellAt(g.at(v))
		if !ok {
			end()
			continue
		}
		if len(run) == 0 {
			run = append(run, c)
			continue
		}
		if c == run[len(run)-1] {
			continue
		}
		step, ok := l.route(run[len(run)-1], c)
		if !ok || len(step) < 2 {
			end()
			run = append(run, c)
			continue
		}
		run = append(run, step[1:]...)
	}
	end()
	return all
}

func named(f mapfile.Feature) string {
	if f.ID != "" {
		return "the feature " + f.ID
	}
	return "a feature without an id"
}

// kindOf reads a table's entry: a kind's name, or {kind, width}
func (l *leveler) kindOf(n *yaml.Node) (kind string, width float32, err error) {
	if n.Kind == yaml.ScalarNode {
		kind = n.Value
	} else {
		keys, err := l.keys(n, "kind", "width")
		if err != nil {
			return "", 0, err
		}
		if keys["kind"] == nil {
			return "", 0, l.errorf(n, "want a kind, or {kind: road, width: 6}")
		}
		kind = keys["kind"].Value
		if w := keys["width"]; w != nil && w.Decode(&width) != nil {
			return "", 0, l.errorf(w, "width: want a number")
		}
	}
	return kind, width, l.cellKind(n, kind)
}

// lines is the map's lines tagged highway as the roads the geography's table lays them as
func (l *leveler) lines(g *geography) ([]laidRoad, error) {
	if g == nil || len(g.cfg.Roads) == 0 {
		return nil, nil
	}
	byKind := map[string]*laidRoad{}
	for _, f := range g.file.Features {
		highway := f.Tags["highway"]
		if f.Shape != mapfile.Line || highway == "" {
			continue
		}
		n, ok := g.cfg.Roads[highway]
		if !ok {
			return nil, fmt.Errorf("board: %s: %s: the level's geography lays no road for "+
				"highway=%s", g.name, named(f), highway)
		}
		kind, width, err := l.kindOf(&n)
		if err != nil {
			return nil, err
		}
		if s := g.file.Sections[highway]; width == 0 {
			width = float32(s.Lanes) * s.Lane * g.units
		}
		if width <= 0 {
			return nil, l.errorf(&n, "the %s has no width, here or in the map's roads", kind)
		}
		r := byKind[kind]
		if r == nil {
			r = &laidRoad{kind: kind, width: width}
			byKind[kind] = r
		}
		for _, cells := range l.cells(g, f.Path) {
			r.paths = append(r.paths, roadPath{cells: cells, from: g.name + ": " + named(f)})
		}
	}
	var out []laidRoad
	for _, kind := range slices.Sorted(maps.Keys(byKind)) {
		out = append(out, *byKind[kind])
	}
	return out, nil
}

// waters is the map's lines tagged waterway as rivers, each handed to run: the last cell
// where the water goes, as a level's own river ends
func (l *leveler) waters(g *geography, run func(cells []cell.ID, nodes []network.Node)) error {
	if g == nil || len(g.cfg.Rivers) == 0 {
		return nil
	}
	for _, f := range g.file.Features {
		waterway := f.Tags["waterway"]
		if f.Shape != mapfile.Line || waterway == "" {
			continue
		}
		n, ok := g.cfg.Rivers[waterway]
		if !ok {
			return fmt.Errorf("board: %s: %s: the level's geography lays no river for "+
				"waterway=%s", g.name, named(f), waterway)
		}
		kind, width, err := l.kindOf(&n)
		if err != nil {
			return err
		}
		if width <= 0 {
			return l.errorf(&n, "the %s needs a width: {kind: %s, width: 6}", kind, kind)
		}
		for _, cells := range l.cells(g, f.Path) {
			nodes := make([]network.Node, len(cells)-1)
			for i := range nodes {
				nodes[i] = network.Node{Kind: kind, Width: width}
			}
			run(cells, nodes)
		}
	}
	return nil
}

// areas is the kind each cell lies as whose middle is inside an area of the map the
// geography's table names, a later area over an earlier
func (l *leveler) areas(g *geography) (map[cell.ID]string, error) {
	under := map[cell.ID]string{}
	if g == nil || len(g.cfg.Areas) == 0 {
		return under, nil
	}
	tags := slices.Sorted(maps.Keys(g.cfg.Areas))
	for _, tag := range tags {
		if _, ok := l.p.kinds.Get(g.cfg.Areas[tag]); !ok {
			return nil, fmt.Errorf("board: %s: the geography's area %s is laid as %q, no kind "+
				"of cell", l.path, tag, g.cfg.Areas[tag])
		}
	}
	brd := &l.p.board.read
	cols, rows := int(l.shape.Cols), int(l.shape.Rows)
	for _, f := range g.file.Features {
		if f.Shape != mapfile.Area {
			continue
		}
		kind := ""
		for _, tag := range tags {
			key, value, valued := strings.Cut(tag, "=")
			if got, ok := f.Tags[key]; ok && (!valued || got == value) {
				kind = g.cfg.Areas[tag]
				break
			}
		}
		if kind == "" {
			continue
		}
		lo, hi := f.Path[0], f.Path[0]
		for _, v := range f.Path {
			lo = geom.NewVec(min(lo.X, v.X), min(lo.Y, v.Y))
			hi = geom.NewVec(max(hi.X, v.X), max(hi.Y, v.Y))
		}
		// the cells round the area's box, a cell to spare either way: a hex board's columns are
		// √3 of its cell apart, its rows one and a half
		wide, tall := l.shape.Cell/g.units, l.shape.Cell/g.units
		if l.shape.Hex {
			wide, tall = wide*1.7320508, tall*1.5
		}
		x0, x1 := max(int(lo.X/wide)-1, 0), min(int(hi.X/wide)+1, cols-1)
		y0, y1 := max(int(lo.Y/tall)-1, 0), min(int(hi.Y/tall)+1, rows-1)
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				c := brd.At(uint32(x), uint32(y))
				mid := brd.CellCenter(c)
				mid = geom.NewVec(mid.X/g.units, mid.Y/g.units)
				if !within(mid, f.Path) {
					continue
				}
				holed := false
				for _, hole := range f.Holes {
					holed = holed || within(mid, hole)
				}
				if !holed {
					under[c] = kind
				}
			}
		}
	}
	return under, nil
}

// within is whether a place lies inside an outline
func within(p geom.Vec, outline []geom.Vec) bool {
	in := false
	for k, a := range outline {
		b := outline[(k+len(outline)-1)%len(outline)]
		if (a.Y > p.Y) != (b.Y > p.Y) && p.X < (b.X-a.X)*(p.Y-a.Y)/(b.Y-a.Y)+a.X {
			in = !in
		}
	}
	return in
}
