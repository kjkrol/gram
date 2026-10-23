package look

import (
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

// Look is how a Map's cells lie: flat on the ground, flat with what sways on them leaning in the
// wind, or not at all, the Map drawing its ground some other way.
type Look uint8

const (
	// Flat lays each cell's sprite over its box on the ground.
	Flat Look = iota
	// Swaying lays them as Flat does, the tile of a kind that sways (cell.Kind.Sway) and stands at
	// a Height leant with the frame's wind, as air.Weather.Sway leans a unit of height.
	Swaying
	// Nothing lays no tile.
	Nothing
)

// Dressing is what a Map lays over the board's tiles beyond their sprites.
type Dressing interface {
	// Begin readies the dressing for a frame through cam.
	Begin(f *render.Frame, cam camera.Camera)
	// Base is the sprite c's tile is drawn in, own its kind's.
	Base(c cell.ID, own render.SpriteID) render.SpriteID
}

// GroundDressing is a Dressing that lays something over the drawn ground itself, part of it:
// the clouds' shadows of an atmosphere.
type GroundDressing interface {
	// DressGround draws over the ground just drawn into t through cam, under the frame's u.
	DressGround(t render.Target, cam camera.Camera, u render.Uniforms)
}

// Laid is a Dressing whose tiles the GPU lays: what runs across each cell, laid as bands from its
// middle towards the neighbours its Links name, cut out of the way kind's sprite, and the covers
// on it, each faded in over its neighbours as far as they are covered too.
type Laid interface {
	// Way is what runs across c.
	Way(c cell.ID) cell.Way
	// Covers are the covers' sprites in the order they are laid; Covered which lie on c, bit i the
	// i-th's.
	Covers() []render.SpriteID
	Covered(c cell.ID) uint8
}

// Overlaid is a Dressing laying a material over the tile of some cells, as render.Overlay lays one
// over a sprite: World the tile's box, Red red, Fraction 1.
type Overlaid interface {
	// Overlay is the material laid over c's tile and its red; false for none.
	Overlay(c cell.ID) (m render.MaterialID, red float32, ok bool)
}
