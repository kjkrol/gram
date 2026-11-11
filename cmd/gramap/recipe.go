package main

import (
	"bytes"
	"fmt"
	"maps"
	"math"
	"os"
	"slices"
	"strconv"

	"github.com/kjkrol/gram/plugins/board/ground"
	roadsite "github.com/kjkrol/gram/plugins/roads/site"
	"go.yaml.in/yaml/v3"
)

type recipe struct {
	Area        area           `yaml:"area"`
	Zoom        int            `yaml:"zoom"` // 0: one fine enough for the cells
	Kinds       kinds          `yaml:"kinds"`
	Ground      groundCfg      `yaml:"ground"`
	Rivers      riversCfg      `yaml:"rivers"`
	Roads       roadsCfg       `yaml:"roads"`
	Tunnels     tunnelsCfg     `yaml:"tunnels"`
	Fields      []fieldCfg     `yaml:"fields"`
	Settlements settlementsCfg `yaml:"settlements"`
	Flat        flatCfg        `yaml:"flat"`
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
	// how many times bigger than life the figures are drawn; 0: as in life
	Figures float64 `yaml:"figures"`
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
	Built  string `yaml:"built"`
	Tunnel string `yaml:"tunnel"`
}

// land is the kinds the land's cells are drawn as
func (k kinds) land() []string { return []string{k.Earth, k.Sand, k.Rock, k.Forest, k.Built} }

// a field over the cells of a kind in each block of the board, a square grain cells a side counted
// from its first cell, where at least least of them lie
type fieldCfg struct {
	Cells string `yaml:"cells"` // the cells' kind
	Kind  string `yaml:"kind"`  // the field's, a kind of entity
	Grain int    `yaml:"grain"`
	Least int    `yaml:"least"`
}

// the cells laid round the places, as many as a place's size: a word of sizes, none or a number
type settlementsCfg struct {
	Cells   string         `yaml:"cells"` // the kind laid
	Sizes   map[string]int `yaml:"sizes"` // cells
	Default string         `yaml:"default"`
	// osm: the cells are those OpenStreetMap has Least buildings on or more, no place's size
	Source string `yaml:"source"`
	Least  int    `yaml:"least"`
}

// cells is how many cells a place of size is settled on
func (s settlementsCfg) cells(size string) (int, error) {
	if size == "" {
		size = s.Default
	}
	if n, ok := s.Sizes[size]; ok {
		return n, nil
	}
	if size == "" || size == "none" {
		return 0, nil
	}
	if n, err := strconv.Atoi(size); err == nil && n >= 0 {
		return n, nil
	}
	return 0, fmt.Errorf("no size %q", size)
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
	Width     float64          `yaml:"width"`     // world units
	Deck      float64          `yaml:"deck"`      // world units
	Thickness float64          `yaml:"thickness"` // metres
	Cut       float64          `yaml:"cut"`       // metres a corner is dug down, at most
	Fill      float64          `yaml:"fill"`      // metres a corner is raised, at most
	Earth     float64          `yaml:"earth"`     // m³ cut and filled in all, at most
	Step      float64          `yaml:"step"`      // metres; 0: no ground off a road moves
	Spread    int              `yaml:"spread"`    // cells off a road the ground follows
	Slope     float64          `yaml:"slope"`     // degrees, the steepest ramp
	Fords     bool             `yaml:"fords"`     // through a river's fords
	Metre     float64          `yaml:"metre"`     // a metre of road's price
	Cubic     float64          `yaml:"cubic"`     // a cubic metre of earth's price
	Climb     float64          `yaml:"climb"`     // a metre dearer by it per rise over run
	Rock      float64          `yaml:"rock"`      // times a metre's price, over rock
	Bridge    float64          `yaml:"bridge"`    // times a metre's price, on a bridge
	Longest   int              `yaml:"longest"`   // cells a bridge spans, at most
	High      float64          `yaml:"high"`      // metres
	Figure    float64          `yaml:"figure"`    // metres, the tallest figure as drawn
	Route     allowance        `yaml:"route"`
	Places    map[string]place `yaml:"places"`
	Routes    [][]string       `yaml:"routes"` // each a chain of places
	// osm: the roads are OpenStreetMap's of Classes, followed; else planned along Routes
	Source  string   `yaml:"source"`
	Classes []string `yaml:"classes"`
}

// a place roads run to, at a longitude and latitude, settled on as its size says, the
// settlements' default for a bare pair
type place struct {
	At   [2]float64
	Size string
}

func (p *place) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.SequenceNode {
		return n.Decode(&p.At)
	}
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: a place is [longitude, latitude] or {at, size}", n.Line)
	}
	at := false
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		switch k.Value {
		case "at":
			if err := v.Decode(&p.At); err != nil {
				return err
			}
			at = true
		case "size":
			if v.Kind != yaml.ScalarNode {
				return fmt.Errorf("line %d: a place's size is a word or a number", v.Line)
			}
			p.Size = v.Value
		default:
			return fmt.Errorf("line %d: a place has no %s, only at and size", k.Line, k.Value)
		}
	}
	if !at {
		return fmt.Errorf("line %d: a place needs its at", n.Line)
	}
	return nil
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
	if err := r.validate(); err != nil {
		return r, fmt.Errorf("%s: %w", path, err)
	}
	return r, nil
}

func (r recipe) validate() error {
	land := r.Kinds.land()
	for i, f := range r.Fields {
		switch {
		case f.Cells == "" || f.Kind == "":
			return fmt.Errorf("fields: field %d names no cells or no kind", i+1)
		case f.Grain < 1 || f.Least < 1:
			return fmt.Errorf("fields: %s over %s wants a grain and a least of a cell or more",
				f.Kind, f.Cells)
		case f.Cells != r.Kinds.Sea && !slices.Contains(land, f.Cells):
			return fmt.Errorf("fields: %s lies over %q, no kind of the recipe's", f.Kind, f.Cells)
		}
	}
	s := r.Settlements
	if _, err := s.cells(""); err != nil {
		return fmt.Errorf("settlements: the default: %w", err)
	}
	for word, n := range s.Sizes {
		if n < 0 {
			return fmt.Errorf("settlements: %s is %d cells", word, n)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(r.Roads.Places)) {
		n, err := s.cells(r.Roads.Places[name].Size)
		switch {
		case err != nil:
			return fmt.Errorf("%s: %w", name, err)
		case n > 0 && !slices.Contains(land, s.Cells):
			return fmt.Errorf("%s is settled on %d cells of %q, no kind of the recipe's land",
				name, n, s.Cells)
		}
	}
	return nil
}

func defaultRecipe() recipe {
	var r recipe
	r.Area.Units = 32
	r.Kinds = kinds{
		Sea: "water", Earth: "earth", Sand: "sand", Rock: "rock", Forest: "forest",
		Brook: "brook", Stream: "stream", River: "river",
		Road: "road", Bridge: "bridge", Ford: "ford", Built: "built", Tunnel: "tunnel",
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
	r.Tunnels = tunnelsCfg{Clear: 3, Width: 4, Cover: 5, Grade: 0.1, Longest: 40, Least: 2, Reshape: 60}
	return r
}
