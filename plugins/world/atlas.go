package world

import (
	"fmt"
	"time"

	"github.com/kjkrol/ceremony/effect"
	cgoke "github.com/kjkrol/ceremony/goke"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/gram/render"
)

// Atlas is the world's sprite sheet under construction, where every kind's look is declared.
// Close it and hand it to WithRenderer, which applies what was declared itself.
type Atlas struct {
	p        *Plugin
	atlas    *render.Atlas
	turned   map[render.SpriteID]bool
	faced    map[render.SpriteID][]render.SpriteID // a sprite's directional twins
	unders   map[render.SpriteID][]render.SpriteID
	shaded   map[render.SpriteID]render.MaterialID
	animated map[render.SpriteID]animation // frames by the clock, by the sprite they stand for
	moving   map[render.SpriteID]bool      // an animation played while the entity moves alone
	phased   map[render.SpriteID]phase     // by the twin under the effect
	lying    map[render.SpriteID]bool
	upright  map[render.SpriteID]bool
	edges    map[render.SpriteID]render.SpriteID
	twins    map[tag.Tag[effect.States]]map[render.SpriteID]render.SpriteID // under an effect
	// the effects in the order first looked under: the last wins
	marks []tag.Tag[effect.States]
}

type animation struct {
	frames []render.SpriteID
	period time.Duration
}

type phase struct {
	look   render.Phases
	frames []render.SpriteID
	effect effect.Effect
	lasts  time.Duration
}

// progress is how far the effect has run on an entity: 0 as it begins, 1 as it runs out
func (p phase) progress(left time.Duration, under bool) float32 {
	if !under || left <= 0 {
		return 1
	}
	return 1 - float32(left)/float32(p.lasts)
}

var _ render.AtlasSource = (*Atlas)(nil)

// NewAtlas starts the world's atlas.
func (p *Plugin) NewAtlas() *Atlas {
	return &Atlas{
		p:        p,
		atlas:    render.NewAtlas(),
		turned:   map[render.SpriteID]bool{},
		faced:    map[render.SpriteID][]render.SpriteID{},
		unders:   map[render.SpriteID][]render.SpriteID{},
		shaded:   map[render.SpriteID]render.MaterialID{},
		animated: map[render.SpriteID]animation{},
		moving:   map[render.SpriteID]bool{},
		phased:   map[render.SpriteID]phase{},
		lying:    map[render.SpriteID]bool{},
		upright:  map[render.SpriteID]bool{},
		edges:    map[render.SpriteID]render.SpriteID{},
		twins:    map[tag.Tag[effect.States]]map[render.SpriteID]render.SpriteID{},
	}
}

// twin is the slot drawn under e in place of the sprite of, issued the first time it is asked
// for. It panics for a first look after the world's renderer was made.
func (a *Atlas) twin(e effect.Effect, of render.SpriteID) render.SpriteID {
	twins := a.twins[e.Mark()]
	if id, ok := twins[of]; ok {
		return id
	}
	if a.p.looked {
		panic(fmt.Sprintf("world: the first look under effect %d comes after the renderer was made",
			e.Mark()))
	}
	if twins == nil {
		twins = map[render.SpriteID]render.SpriteID{}
		a.twins[e.Mark()] = twins
		a.marks = append(a.marks, e.Mark())
	}
	id := a.p.kinds.NewSprite()
	twins[of] = id
	return id
}

// Add takes look on as of's own and hands back a Slot to chain the rest of its look on. A
// SpriteDrawer is drawn once into the sheet, size x size; a MaterialID is worked out per pixel in
// the entity's box every frame.
func (a *Atlas) Add[L render.Look](of render.Sprited, size int, look L) Slot {
	id := of.SpriteID()
	switch l := any(look).(type) {
	case render.MaterialID:
		a.shaded[id] = l
	case render.SpriteDrawer:
		a.atlas.Add(of, size, l)
	case func(dst *render.Canvas, size int):
		a.atlas.Add(of, size, l)
	}
	return Slot{a: a, id: id, size: size}
}

// Slot is one sprite added to the world's Atlas, which the rest of its look chains on.
type Slot struct {
	a    *Atlas
	id   render.SpriteID
	size int
}

// frameDrawer draws a frame, with the angle a facing sprite looks at or without.
type frameDrawer interface {
	~func(frame int) render.SpriteDrawer |
		~func(angleDeg float64, frame int) render.SpriteDrawer
}

// Under adds the sprite's look under the effect e, the same size, drawn in its place while the
// effect's marker is on. A render.Phases spreads its frames along the effect's course, which
// must Last, a render.Loops shows them round and round by the clock; one drawing each frame at
// an angle (PhasedFacing, LoopedFacing) draws it every way the sprite faces. A material under an
// effect reads how far it has run as its fraction. Under two effects at once the entity is drawn
// by the look of the effect looked under later, in any kind.
func (s Slot) Under[L render.Look](e effect.Effect, look L) Slot {
	twin := s.a.twin(e, s.id)
	switch l := any(look).(type) {
	case render.MaterialID:
		s.a.shaded[twin] = l
		s.a.phased[twin] = phase{effect: e, lasts: s.a.p.Effects().Lasts(e)}
	case render.Phases:
		lasts := s.a.p.Effects().Lasts(e)
		if lasts == effect.Forever {
			panic(fmt.Sprintf("world: sprite %d phased under effect %d, which never runs out",
				s.id, e.Mark()))
		}
		s.eachWay(twin, l.DrawFacing != nil, func(on render.SpriteID, angle float64) {
			frames := s.frames(on, l.N, func(i int) render.SpriteDrawer {
				if l.DrawFacing != nil {
					return l.DrawFacing(angle, i)
				}
				return l.Draw(i)
			})
			s.a.phased[on] = phase{look: l, frames: frames, effect: e, lasts: lasts}
		})
	case render.Loops:
		s.eachWay(twin, l.DrawFacing != nil, func(on render.SpriteID, angle float64) {
			frames := s.frames(on, l.N, func(i int) render.SpriteDrawer {
				if l.DrawFacing != nil {
					return l.DrawFacing(angle, i)
				}
				return l.Draw(i)
			})
			s.a.animated[on] = animation{frames: frames, period: l.Period}
		})
	case render.SpriteDrawer:
		s.a.atlas.Add(twin, s.size, l)
	case func(dst *render.Canvas, size int):
		s.a.atlas.Add(twin, s.size, l)
	}
	s.a.unders[s.id] = append(s.a.unders[s.id], twin)
	return s
}

// eachWay calls add for the sprite on, or, facing, once for each of its ways with a twin of on
// to stand for the way.
func (s Slot) eachWay(on render.SpriteID, facing bool,
	add func(on render.SpriteID, angle float64)) {
	ways := s.a.faced[s.id]
	if !facing || len(ways) == 0 {
		add(on, 0)
		return
	}
	twins := make([]render.SpriteID, len(ways))
	for i := range twins {
		twins[i] = s.a.p.Kinds().NewSprite()
		add(twins[i], float64(i)*360/float64(len(ways)))
	}
	s.a.faced[on] = twins
}

// frames draws n frames, the first into the sprite on itself where it is not drawn yet (0
// for one that is), the rest into new ones.
func (s Slot) frames(on render.SpriteID, n int,
	draw func(i int) render.SpriteDrawer) []render.SpriteID {
	frames := make([]render.SpriteID, n)
	for i := range frames {
		if i == 0 && on != 0 {
			frames[i] = on
		} else {
			frames[i] = s.a.p.Kinds().NewSprite()
		}
		s.a.atlas.Add(frames[i], s.size, draw(i))
	}
	return frames
}

// Turning has the sprite drawn turned the way its entity is headed. Author it facing east, its
// content within the circle inscribed in its square box.
func (s Slot) Turning() Slot {
	s.a.turned[s.id] = true
	return s
}

// Lying has the sprite laid flat on the ground at its entity's Altitude where a view in relief
// stands the rest up. Its looks under effects, its frames and its twins lie too.
func (s Slot) Lying() Slot {
	s.a.lying[s.id] = true
	return s
}

// Upright has the sprite stand true on the four sides and the top of its entity's box where a
// view in relief stands the rest up as billboards. Its looks under effects, its frames and its
// twins stand so too.
func (s Slot) Upright() Slot {
	s.a.upright[s.id] = true
	return s
}

// Edges adds the sprite standing along the long sides of a lying one while its entity's Lie is
// Edged. Author it seen from the side, its length along x and its height along y, filling its
// square.
func (s Slot) Edges(draw render.SpriteDrawer) Slot {
	edge := s.a.p.Kinds().NewSprite()
	s.a.atlas.Add(edge, s.size, draw)
	s.a.edges[s.id] = edge
	return s
}

// Animated adds the sprite's n frames, each shown period of game time in turn, drawn by draw for
// its frame: after Facing, for each way the sprite looks, a draw taking the angle drawing each
// way its own. Facing comes first.
func (s Slot) Animated[D frameDrawer](n int, period time.Duration, draw D) Slot {
	if n <= 0 || period <= 0 {
		panic(fmt.Sprintf("world: sprite %d animated over %d frames of %v", s.id, n, period))
	}
	ways := s.a.faced[s.id]
	if len(ways) == 0 {
		frames := s.frames(0, n, func(i int) render.SpriteDrawer {
			return drawFrame(draw, 0, i)
		})
		s.a.animated[s.id] = animation{frames: frames, period: period}
		return s
	}
	for k, way := range ways {
		angle := float64(k) * 360 / float64(len(ways))
		frames := s.frames(0, n, func(i int) render.SpriteDrawer {
			return drawFrame(draw, angle, i)
		})
		s.a.animated[way] = animation{frames: frames, period: period}
	}
	return s
}

func drawFrame[D frameDrawer](draw D, angle float64, frame int) render.SpriteDrawer {
	switch d := any(draw).(type) {
	case func(frame int) render.SpriteDrawer:
		return d(frame)
	case func(angleDeg float64, frame int) render.SpriteDrawer:
		return d(angle, frame)
	}
	panic(fmt.Sprintf("world: a frame drawn by %T", draw))
}

// WhileMoving has the sprite's Animated frames go only while its entity moves; standing it is
// drawn as it faces. Animated comes first.
func (s Slot) WhileMoving() Slot {
	on := append([]render.SpriteID{s.id}, s.a.faced[s.id]...)
	marked := false
	for _, id := range on {
		if _, ok := s.a.animated[id]; ok {
			s.a.moving[id] = true
			marked = true
		}
	}
	if !marked {
		panic(fmt.Sprintf("world: sprite %d moves while animated by nothing: Animated comes first",
			s.id))
	}
	return s
}

// Facing adds the sprite's n directional twins, east first against the clock, each drawn by
// draw at its angle in degrees. Animated frames and looks under effects drawn with the angle
// follow it; the rest keep their own look whichever way it goes.
func (s Slot) Facing(n int, draw func(angleDeg float64) render.SpriteDrawer) Slot {
	if n <= 0 {
		panic(fmt.Sprintf("world: sprite %d faces %d ways", s.id, n))
	}
	if _, ok := s.a.animated[s.id]; ok {
		panic(fmt.Sprintf("world: sprite %d is animated already: Facing comes first", s.id))
	}
	twins := make([]render.SpriteID, n)
	for i := range twins {
		twins[i] = s.a.p.Kinds().NewSprite()
		s.a.atlas.Add(twins[i], s.size, draw(float64(i)*360/float64(n)))
	}
	s.a.faced[s.id] = twins
	return s
}

// Close lays the sheet out and bakes it; call once, after the last Add.
func (a *Atlas) Close() { a.atlas.Close() }

// Atlas is the baked sheet.
func (a *Atlas) Atlas() *render.Image { return a.atlas.Atlas() }

// UV is a sprite's rectangle on the sheet.
func (a *Atlas) UV(id render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return a.atlas.UV(id) }

// White is the white texel plain colours sample.
func (a *Atlas) White() (u, v float32) { return a.atlas.White() }

func (a *Atlas) lain() map[render.SpriteID]bool { return a.withKin(a.lying) }

func (a *Atlas) stood() map[render.SpriteID]bool { return a.withKin(a.upright) }

// kin visits id and every sprite drawn in its place: its twins under effects, its directional
// twins, its frames, and theirs.
func (a *Atlas) kin(id render.SpriteID, visit func(render.SpriteID)) {
	seen := map[render.SpriteID]bool{}
	var walk func(render.SpriteID)
	walk = func(id render.SpriteID) {
		if seen[id] {
			return
		}
		seen[id] = true
		visit(id)
		for _, twin := range a.unders[id] {
			walk(twin)
		}
		for _, twin := range a.faced[id] {
			walk(twin)
		}
		for _, frame := range a.animated[id].frames {
			walk(frame)
		}
		for _, frame := range a.phased[id].frames {
			walk(frame)
		}
	}
	walk(id)
}

func (a *Atlas) withKin(marked map[render.SpriteID]bool) map[render.SpriteID]bool {
	out := map[render.SpriteID]bool{}
	for id := range marked {
		a.kin(id, func(k render.SpriteID) { out[k] = true })
	}
	return out
}

func (a *Atlas) edged() map[render.SpriteID]render.SpriteID {
	out := map[render.SpriteID]render.SpriteID{}
	for id, edge := range a.edges {
		a.kin(id, func(k render.SpriteID) { out[k] = edge })
	}
	return out
}

// in their order: the facing swaps, the frames by the clock, the frames along an effect, then
// the turning, after the effects' own swaps
func (a *Atlas) rules(now func() time.Duration) []render.Rule {
	var out []render.Rule
	if len(a.faced) > 0 {
		faced := a.faced
		out = append(out, render.With(func(ap render.Appearance, b Base) render.Appearance {
			if twins := faced[ap.SpriteID]; len(twins) > 0 {
				ap.SpriteID = twins[headingIndex(b.Vel.Dir, len(twins))]
			}
			return ap
		}))
	}
	if len(a.animated) > 0 {
		animated, moving := a.animated, a.moving
		out = append(out, render.With(func(ap render.Appearance, b Base) render.Appearance {
			an, ok := animated[ap.SpriteID]
			if !ok || moving[ap.SpriteID] && b.Vel.Value == 0 && b.Vel.Side == 0 {
				return ap
			}
			ap.SpriteID = an.frames[int(now()/an.period)%len(an.frames)]
			return ap
		}))
	}
	if a.anyPhased() {
		phased := a.phased
		along := func(ap render.Appearance, under cgoke.Active) render.Appearance {
			if ph, ok := phased[ap.SpriteID]; ok && len(ph.frames) > 0 {
				left, on := under.Left(ph.effect)
				ap.SpriteID = ph.frames[ph.look.Frame(ph.progress(left, on))]
			}
			return ap
		}
		out = append(out, render.With(along))
	}
	if len(a.turned) > 0 {
		turned := a.withKin(a.turned)
		out = append(out, render.With(func(ap render.Appearance, b Base) render.Appearance {
			if turned[ap.SpriteID] && (b.Vel.Dir.X != 0 || b.Vel.Dir.Y != 0) {
				ap.Angle = angleOf(b.Vel.Dir)
			}
			return ap
		}))
	}
	return out
}

func (a *Atlas) anyPhased() bool {
	for _, ph := range a.phased {
		if len(ph.frames) > 0 {
			return true
		}
	}
	return false
}
