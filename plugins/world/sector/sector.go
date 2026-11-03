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

// Keen keeps the sector its entity is in and the eight round it ticked at Level at most while
// the entity lives.
type Keen struct{ Level lod.Level }

// Observers are the cameras the world lives round; a camera keeping the whole world in view, a
// minimap's, is none.
type Observers interface {
	Observers(each func(cam camera.Camera))
}
