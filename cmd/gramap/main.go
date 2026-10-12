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
// Transport Tycoon lays them and its bridges and fords. A units: section in the file made before
// stays as it was. The map is laid on a board before it is written: what a board refuses, gramap
// refuses.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

func main() {
	recipePath := flag.String("recipe", "", "the recipe, YAML")
	out := flag.String("out", "", "the map to write")
	flat := flag.Bool("flat", false, "a map without heights, its bridges a cell long")
	cache := flag.String("cache", "", "where the tiles are kept (default: the user's cache)")
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
	c, err := build(r, newTerrarium(zoom, cache).at, flat)
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
		fmt.Fprintf(os.Stderr, "gramap: no road from %s to %s: the ground would be cut or filled "+
			"deeper than %v m (roads.deep)\n", leg[0], leg[1], r.Roads.Deep)
	}
	fmt.Printf("%d x %d cells, the recipe's area from [%d, %d]\n", c.cols, c.rows, c.origin[0],
		c.origin[1])
	names := make([]string, 0, len(c.roads.places))
	for name := range c.roads.places {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		x, y := c.xy(c.roads.places[name])
		fmt.Printf("%s: [%d, %d]\n", name, x, y)
	}
	return nil
}
