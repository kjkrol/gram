package collision

import (
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Boxes has Shown drawn of every entity's box (Settings.Boxes): a look for finding out why things
// touch or miss. Shift+X by default, in turn the bases, the solids and none.
type Boxes struct{ Shown BoxesShown }

func (Boxes) Command() {}

var _ command.Command = Boxes{}

// Settings is what collision lets the players adjust, saved with the game.
type Settings struct {
	Boxes BoxesShown
}

// Resources is what collision publishes: its Settings.
type Resources struct {
	Settings Settings
}

// BoxesShown is what is drawn of the entities' boxes.
type BoxesShown uint8

const (
	// NoBoxes draws none.
	NoBoxes BoxesShown = iota
	// Bases outlines every entity's box where it stands, under its figure.
	Bases
	// Solids draws the edges of every entity's box from its altitude to its top, over every
	// figure and under the ground, in a world with heights.
	Solids
)

// Queues are where Boxes lands.
func (p *Plugin) Queues() []command.CommandQueue { return []command.CommandQueue{&p.boxes} }

// DefaultBindings are Shift+X switching what is drawn of the entities' boxes.
func (p *Plugin) DefaultBindings() []control.Binding {
	return []control.Binding{
		control.Build(control.KeyPress{Key: control.KeyX, Mods: control.Mods{Shift: true}},
			"Show the entities' boxes: their bases, their solids, none",
			func(control.Context) (Boxes, bool) { return Boxes{Shown: p.next()}, true }),
	}
}

// Settings is the one setting, what is drawn of the entities' boxes (plugin.Tunable).
func (p *Plugin) Settings() []control.Setting {
	options := []string{"none", "bases"}
	if p.worldPlugin.HasHeights() {
		options = append(options, "solids")
	}
	return []control.Setting{control.Choice{
		Label:   "Entities' boxes",
		Options: options,
		Get:     func() int { return int(p.Res.Settings.Boxes) },
		Set:     func(i int) command.Command { return Boxes{Shown: BoxesShown(i)} },
	}}
}

var boxColor = color.RGBA{R: 230, G: 40, B: 40, A: 255}

const boxWidth = 1

const (
	// a box's place among what lies where it stands: after its figure, so with no depth to meet
	// it lies over it
	boxLift = world.FigureLift + 0.5
	// how far over the ground under it a base lies: the ground's own depth does not hide it
	baseRaise = 0.05
)

// boxesView draws every entity's box in sight of the camera as BoxesShown says.
type boxesView struct {
	w         *world.Plugin
	settings  *Settings
	q         *goke.Query
	base      goke.Comp[world.Base]
	z         goke.OptComp[world.Z]
	footprint []render.Corners
}

var _ render.Overlying = (*boxesView)(nil)

func (v *boxesView) Init(si *goke.SysInit) {
	v.q = si.NewQueryBuilder(&v.base).Optional(&v.z).Build()
}

// Overlies: drawn over the figures' ground (render.Overlying).
func (*boxesView) Overlies() {}

func (v *boxesView) Compose(f *render.Frame, cam camera.Camera) {
	shown := v.settings.Boxes
	if shown == NoBoxes {
		return
	}
	in := v.w.ViewFor(cam)
	look := v.w.Look()
	for v.q.All(); v.q.Next(); {
		cur := v.q.Cursor()
		bases, zs := v.base.Slice(cur), v.z.Slice(cur)
		for i, id := range cur.IDs {
			if in != nil && !in.Contains(id) {
				continue
			}
			var z world.Z
			if zs != nil {
				z = zs[i]
			}
			box, mid := bases[i].Pos.AABB, bases[i].Pos.Center()
			alt := float32(z.Altitude)
			at := f.Lifted(float32(mid.X), float32(mid.Y), alt, boxLift)
			if shown == Solids {
				v.solid(f, cam, at.OverFigures(), box.AABB, alt, float32(z.Top()))
				continue
			}
			base := alt + baseRaise
			v.footprint = look.Footprint(cam, box.AABB, base, v.footprint[:0])
			for _, c := range v.footprint {
				for _, e := range [4][2]int{{0, 1}, {1, 3}, {3, 2}, {2, 0}} {
					a, b := c[e[0]], c[e[1]]
					f.Edge(at, a[0], a[1], base, b[0], b[1], base, boxWidth, boxColor)
				}
			}
		}
	}
}

// solid draws the twelve edges of the box from alt up to top
func (v *boxesView) solid(f *render.Frame, cam camera.Camera, at render.At, box geom.AABB,
	alt, top float32) {
	x0, y0 := float32(box.TopLeft.X), float32(box.TopLeft.Y)
	x1, y1 := float32(box.BottomRight.X), float32(box.BottomRight.Y)
	floor := [4][2]float32{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
	var low, high [4][2]float32
	for k, p := range floor {
		low[k][0], low[k][1] = cam.Project(p[0], p[1], alt)
		high[k][0], high[k][1] = cam.Project(p[0], p[1], top)
	}
	for k := range floor {
		n := (k + 1) % len(floor)
		f.Edge(at, low[k][0], low[k][1], alt, low[n][0], low[n][1], alt, boxWidth, boxColor)
		f.Edge(at, high[k][0], high[k][1], top, high[n][0], high[n][1], top, boxWidth, boxColor)
		f.Edge(at, low[k][0], low[k][1], alt, high[k][0], high[k][1], top, boxWidth, boxColor)
	}
}

// ShowBoxes has the entities' boxes drawn as shown says (Settings.Boxes), as the game starts.
func (p *Plugin) ShowBoxes(shown BoxesShown) *Plugin {
	p.Res.Settings.Boxes = shown
	return p
}

// BoxesShown is what is drawn of the entities' boxes.
func (p *Plugin) BoxesShown() BoxesShown { return p.Res.Settings.Boxes }

// next is what Shift+X switches to: the solids only where there are heights to raise them by
func (p *Plugin) next() BoxesShown {
	switch p.Res.Settings.Boxes {
	case NoBoxes:
		return Bases
	case Bases:
		if p.worldPlugin.HasHeights() {
			return Solids
		}
	}
	return NoBoxes
}

func (p *Plugin) switchBoxes() {
	p.boxes.Drain(func(i command.Issued[Boxes]) { p.ShowBoxes(i.Command.Shown) })
}
