package water

import (
	"embed"

	"github.com/kjkrol/gram/render"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

// the shaders call the clouds' functions (plugins/atmosphere/air)
var (
	materials    = render.RegisterMaterials(render.Files(shaders, "shaders/water.wgsl", "shaders/sea.wgsl", "shaders/stream.wgsl"), nil, "SeaGlint", "RunningWater")
	seaGlint     = materials[0]
	runningWater = materials[1]
)

// SeaGlint is the material Glint lays, for a test telling its overlays apart.
func SeaGlint() render.MaterialID { return seaGlint }

// RunningWater is the material Stream lays, for a test telling its overlays apart.
func RunningWater() render.MaterialID { return runningWater }

// Shore is the way from a point of water to the nearest shore. X, Y are of length 1 (0 with none
// near), Dist is in world units, Near 1 on the shore down to 0 where the open water begins.
type Shore struct{ X, Y, Dist, Near float32 }

// Shores is the shore from each corner of what glints: top-left, top-right, bottom-left,
// bottom-right. The zero Shores is open water.
type Shores [4]Shore

// Flow is how fast water runs at each corner of what streams (top-left, top-right, bottom-left,
// bottom-right), in world units a second along x and y.
type Flow [4][2]float32

// Glint lays still water over the last sprite added to f, whose corners lie at w, as shiny at
// each corner as shine and lit of the sun reaching it.
func Glint(f *render.Frame, w render.World, shine, lit [4]float32, shores Shores) {
	o := render.Overlay{Material: seaGlint, World: w, Red: shine, Fraction: lit}
	for k, c := range shores {
		o.Custom[k] = [4]float32{c.X, c.Y, c.Dist, c.Near}
	}
	f.Overlay(&o)
}

// Stream lays water running at flow over the last sprite added to f, whose corners lie at w.
// Over a sprite drawn blended it shows only where the sprite does.
func Stream(f *render.Frame, w render.World, shine, lit [4]float32, flow Flow) {
	o := render.Overlay{Material: runningWater, World: w, Red: shine, Fraction: lit, Blended: true}
	for k, v := range flow {
		o.Custom[k] = [4]float32{v[0], v[1], 0, 0}
	}
	f.Overlay(&o)
}

// The layers the ground's water is painted in, each value times the coverage W of its layer,
// a flow v as v/(2·FlowSpan)+½: FlowLayer (vx, vy, Wrun), ShineLayer (run shine·Wrun, sea
// shine·Wsea, Wsea), MouthLayer (way glint·Wglint, Wglint, 0). What covers the water paints
// them black.
const (
	FlowLayer = iota
	ShineLayer
	MouthLayer
	Layers
)

// FlowSpan is the fastest flow the water's layers hold, in world units a second.
const FlowSpan = 64
