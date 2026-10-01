package water

import (
	"embed"

	"github.com/kjkrol/gram/render"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

// seaGlint and runningWater are the materials of water (shaders/): the sea's waves and surf, and
// water running down its slope; they call the clouds' functions (plugins/atmosphere/air).
var (
	materials    = render.RegisterMaterials(render.Files(shaders, "shaders/water.wgsl", "shaders/sea.wgsl", "shaders/stream.wgsl"), nil, "SeaGlint", "RunningWater")
	seaGlint     = materials[0]
	runningWater = materials[1]
)

// SeaGlint and RunningWater are the materials Glint and Stream lay, for a test telling their
// overlays apart.
func SeaGlint() render.MaterialID     { return seaGlint }
func RunningWater() render.MaterialID { return runningWater }

// Shore is the way from a point of water to the nearest shore (X, Y, of length 1, or 0 with none
// near), how far it is in world units, and how near: 1 on the shore down to 0 where the open water
// begins.
type Shore struct{ X, Y, Dist, Near float32 }

// Shores is where the nearest shore lies from each corner of what glints — top-left, top-right,
// bottom-left, bottom-right; the zero Shores is open water.
type Shores [4]Shore

// Flow is how fast water runs at each corner of what streams — top-left, top-right, bottom-left,
// bottom-right — in world units a second along x and y.
type Flow [4][2]float32

// Glint lays over the last sprite added to f, whose corners lie at w, water rippled by small waves
// the shader runs across it as time goes by, as shiny at each corner as shine, lit of the sun
// reaching each corner: it throws the sun back at the eye where it faces halfway between them and
// the sky the flatter the eye looks; near a shore the waves turn to face it and break into foam.
func Glint(f *render.Frame, w render.World, shine, lit [4]float32, shores Shores) {
	o := render.Overlay{Material: seaGlint, World: w, Red: shine, Fraction: lit}
	for k, c := range shores {
		o.Custom[k] = [4]float32{c.X, c.Y, c.Dist, c.Near}
	}
	f.Overlay(&o)
}

// Stream lays over the last sprite added to f, whose corners lie at w, water running at flow, as
// shiny at each corner as shine: ripples and flecks of foam carried down with the current, white
// where it runs fast — a rapid, a waterfall; over a sprite drawn blended it shows only where the
// sprite does.
func Stream(f *render.Frame, w render.World, shine, lit [4]float32, flow Flow) {
	o := render.Overlay{Material: runningWater, World: w, Red: shine, Fraction: lit, Blended: true}
	for k, v := range flow {
		o.Custom[k] = [4]float32{v[0], v[1], 0, 0}
	}
	f.Overlay(&o)
}

// The layers the ground's water is painted in for the ground drawn on the GPU, Layers of them,
// each value times the coverage W of its layer, the flows v as v/(2·FlowSpan)+½: FlowLayer (vx,
// vy, Wrun), ShineLayer (run shine·Wrun, sea shine·Wsea, Wsea), MouthLayer (way glint·Wglint,
// Wglint, 0); what covers the water paints them black.
const (
	FlowLayer = iota
	ShineLayer
	MouthLayer
	Layers
)

// FlowSpan is the fastest water the water's flow holds, world units a second: faster runs white
// anyway.
const FlowSpan = 64
