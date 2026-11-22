package board

import (
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
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
	// the kind of entity a highway's line tagged bridge or tunnel is, by the highway, and
	// the kind a ford is
	Bridges map[string]string `yaml:"bridges"`
	Tunnels map[string]string `yaml:"tunnels"`
	Fords   string            `yaml:"fords"`
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
	entities := map[string]string{"ford": cfg.Fords}
	for _, k := range cfg.Bridges {
		entities["bridge"] = k
	}
	for _, k := range cfg.Tunnels {
		entities["tunnel"] = k
	}
	for _, how := range slices.Sorted(maps.Keys(entities)) {
		k := entities[how]
		if _, _, _, ok := l.p.worldPlugin.Kinds().Lookup(k); k != "" && !ok {
			return nil, fmt.Errorf("board: %s: the geography lays a %s as %q, no kind of "+
				"entity", l.path, how, k)
		}
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

// land is the map's heights as the level's relief: at its cells' corners under a square
// board, a cell's the height at its middle under a hex one
func (l *leveler) land(g *geography) func(p geom.Vec) float32 {
	at := func(p geom.Vec) float32 {
		return g.file.HeightAt(p.X/g.units, p.Y/g.units) * g.units
	}
	if !l.shape.Hex {
		// at the corner nearest, as a relief in numbers answers: a height asked for between
		// two corners is one of theirs, the same whoever asks
		size, cols, rows := l.shape.Cell, float32(l.shape.Cols), float32(l.shape.Rows)
		return func(p geom.Vec) float32 {
			x := min(max(f32.Round(p.X/size), 0), cols)
			y := min(max(f32.Round(p.Y/size), 0), rows)
			return at(geom.NewVec(x*size, y*size))
		}
	}
	brd := &l.p.board.read
	return ground.MeanOfCells(brd, func(c cell.ID) float32 { return at(brd.CellCenter(c)) })
}

// stretch is a part of a map's line lying on the board as the cells it runs over: points are
// the line's points it has, at the place of each one's cell among cells
type stretch struct {
	cells  []cell.ID
	points []int
	at     []int
}

// stretches is a line of the map stretch by stretch that lies on the board: its points' cells
// joined as a level's path joins its own
func (l *leveler) stretches(g *geography, line []geom.Vec) []stretch {
	brd := &l.p.board.read
	var all []stretch
	var run stretch
	end := func() {
		if len(run.cells) > 1 {
			all = append(all, run)
		}
		run = stretch{}
	}
	begin := func(i int, c cell.ID) {
		run = stretch{cells: []cell.ID{c}, points: []int{i}, at: []int{0}}
	}
	for i, v := range line {
		c, ok := brd.CellAt(g.at(v))
		if !ok {
			end()
			continue
		}
		if len(run.cells) == 0 {
			begin(i, c)
			continue
		}
		last := run.cells[len(run.cells)-1]
		if c == last {
			continue
		}
		step, ok := l.route(last, c)
		if !ok || len(step) < 2 {
			end()
			begin(i, c)
			continue
		}
		run.cells = append(run.cells, step[1:]...)
		run.points = append(run.points, i)
		run.at = append(run.at, len(run.cells)-1)
	}
	end()
	return all
}

func (l *leveler) cells(g *geography, line []geom.Vec) [][]cell.ID {
	var all [][]cell.ID
	for _, s := range l.stretches(g, line) {
		all = append(all, s.cells)
	}
	return all
}

// widths is a line's width tag, metres: nothing, one for the whole line, or one a point
// between semicolons
func widths(f mapfile.Feature) ([]float32, error) {
	tag, ok := f.Tags["width"]
	if !ok {
		return nil, nil
	}
	parts := strings.Split(tag, ";")
	if len(parts) != 1 && len(parts) != len(f.Path) {
		return nil, fmt.Errorf("%d widths for its %d points: want one, or one a point",
			len(parts), len(f.Path))
	}
	wide := make([]float32, len(parts))
	for i, p := range parts {
		w, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil || w <= 0 {
			return nil, fmt.Errorf("the width %q is no number over zero", p)
		}
		wide[i] = float32(w)
	}
	return wide, nil
}

// across is how wide a line is at each cell of s but the last: as wide as at the point
// there, evenly from point to point between
func (s stretch) across(wide []float32) []float32 {
	out := make([]float32, len(s.cells)-1)
	k := 0
	for i := range out {
		for k+1 < len(s.at) && s.at[k+1] <= i {
			k++
		}
		out[i] = wide[s.points[k]]
		if k+1 < len(s.at) {
			share := float32(i-s.at[k]) / float32(s.at[k+1]-s.at[k])
			out[i] += (wide[s.points[k+1]] - out[i]) * share
		}
	}
	return out
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
		from := g.name + ": " + named(f)
		runs := l.cells(g, f.Path)
		how, entity, err := g.crossing(f, highway)
		if err != nil {
			return nil, err
		}
		if how == "" {
			for _, cells := range runs {
				r.paths = append(r.paths, roadPath{cells: cells, from: from})
			}
			continue
		}
		if len(runs) != 1 || len(runs[0]) < 3 {
			return nil, fmt.Errorf("board: %s: a %s runs from the cell before it to the cell "+
				"after, whole on the board: three cells at least", from, how)
		}
		cells := runs[0]
		last := len(cells) - 1
		r.paths = append(r.paths,
			roadPath{cells: cells[:2], from: from, last: true},
			roadPath{cells: cells[last-1:], from: from, first: true})
		switch how {
		case "ford":
			for k := 1; k < last; k++ {
				l.ford(entity, from, cells[k], cells[k-1])
			}
		case "tunnel":
			var via []string
			for k := 1; k < last; k++ {
				if l.wayOf(cells[k-1], cells[k]) != l.wayOf(cells[k], cells[k+1]) {
					via = append(via, l.xy(cells[k]))
				}
			}
			l.make(from, "{kind: %q, from: %s, to: %s, via: [%s]}", entity, l.xy(cells[0]),
				l.xy(cells[last]), strings.Join(via, ", "))
		default:
			l.make(from, "{kind: %q, from: %s, to: %s}", entity, l.xy(cells[0]),
				l.xy(cells[last]))
		}
	}
	var out []laidRoad
	for _, kind := range slices.Sorted(maps.Keys(byKind)) {
		out = append(out, *byKind[kind])
	}
	return out, nil
}

// crossing is how a highway's line crosses what it runs over, a bridge, a tunnel or a ford
// by its tags, and the kind of entity the geography's tables lay it as; "" on the ground
func (g *geography) crossing(f mapfile.Feature, highway string) (how, entity string, err error) {
	for _, how := range []string{"bridge", "tunnel", "ford"} {
		if v := f.Tags[how]; v == "" || v == "no" {
			continue
		}
		entity := g.cfg.Fords
		switch how {
		case "bridge":
			entity = g.cfg.Bridges[highway]
		case "tunnel":
			entity = g.cfg.Tunnels[highway]
		}
		if entity == "" {
			return "", "", fmt.Errorf("board: %s: %s: the level's geography lays no %s for "+
				"highway=%s", g.name, named(f), how, highway)
		}
		return how, entity, nil
	}
	return "", "", nil
}

// made is a row of the level's entities a feature of its geography gives, and the feature
type made struct {
	node *yaml.Node
	from string
}

// make notes the row a feature gives, written as a level's file writes one
func (l *leveler) make(from, format string, args ...any) {
	var doc yaml.Node
	if err := yaml.Unmarshal(fmt.Appendf(nil, format, args...), &doc); err != nil {
		panic(err)
	}
	l.made = append(l.made, made{node: doc.Content[0], from: from})
}

func (l *leveler) xy(c cell.ID) string {
	x, y, _ := (&l.p.board.read).Coords(c)
	return fmt.Sprintf("[%d, %d]", x, y)
}

// wayOf is the direction from a cell to the one beside it
func (l *leveler) wayOf(from, to cell.ID) int {
	brd := &l.p.board.read
	for i := range brd.Directions() {
		if c, ok := brd.Toward(from, i); ok && c == to {
			return i
		}
	}
	return 0
}

// ford notes a ford at a cell, the way across it the way there from the cell before; a cell
// forded already has its ford
func (l *leveler) ford(entity, from string, at, before cell.ID) {
	if l.forded[at] {
		return
	}
	l.forded[at] = true
	da, db := (&l.p.board.read).Offset(l.wayOf(before, at))
	l.make(from, "{kind: %q, at: %s, way: [%d, %d]}", entity, l.xy(at), da, db)
}

// fords is the map's points tagged ford as fords of the rivers they lie on, across the
// water's way; one a road's line fords already is that ford
func (l *leveler) fords(g *geography, runs [][]cell.ID) error {
	if g == nil {
		return nil
	}
	brd := &l.p.board.read
	for _, f := range g.file.Features {
		if v := f.Tags["ford"]; f.Shape != mapfile.Point || v == "" || v == "no" {
			continue
		}
		from := g.name + ": " + named(f)
		if g.cfg.Fords == "" {
			return fmt.Errorf("board: %s: the level's geography lays no ford", from)
		}
		at, ok := brd.CellAt(g.at(f.Path[0]))
		if !ok {
			continue
		}
		if l.forded[at] {
			continue
		}
		down := -1
		for _, cells := range runs {
			if k := slices.Index(cells, at); k >= 0 && k < len(cells)-1 {
				down = l.wayOf(at, cells[k+1])
				break
			}
		}
		if down < 0 {
			return fmt.Errorf("board: %s: the ford lies on no river", from)
		}
		along := brd.Step(down)
		da, db := brd.Offset(grid.Nearest(l.p.board.Grid, geom.NewVec(-along.Y, along.X)))
		l.forded[at] = true
		l.make(from, "{kind: %q, at: %s, way: [%d, %d]}", g.cfg.Fords, l.xy(at), da, db)
	}
	return nil
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
		wide, err := widths(f)
		if err != nil {
			return fmt.Errorf("board: %s: %s: %w", g.name, named(f), err)
		}
		if wide == nil && width <= 0 {
			return l.errorf(&n, "the %s needs a width, here or as the line's width: "+
				"{kind: %s, width: 6}", kind, kind)
		}
		for _, s := range l.stretches(g, f.Path) {
			nodes := make([]network.Node, len(s.cells)-1)
			for i := range nodes {
				nodes[i] = network.Node{Kind: kind, Width: width}
			}
			if len(wide) == 1 {
				for i := range nodes {
					nodes[i].Width = wide[0] * g.units
				}
			}
			if len(wide) > 1 {
				for i, w := range s.across(wide) {
					nodes[i].Width = w * g.units
				}
			}
			run(s.cells, nodes)
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
