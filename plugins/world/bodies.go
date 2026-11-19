package world

import (
	"fmt"
	"math"
	"time"

	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/render"
)

// bodyElevation is how high over the level a body's cards look down at it from, radians: the
// 2:1 view's (body.wgsl's shapeElevation, the same).
const bodyElevation = math.Pi / 6

// what a photograph of a body takes (body_shot.wgsl): the body in its tones under the
// photograph's own light, its paint alone, or the ways out of it
const (
	shotLit = iota
	shotPaint
	shotWays
)

// the WGSL of the bodies' parts, paints and tones (body.wgsl), registered for every shader: the
// photograph's calls it and so does a Look's that stands the bodies (BodyLook)
var _ = render.RegisterMaterials(render.Files(shaders, "shaders/body.wgsl"), nil)

var shotShader = render.NewMeshShaderWith("world's body shot",
	render.Files(shaders, "shaders/body_shot.wgsl"), []render.Uniform{
		{Name: "ShotBody", Size: 4},
		{Name: "ShotFrom", Size: 4},
	})

// Bodied is a kind's body as a Look stands it: its Row of the bodies' Table and how many Parts
// it has, how Wide and Tall it is, its cards and Above, it seen from straight above in the
// photograph's own light. A card (Card) is its paint with the ways out of it (body_shot.wgsl):
// Views and Ways from BodyViews sides round it, the first looking along its x and each next an
// eighth of a turn on towards its y, from Elevation over the level and reaching Margin under
// its foot and over its top, leaning back about TopAt over its foot for an eye looking steeper
// (body.wgsl's shapeRow); Top and TopWays from straight above, held to the eye there.
type Bodied struct {
	Table     *render.Image
	Row       int
	Parts     int
	Wide      float32
	Tall      float32
	Views     [BodyViews]render.SpriteID
	Ways      [BodyViews]render.SpriteID
	Top       render.SpriteID
	TopWays   render.SpriteID
	Above     render.SpriteID
	Elevation float32
	Margin    float32
	TopAt     float32
	// how steeply an eye looks down for the card from above (Body.steep); 0 never
	steep float32
}

// Card is the card of the body, turned by turn about its foot, for an eye looking at it along
// way: the view of its side nearest and the ways out of it there. Looking down so steeply that
// the one from above shows the body's parts truer, or no card stands, it is that one, which
// lies.
func (b *Bodied) Card(way [3]float32, turn float32) (view, ways render.SpriteID, lies bool) {
	if b.steep > 0 && way[2] < 0 && way[2]*way[2] > b.steep*(way[0]*way[0]+way[1]*way[1]) {
		return b.Top, b.TopWays, true
	}
	const eighth = 2 * math.Pi / BodyViews
	k := int(f32.Floor(eighths(way[0], way[1])-turn/eighth+0.5)) % BodyViews
	if k < 0 {
		k += BodyViews
	}
	return b.Views[k], b.Ways[k], false
}

// eighths is the angle of (x, y) in eighths of a whole turn, 0 to 8 from x towards y, within a
// two-hundredth of one: enough to pick a card's side by, where an arc tangent a card, thousands
// of them in view, was a fifth of the frame's composing
func eighths(x, y float32) float32 {
	ax, ay := f32.Abs(x), f32.Abs(y)
	lo, hi := min(ax, ay), max(ax, ay)
	if hi == 0 {
		return 0
	}
	z := lo / hi
	a := z * (1 + 0.3477*(1-z))
	if ay > ax {
		a = 2 - a
	}
	if x < 0 {
		a = 4 - a
	}
	if y < 0 {
		a = 8 - a
	}
	return a
}

// AddBody takes body on as of's own and hands back a Slot to chain the rest of its look on: a
// view in relief stands it whole near the camera and as a card of it beyond (BodyLook), a view
// from above draws it photographed from above, turned as it stands. Its sprite, size x size, is
// the body photographed from the side, and it is whole, a cutout (Slot.Cutout).
func (a *Atlas) AddBody(of render.Sprited, size int, body Body) Slot {
	id := of.SpriteID()
	body.Check(fmt.Sprintf("sprite %d", id))
	b := &Bodied{
		Row:       len(a.bodies),
		Parts:     len(body.Parts),
		Wide:      body.Wide,
		Tall:      body.Tall,
		Elevation: bodyElevation,
		Margin:    body.Wide / 2 * float32(math.Tan(bodyElevation)),
		TopAt:     body.topAt(),
		steep:     body.steep(bodyElevation),
	}
	a.bodies = append(a.bodies, body)
	a.bodied[id] = b
	a.cutout[id] = true
	a.atlas.AddShot(of, size, a.shot(b, 0, 0, shotLit, false))
	b.Above = a.p.Kinds().NewSprite()
	b.Top = a.p.Kinds().NewSprite()
	b.TopWays = a.p.Kinds().NewSprite()
	a.atlas.AddShot(b.Above, size, a.shot(b, 0, 0, shotLit, true))
	a.atlas.AddShot(b.Top, size, a.shot(b, 0, 0, shotPaint, true))
	a.atlas.AddShot(b.TopWays, size, a.shot(b, 0, 0, shotWays, true))
	for k := range BodyViews {
		heading := float32(k) * 2 * math.Pi / BodyViews
		view, ways := a.p.Kinds().NewSprite(), a.p.Kinds().NewSprite()
		a.atlas.AddShot(view, size, a.shot(b, heading, bodyElevation, shotPaint, false))
		a.atlas.AddShot(ways, size, a.shot(b, heading, bodyElevation, shotWays, false))
		b.Views[k], b.Ways[k] = view, ways
	}
	// its sprite has no pixels before the GPU: far off it is a blob of its paints' mean
	a.owns = append(a.owns, owned{id: id, size: size, draw: render.Diamond(body.mean())})
	return Slot{a: a, id: id, size: size}
}

// BodyOf is the body of's kind was added with, as a Look stands it; nil without one.
func (a *Atlas) BodyOf(of render.Sprited) *Bodied {
	a.bodyTable()
	return a.bodied[of.SpriteID()]
}

// Above has the kind's look from level on to the next level declared be its body photographed
// from straight above, as the sheets take it, in place of every look it has under its effects,
// ways and frames; a kind without a body panics.
func (s Slot) Above(level lod.Level) Slot {
	b := s.a.bodied[s.id]
	if b == nil {
		panic(fmt.Sprintf("world: sprite %d seen from above at level %d has no body", s.id,
			level))
	}
	s.mayDeclare(level)
	s.a.declared[s.id] = append(s.a.declared[s.id],
		leveled{from: level, version: version{id: b.Above, drawn: true}})
	return s
}

// shot is the photograph of the body b taken on the GPU into a sprite's place on the sheet:
// looking along heading, radians off its x, from elevation over the level, or from straight
// above
func (a *Atlas) shot(b *Bodied, heading, elevation float32, takes int,
	above bool) func(dst *render.Image) {
	a.shots++
	return func(dst *render.Image) {
		opts := render.DrawMeshOptions{
			Vertices:   6 * MaxParts,
			Depth:      a.shotDepth,
			WriteDepth: true,
			Uniforms: map[string]any{
				"ShotBody": []float32{float32(b.Row), b.Wide, b.Tall, float32(b.Parts)},
				"ShotFrom": []float32{heading, elevation, float32(takes), 0},
			},
		}
		if above {
			opts.Uniforms["ShotFrom"] = []float32{0, 0, float32(takes), 1}
		}
		// the photographs lie apart on the sheet: one depth buffer, cleared before the first
		if a.shotDepth == nil {
			a.shotDepth = render.NewDepth()
			opts.Depth, opts.ClearDepth = a.shotDepth, true
		}
		opts.Images[0] = a.bodyTable()
		dst.DrawMesh(nil, shotShader, &opts)
		if a.shots--; a.shots == 0 {
			a.shotDepth.Release()
			a.shotDepth = nil
		}
	}
}

// bodyTable is the table the bodies' shaders read, a row a body (Body.row), made the first
// time it is asked for
func (a *Atlas) bodyTable() *render.Image {
	if a.table == nil && len(a.bodies) > 0 {
		pix := make([]byte, 0, 4*tableWide*len(a.bodies))
		for _, b := range a.bodies {
			pix = append(pix, b.row()...)
		}
		a.table = render.NewImage(tableWide, len(a.bodies))
		a.table.WritePixels(pix)
		for _, b := range a.bodied {
			b.Table = a.table
		}
	}
	return a.table
}

// BodiesLever is the bodies' reach as the cameras' frame budget turns it (cameras.Lever, given
// in cameras.Config.Levers). A reach let out and drawn in again at once cost the budget: it
// waits a minute to be let out again, twice as long after each such try, sixteen at the most.
type BodiesLever struct{ p *Plugin }

// bodiesTries is how the frame budget's tries at letting the bodies' reach out have gone: when
// it was last let out, whether it has been drawn back in since, and how long after the letting
// out none is tried again.
type bodiesTries struct {
	out  time.Time
	back bool
	wait time.Duration
}

const (
	// a reach drawn in again within bodiesFailed of its letting out took the frame budget
	bodiesFailed   = 10 * time.Second
	bodiesWait     = time.Minute
	bodiesWaitMost = 16 * time.Minute
)

// Bodies is the lever of how far the kinds with a body stand whole.
func (p *Plugin) Bodies() BodiesLever { return BodiesLever{p} }

// Following is whether the bodies' reach follows the frame budget.
func (l BodiesLever) Following() bool { return !l.p.Res.Settings.StillBodies && l.p.read.Bodied() }

// Coarser draws the bodies' reach in a level, the close scale the nearest, true where it could.
func (l BodiesLever) Coarser() bool {
	s := &l.p.Res.Settings
	if int(s.BodiesShort)+int(s.BodiesPulled) >= 2 {
		return false
	}
	s.BodiesPulled++
	t := &l.p.bodiesTries
	if !t.back && !t.out.IsZero() && l.p.now().Sub(t.out) < bodiesFailed {
		t.wait = min(max(2*t.wait, bodiesWait), bodiesWaitMost)
	}
	t.back = true
	return true
}

// Finer lets the bodies' reach out a level, as far as the Settings say, true where it could:
// not while a try that cost the budget is waited out.
func (l BodiesLever) Finer() bool {
	s := &l.p.Res.Settings
	if s.BodiesPulled == 0 {
		return false
	}
	t := &l.p.bodiesTries
	now := l.p.now()
	if !t.back && now.Sub(t.out) >= bodiesFailed {
		t.wait = 0
	}
	if now.Before(t.out.Add(t.wait)) {
		return false
	}
	s.BodiesPulled--
	t.out, t.back = now, false
	return true
}
