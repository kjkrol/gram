package main

import (
	"bytes"
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"
)

type recipe struct {
	Area   area      `yaml:"area"`
	Zoom   int       `yaml:"zoom"` // the tiles'; 0 finds one fine enough for the cells
	Kinds  kinds     `yaml:"kinds"`
	Ground groundCfg `yaml:"ground"`
	Rivers riversCfg `yaml:"rivers"`
	Roads  roadsCfg  `yaml:"roads"`
}

type area struct {
	Centre [2]float64 `yaml:"centre"` // longitude, latitude
	Cells  [2]int     `yaml:"cells"`  // columns, rows
	Cell   float64    `yaml:"cell"`   // metres
	Units  uint32     `yaml:"units"`  // world units a cell
}

type kinds struct {
	Sea    string `yaml:"sea"`
	Earth  string `yaml:"earth"`
	Sand   string `yaml:"sand"`
	Rock   string `yaml:"rock"`
	Forest string `yaml:"forest"`
	Brook  string `yaml:"brook"`
	Stream string `yaml:"stream"`
	River  string `yaml:"river"`
	Road   string `yaml:"road"`
	Bridge string `yaml:"bridge"`
	Ford   string `yaml:"ford"`
}

type groundCfg struct {
	Rock struct {
		Slope float64 `yaml:"slope"` // rise over run across a cell
		Above float64 `yaml:"above"` // metres
	} `yaml:"rock"`
	Sand struct {
		Within float64 `yaml:"within"` // cells from the sea
		Below  float64 `yaml:"below"`  // metres
		Share  float64 `yaml:"share"`
	} `yaml:"sand"`
	Forest struct {
		From  float64 `yaml:"from"` // metres
		To    float64 `yaml:"to"`
		Share float64 `yaml:"share"`
	} `yaml:"forest"`
}

type riversCfg struct {
	Brook   float64    `yaml:"brook"` // km² drained
	Stream  float64    `yaml:"stream"`
	River   float64    `yaml:"river"`
	Depth   [3]float64 `yaml:"depth"`   // metres: a brook's bed, a stream's, a river's
	Rain    float64    `yaml:"rain"`    // metres up for twice the rain; 0 rains evenly
	Width   float64    `yaml:"width"`   // world units by the square root of the cells drained
	Meander float64    `yaml:"meander"` // metres
	Ford    struct {
		Every int     `yaml:"every"`
		Slope float64 `yaml:"slope"` // rise over run
	} `yaml:"ford"`
}

// roadsCfg prices a step of a road: one cell along, or the diagonal's, times what climbing, rock
// and laying it anew add.
type roadsCfg struct {
	Width  float64               `yaml:"width"` // world units
	Climb  float64               `yaml:"climb"` // per rise over run
	Turn   float64               `yaml:"turn"`
	Grade  float64               `yaml:"grade"` // per metre of earth moved over a cell's metres
	Rock   float64               `yaml:"rock"`  // times
	Reuse  float64               `yaml:"reuse"` // times, on a road laid already
	Bridge float64               `yaml:"bridge"`
	Span   int                   `yaml:"span"`   // cells: the longest bridge, in relief
	High   float64               `yaml:"high"`   // metres: the highest a bridge's deck stands
	Deep   float64               `yaml:"deep"`   // metres: the most a road cuts or fills
	Figure float64               `yaml:"figure"` // metres: the tallest figure, as drawn
	Bank   float64               `yaml:"bank"`
	Places map[string][2]float64 `yaml:"places"` // longitude, latitude
	Routes [][]string            `yaml:"routes"` // each a chain of places
}

func readRecipe(path string) (recipe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return recipe{}, err
	}
	return parseRecipe(data, path)
}

func parseRecipe(data []byte, path string) (recipe, error) {
	r := defaultRecipe()
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&r); err != nil {
		return r, fmt.Errorf("%s: %w", path, err)
	}
	a := r.Area
	if a.Cells[0] < 2 || a.Cells[1] < 2 || a.Cell <= 0 || a.Units <= 0 {
		return r, fmt.Errorf("%s: area: want cells of two or more, a cell's metres and units", path)
	}
	for _, route := range r.Roads.Routes {
		for _, name := range route {
			if _, ok := r.Roads.Places[name]; !ok {
				return r, fmt.Errorf("%s: a route goes through %q, no place", path, name)
			}
		}
	}
	return r, nil
}

func defaultRecipe() recipe {
	var r recipe
	r.Area.Units = 32
	r.Kinds = kinds{
		Sea: "water", Earth: "earth", Sand: "sand", Rock: "rock", Forest: "forest",
		Brook: "brook", Stream: "stream", River: "river",
		Road: "road", Bridge: "bridge", Ford: "ford",
	}
	r.Ground.Rock.Slope = 0.5
	r.Ground.Rock.Above = 1e9
	r.Rivers.Depth = [3]float64{2, 4, 8}
	r.Rivers.Width = 1.4
	r.Roads.Width = 7
	r.Roads.Climb = 6
	r.Roads.Turn = 1
	r.Roads.Grade = 30
	r.Roads.Rock = 4
	r.Roads.Reuse = 0.3
	r.Roads.Bridge = 4
	r.Roads.Span = 6
	r.Roads.High = 60
	r.Roads.Deep = 60
	r.Roads.Bank = 4
	return r
}
