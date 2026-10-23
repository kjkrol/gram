package render

import (
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render/internal/feed"
)

// Surface is a picture shown at a size whoever shows it gives it.
type Surface interface {
	// Resize gives the surface its size in pixels.
	Resize(w, h int)
	// Draw is the picture for this frame, drawn anew; nil while it has no size.
	Draw() *Image
}

// Feed is the world seen through a camera: a Picture drawn through it every frame at its
// camera's resolution, in layers or with its figures sharp as the camera says, shown at its size.
type Feed = feed.Feed

// Picture is what is in the world before a camera, drawn through the camera it is handed.
type Picture = feed.Picture

// Laid is a Picture drawn in layers from one composition (a Composer).
type Laid = feed.Laid

// LayerFeather is the shown pixels an inner layer's edge fades over.
const LayerFeather = feed.LayerFeather

var _ Surface = (*Feed)(nil)

// NewFeed is the world picture draws seen through cam.
func NewFeed(cam camera.Camera, picture Picture) *Feed { return feed.NewFeed(cam, picture) }
