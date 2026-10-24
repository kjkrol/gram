package cameras

import (
	"cmp"
	"fmt"
	"math"
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/render"
)

// Strategy is how the pictures' resolution follows the frame budget: not at all, the whole
// picture's, or layer by layer (Knobs.Strategy).
type Strategy uint8

const (
	// Still keeps the resolution as set.
	Still Strategy = iota
	// Uniform steps the whole picture's resolution.
	Uniform
	// InLayers has the picture drawn in layers and steps them, the outer layers first.
	InLayers
)

// Profile is which Knobs a picture is drawn by, as its camera's state says (ProfileOf).
type Profile uint8

const (
	// Tactical is the profile of a camera free, over or behind an entity, in any view.
	Tactical Profile = iota
	// Riding is the profile of a camera inside an entity, in first person (camera.Inside).
	Riding
)

// ProfileOf is the profile cam's picture is drawn by: Riding inside an entity, Tactical else.
func ProfileOf(cam camera.Camera) Profile {
	if camera.HowOf(cam) == camera.Inside {
		return Riding
	}
	return Tactical
}

func (p Profile) String() string {
	if p == Riding {
		return "riding"
	}
	return "tactical"
}

// Knobs are how the pictures of a profile are drawn: the Strategy following the frame budget,
// the resolution, the figures at every pixel or not, and the layers.
type Knobs struct {
	Strategy   Strategy
	Resolution float32 // of the pixels, the centre layer's in layers; zero, all of them
	// the figures at the ground's resolution; zero, at every pixel
	SoftFigures bool
	Layers      bool
	// the inner and the middle layer's shares of the frame (zero, 0.35 and 0.6), the outer and
	// the middle layer's resolutions (zero, 0.6 and 0.8)
	Inner     float32
	Middle    float32
	OuterRes  float32
	MiddleRes float32
	// the layers' edges outlined over the picture
	ShowLayers bool
}

// filled is k with the defaults in its zero floats, which a save from before the profiles leaves
// zero, and its resolutions LeastResolution up to 1
func (k Knobs) filled() Knobs {
	k.Resolution = clamped(cmp.Or(k.Resolution, 1))
	k.Inner = cmp.Or(k.Inner, 0.35)
	k.Middle = cmp.Or(k.Middle, 0.6)
	k.OuterRes = clamped(cmp.Or(k.OuterRes, 0.6))
	k.MiddleRes = clamped(cmp.Or(k.MiddleRes, 0.8))
	return k
}

// Settings is what the cameras let the players adjust, saved with the game: the scroll zones,
// the frame budget and the Knobs of each Profile.
type Settings struct {
	Zones       bool
	StillEdges  bool    // the cursor at a picture's edge scrolls nothing
	EdgeSpeed   float32 // screen pixels a tick at the very edge; zero, the keys' own
	FrameBudget time.Duration
	Sampling    time.Duration // the frame rate averaged over this before it is followed
	Under       float32       // of the budget's rate, under which the picture goes coarser
	Met         float32       // over which it goes finer
	Tactical    Knobs
	Riding      Knobs
}

// knobs are profile's.
func (s *Settings) knobs(profile Profile) *Knobs {
	if profile == Riding {
		return &s.Riding
	}
	return &s.Tactical
}

// drawnSettings are the cameras' settings as the frame drawn holds them, the live ones before any
// was published
type drawnSettings struct {
	live    *Settings
	buf     render.Buffered[Settings]
	swapped bool
}

// Publish hands the frame drawn next the settings as they stand.
func (d *drawnSettings) Publish() { *d.buf.Back() = *d.live }

// Swap has the settings published last drawn.
func (d *drawnSettings) Swap() {
	d.buf.Swap()
	d.swapped = true
}

func (d *drawnSettings) drawn() *Settings {
	if !d.swapped {
		return d.live
	}
	return d.buf.Front()
}

// Resources is what the cameras publish: their Settings.
type Resources struct {
	Settings Settings
}

func (p *Plugin) knobs(profile Profile) *Knobs { return p.Res.Settings.knobs(profile) }

// still stops profile's following: a hand on a knob the adapter turns
func (p *Plugin) still(profile Profile) {
	p.knobs(profile).Strategy = Still
}

func (p *Plugin) anyFollowing() bool {
	s := &p.Res.Settings
	return s.Tactical.Strategy != Still || s.Riding.Strategy != Still
}

func (p *Plugin) redress() {
	for _, cam := range p.cameras {
		p.dress(cam)
	}
}

// what a camera was dressed for: its profile and whether it had an eye
type dressed struct {
	profile     Profile
	perspective bool
}

func dressFor(cam camera.Camera) dressed {
	d := dressed{profile: ProfileOf(cam)}
	if e, ok := cam.(camera.Eyed); ok {
		_, _, _, d.perspective = e.Eye()
	}
	return d
}

// dress has cam drawn as its profile's Knobs say, in their layers or whole at their resolution,
// its figures at every pixel or not, the layers on the frame's bottom edge in a tactical
// perspective, a minimap's as it is. A Pan, CenterOn or Follow{On: false} given a camera riding
// (no key of the plugin's) leaves it dressed tactical inside its entity for a tick.
func (p *Plugin) dress(cam camera.Camera) {
	if fitsWhole(cam) {
		return
	}
	d := dressFor(cam)
	p.worn[cam] = d
	k := p.knobs(d.profile)
	if c, ok := cam.(camera.Layered); ok {
		var layers []camera.Layer
		if k.Layers {
			layers = []camera.Layer{
				{Share: 1, Resolution: k.OuterRes},
				{Share: k.Middle, Resolution: k.MiddleRes},
				{Share: k.Inner, Resolution: k.Resolution},
			}
		}
		layout := camera.Inset
		if d == (dressed{profile: Tactical, perspective: true}) {
			layout = camera.Footed
		}
		c.SetLayers(layers, layout)
	}
	if r, ok := cam.(camera.Resolved); ok {
		r.SetResolution(k.Resolution)
	}
	if f, ok := cam.(camera.Figured); ok {
		f.SetSharpFigures(!k.SoftFigures)
	}
}

// redressChanged dresses again every camera whose state is no longer what it was dressed for
func (p *Plugin) redressChanged() {
	for _, cam := range p.cameras {
		if !fitsWhole(cam) && p.worn[cam] != dressFor(cam) {
			p.dress(cam)
		}
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

// note copies the resolutions cam is drawn at into its profile's Knobs where cam is the lead
// camera
func (p *Plugin) note(cam camera.Camera) {
	if cam == nil || cam != p.lead() {
		return
	}
	k := p.knobs(ProfileOf(cam))
	if layers := camera.LayersOf(cam); len(layers) == 3 {
		k.OuterRes = layers[0].Resolution
		k.MiddleRes = layers[1].Resolution
		k.Resolution = layers[2].Resolution
		return
	}
	k.Resolution = camera.ResolutionOf(cam)
}

// budgetRate is the frames a second budget allows, whole
func budgetRate(budget time.Duration) float64 {
	return math.Round(float64(time.Second) / float64(budget))
}

// Settings are the scroll zones (scrolling, shown, their speed), the frame budget and each
// profile's knobs (the strategy following the budget, the resolution, the figures at every
// pixel, the layers and their edges shown) (plugin.Tunable).
func (p *Plugin) Settings() []control.Setting {
	out := []control.Setting{
		control.Switch{
			Label: "Scroll zones",
			Get:   p.read.EdgesScroll,
			Set:   func(on bool) command.Command { return EdgeScroll{On: on} },
		},
		control.Switch{
			Label: "Show the zones",
			Get:   p.read.ZonesShown,
			Set:   func(on bool) command.Command { return Zones{Shown: on} },
		},
		control.Range{
			Label: "Edge scroll speed",
			Min:   2,
			Max:   24,
			Step:  1,
			Get: func() float64 {
				if s := p.read.EdgeSpeed(); s > 0 {
					return float64(s)
				}
				return DefaultScrollSpeed
			},
			Set:   func(v float64) command.Command { return EdgeSpeed{Speed: float32(v)} },
			Shows: func(v float64) string { return fmt.Sprintf("%.0f px a tick", v) },
		},
		control.Range{
			Label:   "Frame budget",
			Section: control.Picture,
			Min:     30,
			Max:     120,
			Step:    5,
			Get:     func() float64 { return budgetRate(p.Res.Settings.FrameBudget) },
			Set: func(v float64) command.Command {
				return Budget{Frame: time.Second / time.Duration(v)}
			},
			Shows: func(v float64) string { return fmt.Sprintf("%.0f FPS", v) },
		},
		control.Range{
			Label:   "Samples the frame rate over",
			Section: control.Picture,
			Min:     LeastSampling.Seconds(),
			Max:     MostSampling.Seconds(),
			Step:    1,
			Get:     func() float64 { return p.Res.Settings.Sampling.Seconds() },
			Set: func(v float64) command.Command {
				return Sampling{Over: time.Duration(v * float64(time.Second))}
			},
			Shows: func(v float64) string { return fmt.Sprintf("%.0f s", v) },
		},
		control.Range{
			Label:   "Coarser under",
			Section: control.Picture,
			Min:     50,
			Max:     100,
			Step:    1,
			Get:     func() float64 { return math.Round(float64(p.Res.Settings.Under) * 100) },
			Set: func(v float64) command.Command {
				return Thresholds{Under: float32(v / 100), Met: p.Res.Settings.Met}
			},
			Shows: percent,
		},
		control.Range{
			Label:   "Finer over",
			Section: control.Picture,
			Min:     50,
			Max:     100,
			Step:    1,
			Get:     func() float64 { return math.Round(float64(p.Res.Settings.Met) * 100) },
			Set: func(v float64) command.Command {
				return Thresholds{Under: p.Res.Settings.Under, Met: float32(v / 100)}
			},
			Shows: percent,
		},
	}
	out = append(out,
		p.knobSettings(Tactical, control.PictureTactical, control.PictureTacticalLayers)...)
	return append(out,
		p.knobSettings(Riding, control.PictureRiding, control.PictureRidingLayers)...)
}

// knobSettings are profile's knobs, the layers' under layersSection, the rest under section
func (p *Plugin) knobSettings(profile Profile, section, layersSection string) []control.Setting {
	k := p.knobs(profile)
	hundredths := func(v float64) string { return fmt.Sprintf("%.2f", v) }
	return []control.Setting{
		control.Choice{
			Label:   "Follows the frame budget",
			Section: section,
			Options: []string{"no", "the whole picture", "in layers"},
			Get:     func() int { return int(k.Strategy) },
			Set: func(i int) command.Command {
				return Adapt{Profile: profile, Strategy: Strategy(i)}
			},
		},
		control.Range{
			Label:   "Resolution",
			Section: section,
			Min:     LeastResolution,
			Max:     1,
			Step:    0.05,
			Get:     func() float64 { return float64(k.Resolution) },
			Set: func(v float64) command.Command {
				return Picture{Profile: profile, Resolution: float32(v)}
			},
			Shows: hundredths,
		},
		control.Switch{
			Label:   "Figures at every pixel",
			Section: section,
			Get:     func() bool { return !k.SoftFigures },
			Set: func(on bool) command.Command {
				return SharpFigures{Profile: profile, On: on}
			},
		},
		control.Switch{
			Label:   "Drawn in layers",
			Section: layersSection,
			Get:     func() bool { return k.Layers },
			Set:     func(on bool) command.Command { return Layers{Profile: profile, On: on} },
		},
		control.Range{
			Label:   "Inner layer",
			Section: layersSection,
			Min:     20,
			Max:     60,
			Step:    5,
			Get:     func() float64 { return math.Round(float64(k.Inner) * 100) },
			Set: func(v float64) command.Command {
				return LayerShares{
					Profile: profile,
					Inner:   float32(v / 100),
					Middle:  max(k.Middle, float32(v/100+0.1)),
				}
			},
			Shows: percent,
		},
		control.Range{
			Label:   "Middle layer",
			Section: layersSection,
			Min:     40,
			Max:     90,
			Step:    5,
			Get:     func() float64 { return math.Round(float64(k.Middle) * 100) },
			Set: func(v float64) command.Command {
				return LayerShares{
					Profile: profile,
					Inner:   min(k.Inner, float32(v/100-0.1)),
					Middle:  float32(v / 100),
				}
			},
			Shows: percent,
		},
		control.Range{
			Label:   "Outer layer resolution",
			Section: layersSection,
			Min:     LeastResolution,
			Max:     1,
			Step:    0.05,
			Get:     func() float64 { return float64(k.OuterRes) },
			Set: func(v float64) command.Command {
				return LayerResolutions{Profile: profile, Outer: float32(v), Middle: k.MiddleRes}
			},
			Shows: hundredths,
		},
		control.Range{
			Label:   "Middle layer resolution",
			Section: layersSection,
			Min:     LeastResolution,
			Max:     1,
			Step:    0.05,
			Get:     func() float64 { return float64(k.MiddleRes) },
			Set: func(v float64) command.Command {
				return LayerResolutions{Profile: profile, Outer: k.OuterRes, Middle: float32(v)}
			},
			Shows: hundredths,
		},
		control.Switch{
			Label:   "Show the layers",
			Section: layersSection,
			Get:     func() bool { return k.ShowLayers },
			Set: func(on bool) command.Command {
				return ShowLayers{Profile: profile, Shown: on}
			},
		},
	}
}

// percent writes v as a whole percentage, "60%"
func percent(v float64) string { return fmt.Sprintf("%.0f%%", v) }
