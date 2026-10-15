package board

import (
	"bytes"
	"fmt"
	"io/fs"
	"maps"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/network"
	"go.yaml.in/yaml/v3"
)

// Sides are the commands the entities of each side a level names give themselves as they are
// made, by the side's name.
type Sides map[string][]command.Command

// Level is a board laid out of a file by Plugin.Lay, its entities waiting for Spawn.
type Level struct {
	l       *leveler
	heights func(p geom.Vec) float64
	waiting []waiting // in the file's order
}

type waiting struct {
	node    *yaml.Node
	kind    *yaml.Node
	row     reflect.Type
	spawned bool
}

// Heights is the ground's heights the level's relief gives, in world units; nil for a level
// without one.
func (l *Level) Heights() func(p geom.Vec) float64 { return l.heights }

// Lay seeds the board out of the YAML level at path of fsys and keeps its entities for Spawn.
func (p *Plugin) Lay(fsys fs.FS, path string) (*Level, error) {
	if err := p.worldPlugin.InSection("a level laid", section.Spawn); err != nil {
		return nil, fmt.Errorf("board: %w", err)
	}
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("board: %w", err)
	}
	var file levelFile
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&file); err != nil {
		return nil, fmt.Errorf("board: %s: %w", path, err)
	}
	shape, ok := grid.ShapeOf(p.Board().Grid)
	if !ok {
		return nil, fmt.Errorf("board: %s: a level lays a board of grid.DefaultGrids only", path)
	}
	l := &leveler{p: p, path: path, shape: shape, names: map[string]int{}}
	level := &Level{l: l}
	var layout Layout
	if file.Board != nil {
		if layout, err = l.terrain(file.Board); err != nil {
			return nil, err
		}
	}
	if level.heights, err = l.relief(&file.Relief); err != nil {
		return nil, err
	}
	if err := l.ways(&file.Ways, &layout, level.heights); err != nil {
		return nil, err
	}
	var standing []yaml.Node
	if file.Board != nil {
		standing = file.Board.Entities
	}
	for _, n := range append(standing, file.Units...) {
		w, err := l.waiting(&n)
		if err != nil {
			return nil, err
		}
		level.waiting = append(level.waiting, w)
	}
	if file.Board != nil || file.Ways.given() {
		p.Seed(layout)
	}
	p.level = level
	return level, nil
}

// Spawn seeds the world with the entities of level whose kinds have rows of R, sides being the
// commands of the sides the entries name; an entity no Spawn takes fails the board's Populate.
func Spawn[R any](level *Level, sides Sides) error {
	l := level.l
	if err := l.p.worldPlugin.InSection("a level's entities spawned", section.Spawn); err != nil {
		return fmt.Errorf("board: %w", err)
	}
	row := reflect.TypeFor[R]()
	if row.Kind() != reflect.Struct {
		return fmt.Errorf("board: %s: a row is a struct, %v is not", l.path, row)
	}
	for _, f := range reflect.VisibleFields(row) {
		if key, ok := tagOf(f); ok && slices.Contains(entryKeys, key) {
			return fmt.Errorf("board: %s: %s tags its field %s %q, a key every entity keeps to "+
				"itself", l.path, row.Name(), f.Name, key)
		}
	}
	var entries []kind.Entry
	var taken []*waiting
	for i := range level.waiting {
		w := &level.waiting[i]
		if w.row != row {
			continue
		}
		e, r, err := l.entry(w, sides)
		if err != nil {
			return err
		}
		if at := l.p.cellOf[w.kind.Value]; at != nil {
			if l.p.where == nil {
				l.p.where = map[cell.ID]string{}
			}
			l.p.where[at(r)] = fmt.Sprintf("%s:%d", l.path, w.node.Line)
		}
		entries = append(entries, e)
		taken = append(taken, w)
	}
	for _, w := range taken {
		w.spawned = true
	}
	l.p.worldPlugin.Seed(entries...)
	return nil
}

// Each hands f, in the file's order, every row of R the level's entities have, with the kind it
// names and the file and line it is written at, and takes them as Spawn does; f's error ends the
// walk.
func Each[R any](level *Level, f func(kind string, row R, at string) error) error {
	l := level.l
	if err := l.p.worldPlugin.InSection("a level's rows taken", section.Spawn); err != nil {
		return fmt.Errorf("board: %w", err)
	}
	row := reflect.TypeFor[R]()
	for i := range level.waiting {
		w := &level.waiting[i]
		if w.row != row {
			continue
		}
		_, r, err := l.entry(w, nil)
		if err != nil {
			return err
		}
		at := fmt.Sprintf("%s:%d", l.path, w.node.Line)
		if err := f(w.kind.Value, r.(R), at); err != nil {
			return fmt.Errorf("%s: %w", at, err)
		}
		w.spawned = true
	}
	return nil
}

func (level *Level) unspawned() error {
	for _, w := range level.waiting {
		if !w.spawned {
			name := w.row.Name()
			return level.l.errorf(w.kind, "nobody spawns the rows of %q: board.Spawn[%s]",
				w.kind.Value, name)
		}
	}
	return nil
}

type levelFile struct {
	Board  *levelBoard `yaml:"board"`
	Relief levelRelief `yaml:"relief"`
	Ways   levelWays   `yaml:"ways"`
	Units  []yaml.Node `yaml:"units"`
}

type levelBoard struct {
	Default  string               `yaml:"default"`
	Legend   map[string]yaml.Node `yaml:"legend"`
	Cells    yaml.Node            `yaml:"cells"`
	Entities []yaml.Node          `yaml:"entities"`
}

type levelRelief struct {
	Cells   yaml.Node `yaml:"cells"`
	Corners yaml.Node `yaml:"corners"`
}

type levelWays struct {
	Roads  map[string]yaml.Node `yaml:"roads"`
	Rivers []yaml.Node          `yaml:"rivers"`
}

func (w *levelWays) given() bool { return len(w.Roads) > 0 || len(w.Rivers) > 0 }

type leveler struct {
	p     *Plugin
	path  string
	shape grid.Shape
	names map[string]int // by the line giving each
}

func (l *leveler) errorf(n *yaml.Node, format string, args ...any) error {
	return fmt.Errorf("board: %s:%d: %s", l.path, n.Line, fmt.Sprintf(format, args...))
}

func (l *leveler) terrain(b *levelBoard) (Layout, error) {
	layout := Layout{Default: b.Default}
	if b.Default != "" {
		if _, ok := l.p.kinds.Get(b.Default); !ok {
			err := fmt.Errorf("board: %s: the default %q is no kind of cell", l.path, b.Default)
			return layout, err
		}
	}
	legend := map[rune]cell.Entry{}
	for _, key := range slices.Sorted(maps.Keys(b.Legend)) {
		n := b.Legend[key]
		r, size := utf8.DecodeRuneInString(key)
		if size != len(key) || r == ' ' {
			err := l.errorf(&n, "the legend's %q is no one character but a space", key)
			return layout, err
		}
		e, err := l.legend(&n)
		if err != nil {
			return layout, err
		}
		legend[r] = e
	}
	if b.Cells.Kind == 0 {
		return layout, nil
	}
	first := b.Cells.Line
	if b.Cells.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		first++
	}
	rows := lines(b.Cells.Value)
	if len(rows) != int(l.shape.Rows) {
		err := l.errorf(&b.Cells, "the drawing has %d rows, the board %d", len(rows), l.shape.Rows)
		return layout, err
	}
	brd := l.p.Board()
	for y, row := range rows {
		x := 0
		for _, r := range row {
			if r == ' ' {
				continue
			}
			at := &yaml.Node{Line: first + y}
			e, ok := legend[r]
			if !ok {
				return layout, l.errorf(at, "%q is not in the legend", r)
			}
			if x >= int(l.shape.Cols) {
				return layout, l.errorf(at, "the row is longer than the board's %d", l.shape.Cols)
			}
			if e.Kind == b.Default && e.Name == "" && e.Group == "" && e.Roles == 0 {
				x++
				continue
			}
			if e.Name != "" {
				if err := l.named(at, e.Name); err != nil {
					return layout, err
				}
			}
			e.Cell = brd.At(uint32(x), uint32(y))
			layout.Cells = append(layout.Cells, e)
			x++
		}
		if x != int(l.shape.Cols) {
			at := &yaml.Node{Line: first + y}
			return layout, l.errorf(at, "the row has %d cells, the board %d", x, l.shape.Cols)
		}
	}
	return layout, nil
}

func (l *leveler) legend(n *yaml.Node) (cell.Entry, error) {
	if n.Kind == yaml.ScalarNode {
		return cell.Entry{Kind: n.Value}, l.cellKind(n, n.Value)
	}
	keys, err := l.keys(n, "kind", "plays", "name", "group")
	if err != nil {
		return cell.Entry{}, err
	}
	var e cell.Entry
	if k := keys["kind"]; k != nil {
		e.Kind = k.Value
		if err := l.cellKind(k, e.Kind); err != nil {
			return e, err
		}
	}
	if k := keys["name"]; k != nil {
		e.Name = k.Value
	}
	if k := keys["group"]; k != nil {
		e.Group = k.Value
	}
	if k := keys["plays"]; k != nil {
		var roles []string
		if err := k.Decode(&roles); err != nil {
			return e, l.errorf(k, "plays: %v", err)
		}
		for _, name := range roles {
			role, ok := l.p.worldPlugin.Roles().Lookup(name)
			if !ok {
				return e, l.errorf(k, "no role is defined as %q", name)
			}
			e = e.Plays(role)
		}
	}
	return e, nil
}

func (l *leveler) cellKind(n *yaml.Node, name string) error {
	if _, ok := l.p.kinds.Get(name); !ok {
		return l.errorf(n, "%q is no kind of cell", name)
	}
	return nil
}

func (l *leveler) named(n *yaml.Node, name string) error {
	if line, ok := l.names[name]; ok {
		return l.errorf(n, "the name %q is given on line %d already", name, line)
	}
	l.names[name] = n.Line
	return nil
}

func (l *leveler) keys(n *yaml.Node, allowed ...string) (map[string]*yaml.Node, error) {
	if n.Kind != yaml.MappingNode {
		return nil, l.errorf(n, "want a mapping")
	}
	out := map[string]*yaml.Node{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if !slices.Contains(allowed, k.Value) {
			want := strings.Join(allowed, ", ")
			return nil, l.errorf(k, "unknown key %q, want one of %s", k.Value, want)
		}
		out[k.Value] = v
	}
	return out, nil
}

func (l *leveler) ways(w *levelWays, layout *Layout, heights func(geom.Vec) float64) error {
	brd := l.p.Board()
	roads, rivers := network.New(brd), network.New(brd)
	laidBy := map[cell.ID]*yaml.Node{}
	for _, name := range slices.Sorted(maps.Keys(w.Roads)) {
		n := w.Roads[name]
		width, paths, err := l.road(&n, name)
		if err != nil {
			return err
		}
		node := network.Node{Kind: name, Width: width}
		for _, path := range paths {
			for i, c := range path.cells {
				if _, ok := roads.Node(c); !ok && !path.onto(i) {
					roads.Set(c, node)
					laidBy[c] = path.node
				}
				if i > 0 {
					roads.Link(path.cells[i-1], c)
				}
			}
		}
	}
	var runs [][]cell.ID
	for i := range w.Rivers {
		cells, nodes, err := l.river(&w.Rivers[i])
		if err != nil {
			return err
		}
		for k, node := range nodes {
			if _, ok := rivers.Node(cells[k]); !ok {
				rivers.Set(cells[k], node)
			}
		}
		runs = append(runs, cells)
	}
	for _, cells := range runs { // every river laid first, so a tributary flows into its cells
		for k := 1; k < len(cells); k++ {
			rivers.Flow(cells[k-1], cells[k])
		}
	}
	for _, way := range roads.Ways() {
		if _, both := rivers.Node(way.Cell); both {
			x, y, _ := brd.Coords(way.Cell)
			return l.errorf(laidBy[way.Cell], "the %s runs over a river at [%d, %d]: lead it onto "+
				"the bridge or the ford there, {onto: [%d, %d]}", way.Kind, x, y, x, y)
		}
		layout.Ways = append(layout.Ways, way)
	}
	if heights != nil {
		if err := l.graded(roads, laidBy, heights); err != nil {
			return err
		}
	}
	layout.Ways = append(layout.Ways, rivers.Ways()...)
	return nil
}

// a graded road lies as Transport Tycoon lays it
func (l *leveler) graded(roads *network.Network, laidBy map[cell.ID]*yaml.Node,
	heights func(geom.Vec) float64) error {
	brd := l.p.Board()
	run := l.shape.Cell
	for _, c := range roads.Cells() {
		node, _ := roads.Node(c)
		if kind, _ := l.p.kinds.Get(node.Kind); !kind.Graded {
			continue
		}
		x, y, _ := brd.Coords(c)
		links := roads.Links(c)
		if l.shape.Hex {
			for i := range brd.Directions() {
				nb, ok := brd.Toward(c, i)
				if !ok || !links.Has(i) {
					continue
				}
				if !ground.RoadJoins(heights(brd.CellCenter(c)), heights(brd.CellCenter(nb))) {
					return l.errorf(laidBy[c], "the %s at [%d, %d] joins cells at different levels",
						node.Kind, x, y)
				}
			}
			continue
		}
		var corners [6]float64
		outline := brd.CellOutline(c, nil)
		for k, p := range outline {
			corners[k] = heights(p)
		}
		if !ground.RoadFits(brd, c, corners[:len(outline)], links, run) {
			return l.errorf(laidBy[c], "the %s at [%d, %d] lies on ground no road may: level, or "+
				"an even ramp it runs straight up", node.Kind, x, y)
		}
	}
	return nil
}

type roadPath struct {
	cells []cell.ID
	node  *yaml.Node
	first bool // leads onto its first cell without running over it
	last  bool
}

func (r roadPath) onto(i int) bool { return i == 0 && r.first || i == len(r.cells)-1 && r.last }

func (l *leveler) road(n *yaml.Node, name string) (width float64, paths []roadPath, err error) {
	if err := l.cellKind(n, name); err != nil {
		return 0, nil, err
	}
	keys, err := l.keys(n, "width", "paths")
	if err != nil {
		return 0, nil, err
	}
	wn, pn := keys["width"], keys["paths"]
	if wn == nil || pn == nil {
		return 0, nil, l.errorf(n, "the %s needs a width and paths", name)
	}
	if wn.Decode(&width) != nil {
		return 0, nil, l.errorf(wn, "width: want a number")
	}
	if pn.Kind != yaml.SequenceNode {
		return 0, nil, l.errorf(pn, "paths: want a list of paths")
	}
	for _, path := range pn.Content {
		r := roadPath{node: path}
		k := 0
		point := func(pt *yaml.Node) (*yaml.Node, error) {
			defer func() { k++ }()
			if pt.Kind != yaml.MappingNode {
				return pt, nil
			}
			if k != 0 && k != len(path.Content)-1 {
				return nil, l.errorf(pt, "a road leads onto a bridge or a ford at its ends alone")
			}
			keys, err := l.keys(pt, "onto")
			if err != nil {
				return nil, err
			}
			if keys["onto"] == nil {
				return nil, l.errorf(pt, "want {onto: [x, y]}")
			}
			r.first, r.last = r.first || k == 0, r.last || k != 0
			return keys["onto"], nil
		}
		if r.cells, _, err = l.points(path, point); err != nil {
			return 0, nil, err
		}
		paths = append(paths, r)
	}
	return width, paths, nil
}

// the last cell is where the water goes, no part of the river
func (l *leveler) river(n *yaml.Node) (cells []cell.ID, nodes []network.Node, err error) {
	type mark struct {
		at    int
		kind  string
		width float64
		wide  bool
	}
	var marks []mark
	point := func(pt *yaml.Node) (*yaml.Node, error) {
		if pt.Kind != yaml.MappingNode {
			return pt, nil
		}
		keys, err := l.keys(pt, "at", "kind", "width")
		if err != nil {
			return nil, err
		}
		if keys["at"] == nil {
			return nil, l.errorf(pt, "a point of a river needs its at: [x, y]")
		}
		var m mark
		if k := keys["kind"]; k != nil {
			if err := l.cellKind(k, k.Value); err != nil {
				return nil, err
			}
			m.kind = k.Value
		}
		if w := keys["width"]; w != nil {
			if w.Decode(&m.width) != nil {
				return nil, l.errorf(w, "width: want a number")
			}
			m.wide = true
		}
		m.at = -1
		marks = append(marks, m)
		return keys["at"], nil
	}
	cells, at, err := l.points(n, point)
	if err != nil {
		return nil, nil, err
	}
	k := 0
	for i, pt := range n.Content {
		if pt.Kind == yaml.MappingNode {
			marks[k].at = at[i]
			k++
		}
	}
	if len(marks) == 0 || marks[0].at != 0 || marks[0].kind == "" || !marks[0].wide {
		return nil, nil, l.errorf(n, "a river begins with its kind and width: "+
			"{at: [x, y], kind: river, width: 6}")
	}
	kind := ""
	for i := range cells[:len(cells)-1] {
		var from, to *mark
		for m := range marks {
			if marks[m].at <= i && marks[m].kind != "" {
				kind = marks[m].kind
			}
			if marks[m].wide && marks[m].at <= i {
				from = &marks[m]
			}
			if marks[m].wide && marks[m].at > i && to == nil {
				to = &marks[m]
			}
		}
		width := from.width
		if to != nil {
			width += (to.width - from.width) * float64(i-from.at) / float64(to.at-from.at)
		}
		nodes = append(nodes, network.Node{Kind: kind, Width: width})
	}
	return cells, nodes, nil
}

// point reads a point down to its [x, y]
func (l *leveler) points(n *yaml.Node, point func(*yaml.Node) (*yaml.Node, error)) (
	cells []cell.ID, at []int, err error,
) {
	if n.Kind != yaml.SequenceNode || len(n.Content) < 2 {
		return nil, nil, l.errorf(n, "a path is two points or more")
	}
	for i, pt := range n.Content {
		xy, err := point(pt)
		if err != nil {
			return nil, nil, err
		}
		c, err := l.cell(xy)
		if err != nil {
			return nil, nil, err
		}
		if i > 0 {
			step, ok := l.route(cells[len(cells)-1], c)
			if !ok || len(step) < 2 {
				return nil, nil, l.errorf(pt, "no way gets to this point from the one before")
			}
			cells = append(cells, step[1:len(step)-1]...)
		}
		cells = append(cells, c)
		at = append(at, len(cells)-1)
	}
	return cells, at, nil
}

// among the grid's equally short ways, the one nearest the straight line
func (l *leveler) route(from, to cell.ID) ([]cell.ID, bool) {
	brd := l.p.Board()
	a, b := brd.CellCenter(from), brd.CellCenter(to)
	off := func(c cell.ID) float64 {
		p := brd.CellCenter(c)
		dx, dy := b.X-a.X, b.Y-a.Y
		if d := dx*dx + dy*dy; d > 0 {
			return math.Abs((p.X-a.X)*dy-(p.Y-a.Y)*dx) / math.Sqrt(d)
		}
		return 0
	}
	return network.Route(brd, from, to, func(x, y cell.ID) float64 {
		return brd.NeighborCost(x, y) + 1e-3*off(y)
	})
}

func (l *leveler) cell(n *yaml.Node) (cell.ID, error) {
	var xy [2]int64
	if n.Kind != yaml.SequenceNode || len(n.Content) != 2 || n.Decode(&xy) != nil {
		return 0, l.errorf(n, "want a cell, [x, y]")
	}
	if xy[0] < 0 || xy[1] < 0 || xy[0] >= int64(l.shape.Cols) || xy[1] >= int64(l.shape.Rows) {
		return 0, l.errorf(n, "[%d, %d] is off the board of %d x %d",
			xy[0], xy[1], l.shape.Cols, l.shape.Rows)
	}
	return l.p.Board().At(uint32(xy[0]), uint32(xy[1])), nil
}

func (l *leveler) relief(r *levelRelief) (func(p geom.Vec) float64, error) {
	cells, corners := r.Cells.Kind != 0, r.Corners.Kind != 0
	if !cells && !corners {
		return nil, nil
	}
	n := &r.Cells
	if !cells {
		n = &r.Corners
	} else if corners {
		return nil, l.errorf(&r.Corners, "the relief gives its cells or its corners, not both")
	}
	cols, rows := int(l.shape.Cols), int(l.shape.Rows)
	if corners {
		if l.shape.Hex || l.shape.WrapX || l.shape.WrapY {
			return nil, l.errorf(n, "corners are a square grid's that wraps along no axis")
		}
		cols, rows = cols+1, rows+1
	}
	values, err := l.numbers(n, cols, rows)
	if err != nil {
		return nil, err
	}
	brd := l.p.Board()
	if cells {
		return ground.MeanOfCells(brd, func(c cell.ID) float64 {
			x, y, _ := brd.Coords(c)
			return values[int(y)*cols+int(x)]
		}), nil
	}
	size := l.shape.Cell
	return func(p geom.Vec) float64 {
		x := min(max(int(math.Round(p.X/size)), 0), cols-1)
		y := min(max(int(math.Round(p.Y/size)), 0), rows-1)
		return values[y*cols+x]
	}, nil
}

func (l *leveler) numbers(n *yaml.Node, cols, rows int) ([]float64, error) {
	first := n.Line
	if n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		first++
	}
	lns := lines(n.Value)
	if len(lns) != rows {
		return nil, l.errorf(n, "the relief has %d rows, want %d", len(lns), rows)
	}
	scale := l.p.worldPlugin.Scale()
	values := make([]float64, 0, cols*rows)
	for y, ln := range lns {
		at := &yaml.Node{Line: first + y}
		fields := strings.Fields(ln)
		if len(fields) != cols {
			return nil, l.errorf(at, "the row has %d numbers, want %d", len(fields), cols)
		}
		for _, f := range fields {
			m, err := strconv.ParseFloat(f, 64)
			if err != nil {
				return nil, l.errorf(at, "%q is no number", f)
			}
			values = append(values, scale.Units(m))
		}
	}
	return values, nil
}

func (l *leveler) waiting(n *yaml.Node) (waiting, error) {
	if n.Kind != yaml.MappingNode {
		return waiting{}, l.errorf(n, "an entity is a mapping: its kind, side, name, group, row")
	}
	own := ownKeys(n)
	kn := own["kind"]
	if kn == nil {
		return waiting{}, l.errorf(n, "an entity needs a kind")
	}
	_, _, row, ok := l.p.worldPlugin.Kinds().Lookup(kn.Value)
	if !ok {
		return waiting{}, l.errorf(kn, "no kind is defined as %q", kn.Value)
	}
	if v := own["name"]; v != nil {
		if err := l.named(v, v.Value); err != nil {
			return waiting{}, err
		}
	}
	return waiting{node: n, kind: kn, row: row}, nil
}

func (l *leveler) entry(w *waiting, sides Sides) (kind.Entry, any, error) {
	row := reflect.New(w.row).Elem()
	n := w.node
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if slices.Contains(entryKeys, k.Value) {
			continue
		}
		f, ok := tagged(w.row, k.Value)
		if !ok {
			return kind.Entry{}, nil, l.errorf(k, "%s has no field tagged %q", w.row.Name(),
				k.Value)
		}
		if err := l.value(v, row.FieldByIndex(f.Index)); err != nil {
			return kind.Entry{}, nil, err
		}
	}
	r := row.Interface()
	e := kind.EntryOf(w.kind.Value, r)
	own := ownKeys(n)
	if v := own["name"]; v != nil {
		e = e.Named(v.Value)
	}
	if v := own["group"]; v != nil {
		e = e.InGroup(v.Value)
	}
	if v := own["side"]; v != nil {
		cmds, ok := sides[v.Value]
		if !ok {
			return e, r, l.errorf(v, "the game gives no side %q", v.Value)
		}
		e = e.Told(cmds...)
	}
	return e, r, nil
}

func ownKeys(n *yaml.Node) map[string]*yaml.Node {
	own := map[string]*yaml.Node{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if k := n.Content[i]; slices.Contains(entryKeys, k.Value) {
			own[k.Value] = n.Content[i+1]
		}
	}
	return own
}

var entryKeys = []string{"kind", "side", "name", "group"}

// a row's keys are its tagged fields alone, so the file's keys are said where the row is
func tagged(row reflect.Type, key string) (reflect.StructField, bool) {
	for _, f := range reflect.VisibleFields(row) {
		if k, ok := tagOf(f); ok && k == key {
			return f, true
		}
	}
	return reflect.StructField{}, false
}

func tagOf(f reflect.StructField) (string, bool) {
	name, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
	return name, f.IsExported() && !f.Anonymous && name != "" && name != "-"
}

func fieldOf(row reflect.Type, key string) (reflect.StructField, bool) {
	if row.Kind() != reflect.Struct {
		return reflect.StructField{}, false
	}
	for _, f := range reflect.VisibleFields(row) {
		if k, ok := keyOf(f); ok && k == key {
			return f, true
		}
	}
	return reflect.StructField{}, false
}

// a field's key as YAML calls it, so a field and a struct inside it read alike
func keyOf(f reflect.StructField) (string, bool) {
	if !f.IsExported() || f.Anonymous {
		return "", false
	}
	name, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
	switch name {
	case "-":
		return "", false
	case "":
		return strings.ToLower(f.Name), true
	}
	return name, true
}

var cellType = reflect.TypeFor[cell.ID]()

// a cell.ID is read from [x, y] however deep it lies, the rest as YAML reads it
func (l *leveler) value(n *yaml.Node, v reflect.Value) error {
	if !holdsCells(v.Type()) {
		if err := n.Decode(v.Addr().Interface()); err != nil {
			return l.errorf(n, "%v", err)
		}
		return nil
	}
	if v.Type() == cellType {
		c, err := l.cell(n)
		v.Set(reflect.ValueOf(c))
		return err
	}
	switch v.Kind() {
	case reflect.Pointer:
		p := reflect.New(v.Type().Elem())
		v.Set(p)
		return l.value(n, p.Elem())
	case reflect.Slice, reflect.Array:
		if n.Kind != yaml.SequenceNode {
			return l.errorf(n, "want a list")
		}
		if v.Kind() == reflect.Array && v.Len() != len(n.Content) {
			return l.errorf(n, "want a list of %d", v.Len())
		}
		if v.Kind() == reflect.Slice {
			v.Set(reflect.MakeSlice(v.Type(), len(n.Content), len(n.Content)))
		}
		for i, e := range n.Content {
			if err := l.value(e, v.Index(i)); err != nil {
				return err
			}
		}
		return nil
	}
	if n.Kind != yaml.MappingNode {
		return l.errorf(n, "want a mapping")
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		f, ok := fieldOf(v.Type(), k.Value)
		if !ok {
			return l.errorf(k, "a %v has no field %q", v.Type(), k.Value)
		}
		if err := l.value(n.Content[i+1], v.FieldByIndex(f.Index)); err != nil {
			return err
		}
	}
	return nil
}

func holdsCells(t reflect.Type) bool {
	switch {
	case t == cellType:
		return true
	case t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array:
		return holdsCells(t.Elem())
	case t.Kind() == reflect.Struct:
		for _, f := range reflect.VisibleFields(t) {
			if _, ok := keyOf(f); ok && holdsCells(f.Type) {
				return true
			}
		}
	}
	return false
}

func lines(s string) []string {
	out := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(out) == 1 && out[0] == "" {
		return nil
	}
	return out
}
