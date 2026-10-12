package world

import (
	"fmt"
	"time"

	"github.com/kjkrol/gram/render"
)

// Atlas is the world's sprite sheet under construction, where every kind's look is declared.
// Close it and hand it to WithRenderer, which applies what was declared itself.
type Atlas struct {
	p        *Plugin
	atlas    *render.Atlas
	turned   map[render.SpriteID]bool
	faced    map[render.SpriteID][]render.SpriteID
	unders   map[render.SpriteID][]render.SpriteID
	shaded   map[render.SpriteID]render.MaterialID
	animated map[render.SpriteID]animation
	lying    map[render.SpriteID]bool
	upright  map[render.SpriteID]bool
	edges    map[render.SpriteID]render.SpriteID
}

type animation struct {
	frames []render.SpriteID
	period time.Duration
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
		lying:    map[render.SpriteID]bool{},
		upright:  map[render.SpriteID]bool{},
		edges:    map[render.SpriteID]render.SpriteID{},
	}
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

// Under adds the sprite's look under d, the same size, drawn in its place while the effect's
// marker is on.
func (s Slot) Under[L render.Look](d render.Dresser, look L) Slot {
	twin := d.Look(s.id)
	switch l := any(look).(type) {
	case render.MaterialID:
		s.a.shaded[twin] = l
	case render.SpriteDrawer:
		s.a.atlas.Add(twin, s.size, l)
	case func(dst *render.Canvas, size int):
		s.a.atlas.Add(twin, s.size, l)
	}
	s.a.unders[s.id] = append(s.a.unders[s.id], twin)
	return s
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
// its frame. Facing and Animated do not share a slot.
func (s Slot) Animated(n int, period time.Duration, draw func(frame int) render.SpriteDrawer) Slot {
	if n <= 0 || period <= 0 {
		panic(fmt.Sprintf("world: sprite %d animated over %d frames of %v", s.id, n, period))
	}
	if _, ok := s.a.faced[s.id]; ok {
		panic(fmt.Sprintf("world: sprite %d is faced already: Facing and Animated do not share a slot", s.id))
	}
	frames := make([]render.SpriteID, n)
	for i := range frames {
		frames[i] = s.a.p.Kinds().NewSprite()
		s.a.atlas.Add(frames[i], s.size, draw(i))
	}
	s.a.animated[s.id] = animation{frames: frames, period: period}
	return s
}

// Facing adds the sprite's n directional twins, east first against the clock, each drawn by
// draw at its angle in degrees. A look under an effect keeps its own look whichever way it goes.
func (s Slot) Facing(n int, draw func(angleDeg float64) render.SpriteDrawer) Slot {
	if n <= 0 {
		panic(fmt.Sprintf("world: sprite %d faces %d ways", s.id, n))
	}
	if _, ok := s.a.animated[s.id]; ok {
		panic(fmt.Sprintf("world: sprite %d is animated already: Facing and Animated do not share a slot", s.id))
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

func (a *Atlas) lain() map[render.SpriteID]bool { return a.withTwins(a.lying) }

func (a *Atlas) stood() map[render.SpriteID]bool { return a.withTwins(a.upright) }

func (a *Atlas) withTwins(marked map[render.SpriteID]bool) map[render.SpriteID]bool {
	out := map[render.SpriteID]bool{}
	for id := range marked {
		out[id] = true
		for _, twin := range a.unders[id] {
			out[twin] = true
		}
		for _, frame := range a.animated[id].frames {
			out[frame] = true
		}
		for _, twin := range a.faced[id] {
			out[twin] = true
		}
	}
	return out
}

func (a *Atlas) edged() map[render.SpriteID]render.SpriteID {
	out := map[render.SpriteID]render.SpriteID{}
	for id, edge := range a.edges {
		out[id] = edge
		for _, twin := range a.unders[id] {
			out[twin] = edge
		}
		for _, frame := range a.animated[id].frames {
			out[frame] = edge
		}
		for _, twin := range a.faced[id] {
			out[twin] = edge
		}
	}
	return out
}

// in their order: the facing swaps, the frames, then the turning, after the effects' own swaps
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
		animated := a.animated
		out = append(out, render.With(func(ap render.Appearance, _ Base) render.Appearance {
			if an, ok := animated[ap.SpriteID]; ok {
				ap.SpriteID = an.frames[int(now()/an.period)%len(an.frames)]
			}
			return ap
		}))
	}
	if len(a.turned) > 0 {
		turned := map[render.SpriteID]bool{}
		for id := range a.turned {
			turned[id] = true
			for _, twin := range a.unders[id] {
				turned[twin] = true
			}
			for _, frame := range a.animated[id].frames {
				turned[frame] = true
			}
		}
		out = append(out, render.With(func(ap render.Appearance, b Base) render.Appearance {
			if turned[ap.SpriteID] && (b.Vel.Dir.X != 0 || b.Vel.Dir.Y != 0) {
				ap.Angle = angleOf(b.Vel.Dir)
			}
			return ap
		}))
	}
	return out
}
