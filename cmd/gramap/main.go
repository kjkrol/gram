// Command gramap makes a gram map, a level file a board lays, of any stretch of the Earth at any
// scale, from a recipe:
//
//	go run ./cmd/gramap -recipe cmd/gramap/madeira.yaml -out examples/board/topography/map.yaml
//	go run ./cmd/gramap -recipe cmd/gramap/madeira.yaml -out examples/board/flat/map.yaml -flat
//
// The heights are the Terrain Tiles on AWS (Mapzen's Tilezen terrarium; see
// https://github.com/tilezen/joerd/blob/master/docs/attribution.md), fetched once into a cache.
// The recipe says where, what the ground is, what makes a river and the roads as chains of named
// places; the map gets its relief (none with -flat), its kinds, its rivers, its roads laid as
// Transport Tycoon lays them and its bridges and fords. The roads are the roads tool's
// (plugins/roads/site): routed from place to place by its planner, which gramap lets bridge water
// and valleys, and levelled all together, as the recipe's roads: section allows (cut, fill,
// earth, step, spread, slope, fords) and prices (metre, cubic, climb, rock, bridge). A leg is
// routed forgiving its steps none of the cut and fill first, then a quarter, a half and all of
// route: {cut, fill}, each time planned anew where the levelling fails, the ground there dearer;
// a leg that cannot be laid is left out and named. A units: section in the file made before stays
// as it was. The map is laid on a board before it is written: what a board refuses, gramap
// refuses.
//
// With source: osm in its roads: section a map with relief takes its roads from OpenStreetMap
// (© OpenStreetMap contributors, under the ODbL; asked of the Overpass API once, the answer
// kept beside the tiles): the roads of the classes: named, thinned to lines a cell wide and cut
// into legs at their junctions, each leg planned along its trace by the same planner, bridging
// where OpenStreetMap has a bridge or water lies, and levelled a batch at a time with every road
// laid; a leg the ground refuses is tried in halves, so what of a road can be laid is, and the
// stretches that cannot are counted. Its tunnels are dug by the underground's rules
// (plugins/underground/site) as the tunnels: section allows (clear, cover, grade, longest,
// reshape): a head is cut into the ground and the ground over its mouth raised, a cutting and
// a headwall, and drawn back along the tunnel where that is not enough; one that cannot lie
// stays a road. A flat map plans its own roads along the routes.
//
// The recipe's fields: section lays fields over the cells of a kind, an entity a block (grain
// cells a side from the first cell; a sector of the world, so a field folds and unfolds by
// sector) where at least least of its cells are of that kind, written as rows of the map's
// entities: with the kind, the block's first and last cell and how many cells it covers, for
// the game's rules to plant. Its settlements: section lays cells of a kind round the roads'
// places, as many as a place's size names in sizes: (cells, the words the recipe's: hamlet,
// village, town; none for a pass), beside the roads first, or with source: osm where
// OpenStreetMap has least buildings a cell or more; the game says what stands on them. The
// area's figures: says how many times bigger than life the game draws its figures: a deck's
// thickness, a man's height and a tunnel's room are given in metres of life.
package main

import (
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func main() {
	recipePath := flag.String("recipe", "", "the recipe, YAML")
	out := flag.String("out", "", "the map to write")
	flat := flag.Bool("flat", false, "a map without heights, its bridges a cell long")
	cache := flag.String("cache", "", "where the tiles are kept, OpenStreetMap's answers in osm "+
		"beside it (default: the user's cache)")
	flag.Parse()
	if *recipePath == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*recipePath, *out, *cache, *flat); err != nil {
		fmt.Fprintln(os.Stderr, "gramap:", err)
		os.Exit(1)
	}
}

func run(recipePath, out, cache string, flat bool) error {
	r, err := readRecipe(recipePath)
	if err != nil {
		return err
	}
	if cache == "" {
		dir, err := os.UserCacheDir()
		if err != nil {
			return err
		}
		cache = filepath.Join(dir, "gramap", "terrarium")
	}
	zoom := r.Zoom
	if zoom == 0 {
		zoom = zoomFor(r.Area.Cell, r.Area.Centre[1])
	}
	if flat {
		r = r.asFlat()
	}
	over := newOverpass(filepath.Join(filepath.Dir(cache), "osm"))
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
	c, err := build(r, newTerrarium(zoom, cache).at, osm, flat)
	if err != nil {
		return err
	}
	data := c.text(recipePath, "")
	if err := c.check(data); err != nil {
		return err
	}
	if old, err := os.ReadFile(out); err == nil {
		x, y := originOf(old)
		data = c.text(recipePath, shiftUnits(unitsOf(old), c.origin[0]-x, c.origin[1]-y))
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return err
	}
	for _, leg := range c.roads.skipped {
		if c.roads.guide != nil {
			break // too many to name: the summary counts them
		}
		fmt.Fprintf(os.Stderr, "gramap: no road from %s to %s: %v\n", leg.from, leg.to, leg.why)
	}
	for _, name := range c.unsettled {
		fmt.Fprintf(os.Stderr, "gramap: no settlement at %s: no cell of it\n", name)
	}
	fmt.Printf("%d x %d cells, the recipe's area from [%d, %d]\n", c.cols, c.rows, c.origin[0],
		c.origin[1])
	fmt.Println(c.roads.summary())
	if line := c.fieldsSummary(); line != "" {
		fmt.Println(line)
	}
	for _, name := range slices.Sorted(maps.Keys(c.roads.places)) {
		x, y := c.xy(c.roads.places[name])
		fmt.Printf("%s: [%d, %d]", name, x, y)
		if built := len(c.settled[name]); built > 0 {
			fmt.Printf(", %d cells built", built)
			if n, _ := r.Settlements.cells(r.Roads.Places[name].Size); built < n {
				fmt.Printf(" of %d", n)
			}
		}
		fmt.Println()
	}
	return nil
}

// fieldsSummary is how many fields of each kind the map has, in the recipe's order
func (w *chart) fieldsSummary() string {
	counts := map[string]int{}
	for _, f := range w.fields() {
		counts[f.kind]++
	}
	var parts []string
	for i, f := range w.r.Fields {
		if !slices.ContainsFunc(w.r.Fields[:i], func(g fieldCfg) bool { return g.Kind == f.Kind }) {
			parts = append(parts, fmt.Sprintf("%d %s", counts[f.Kind], f.Kind))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "fields: " + strings.Join(parts, ", ")
}
