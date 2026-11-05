package sector

import (
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
)

// The marks of the entities in a sector past L0, by the level it is ticked at: zero-size
// components a system's query includes or excludes. What lies at L0 carries none; L4 is asleep,
// out of the world's space.
type (
	L1 struct{}
	L2 struct{}
	L3 struct{}
	L4 struct{}
)

// The marks of the pace: how many levels coarser than the finest level any observer sees the
// entity's sector is ticked at, zero-size components a system's pace goes by. None is Far0.
type (
	Far1 struct{}
	Far2 struct{}
	Far3 struct{}
)

// Keen keeps the sector its entity is in and the eight round it ticked at Level at most while
// the entity lives.
type Keen struct{ Level lod.Level }

// Abstract marks an entity travelling the abstract graph far from every camera: the collision
// neither pushes it nor is pushed by it, and the board does not slow it.
type Abstract struct{}

// Observers are the cameras the world lives round; a camera keeping the whole world in view, a
// minimap's, is none.
type Observers interface {
	Observers(each func(cam camera.Camera))
}
