package world

import (
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/world/internal/sectorview"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
)

// SwitchSectors has Shown drawn of the sectors (Settings.Sectors): their bands, how finely the
// camera sees them, nothing. The settings screen alone, in a world cut into sectors.
type SwitchSectors struct{ Shown SectorsShown }

func (SwitchSectors) Command() {}

var _ command.Command = SwitchSectors{}

// SectorsShown is what is drawn of the world's sectors over the ground.
type SectorsShown uint8

const (
	NoSectors SectorsShown = iota
	// SectorBands colours each sector by its band, those beyond the last apart.
	SectorBands
	// SectorsSeen colours each sector by how finely the camera drawing sees it (view.Level).
	SectorsSeen
)

// ShowSectors has the sectors drawn as shown says (Settings.Sectors): a look for finding out
// what lives and what is seen where.
func (p *Plugin) ShowSectors(shown SectorsShown) {
	p.Res.Settings.Sectors = shown
	p.module.sectorsShown = shown
}

// SectorsShown is what is drawn of the sectors.
func (p *Plugin) SectorsShown() SectorsShown { return p.Res.Settings.Sectors }

var (
	// the bands nearest first, then beyond the last
	bandColors = [sector.MaxBands + 1]color.RGBA{
		{R: 70, G: 200, B: 90, A: 255},
		{R: 235, G: 200, B: 60, A: 255},
		{R: 240, G: 140, B: 50, A: 255},
		{R: 220, G: 80, B: 170, A: 255},
		{R: 150, G: 90, B: 220, A: 255},
		{R: 210, G: 50, B: 50, A: 255},
	}
	// by view.Level: Hidden, Exact, Coarse, Fogged; apart from the bands' colours
	levelColors = [4]color.RGBA{
		{R: 210, G: 50, B: 50, A: 255},
		{R: 240, G: 240, B: 240, A: 255},
		{R: 60, G: 120, B: 240, A: 255},
		{R: 120, G: 120, B: 130, A: 255},
	}
	sectorLine = sectorview.Line{Width: 2, Inside: 0.22, Edge: 0.85}
)

// how far over the ground the sectors lie: over the cells held, under the sight
const sectorLift = 0.2

// sectorsView draws the sectors as SectorsShown says: over the ground the frame drew on the GPU
// in a world with heights, as quads at the ground's level in a flat one.
type sectorsView struct {
	p       *Plugin
	gpu     *sectorview.Ground
	onGPU   bool
	colors  []color.RGBA
	levels  []view.Level
	corners []render.Corners
}

func newSectorsView(p *Plugin) *sectorsView {
	v := &sectorsView{p: p}
	g := p.module.sectors.grid
	if g == nil {
		return v
	}
	space := p.Res.Config.Space
	v.gpu = sectorview.New(sectorview.Grid{
		Side:   g.Side(),
		Cols:   g.Cols(),
		Rows:   g.Rows(),
		Width:  float64(space.Width),
		Height: float64(space.Height),
		WrapX:  space.Edges.WrapsX(),
		WrapY:  space.Edges.WrapsY(),
	})
	return v
}

func (v *sectorsView) Init(*goke.SysInit) {}

func (v *sectorsView) Compose(f *render.Frame, cam camera.Camera) {
	v.onGPU = false
	shown, g := v.p.module.sectorsShown, v.p.module.sectors.grid
	if g == nil || shown == NoSectors {
		return
	}
	v.colors = v.colors[:0]
	if shown == SectorBands {
		for i := range g.Len() {
			band := g.Band(i)
			if band == g.Beyond() {
				band = sector.MaxBands
			}
			v.colors = append(v.colors, bandColors[band])
		}
	} else {
		v.levels = append(v.levels[:0], make([]view.Level, g.Len())...)
		v.p.ViewFor(cam).Sectors(func(i int, _ geom.AABB, l view.Level) { v.levels[i] = l })
		for _, l := range v.levels {
			v.colors = append(v.colors, levelColors[l])
		}
	}
	if _, rays := cam.(camera.Rays); rays && v.p.HasHeights() {
		v.onGPU = true
		v.gpu.Paint(cam, v.colors)
		f.ReadGround() // the sectors find the ground under them
		f.DrawAt(f.OnGround(sectorLift))
		return
	}
	w, h := cam.Viewport()
	look := v.p.Look()
	for i, c := range v.colors {
		box := g.Box(i)
		midX := (box.TopLeft.X + box.BottomRight.X) / 2
		midY := (box.TopLeft.Y + box.BottomRight.Y) / 2
		at := f.Lifted(float32(midX), float32(midY), 0, sectorLift)
		inside := c
		inside.A = uint8(float32(c.A) * sectorLine.Inside)
		edge := c
		edge.A = uint8(float32(c.A) * sectorLine.Edge)
		v.corners = look.Footprint(cam, box, 0, v.corners[:0])
		for _, q := range v.corners {
			if !onScreen(q, w, h) {
				continue
			}
			f.Quad(at, q, [4]color.RGBA{inside, inside, inside, inside})
			for _, e := range [4][2]int{{0, 1}, {1, 3}, {3, 2}, {2, 0}} {
				a, b := q[e[0]], q[e[1]]
				f.Line(at, a[0], a[1], b[0], b[1], sectorLine.Width, edge)
			}
		}
	}
}

// whether q reaches into a viewport w x h
func onScreen(q render.Corners, w, h float32) bool {
	x0 := min(q[0][0], q[1][0], q[2][0], q[3][0])
	x1 := max(q[0][0], q[1][0], q[2][0], q[3][0])
	y0 := min(q[0][1], q[1][1], q[2][1], q[3][1])
	y1 := max(q[0][1], q[1][1], q[2][1], q[3][1])
	return x1 > 0 && x0 < w && y1 > 0 && y0 < h
}

func (v *sectorsView) Draw(t render.Target, cam camera.Camera, _ render.Uniforms) {
	if v.onGPU {
		v.gpu.Draw(t, cam, sectorLine)
	}
}

func (w *module) switchSectors() {
	w.sectorSwitches.Drain(func(i command.Issued[SwitchSectors]) {
		w.p.ShowSectors(i.Command.Shown)
	})
}
