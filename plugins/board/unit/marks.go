package unit

import (
	"github.com/kjkrol/ceremony/tag"
)

// States is the family of a unit's markers on the board: states switched by a bit, never by a
// component put on or taken off. The board gives it to every unit the world's roster makes.
type States struct{}

// Entered is on for the step a unit's At changed: put on by whoever moves it into another cell,
// taken off by the board as its next pass begins.
const Entered tag.Tag[States] = 0

// EnteredName is the name Entered is defined under, as the saves know it.
const EnteredName = "unit.entered"

// Airborne is on while a unit is in the air, put on and off by whoever carries it there
// (plugins/gravity). A unit in the air has not fallen in where it is over.
const Airborne tag.Tag[States] = 1

// AirborneName is the name Airborne is defined under, as the saves know it.
const AirborneName = "unit.airborne"
