package navigation

// Spacing is how units keep out of each other's way: a cell each, or box beside box.
type Spacing uint8

const (
	// AutoSpacing is BodySpacing when the world's largest box is at most a third of a cell's
	// shorter side, CellSpacing otherwise.
	AutoSpacing Spacing = iota
	// CellSpacing gives a unit a cell to itself in each domain, as the board's Occupancy says, and
	// stands each at its cell's centre.
	CellSpacing
	// BodySpacing keeps units apart by their boxes, several to a cell, and does not ask the
	// board's Occupancy.
	BodySpacing
)

// the most of a cell's shorter side a box may be for AutoSpacing to choose BodySpacing
const bodyShare = 1.0 / 3

func (s Spacing) resolve(largest, cell float32) Spacing {
	if s != AutoSpacing {
		return s
	}
	if largest > 0 && largest <= cell*bodyShare {
		return BodySpacing
	}
	return CellSpacing
}

func (s Spacing) String() string {
	switch s {
	case CellSpacing:
		return "cells"
	case BodySpacing:
		return "bodies"
	}
	return "auto"
}
