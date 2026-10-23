package render

import "github.com/kjkrol/gram/render/internal/instanced"

// Sprites is sprites a Direct source draws on the GPU as instances, by Layer or else in the order
// they came, one call a run sharing an atlas.
type Sprites = instanced.Sprites
