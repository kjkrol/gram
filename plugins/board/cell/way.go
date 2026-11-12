package cell

import "github.com/kjkrol/aabbworld/geom"

// Way is what runs across a cell over its ground, a band Width wide from the cell's middle out
// towards each neighbour its Links name; its Kind decides who may cross and what it costs. The
// zero Way is none.
type Way struct {
	Kind  Kind
	Width float32
	Links Links
	Fade  float32 // how far it has faded out across the cell, 0 to 1 gone
	Mix   float32 // how far its look has turned into another's, 0 to 1
	// Off is where its axis passes the cell, off the cell's middle, in world units.
	Off geom.Vec
	// Lanes is how many lanes it has in all; 0 leaves it unmarked.
	Lanes uint8
	// Out is the Links it may be left by, fewer than its Links where it runs one way; 0 any.
	Out Links
	// Form is how the ways meeting in the cell join, a Roundabout Radius wide.
	Form   Form
	Radius float32
}

// Form is how the ways meeting in a cell join.
type Form uint8

const (
	// Joined ways run into each other.
	Joined Form = iota
	// Roundabout has them meet round an island.
	Roundabout
)

// OneWay reports whether the way may be left by fewer links than it runs on by.
func (w Way) OneWay() bool { return w.Out != 0 && w.Out != w.Links }

// Links is which ways a Way runs on out of its cell: bit i the grid's i-th direction.
type Links uint8

// Has reports whether the way runs out in direction i.
func (l Links) Has(i int) bool { return l&(1<<i) != 0 }

// Runs reports whether anything runs across the cell.
func (w Way) Runs() bool { return w.Width > 0 }

// Over is ground as whoever crosses the cell meets it: who may, the pace and the grading from the
// Way's kind, the rest from the ground.
func (w Way) Over(ground Kind) Kind {
	if !w.Runs() {
		return ground
	}
	ground.Allows, ground.Pace, ground.Paces = w.Kind.Allows, w.Kind.Pace, w.Kind.Paces
	ground.Graded = w.Kind.Graded
	return ground
}
