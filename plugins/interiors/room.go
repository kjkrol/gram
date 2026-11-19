package interiors

import "github.com/kjkrol/uid"

// Room marks a room's whole (world.Whole), whose pieces are the floors with a room of its cells
// (cell.Floor), its floor as high as the whole's world.Z: the plugin keeps who stands in it. What
// of the sky's light gets into it is the whole's light.Shelter.
type Room struct{}

// Inside is the room an entity on the board stands in, its whole; none where it is in none.
type Inside struct{ Room uid.UID64 }
