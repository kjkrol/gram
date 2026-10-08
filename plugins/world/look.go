package world

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	ilook "github.com/kjkrol/gram/plugins/world/internal/look"
	"github.com/kjkrol/gram/render"
)

// Look is how what stands in the world lies on the screen through a camera: the world's way of
// drawing its entities, which the renderer, picking and outlines ask. The world starts with a flat
// look, seen from above; a view plugin puts its own in with Plugin.SetLook.
type Look interface {
	// Sprite hands f the entity's settled Appearance — its sprite of atlas, how much it sways
	// for a Look that knows a wind, the angle it is turned by — for a box standing as z says (at
	// its Altitude, Height tall; the zero Z in a flat world), on the render.Objects tier, in
	// light. A billboard in relief ignores the Angle; the flat look turns the sprite.
	Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, z Z, atlas render.AtlasSource, a render.Appearance, light render.Light)
	// Drawn is the screen quad that sprite covers, for picking.
	Drawn(cam camera.Camera, box geom.AABB, z Z) render.Corners
	// Footprint appends to dst the ground under box on screen, in pieces where it crosses a wrap
	// seam, for outlines.
	Footprint(cam camera.Camera, box geom.AABB, alt float32, dst []render.Corners) []render.Corners
}

// DirectLook is a Look that draws the sprites itself on the GPU, through the cameras it takes,
// rather than handing the frame their pieces: the world's renderer readies it every frame (Begin),
// hands it the sprites in sight through Sprite as ever — the Look keeping them, and handing the
// frame what else it lays — and has it draw them where render.Objects comes (DrawSprites).
type DirectLook interface {
	Look
	Begin(cam camera.Camera)
	DrawSprites(t render.Target, cam camera.Camera, u render.Uniforms)
}

// The world's own look, before any view sets another, is the flat one.
var _ DirectLook = (*ilook.Flat)(nil)
