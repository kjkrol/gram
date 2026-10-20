package cameras

import (
	"fmt"
	"math"
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
)

// Strategy is how the pictures' resolution follows the frame budget: not at all, the whole
// picture's, or layer by layer (Settings.Strategy).
type Strategy uint8

const (
	// Still keeps the resolution as set.
	Still Strategy = iota
	// Uniform steps the whole picture's resolution.
	Uniform
	// InLayers has the picture drawn in layers and steps them, the outer layers first.
	InLayers
)

// Settings is what the cameras let the players adjust, saved with the game: the scroll zones,
// the picture's resolution and the Strategy following the frame budget, the figures drawn at
// every pixel, the layers it is drawn in and whether their edges are shown.
type Settings struct {
	Zones      bool
	Strategy   Strategy
	Resolution float32 // of the pixels, the centre layer's in layers
	// the figures at the ground's resolution; zero, at every pixel, as a save from before the
	// field loads
	SoftFigures bool
	Layers      bool
	Inner       float32 // the inner layer's share of the frame
	Middle      float32 // the middle layer's
	OuterRes    float32 // the outer layer's resolution
	MiddleRes   float32 // the middle layer's
	FrameBudget time.Duration
	ShowLayers  bool
}

// Resources is what the cameras publish: their Settings.
type Resources struct {
	Settings Settings
}

// SetStrategy has every picture's resolution follow the frame budget as s says, or stay
// (Settings.Strategy): the pictures drawn in layers under InLayers, whole under Uniform.
func (p *Plugin) SetStrategy(s Strategy) {
	p.Res.Settings.Strategy = s
	if s != Still {
		p.Res.Settings.Layers = s == InLayers
	}
	p.adapter.on = s != Still
	p.redress()
}

// Strategy is how the pictures' resolution follows the frame budget.
func (p *Plugin) Strategy() Strategy { return p.Res.Settings.Strategy }

// SetLayers has every picture but a minimap's drawn in layers, or whole (Settings.Layers); a
// Strategy saying otherwise stops (Still).
func (p *Plugin) SetLayers(on bool) {
	p.Res.Settings.Layers = on
	if s := p.Res.Settings.Strategy; s == InLayers && !on || s == Uniform && on {
		p.still()
	}
	p.redress()
}

// Layers reports whether the pictures are drawn in layers.
func (p *Plugin) Layers() bool { return p.Res.Settings.Layers }

// SetResolution has every picture but a minimap's drawn at r of its pixels, the centre layer's
// in layers, LeastResolution up to 1 (Settings.Resolution), and stops the following (Still).
func (p *Plugin) SetResolution(r float32) {
	p.Res.Settings.Resolution = clamped(r)
	p.still()
	p.redress()
}

// Resolution is the share of its pixels every picture is drawn at, the centre layer's in layers.
func (p *Plugin) Resolution() float32 { return p.Res.Settings.Resolution }

// SetSharpFigures has every picture but a minimap's draw its figures, all that lies over the
// ground, at every shown pixel while its ground is drawn at the resolution, or at the resolution
// too (Settings.SoftFigures); a picture drawn in layers is drawn as it is.
func (p *Plugin) SetSharpFigures(on bool) {
	p.Res.Settings.SoftFigures = !on
	p.redress()
}

// SharpFigures reports whether the figures are drawn at every pixel.
func (p *Plugin) SharpFigures() bool { return !p.Res.Settings.SoftFigures }

// SetLayerShares has the inner layer cover inner of the frame and the middle layer middle, at
// least a tenth over the inner (Settings.Inner, Middle).
func (p *Plugin) SetLayerShares(inner, middle float32) {
	inner = min(max(inner, 0.1), 0.8)
	p.Res.Settings.Inner = inner
	p.Res.Settings.Middle = min(max(middle, inner+0.1), 0.9)
	p.redress()
}

// LayerShares are the inner and the middle layer's shares of the frame.
func (p *Plugin) LayerShares() (inner, middle float32) {
	return p.Res.Settings.Inner, p.Res.Settings.Middle
}

// SetLayerResolutions has the outer layer drawn at outer of its pixels and the middle layer at
// middle, LeastResolution up to 1 (Settings.OuterRes, MiddleRes), and stops the following
// (Still).
func (p *Plugin) SetLayerResolutions(outer, middle float32) {
	p.Res.Settings.OuterRes = clamped(outer)
	p.Res.Settings.MiddleRes = clamped(middle)
	p.still()
	p.redress()
}

// LayerResolutions are the outer and the middle layer's resolutions.
func (p *Plugin) LayerResolutions() (outer, middle float32) {
	return p.Res.Settings.OuterRes, p.Res.Settings.MiddleRes
}

// SetBudget sets the time a frame may take, which the resolution follows
// (Settings.FrameBudget); zero or less, a sixtieth of a second.
func (p *Plugin) SetBudget(d time.Duration) {
	if d <= 0 {
		d = time.Second / 60
	}
	p.Res.Settings.FrameBudget = d
	p.adapter.budget = d
}

// Budget is the time a frame may take, which the resolution follows.
func (p *Plugin) Budget() time.Duration { return p.Res.Settings.FrameBudget }

// ShowLayers shows the layers' edges over every picture drawn in layers, or hides them
// (Settings.ShowLayers).
func (p *Plugin) ShowLayers(shown bool) { p.Res.Settings.ShowLayers = shown }

// LayersShown reports whether the layers' edges are shown.
func (p *Plugin) LayersShown() bool { return p.Res.Settings.ShowLayers }

// still stops the following: a hand on a knob the adapter turns
func (p *Plugin) still() {
	p.Res.Settings.Strategy = Still
	p.adapter.on = false
}

func (p *Plugin) redress() {
	for _, cam := range p.cameras {
		p.dress(cam)
	}
}

// layer has cam drawn as the Settings say, in their layers or whole at their resolution, its
// figures at every pixel or not, a minimap's as it is
func (p *Plugin) dress(cam camera.Camera) {
	if fitsWhole(cam) {
		return
	}
	s := &p.Res.Settings
	if c, ok := cam.(camera.Layered); ok {
		var layers []camera.Layer
		if s.Layers {
			layers = []camera.Layer{
				{Share: 1, Resolution: s.OuterRes},
				{Share: s.Middle, Resolution: s.MiddleRes},
				{Share: s.Inner, Resolution: s.Resolution},
			}
		}
		c.SetLayers(layers)
	}
	if r, ok := cam.(camera.Resolved); ok {
		r.SetResolution(s.Resolution)
	}
	if f, ok := cam.(camera.Figured); ok {
		f.SetSharpFigures(!s.SoftFigures)
	}
}

// lead is the camera whose picture the Settings mirror, the first but a minimap's; nil without
func (p *Plugin) lead() camera.Camera {
	for _, cam := range p.cameras {
		if _, ok := cam.(camera.Resolved); ok && !fitsWhole(cam) {
			return cam
		}
	}
	return nil
}

// note copies the resolutions cam is drawn at into the Settings where cam is the lead camera
func (p *Plugin) note(cam camera.Camera) {
	if cam == nil || cam != p.lead() {
		return
	}
	s := &p.Res.Settings
	if layers := camera.LayersOf(cam); len(layers) == 3 {
		s.OuterRes = layers[0].Resolution
		s.MiddleRes = layers[1].Resolution
		s.Resolution = layers[2].Resolution
		return
	}
	s.Resolution = camera.ResolutionOf(cam)
}

// budgetRate is the frames a second budget allows, whole
func budgetRate(budget time.Duration) float64 {
	return math.Round(float64(time.Second) / float64(budget))
}

// Settings are the scroll zones and the picture's: the strategy following the frame budget, the
// resolution, the figures at every pixel, the budget, the layers and their edges shown
// (plugin.Tunable).
func (p *Plugin) Settings() []control.Setting {
	s := &p.Res.Settings
	hundredths := func(v float64) string { return fmt.Sprintf("%.2f", v) }
	percent := func(v float64) string { return fmt.Sprintf("%.0f%%", v) }
	return []control.Setting{
		control.Switch{
			Label: "Scroll zones",
			Get:   p.ZonesShown,
			Set:   func(on bool) command.Command { return Zones{Shown: on} },
		},
		control.Choice{
			Label:   "Follows the frame budget",
			Section: control.PictureResolution,
			Options: []string{"no", "the whole picture", "in layers"},
			Get:     func() int { return int(s.Strategy) },
			Set:     func(i int) command.Command { return Adapt{Strategy: Strategy(i)} },
		},
		control.Range{
			Label:   "Resolution",
			Section: control.PictureResolution,
			Min:     LeastResolution,
			Max:     1,
			Step:    0.05,
			Get:     func() float64 { return float64(s.Resolution) },
			Set:     func(v float64) command.Command { return Picture{Resolution: float32(v)} },
			Shows:   hundredths,
		},
		control.Switch{
			Label:   "Figures at every pixel",
			Section: control.PictureResolution,
			Get:     p.SharpFigures,
			Set:     func(on bool) command.Command { return SharpFigures{On: on} },
		},
		control.Range{
			Label:   "Frame budget",
			Section: control.PictureAdaptive,
			Min:     30,
			Max:     120,
			Step:    5,
			Get:     func() float64 { return budgetRate(s.FrameBudget) },
			Set: func(v float64) command.Command {
				return Budget{Frame: time.Second / time.Duration(v)}
			},
			Shows: func(v float64) string { return fmt.Sprintf("%.0f FPS", v) },
		},
		control.Switch{
			Label:   "Drawn in layers",
			Section: control.PictureLayers,
			Get:     p.Layers,
			Set:     func(on bool) command.Command { return Layers{On: on} },
		},
		control.Range{
			Label:   "Inner layer",
			Section: control.PictureLayers,
			Min:     20,
			Max:     60,
			Step:    5,
			Get:     func() float64 { return math.Round(float64(s.Inner) * 100) },
			Set: func(v float64) command.Command {
				inner := float32(v / 100)
				return LayerShares{Inner: inner, Middle: max(s.Middle, float32(v/100+0.1))}
			},
			Shows: percent,
		},
		control.Range{
			Label:   "Middle layer",
			Section: control.PictureLayers,
			Min:     40,
			Max:     90,
			Step:    5,
			Get:     func() float64 { return math.Round(float64(s.Middle) * 100) },
			Set: func(v float64) command.Command {
				return LayerShares{Inner: min(s.Inner, float32(v/100-0.1)), Middle: float32(v / 100)}
			},
			Shows: percent,
		},
		control.Range{
			Label:   "Outer layer resolution",
			Section: control.PictureLayers,
			Min:     LeastResolution,
			Max:     1,
			Step:    0.05,
			Get:     func() float64 { return float64(s.OuterRes) },
			Set: func(v float64) command.Command {
				return LayerResolutions{Outer: float32(v), Middle: s.MiddleRes}
			},
			Shows: hundredths,
		},
		control.Range{
			Label:   "Middle layer resolution",
			Section: control.PictureLayers,
			Min:     LeastResolution,
			Max:     1,
			Step:    0.05,
			Get:     func() float64 { return float64(s.MiddleRes) },
			Set: func(v float64) command.Command {
				return LayerResolutions{Outer: s.OuterRes, Middle: float32(v)}
			},
			Shows: hundredths,
		},
		control.Switch{
			Label:   "Show the layers",
			Section: control.PictureAreas,
			Get:     p.LayersShown,
			Set:     func(on bool) command.Command { return ShowLayers{Shown: on} },
		},
	}
}
