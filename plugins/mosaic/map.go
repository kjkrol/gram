package mosaic

import (
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
)

// Map is a ground: its heights and the roads laid over it.
type Map struct {
	// Ground is the height at a place, world units.
	Ground func(at geom.Vec) float32
	// Apart is how far apart the ground's points lie at their finest, thinned over Rungs rungs
	// where the heights let them; zero, 2 and 6.
	Apart float32
	Rungs int
	Seed  uint64
	// Section is how every road lies across.
	Section   Section
	Roads     []Road
	Junctions []Junction
}

// Section is a road across: its lanes, and a pavement and a verge either side.
type Section struct {
	Lanes int
	Lane  float32
	// Walk is how wide a pavement is, more than half a lane; Kerb how high it stands; Verge how
	// wide the verge past it is.
	Walk  float32
	Kerb  float32
	Verge float32
}

// Road is a road's middle line; an end at a Junction's place stops where the junction begins,
// and runs straight that far.
type Road struct {
	Path []geom.Vec
}

// Junction is where roads' ends meet: the ways they leave it, in the order that turns left,
// and the radius its kerbs round the corners at.
type Junction struct {
	At     geom.Vec
	Ways   []geom.Vec
	Radius float32
}

// Kind is a kind of the ground's surface.
type Kind uint8

// The kinds: the ground itself, a road's lanes, its pavement, its verge, the ground a road
// keeps beside it far off, and a junction's own.
const (
	Ground Kind = iota
	Carriageway
	Pavement
	Verge
	Roadside
	Crossing
	Kinds
)

// Look is the colour each kind of surface is drawn in.
type Look struct {
	Colors [Kinds]color.RGBA
}

// DefaultLook is a green ground under grey roads.
var DefaultLook = Look{Colors: [Kinds]color.RGBA{
	Ground:      {R: 96, G: 142, B: 78, A: 255},
	Carriageway: {R: 78, G: 80, B: 84, A: 255},
	Pavement:    {R: 176, G: 170, B: 158, A: 255},
	Verge:       {R: 120, G: 150, B: 84, A: 255},
	Roadside:    {R: 104, G: 144, B: 80, A: 255},
	Crossing:    {R: 84, G: 86, B: 90, A: 255},
}}
