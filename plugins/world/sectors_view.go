package world

import (
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/world/internal/sectorview"
	"github.com/kjkrol/gram/render"
)

// SwitchSectors has Shown drawn of the sectors (Settings.Sectors): the levels they are ticked
// at, the levels the camera sees them at, nothing. The settings screen alone, in a world cut
// into sectors.
type SwitchSectors struct{ Shown SectorsShown }

func (SwitchSectors) Command() {}

var _ command.Command = SwitchSectors{}

// SectorsShown is what is drawn of the world's sectors over the ground.
type SectorsShown uint8

const (
	NoSectors SectorsShown = iota
	// SectorBands colours each sector by the level it is ticked at.
	SectorBands
	// SectorsSeen colours each sector by the level the camera drawing sees it at (View.Level).
	SectorsSeen
)

var (
	// by level, L0 first
	levelColors = [lod.Levels]color.RGBA{
		{R: 70, G: 200, B: 90, A: 255},
		{R: 235, G: 200, B: 60, A: 255},
		{R: 240, G: 140, B: 50, A: 255},
		{R: 150, G: 90, B: 220, A: 255},
		{R: 210, G: 50, B: 50, A: 255},
	}
	sectorLine = sectorview.Line{Width: 2, Inside: 0.22, Edge: 0.85}
)

// how far over the ground the sectors lie: over the cells held, under the sight
const sectorLift = 0.2

// sectorsView draws the sectors as SectorsShown says, as published once a frame: over the ground
// the frame drew on the GPU in a world with heights, as quads at the ground's level in a flat one.
type sectorsView struct {
	p       *Plugin
	gpu     *sectorview.Ground
	onGPU   bool
	colors  []color.RGBA
	levels  []lod.Level
	corners []render.Corners
	drawn   render.Buffered[shownSectors]
}

// shownSectors are what is shown of the sectors and, their levels ticked shown, each one's
type shownSectors struct {
	shown  SectorsShown
	ticked []lod.Level
}

// Publish takes what is shown of the sectors and the levels they are ticked at as they stand,
// for the frame drawn next.
func (v *sectorsView) Publish() {
	b, s := v.drawn.Back(), v.p.module.sectors
	b.shown, b.ticked = v.p.module.sectorsShown, b.ticked[:0]
	if s.grid == nil || b.shown != SectorBands {
		return
	}
	b.ticked = append(b.ticked, s.level...)
}

// Swap has what was published last drawn.
func (v *sectorsView) Swap() { v.drawn.Swap() }

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
		Width:  float32(space.Width),
		Height: float32(space.Height),
		WrapX:  space.Edges.WrapsX(),
		WrapY:  space.Edges.WrapsY(),
	})
	return v
}

func (v *sectorsView) Compose(f *render.Frame, cam camera.Camera) {
	v.onGPU = false
	d, g := v.drawn.Front(), v.p.module.sectors.grid
	if g == nil || d.shown == NoSectors {
		return
	}
	v.colors = v.colors[:0]
	if d.shown == SectorBands {
		for _, l := range d.ticked {
			v.colors = append(v.colors, levelColors[l])
		}
	} else {
		if len(v.levels) != g.Len() {
			v.levels = make([]lod.Level, g.Len())
		}
		for i := range v.levels {
			v.levels[i] = lod.L4
		}
		if seen := v.p.read.ViewFor(cam); seen != nil {
			seen.Sectors(func(i int, _ geom.AABB, l lod.Level) { v.levels[i] = l })
		}
		for _, l := range v.levels {
			v.colors = append(v.colors, levelColors[l])
		}
	}
	if _, rays := cam.(camera.Rays); rays && v.p.read.HasHeights() {
		v.onGPU = true
		v.gpu.Paint(cam, v.colors)
		f.ReadGround() // the sectors find the ground under them
		f.DrawAt(f.OnGround(sectorLift))
		return
	}
	w, h := cam.Viewport()
	look := v.p.read.Look()
	for i, c := range v.colors {
		box := g.Box(i)
		midX := (box.TopLeft.X + box.BottomRight.X) / 2
		midY := (box.TopLeft.Y + box.BottomRight.Y) / 2
		at := f.Lifted(midX, midY, 0, sectorLift)
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
		w.p.write.ShowSectors(i.Command.Shown)
	})
}
