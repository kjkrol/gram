package interiors

import (
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/uid"
)

// Entered is an entity coming into a room, told once as it does: Room is the room's whole.
type Entered struct {
	ID, Room uid.UID64
}

// Who is the entity coming in.
func (e Entered) Who() uid.UID64 { return e.ID }

// Left is an entity going out of a room, told once as it does: Room is the room's whole.
type Left struct {
	ID, Room uid.UID64
}

// Who is the entity going out.
func (l Left) Who() uid.UID64 { return l.ID }

var (
	_ host.About = Entered{}
	_ host.About = Left{}
)
