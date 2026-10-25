package world

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	ilook "github.com/kjkrol/gram/plugins/world/internal/look"
	"github.com/kjkrol/gram/render"
)

// Look is how an entity lies on the screen through a camera, asked by the renderer, picking and
// outlines. The world's own is flat, seen from above; a view plugin puts its own in with
// Plugin.SetLook.
type Look interface {
	// Sprite hands f the entity's settled Appearance for a box standing as z says, lit as lit
	// says. A billboard in relief ignores the Angle.
	Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, z Z, atlas render.AtlasSource,
		a render.Appearance, lit render.Illumination)
	// Drawn is the screen quad that sprite covers, for picking.
	Drawn(cam camera.Camera, box geom.AABB, z Z) render.Corners
	// Footprint appends to dst the ground under box on screen, in pieces where it crosses a wrap
	// seam.
	Footprint(cam camera.Camera, box geom.AABB, alt float32, dst []render.Corners) []render.Corners
}

// DirectLook is a Look that draws the sprites itself on the GPU: readied every frame with Begin
// (which may ask the frame for what it reads), handed the sprites in sight through Sprite, it
// draws them at DrawSprites.
type DirectLook interface {
	Look
	Begin(f *render.Frame, cam camera.Camera)
	DrawSprites(t render.Target, cam camera.Camera, u render.Uniforms)
}

// LyingLook is a Look that lays the sprites declared Lying flat on the ground at the entity's
// Altitude (Lie), and those of an entity with a Lie over its rectangle (Lay).
type LyingLook interface {
	Look
	Lie(f *render.Frame, cam camera.Camera, box plane.AABB, z Z, atlas render.AtlasSource,
		a render.Appearance, lit render.Illumination)
	// Lay lays the sprite level at z's Altitude over the rectangle lie says, on the middle of box
	// and headed dir, and, lie being Edged, the sprite edge along both its long sides.
	Lay(f *render.Frame, cam camera.Camera, box plane.AABB, dir geom.Vec, lie Lie, z Z,
		atlas render.AtlasSource, a render.Appearance, edge render.SpriteID, lit render.Illumination)
}

// UprightLook is a Look that stands the sprites declared Upright true on the four sides and the
// top of their box, where a view in relief stands the rest up as billboards.
type UprightLook interface {
	Look
	Upright(f *render.Frame, cam camera.Camera, box plane.AABB, z Z, atlas render.AtlasSource,
		a render.Appearance, lit render.Illumination)
}

// Lighting is what the world asks of the light as it publishes its entities, a plugin's
// (Write.SetLighting): it joins its columns to the renderer's query and says how the row i of
// the chunk at cur casts its shadow and which shade it takes. The frame lights them as drawn
// (render.Frame.Lit, Cast).
type Lighting interface {
	Join(qb *goke.QueryBuilder)
	Mark(cur *goke.Cursor, i int) render.Shadowing
}

// FoggyLook is a Look turning what it lays next into the fog by amount, 0 clear to 1 all fog: how
// far an entity lies into the fog before a camera's sight (view.View.Fog).
type FoggyLook interface {
	Look
	Fog(amount float32)
}

// FigureLift is how far over its height a figure lies among what is drawn back to front: over
// whatever a plugin lays on the ground beneath it, at a lift of its own below this one.
const FigureLift = ilook.FigureLift

var (
	_ DirectLook = (*ilook.Flat)(nil)
	_ LyingLook  = (*ilook.Flat)(nil)
	_ FoggyLook  = (*ilook.Flat)(nil)
)
