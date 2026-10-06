package world

import (
	"fmt"
	"time"

	"github.com/kjkrol/gram/render"
)

// Atlas is the world's sprite sheet under construction — everything a look is, declared in one
// place, a scene's Layers: Add each kind's sprite, chain Under for its look under an effect,
// Turning for a sprite turned the way its entity is headed (Vel.Dir), Facing for directional
// twins. Close it and hand it to WithRenderer — it is the render.AtlasSource the renderer draws
// from, and the renderer applies what was declared itself: the effects' swaps, then the facing,
// then the turning, every frame.
type Atlas struct {
	p        *Plugin
	atlas    *render.Atlas
	turned   map[render.SpriteID]bool
	faced    map[render.SpriteID][]render.SpriteID
	unders   map[render.SpriteID][]render.SpriteID // the Under twins of each sprite: Turning reaches them
	shaded   map[render.SpriteID]render.MaterialID // the sprites worked out per pixel instead of drawn
	animated map[render.SpriteID]animation         // the sprites drawn frame after frame
}

// animation is one sprite's frames and how long each is shown.
type animation struct {
	frames []render.SpriteID
	period time.Duration
}

var _ render.AtlasSource = (*Atlas)(nil)

// NewAtlas starts the world's atlas; the twins Facing declares take their slots from the
// world's kinds.
func (p *Plugin) NewAtlas() *Atlas {
	return &Atlas{p: p, atlas: render.NewAtlas(), turned: map[render.SpriteID]bool{}, faced: map[render.SpriteID][]render.SpriteID{}, unders: map[render.SpriteID][]render.SpriteID{}, shaded: map[render.SpriteID]render.MaterialID{}, animated: map[render.SpriteID]animation{}}
}

// Add takes look on as of's own — a kind's handle, an ammo, a slot of the kinds' own — and hands
// it back as a Slot to chain the rest of its look on. look paints one of the two ways
// (render.Look): a SpriteDrawer drawn once into the sheet, size x size, or a MaterialID worked
// out per pixel in the entity's box every frame — the renderer fills the material's inputs with
// the entity's own state (see the material contract in the package doc).
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

// Slot is one sprite added to the world's Atlas: what Under, Turning and Facing chain on.
type Slot struct {
	a    *Atlas
	id   render.SpriteID
	size int
}

// Under adds the sprite's look under d, the same size — in its place while the effect's marker
// is on: the witch gone white under frozen, a calmed ward a plain sprite again. look paints
// either way (render.Look), so a sprite dims into a material and a material back into a sprite.
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

// Turning has the sprite drawn turned the way its entity is headed — Appearance.Angle from
// Vel.Dir, smoothly, in the engine's one convention (render.Arrow): degrees, 0 east, against
// the clock with the screen's y down. Author the sprite facing east, its content within the
// circle inscribed in its square box (render.Appearance.Angle); one that never moved keeps the
// Angle its Appearance holds. A look under an effect turns the same way.
func (s Slot) Turning() Slot {
	s.a.turned[s.id] = true
	return s
}

// Animated adds the sprite's frames — n twins, each shown period of game time in turn, drawn by
// draw for its frame: a walker's legs, a banner in the wind. Frames go by the tactical clock, so
// the pause is a freeze-frame and the tempo hurries the gait; a look under an effect is one still
// frame of its own (Under), and a Turning sprite's frames turn with it. Facing and Animated do
// not share a slot.
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
// draw at its angle in degrees: the sprite is swapped for the twin of the way its entity is
// headed (Vel.Dir). A look under an effect keeps that look whichever way it goes — give the
// effect's look its own twins where it needs them.
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

// Atlas is the baked sheet — see render.AtlasSource.
func (a *Atlas) Atlas() *render.Image { return a.atlas.Atlas() }

// UV is a sprite's rectangle on the sheet — see render.AtlasSource.
func (a *Atlas) UV(id render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return a.atlas.UV(id) }

// White is the white texel plain colours sample — see render.AtlasSource.
func (a *Atlas) White() (u, v float32) { return a.atlas.White() }

// rules are what the renderer applies for the looks declared on the atlas, in their order: the
// facing swaps, the animations' frames (now is the tactical clock as shown), then the turning —
// after the effects' own swaps. For WithRenderer.
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
		for id := range a.turned { // a turned sprite's looks under effects and its frames turn with it
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
