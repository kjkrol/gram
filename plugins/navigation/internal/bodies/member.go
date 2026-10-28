package bodies

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/pathfind"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// Member is a unit as a tick of navigation sees it.
type Member struct {
	ID     uid.UID64
	Cell   cell.ID
	From   cell.ID // where a new route sets off: its cell, or where its step ends
	Leg    order.Leg
	Domain cell.Domain
	Pos    world.Position
	Vel    geom.Vec // world units a second
	Facing geom.Vec // the way it faces, standing too
	// the way off the units it touches, an axis at a time: -1, 0 or 1, and 0 between two
	Touching geom.Vec
	Z        world.Z
	Lift     float32
	Brake    float32 // world units a second²
	Share    float32 // of its top speed, what the ground leaves it this tick: 1 without a Pace
	MaxSpeed float32 // world units a second
	Order    *order.MoveOrder
	// who came at m last tick, a step into m's cell refused
	PressedBy uid.UID64
	Pressed   bool
	Owners    tag.Tags[owner.Family]
}

// Centre is the middle of m's box.
func (m Member) Centre() geom.Vec { return m.Pos.Center() }

// BodyOf is the body a search for a route takes standing as z says over size, its box on the
// ground, zero fitting any lane.
func BodyOf(z world.Z, size geom.Vec) pathfind.Body {
	return pathfind.Body{Feet: z.Altitude, Step: z.Reach(), Height: z.Height, Size: size}
}

// Body is m as a search for its route takes it.
func (m Member) Body() pathfind.Body { return BodyOf(m.Z, m.Pos.Size) }
