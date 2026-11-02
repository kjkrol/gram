package main

import (
	"bytes"
	"fmt"
	"math"
	"os"

	"github.com/kjkrol/gram/plugins/board/ground"
	roadsite "github.com/kjkrol/gram/plugins/roads/site"
	"go.yaml.in/yaml/v3"
)

type recipe struct {
	Area   area      `yaml:"area"`
	Zoom   int       `yaml:"zoom"` // 0: one fine enough for the cells
	Kinds  kinds     `yaml:"kinds"`
	Ground groundCfg `yaml:"ground"`
	Rivers riversCfg `yaml:"rivers"`
	Roads  roadsCfg  `yaml:"roads"`
	Flat   flatCfg   `yaml:"flat"`
}

// what a map with -flat takes in place of the rest's; 0, as the rest has it
type flatCfg struct {
	Units   uint32  `yaml:"units"`
	Road    float64 `yaml:"road"`
	Deck    float64 `yaml:"deck"`
	Longest int     `yaml:"longest"`
}

func (r recipe) asFlat() recipe {
	f := r.Flat
	if f.Units > 0 {
		r.Rivers.Width *= float64(f.Units) / float64(r.Area.Units)
		r.Area.Units = f.Units
	}
	if f.Road > 0 {
		r.Roads.Width = f.Road
	}
	if f.Deck > 0 {
		r.Roads.Deck = f.Deck
	}
	if f.Longest > 0 {
		r.Roads.Longest = f.Longest
	}
	return r
}

type area struct {
	Centre [2]float64 `yaml:"centre"` // longitude, latitude
	Cells  [2]int     `yaml:"cells"`  // columns, rows
	Cell   float64    `yaml:"cell"`   // metres
	Units  uint32     `yaml:"units"`  // world units a cell
	Sea    int        `yaml:"sea"`    // cells of sea between the land and every edge
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

// what the roads are allowed (cut, fill, earth, step, spread, slope, fords) and priced (metre,
// cubic, climb, rock, bridge), as the roads tool's site has them
type roadsCfg struct {
	Width     float64               `yaml:"width"`     // world units
	Deck      float64               `yaml:"deck"`      // world units
	Thickness float64               `yaml:"thickness"` // metres
	Cut       float64               `yaml:"cut"`       // metres a corner is dug down, at most
	Fill      float64               `yaml:"fill"`      // metres a corner is raised, at most
	Earth     float64               `yaml:"earth"`     // m³ cut and filled in all, at most
	Step      float64               `yaml:"step"`      // metres; 0: no ground off a road moves
	Spread    int                   `yaml:"spread"`    // cells off a road the ground follows
	Slope     float64               `yaml:"slope"`     // degrees, the steepest ramp
	Fords     bool                  `yaml:"fords"`     // through a river's fords
	Metre     float64               `yaml:"metre"`     // a metre of road's price
	Cubic     float64               `yaml:"cubic"`     // a cubic metre of earth's price
	Climb     float64               `yaml:"climb"`     // a metre dearer by it per rise over run
	Rock      float64               `yaml:"rock"`      // times a metre's price, over rock
	Bridge    float64               `yaml:"bridge"`    // times a metre's price, on a bridge
	Longest   int                   `yaml:"longest"`   // cells a bridge spans, at most
	High      float64               `yaml:"high"`      // metres
	Figure    float64               `yaml:"figure"`    // metres, the tallest figure as drawn
	Route     allowance             `yaml:"route"`
	Places    map[string][2]float64 `yaml:"places"` // longitude, latitude
	Routes    [][]string            `yaml:"routes"` // each a chain of places
}

// the most cut and fill a route between places is planned with, metres, the levelling's where it
// gives none: a step steeper than a road may run is forgiven only that much, so the planner keeps
// to ground the levelling can lay a road on
type allowance struct {
	Cut  *float64 `yaml:"cut"`
	Fill *float64 `yaml:"fill"`
}

// rules are the site's in world units: a map maker's roads cross and join, end at their places,
// run any length and take a bridge only from its head
func (c roadsCfg) rules(perUnit float64, cells int) roadsite.Rules {
	cubed := perUnit * perUnit * perUnit
	return roadsite.Rules{
		Cut:       float32(c.Cut / perUnit),
		Fill:      float32(c.Fill / perUnit),
		Earth:     float32(c.Earth / cubed),
		Spread:    c.Spread,
		Slope:     c.Slope * math.Pi / 180,
		Crossings: true,
		Fords:     c.Fords,
		Longest:   cells,
		Ends:      true,
		Metre:     float32(c.Metre * perUnit),
		Cubic:     float32(c.Cubic * cubed),
		Climb:     float32(c.Climb),
	}
}

// routing are the rules a route is planned under: the levelling's, with share of Route's cut and
// fill
func (c roadsCfg) routing(perUnit float64, cells int, share float64) roadsite.Rules {
	r := c.rules(perUnit, cells)
	cut, fill := c.Cut, c.Fill
	if c.Route.Cut != nil {
		cut = *c.Route.Cut
	}
	if c.Route.Fill != nil {
		fill = *c.Route.Fill
	}
	r.Cut, r.Fill = float32(share*cut/perUnit), float32(share*fill/perUnit)
	return r
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
	r.Roads.Deck = 12
	r.Roads.Thickness = 2
	r.Roads.Cut = 60
	r.Roads.Fill = 60
	r.Roads.Earth = math.Inf(1)
	r.Roads.Slope = ground.RoadSlope * 180 / math.Pi
	r.Roads.Fords = true
	r.Roads.Metre = 1
	r.Roads.Cubic = 0.001
	r.Roads.Climb = 6
	r.Roads.Rock = 4
	r.Roads.Bridge = 4
	r.Roads.Longest = 3
	r.Roads.High = 60
	return r
}
