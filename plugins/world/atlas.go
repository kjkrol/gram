package world

import (
	"fmt"
	"slices"
	"time"

	"github.com/kjkrol/ceremony/effect"
	cgoke "github.com/kjkrol/ceremony/goke"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/render"
)

// Atlas is the world's sprite sheet under construction, where every kind's look is declared;
// Close bakes it and the world takes it as its looks.
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
	marks  []tag.Tag[effect.States]
	styles render.Styles
	owns   []owned
	// the looks declared at levels, by the kind's own sprite
	declared map[render.SpriteID][]leveled
	// the kinds never thinned (Slot.Always), by their own sprite
	always map[render.SpriteID]bool
	// after Close: each kind's look at each level, by its own sprite, and every sprite's
	versions map[render.SpriteID][lod.Levels]version
	levels   byLevel
	// after Close: by sprite, whether it is drawn in the place of a kind never thinned
	unthinned []bool
	// after Close: by sprite, the levels it is drawn at in a look its kind declared, a bit each
	declaring []uint8
	// after Close: by sprite, the straight colour its kind is drawn in far off, clear for none
	colours [][4]uint8
	// after Close: by sprite, its kind's banners
	banners []kindBanner
}

// kindBanner is how a kind's banners are drawn, as its BannerStyle says, and the levels it is drawn
// in banners at, a bit each
type kindBanner struct {
	style  BannerStyle
	levels uint8
}

// owned is a kind's own look as Add took it, its drawer nil for a material
type owned struct {
	id   render.SpriteID
	size int
	draw render.SpriteDrawer
}

// leveled is a look declared from a level on, draw its drawer where it is drawn of one
type leveled struct {
	from lod.Level
	version
	draw render.SpriteDrawer
}

// version is the sprite something is drawn as at a level, none where it is not drawn
type version struct {
	id    render.SpriteID
	drawn bool
}

// byLevel is what each sprite is drawn as at each level, by the sprite; one past its end is drawn
// as itself
type byLevel [][lod.Levels]version

// at is what the sprite id is drawn as at level, false for nothing
func (b byLevel) at(id render.SpriteID, level lod.Level) (render.SpriteID, bool) {
	if int(id) >= len(b) {
		return id, true
	}
	v := b[id][level]
	return v.id, v.drawn
}

// itself is a sprite drawn as itself at every level
func itself(id render.SpriteID) [lod.Levels]version {
	var t [lod.Levels]version
	for l := range t {
		t[l] = version{id: id, drawn: true}
	}
	return t
}

type animation struct {
	frames []render.SpriteID
	loops  render.Loops // its timing: Period or Spans
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

// NewAtlas starts the world's atlas, for looks declared by hand; Dress is the figures' way.
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
		declared: map[render.SpriteID][]leveled{},
		always:   map[render.SpriteID]bool{},
		versions: map[render.SpriteID][lod.Levels]version{},
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
	own := owned{id: id, size: size}
	switch l := any(look).(type) {
	case render.MaterialID:
		a.shaded[id] = l
	case render.SpriteDrawer:
		a.atlas.Add(of, size, l)
		own.draw = l
	case func(dst *render.Canvas, size int):
		a.atlas.Add(of, size, l)
		own.draw = l
	}
	a.owns = append(a.owns, own)
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

// Styled declares how a plugin draws about the kind: its outline, its cone, its route
// (selection.Highlight, vision.Cone, navigation.RouteStyle), read off the world's Styles.
func (s Slot) Styled(st render.Style) Slot {
	if s.a.p.looked {
		panic("world: a style declared after the renderer was made")
	}
	s.a.styles.Set(s.id, st)
	return s
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
		s.a.phased[twin] = phase{effect: e, lasts: s.a.p.write.Effects().Lasts(e)}
	case render.Phases:
		lasts := s.a.p.write.Effects().Lasts(e)
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
			s.a.animated[on] = animation{frames: frames, loops: l}
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

// At has the kind drawn in look from level on to the next level declared, a close one to
// Standard at most, in place of every look it has under its effects, ways and frames: a
// SpriteDrawer drawn into the sheet at its size or a MaterialID worked out per pixel.
func (s Slot) At[L render.Look](level lod.Level, look L) Slot {
	s.mayDeclare(level)
	id := s.a.p.Kinds().NewSprite()
	var draw render.SpriteDrawer
	switch l := any(look).(type) {
	case render.MaterialID:
		s.a.shaded[id] = l
	case render.SpriteDrawer:
		draw = l
	case func(dst *render.Canvas, size int):
		draw = l
	default:
		panic(fmt.Sprintf("world: sprite %d at level %d in a %T: a level's look is drawn once "+
			"or a material, never timed", s.id, level, look))
	}
	if draw != nil {
		s.a.atlas.Add(id, s.size, draw)
	}
	s.a.declared[s.id] = append(s.a.declared[s.id],
		leveled{from: level, version: version{id: id, drawn: true}, draw: draw})
	return s
}

// Nothing has the kind drawn at no level from level on to the next level declared, a close one
// to Standard at most.
func (s Slot) Nothing(level lod.Level) Slot {
	s.mayDeclare(level)
	s.a.declared[s.id] = append(s.a.declared[s.id], leveled{from: level})
	return s
}

func (s Slot) mayDeclare(level lod.Level) {
	switch {
	case s.a.p.looked:
		panic(fmt.Sprintf("world: sprite %d's look at level %d declared after the renderer was "+
			"made", s.id, level))
	case level == lod.Standard || level >= lod.Omitted:
		panic(fmt.Sprintf("world: sprite %d's look declared at level %d: Standard is the look "+
			"Add took and nothing is drawn at Omitted", s.id, level))
	}
	for _, d := range s.a.declared[s.id] {
		if d.from == level {
			panic(fmt.Sprintf("world: sprite %d's look at level %d declared twice", s.id, level))
		}
	}
}

// Always has the kind never thinned by the world's lod.Ladder.Keep: a camera publishes every one
// of its entities at every level it draws them at.
func (s Slot) Always() Slot {
	if s.a.p.looked {
		panic(fmt.Sprintf("world: sprite %d kept from thinning after the renderer was made", s.id))
	}
	s.a.always[s.id] = true
	return s
}

// isAlways reports whether the sprite id is drawn in the place of a kind never thinned, once the
// atlas is closed
func (a *Atlas) isAlways(id render.SpriteID) bool {
	return int(id) < len(a.unthinned) && a.unthinned[id]
}

// declares reports whether the sprite id is drawn at level in a look its kind declared for it
// (Slot.At), once the atlas is closed
func (a *Atlas) declares(id render.SpriteID, level lod.Level) bool {
	return int(id) < len(a.declaring) && a.declaring[id]&(1<<level) != 0
}

// bannered reports whether the sprite id's kind is drawn in banners at level, where it declares a
// look there or a BannerStyle and is drawn there, once the atlas is closed
func (a *Atlas) bannered(id render.SpriteID, level lod.Level) bool {
	return int(id) < len(a.banners) && a.banners[id].levels&(1<<level) != 0
}

// bannerStyle is how the banners of the sprite id's kind are drawn, colour its far colour, once
// the atlas is closed; the defaults of a nil Atlas
func (a *Atlas) bannerStyle(id render.SpriteID, colour [4]uint8) BannerStyle {
	var st BannerStyle
	if a != nil && int(id) < len(a.banners) {
		st = a.banners[id].style
	}
	return st.filled(colour)
}

// farColour is the straight colour the sprite id's kind is drawn in at the strategic scale, its
// look's at L4, else its own's, once the atlas is closed; false for a kind of neither
func (a *Atlas) farColour(id render.SpriteID) ([4]uint8, bool) {
	if int(id) >= len(a.colours) {
		return [4]uint8{}, false
	}
	c := a.colours[id]
	return c, c[3] != 0
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
		s.a.animated[s.id] = animation{frames: frames, loops: render.Loops{N: n, Period: period}}
		return s
	}
	for k, way := range ways {
		angle := float64(k) * 360 / float64(len(ways))
		frames := s.frames(0, n, func(i int) render.SpriteDrawer {
			return drawFrame(draw, angle, i)
		})
		s.a.animated[way] = animation{frames: frames, loops: render.Loops{N: n, Period: period}}
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

// Close lays the sheet out, bakes it and hands it to the world (Plugin.SetAtlas); call once,
// after the last look.
func (a *Atlas) Close() {
	if a.levels == nil {
		a.level()
	}
	a.atlas.Close()
	a.p.write.SetAtlas(a)
}

// Atlas is the baked sheet.
func (a *Atlas) Atlas() *render.Image { return a.atlas.Atlas() }

// UV is a sprite's rectangle on the sheet.
func (a *Atlas) UV(id render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return a.atlas.UV(id) }

// White is the white texel plain colours sample.
func (a *Atlas) White() (u, v float32) { return a.atlas.White() }

func (a *Atlas) lain() map[render.SpriteID]bool { return a.withKin(a.lying) }

func (a *Atlas) stood() map[render.SpriteID]bool { return a.withKin(a.upright) }

// kin visits id and every sprite drawn in its place: its twins under effects, its directional
// twins, its frames, its looks at the levels once closed, and theirs.
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
		for _, v := range a.versions[id] {
			if v.drawn {
				walk(v.id)
			}
		}
	}
	walk(id)
}

// level works out what each kind is drawn as at each level, then what every sprite drawn in a
// kind's place is: the kind's own from L0 to L3, a blob of its own look at L4 and L5 (a material
// itself), nothing past, unless declared otherwise
func (a *Atlas) level() {
	for _, own := range a.owns {
		a.versions[own.id] = a.versionsOf(own)
	}
	a.levels = byLevel{}
	for _, own := range a.owns {
		of, always := a.versions[own.id], a.always[own.id]
		declaring := a.declaredLevels(own.id)
		colour := a.farColourOf(own)
		banner := a.bannerOf(own.id, of, declaring)
		a.kin(own.id, func(k render.SpriteID) {
			for int(k) >= len(a.levels) {
				a.levels = append(a.levels, itself(render.SpriteID(len(a.levels))))
			}
			for l, v := range of {
				if v.drawn && v.id == own.id {
					v.id = k
				}
				a.levels[k][l] = v
			}
			if always {
				for int(k) >= len(a.unthinned) {
					a.unthinned = append(a.unthinned, false)
				}
				a.unthinned[k] = true
			}
			if declaring != 0 {
				for int(k) >= len(a.declaring) {
					a.declaring = append(a.declaring, 0)
				}
				a.declaring[k] = declaring
			}
			if colour[3] != 0 {
				for int(k) >= len(a.colours) {
					a.colours = append(a.colours, [4]uint8{})
				}
				a.colours[k] = colour
			}
			if banner != (kindBanner{}) {
				for int(k) >= len(a.banners) {
					a.banners = append(a.banners, kindBanner{})
				}
				a.banners[k] = banner
			}
		})
	}
}

// bannerOf is how the banners of the kind own, drawn at each level as of says and in a look
// declared at the levels declaring, are drawn: at the strategic levels it declares a look at, or
// is drawn at under a BannerStyle
func (a *Atlas) bannerOf(own render.SpriteID, of [lod.Levels]version, declaring uint8) kindBanner {
	far := uint8(1<<lod.Strategic | 1<<lod.L5)
	st, styled := render.StyleOf[BannerStyle](&a.styles, own)
	b := kindBanner{style: st, levels: declaring & far}
	if styled {
		for _, l := range [...]lod.Level{lod.Strategic, lod.L5} {
			if of[l].drawn {
				b.levels |= 1 << l
			}
		}
	}
	return b
}

// farColourOf is the straight mean colour of the look the kind own is drawn in at L4 where it
// declares one drawn, else of its own look: a blob's there; clear for a material
func (a *Atlas) farColourOf(own owned) [4]uint8 {
	at := a.versions[own.id][lod.Strategic]
	for _, d := range a.declared[own.id] {
		if d.draw == nil || !at.drawn || d.id != at.id {
			continue
		}
		if c := render.MeanColour(d.draw, own.size); c.A != 0 {
			return [4]uint8{c.R, c.G, c.B, c.A}
		}
	}
	if own.draw == nil {
		return [4]uint8{}
	}
	c := render.MeanColour(own.draw, own.size)
	return [4]uint8{c.R, c.G, c.B, c.A}
}

// declaredLevels is the levels the kind own is drawn at in a look declared for it, a bit each
func (a *Atlas) declaredLevels(own render.SpriteID) uint8 {
	var bits uint8
	for l, v := range a.versions[own] {
		for _, d := range a.declared[own] {
			if v.drawn && d.drawn && v.id == d.id {
				bits |= 1 << l
			}
		}
	}
	return bits
}

// versionsOf is what the kind is drawn as at each level, a declaration lasting to the next and a
// close one to Standard at most; a blob is drawn of its own look where none is declared for
// Strategic, past Standard, L5 drawn as L4 unless declared, and it sorts the kind's declarations
func (a *Atlas) versionsOf(own owned) [lod.Levels]version {
	self := version{id: own.id, drawn: true}
	of := [lod.Levels]version{self, self, self, self, self}
	declared := a.declared[own.id]
	slices.SortFunc(declared, func(x, y leveled) int { return int(x.from) - int(y.from) })
	if far := beyond(declared); own.draw != nil && (len(far) == 0 || far[0].from > lod.Strategic) {
		blob := a.p.Kinds().NewSprite()
		a.atlas.Add(blob, own.size, render.Blob(own.draw))
		of[lod.Strategic] = version{id: blob, drawn: true}
	}
	of[lod.L5] = of[lod.Strategic]
	for i, d := range declared {
		to := lod.Omitted
		if i+1 < len(declared) {
			to = declared[i+1].from
		}
		if d.from < lod.Standard {
			to = min(to, lod.Standard)
		}
		for l := d.from; l < to; l++ {
			of[l] = d.version
		}
	}
	return of
}

// beyond is those of declared, sorted, past Standard
func beyond(declared []leveled) []leveled {
	for i, d := range declared {
		if d.from > lod.Standard {
			return declared[i:]
		}
	}
	return nil
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

// the effects' swaps, the look declared later first: a swap finds the kind's own sprite, so
// under two effects at once the first swap that matches is the one that holds, and the rest
// miss the twin it put in
func (a *Atlas) swaps() []Rule {
	out := make([]Rule, 0, len(a.marks))
	for i := len(a.marks) - 1; i >= 0; i-- {
		mark := a.marks[i]
		out = append(out, Swap(a.twins[mark], mark.In))
	}
	return out
}

// in their order: the facing swaps, the frames by the clock, the frames along an effect, then
// the turning, after the effects' own swaps
func (a *Atlas) rules(now func() time.Duration) []Rule {
	var out []Rule
	if len(a.faced) > 0 {
		faced := a.faced
		out = append(out, With(func(ap render.Appearance, b Base) render.Appearance {
			if twins := faced[ap.SpriteID]; len(twins) > 0 {
				ap.SpriteID = twins[headingIndex(b.Vel.Dir, len(twins))]
			}
			return ap
		}))
	}
	if len(a.animated) > 0 {
		animated, moving := a.animated, a.moving
		out = append(out, With(func(ap render.Appearance, b Base) render.Appearance {
			an, ok := animated[ap.SpriteID]
			if !ok || moving[ap.SpriteID] && b.Vel.Value == 0 && b.Vel.Side == 0 {
				return ap
			}
			ap.SpriteID = an.frames[an.loops.Frame(now())]
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
		out = append(out, With(along))
	}
	if len(a.turned) > 0 {
		turned := a.withKin(a.turned)
		out = append(out, With(func(ap render.Appearance, b Base) render.Appearance {
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
