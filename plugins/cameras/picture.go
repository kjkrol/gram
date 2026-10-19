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
// picture's, or ring by ring (Settings.Strategy).
type Strategy uint8

const (
	// Still keeps the resolution as set.
	Still Strategy = iota
	// Uniform steps the whole picture's resolution.
	Uniform
	// InRings has the picture drawn in rings and steps them, the outer rings first.
	InRings
)

// Settings is what the cameras let the players adjust, saved with the game: the scroll zones,
// the picture's resolution and the Strategy following the frame budget, the rings it is drawn
// in and whether their edges are shown.
type Settings struct {
	Zones       bool
	Strategy    Strategy
	Resolution  float32 // of the pixels, the centre ring's in rings
	Rings       bool
	Inner       float32 // the inner ring's share of the frame
	Middle      float32 // the middle ring's
	OuterRes    float32 // the outer ring's resolution
	MiddleRes   float32 // the middle ring's
	FrameBudget time.Duration
	ShowRings   bool
}

// Resources is what the cameras publish: their Settings.
type Resources struct {
	Settings Settings
}

// SetStrategy has every picture's resolution follow the frame budget as s says, or stay
// (Settings.Strategy): the pictures drawn in rings under InRings, whole under Uniform.
func (p *Plugin) SetStrategy(s Strategy) {
	p.Res.Settings.Strategy = s
	if s != Still {
		p.Res.Settings.Rings = s == InRings
	}
	p.adapter.on = s != Still
	p.rering()
}

// Strategy is how the pictures' resolution follows the frame budget.
func (p *Plugin) Strategy() Strategy { return p.Res.Settings.Strategy }

// SetRings has every picture but a minimap's drawn in rings, or whole (Settings.Rings); a
// Strategy saying otherwise stops (Still).
func (p *Plugin) SetRings(on bool) {
	p.Res.Settings.Rings = on
	if s := p.Res.Settings.Strategy; s == InRings && !on || s == Uniform && on {
		p.still()
	}
	p.rering()
}

// Rings reports whether the pictures are drawn in rings.
func (p *Plugin) Rings() bool { return p.Res.Settings.Rings }

// SetResolution has every picture but a minimap's drawn at r of its pixels, the centre ring's
// in rings, LeastResolution up to 1 (Settings.Resolution), and stops the following (Still).
func (p *Plugin) SetResolution(r float32) {
	p.Res.Settings.Resolution = clamped(r)
	p.still()
	p.rering()
}

// Resolution is the share of its pixels every picture is drawn at, the centre ring's in rings.
func (p *Plugin) Resolution() float32 { return p.Res.Settings.Resolution }

// SetRingShares has the inner ring cover inner of the frame and the middle ring middle, at
// least a tenth over the inner (Settings.Inner, Middle).
func (p *Plugin) SetRingShares(inner, middle float32) {
	inner = min(max(inner, 0.1), 0.8)
	p.Res.Settings.Inner = inner
	p.Res.Settings.Middle = min(max(middle, inner+0.1), 0.9)
	p.rering()
}

// RingShares are the inner and the middle ring's shares of the frame.
func (p *Plugin) RingShares() (inner, middle float32) {
	return p.Res.Settings.Inner, p.Res.Settings.Middle
}

// SetRingResolutions has the outer ring drawn at outer of its pixels and the middle ring at
// middle, LeastResolution up to 1 (Settings.OuterRes, MiddleRes), and stops the following
// (Still).
func (p *Plugin) SetRingResolutions(outer, middle float32) {
	p.Res.Settings.OuterRes = clamped(outer)
	p.Res.Settings.MiddleRes = clamped(middle)
	p.still()
	p.rering()
}

// RingResolutions are the outer and the middle ring's resolutions.
func (p *Plugin) RingResolutions() (outer, middle float32) {
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

// ShowRings shows the rings' edges over every picture drawn in rings, or hides them
// (Settings.ShowRings).
func (p *Plugin) ShowRings(shown bool) { p.Res.Settings.ShowRings = shown }

// RingsShown reports whether the rings' edges are shown.
func (p *Plugin) RingsShown() bool { return p.Res.Settings.ShowRings }

// still stops the following: a hand on a knob the adapter turns
func (p *Plugin) still() {
	p.Res.Settings.Strategy = Still
	p.adapter.on = false
}

func (p *Plugin) rering() {
	for _, cam := range p.cameras {
		p.ring(cam)
	}
}

// ring has cam drawn as the Settings say, in their rings or whole at their resolution, a
// minimap's as it is
func (p *Plugin) ring(cam camera.Camera) {
	if fitsWhole(cam) {
		return
	}
	s := &p.Res.Settings
	if c, ok := cam.(camera.Ringed); ok {
		var rings []camera.Ring
		if s.Rings {
			rings = []camera.Ring{
				{Share: 1, Resolution: s.OuterRes},
				{Share: s.Middle, Resolution: s.MiddleRes},
				{Share: s.Inner, Resolution: s.Resolution},
			}
		}
		c.SetRings(rings)
	}
	if r, ok := cam.(camera.Resolved); ok {
		r.SetResolution(s.Resolution)
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
	if rings := camera.RingsOf(cam); len(rings) == 3 {
		s.OuterRes = rings[0].Resolution
		s.MiddleRes = rings[1].Resolution
		s.Resolution = rings[2].Resolution
		return
	}
	s.Resolution = camera.ResolutionOf(cam)
}

// budgetRate is the frames a second budget allows, whole
func budgetRate(budget time.Duration) float64 {
	return math.Round(float64(time.Second) / float64(budget))
}

// Settings are the scroll zones and the picture's: the strategy following the frame budget, the
// resolution, the budget, the rings and their edges shown (plugin.Tunable).
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
			Label:   "Resolution follows the frame budget",
			Section: control.PictureResolution,
			Options: []string{"no", "the whole picture", "in rings"},
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
			Label:   "Drawn in rings",
			Section: control.PictureRings,
			Get:     p.Rings,
			Set:     func(on bool) command.Command { return Rings{On: on} },
		},
		control.Range{
			Label:   "Inner ring",
			Section: control.PictureRings,
			Min:     20,
			Max:     60,
			Step:    5,
			Get:     func() float64 { return math.Round(float64(s.Inner) * 100) },
			Set: func(v float64) command.Command {
				inner := float32(v / 100)
				return RingShares{Inner: inner, Middle: max(s.Middle, float32(v/100+0.1))}
			},
			Shows: percent,
		},
		control.Range{
			Label:   "Middle ring",
			Section: control.PictureRings,
			Min:     40,
			Max:     90,
			Step:    5,
			Get:     func() float64 { return math.Round(float64(s.Middle) * 100) },
			Set: func(v float64) command.Command {
				return RingShares{Inner: min(s.Inner, float32(v/100-0.1)), Middle: float32(v / 100)}
			},
			Shows: percent,
		},
		control.Range{
			Label:   "Outer ring resolution",
			Section: control.PictureRings,
			Min:     LeastResolution,
			Max:     1,
			Step:    0.05,
			Get:     func() float64 { return float64(s.OuterRes) },
			Set: func(v float64) command.Command {
				return RingResolutions{Outer: float32(v), Middle: s.MiddleRes}
			},
			Shows: hundredths,
		},
		control.Range{
			Label:   "Middle ring resolution",
			Section: control.PictureRings,
			Min:     LeastResolution,
			Max:     1,
			Step:    0.05,
			Get:     func() float64 { return float64(s.MiddleRes) },
			Set: func(v float64) command.Command {
				return RingResolutions{Outer: s.OuterRes, Middle: float32(v)}
			},
			Shows: hundredths,
		},
		control.Switch{
			Label:   "Show the rings",
			Section: control.PictureAreas,
			Get:     p.RingsShown,
			Set:     func(on bool) command.Command { return ShowRings{Shown: on} },
		},
	}
}
