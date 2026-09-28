package navigation

// Spacing is how units keep out of each other's way: a cell each, or box beside box.
type Spacing uint8

const (
	// AutoSpacing is BodySpacing when the world's largest box is at most a third of a cell's
	// shorter side, where three units stand side by side with room, and CellSpacing otherwise.
	AutoSpacing Spacing = iota
	// CellSpacing gives a unit a cell to itself in each domain, as the board's Occupancy says: it
	// holds every cell its step touches, a group spreads a unit to a free cell, and each stands at
	// the cell's centre.
	CellSpacing
	// BodySpacing keeps units apart by their boxes: a group stands box beside box round the point
	// ordered, units on the move steer round each other, and the board's Occupancy is not asked.
	BodySpacing
)

// bodyShare is the most a unit's box may be of a cell's shorter side for AutoSpacing to keep units
// apart by their boxes.
const bodyShare = 1.0 / 3

// resolve is s, AutoSpacing decided for units at most largest a side on cells cell a side.
func (s Spacing) resolve(largest, cell float64) Spacing {
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
