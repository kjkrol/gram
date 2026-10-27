package cameras

import (
	"cmp"
	"fmt"
	"math"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/cameras/internal/stick"
)

// Arm is how a camera is held behind an entity (camera.Behind): on an arm Distance cells long,
// tilted Pitch degrees over the ground in perspective, aimed Aim of the entity's height over
// its feet (0 its feet, 1 its top; the higher, the lower the entity on the screen); Tilts lets
// the player tilt it with the mouse, Zooms draw it in and out with the zoom keys and the wheel.
// Zero: DefaultArm, nothing the player's.
type Arm struct {
	Pitch, Distance, Aim float32
	Tilts, Zooms         bool
}

// DefaultArm is the arm a zero Config.Arm gives: 30 degrees on two cells, aimed at the feet.
var DefaultArm = Arm{Pitch: 30, Distance: 2}

// LeastPitch and MostPitch are the degrees the arm tilts between, LeastArm and MostArm the cells
// it is drawn between, MostAim the entity's heights it is aimed up to at most.
const (
	LeastPitch = 5
	MostPitch  = 89
	LeastArm   = 0.5
	MostArm    = 8
	MostAim    = 3
)

// filled is the arm with its zero lengths and angles the default's
func (a Arm) filled() Arm {
	a.Pitch = cmp.Or(a.Pitch, DefaultArm.Pitch)
	a.Distance = cmp.Or(a.Distance, DefaultArm.Distance)
	return a
}

func (a Arm) bounded() Arm {
	a.Pitch = min(max(a.Pitch, LeastPitch), MostPitch)
	a.Distance = min(max(a.Distance, LeastArm), MostArm)
	a.Aim = min(max(a.Aim, 0), MostAim)
	return a
}

// Hold sets how the cameras are held behind an entity (Settings.Arm). The settings screen
// alone.
type Hold struct{ Arm Arm }

func (Hold) Command() {}

var _ command.Command = Hold{}

// Arm is how the cameras are held behind an entity.
func (r *Read) Arm() Arm { return r.p.Res.Settings.Arm.filled() }

// SetArm sets how the cameras are held behind an entity (Settings.Arm), within the bounds.
func (w *Write) SetArm(a Arm) { w.p.Res.Settings.Arm = a.filled().bounded() }

// arms hands the keeper the arm in radians and takes the mouse's tilt
type arms struct{ p *Plugin }

func (a arms) Arm() stick.Arm {
	s := a.p.read.Arm()
	return stick.Arm{
		Pitch:    s.Pitch * math.Pi / 180,
		Distance: s.Distance,
		Aim:      s.Aim,
		Tilts:    s.Tilts,
	}
}

func (a arms) Tilt(pitch float32) {
	s := a.p.read.Arm()
	s.Pitch = pitch * 180 / math.Pi
	a.p.write.SetArm(s)
}

// armSettings are the arm's rows of the settings screen: the mouse's tilt with the pitch
// beside, the zoom keys' draw with the distance beside, the aim
func (p *Plugin) armSettings() []control.Setting {
	hold := func(change func(a *Arm)) command.Command {
		a := p.read.Arm()
		change(&a)
		return Hold{Arm: a}
	}
	return []control.Setting{
		control.Switch{
			Label: "Tilt behind the unit",
			Get:   func() bool { return p.read.Arm().Tilts },
			Set: func(on bool) command.Command {
				return hold(func(a *Arm) { a.Tilts = on })
			},
		},
		control.Range{
			Label:  "Pitch behind the unit",
			Beside: "Tilt behind the unit",
			Min:    LeastPitch,
			Max:    MostPitch,
			Step:   1,
			Get:    func() float64 { return math.Round(float64(p.read.Arm().Pitch)) },
			Set: func(v float64) command.Command {
				return hold(func(a *Arm) { a.Pitch = float32(v) })
			},
			Shows: func(v float64) string { return fmt.Sprintf("%.0f°", v) },
		},
		control.Switch{
			Label: "Zoom behind the unit",
			Get:   func() bool { return p.read.Arm().Zooms },
			Set: func(on bool) command.Command {
				return hold(func(a *Arm) { a.Zooms = on })
			},
		},
		control.Range{
			Label:  "Distance behind the unit",
			Beside: "Zoom behind the unit",
			Min:    LeastArm,
			Max:    MostArm,
			Step:   0.5,
			Get:    func() float64 { return float64(p.read.Arm().Distance) },
			Set: func(v float64) command.Command {
				return hold(func(a *Arm) { a.Distance = float32(v) })
			},
			Shows: func(v float64) string { return fmt.Sprintf("%.1f cells", v) },
		},
		control.Range{
			Label: "Aim behind the unit",
			Min:   0,
			Max:   MostAim,
			Step:  0.25,
			Get:   func() float64 { return float64(p.read.Arm().Aim) },
			Set: func(v float64) command.Command {
				return hold(func(a *Arm) { a.Aim = float32(v) })
			},
			Shows: func(v float64) string { return fmt.Sprintf("%.2f of its height", v) },
		},
	}
}
