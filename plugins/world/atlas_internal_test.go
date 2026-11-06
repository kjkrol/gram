package world

import (
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	ilook "github.com/kjkrol/gram/plugins/world/internal/look"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
)

// a sprite faced four ways (20..23), its frames per way while moving (30.., 40..)
func facedAtlas(moving bool) *Atlas {
	a := &Atlas{
		faced:    map[render.SpriteID][]render.SpriteID{1: {20, 21, 22, 23}},
		animated: map[render.SpriteID]animation{},
		moving:   map[render.SpriteID]bool{},
	}
	for k, way := range a.faced[1] {
		base := render.SpriteID(30 + 10*k)
		a.animated[way] = animation{frames: []render.SpriteID{way, base + 1, base + 2},
			loops: render.Loops{N: 3, Period: time.Second}}
		a.moving[way] = moving
	}
	return a
}

func TestAtlas_FramesFollowTheWayTheSpriteFaces(t *testing.T) {
	a := facedAtlas(false)
	for dir, want := range map[geom.Vec]render.SpriteID{
		geom.NewVec(1, 0):  31, // east, the second frame at one period
		geom.NewVec(0, -1): 41, // north
		geom.NewVec(-1, 0): 51,
		geom.NewVec(0, 1):  61,
	} {
		look := drawThrough(t, nil, a.rules(func() time.Duration { return time.Second }), dir, quarters[0])
		if len(look.sprites) != 1 || look.sprites[0] != want {
			t.Errorf("facing %v drew %v, want %v", dir, look.sprites, want)
		}
	}
}

func TestAtlas_WhileMovingStandsAsItFacesAndStepsOnTheMove(t *testing.T) {
	a := facedAtlas(true)
	rules := a.rules(func() time.Duration { return time.Second })
	if look := drawThrough(t, nil, rules, geom.NewVec(0, 1), quarters[0]); len(look.sprites) != 1 || look.sprites[0] != 23 {
		t.Errorf("standing, facing south: drew %v, want the facing sprite 23", look.sprites)
	}
	drawSpeed = 5
	defer func() { drawSpeed = 0 }()
	if look := drawThrough(t, nil, rules, geom.NewVec(0, 1), quarters[0]); len(look.sprites) != 1 || look.sprites[0] != 61 {
		t.Errorf("walking south: drew %v, want the frame 61", look.sprites)
	}
}

func TestAtlas_ALookUnderAnEffectFacesEveryWayAndRunsItsFrames(t *testing.T) {
	w := NewPlugin(Config{
		Space:    SpaceCfg{Width: 100, Height: 100},
		Entities: EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	w.write.Effects().Define("pulling", effect.Spec{effect.Lasts(time.Second)})
	w.write.Effects().Define("bored", effect.Spec{})
	pulling, bored := w.write.Effects().Named("pulling"), w.write.Effects().Named("bored")
	hero := w.Kinds().NewSprite()
	blank := func(float64, int) render.SpriteDrawer { return render.Solid(color.RGBA{}) }
	a := w.NewAtlas()
	a.Add(hero, 8, render.Solid(color.RGBA{})).
		Facing(8, func(float64) render.SpriteDrawer { return render.Solid(color.RGBA{}) }).
		Animated(4, 100*time.Millisecond, blank).WhileMoving().
		Under(bored, render.LoopedFacing(2, 200*time.Millisecond, blank)).
		Under(pulling, render.PhasedFacing(3, blank))
	if n := len(a.faced[hero]); n != 8 {
		t.Fatalf("the hero faces %d ways, want 8", n)
	}
	for _, way := range a.faced[hero] {
		if an := a.animated[way]; len(an.frames) != 4 || an.frames[0] == way || !a.moving[way] {
			t.Errorf("way %d: frames %v, moving %v; want 4 of their own, while moving",
				way, an.frames, a.moving[way])
		}
	}
	for _, e := range []effect.Effect{bored, pulling} {
		twin := a.twins[e.Mark()][hero]
		ways := a.faced[twin]
		if len(ways) != 8 {
			t.Fatalf("under effect %d the twin faces %d ways, want 8", e.Mark(), len(ways))
		}
		for _, way := range ways {
			if e == bored {
				if an := a.animated[way]; len(an.frames) != 2 || an.frames[0] != way {
					t.Errorf("bored, way %d: frames %v, want 2 looped from the way", way, an.frames)
				}
				continue
			}
			if ph := a.phased[way]; len(ph.frames) != 3 || ph.frames[0] != way || ph.lasts != time.Second {
				t.Errorf("pulling, way %d: %+v, want 3 frames along a second", way, ph)
			}
		}
	}
	if len(a.marks) != 2 || a.marks[0] != bored.Mark() || a.marks[1] != pulling.Mark() {
		t.Errorf("the effects looked under come %v, want bored then pulling: the later wins", a.marks)
	}
	kin := 0
	a.kin(hero, func(render.SpriteID) { kin++ })
	// the hero, 8 ways, 8 x 4 frames, 2 twins, 8 ways x 2 and 8 ways x 3 under them
	if want := 1 + 8 + 8*4 + 2 + 8*2 + 8*3; kin != want {
		t.Errorf("the hero's kin counts %d sprites, want %d", kin, want)
	}
}

func TestAtlas_UnderTwoEffectsTheLookDeclaredLaterIsDrawn(t *testing.T) {
	w := NewPlugin(Config{
		Space:    SpaceCfg{Width: 100, Height: 100},
		Entities: EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	w.write.Effects().Define("blinking", effect.Spec{})
	w.write.Effects().Define("stamp", effect.Spec{})
	blinking, stamp := w.write.Effects().Named("blinking"), w.write.Effects().Named("stamp")
	hero := w.Kinds().NewSprite()
	blank := render.Solid(color.RGBA{})
	a := w.NewAtlas()
	a.Add(hero, 8, blank).Under(blinking, blank).Under(stamp, blank)
	var drawing Rules
	if err := drawing.Add(a.swaps()...); err != nil {
		t.Fatal(err)
	}
	var look goke.Comp[render.Appearance]
	var marks goke.Comp[tag.Tags[effect.States]]
	var q *goke.Query
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&look, &marks)
		f.Create(1)
		for f.Next() {
			look.Slice(&f.Cursor)[0] = render.Appearance{SpriteID: hero}
		}
		qb := si.NewQueryBuilder(&look, &marks)
		Own(&drawing, &marks)
		drawing.Bind(qb)
		q = qb.Build()
	}})
	drawn := func(under ...effect.Effect) render.SpriteID {
		q.All()
		q.Next()
		cur := q.Cursor()
		var m tag.Tags[effect.States]
		for _, e := range under {
			m = m.With(e.Mark())
		}
		marks.Slice(cur)[0] = m
		layers := [][]render.Appearance{{look.Slice(cur)[0]}}
		drawing.Run(cur, layers, nil)
		return layers[0][0].SpriteID
	}
	if got := drawn(blinking, stamp); got != a.twins[stamp.Mark()][hero] {
		t.Errorf("under both, sprite %d drawn, want stamp's look %d: the later declared wins", got,
			a.twins[stamp.Mark()][hero])
	}
	if got := drawn(blinking); got != a.twins[blinking.Mark()][hero] {
		t.Errorf("under blinking alone, sprite %d drawn, want its look", got)
	}
	if got := drawn(); got != hero {
		t.Errorf("under nothing, sprite %d drawn, want the hero's own", got)
	}
}

type outlined struct{ width int }

func (outlined) Style() {}

func TestAtlas_StyledIsReadOffTheWorldsStylesOnceTheRendererIsMade(t *testing.T) {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 100, Height: 100},
		Entities: EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	a := p.NewAtlas()
	id := p.Kinds().NewSprite()
	slot := a.Add(id, 4, render.Solid(color.RGBA{})).Styled(outlined{width: 3})
	if p.read.Styles() != nil {
		t.Error("the world has styles before its renderer is made")
	}
	a.Close()
	if got, ok := render.StyleOf[outlined](p.read.Styles(), id); !ok || got.width != 3 {
		t.Errorf("the style of sprite %d is %v, %v; want width 3", id, got, ok)
	}
	defer func() {
		if recover() == nil {
			t.Error("a style declared after the renderer was made was taken")
		}
	}()
	slot.Styled(outlined{width: 5})
}

// leveledAtlas is a world's atlas with a kind of its own look, declared as declare says, and
// another drawn by a material, closed; the kind's look, red, and the material's sprite
func leveledAtlas(declare func(Slot)) (a *Atlas, own, shaded render.SpriteID) {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 100, Height: 100},
		Entities: EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	own, shaded = p.Kinds().NewSprite(), p.Kinds().NewSprite()
	a = p.NewAtlas()
	declare(a.Add(own, 8, render.Solid(color.RGBA{R: 255, A: 255})))
	a.Add(shaded, 8, render.MaterialID(3))
	a.Close()
	return a, own, shaded
}

func TestAtlas_Close_GivesEveryKindItsLookAtEachLevel(t *testing.T) {
	flat := render.Solid(color.RGBA{G: 255, A: 255})
	dot := render.Solid(color.RGBA{B: 255, A: 255})
	// L0 to L6: own the kind's look, blob the one derived of it, x and y the looks declared in
	// turn, - none
	for _, tc := range []struct {
		name    string
		declare func(Slot)
		want    string
	}{
		{"nothing declared", func(Slot) {}, "own own own own blob blob -"},
		{"at L3", func(s Slot) { s.At(lod.L3, flat) }, "own own own x x x -"},
		{"at L4", func(s Slot) { s.At(lod.L4, flat) }, "own own own own x x -"},
		{"at L5", func(s Slot) { s.At(lod.L5, flat) }, "own own own own blob x -"},
		{"nothing from L3", func(s Slot) { s.Nothing(lod.L3) }, "own own own - - - -"},
		{"at L3, nothing from L5", func(s Slot) { s.At(lod.L3, flat).Nothing(lod.L5) },
			"own own own x x - -"},
		{"at L4, then L3", func(s Slot) { s.At(lod.L4, dot).At(lod.L3, flat) },
			"own own own y x x -"},
		{"at L0", func(s Slot) { s.At(lod.L0, flat) }, "x x own own blob blob -"},
		{"at L1", func(s Slot) { s.At(lod.L1, flat) }, "own x own own blob blob -"},
		{"at L0, then L1", func(s Slot) { s.At(lod.L0, flat).At(lod.L1, dot) },
			"x y own own blob blob -"},
		{"nothing at L0", func(s Slot) { s.Nothing(lod.L0) }, "- - own own blob blob -"},
		{"at L0 and L4", func(s Slot) { s.At(lod.L0, flat).At(lod.L4, dot) },
			"x x own own y y -"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, own, _ := leveledAtlas(tc.declare)
			// the kinds' sprites first, then the looks declared, then the blob
			ids := map[string]render.SpriteID{"own": own, "x": own + 2, "y": own + 3}
			blob := render.SpriteID(0)
			for l, w := range strings.Fields(tc.want) {
				v := a.versions[own][l]
				switch {
				case w == "-":
					if v.drawn {
						t.Errorf("at L%d sprite %d drawn, want nothing", l, v.id)
					}
				case w == "blob":
					if !v.drawn || v.id == own || blob != 0 && v.id != blob {
						t.Errorf("at L%d %+v, want a blob of the kind's own", l, v)
					}
					blob = v.id
				case !v.drawn || v.id != ids[w]:
					t.Errorf("at L%d %+v, want %s, sprite %d", l, v, w, ids[w])
				}
			}
			if blob != 0 {
				a.UV(blob) // a blob nobody drew has no place on the sheet: a panic
			}
		})
	}
}

func TestAtlas_Close_DrawsAMaterialItselfToL5(t *testing.T) {
	a, _, shaded := leveledAtlas(func(Slot) {})
	self := version{id: shaded, drawn: true}
	want := [lod.Levels]version{self, self, self, self, self, self}
	if got := a.versions[shaded]; got != want {
		t.Errorf("the material's versions %+v, want itself to L5 and nothing past", got)
	}
}

func TestAtlas_Close_TakesOneSpriteABlob(t *testing.T) {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 100, Height: 100},
		Entities: EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	p.write.Effects().Define("wet", effect.Spec{})
	wet := p.write.Effects().Named("wet")
	red := render.Solid(color.RGBA{R: 255, A: 255})
	a := p.NewAtlas()
	a.Add(p.Kinds().NewSprite(), 8, red).Under(wet, red).
		Facing(4, func(float64) render.SpriteDrawer { return red })
	a.Add(p.Kinds().NewSprite(), 8, red).At(lod.L4, red)
	a.Add(p.Kinds().NewSprite(), 8, render.MaterialID(3))
	before := p.Kinds().NewSprite()
	a.Close()
	// the first kind's blob alone: the second has its L4 declared, the third is a material
	if took := p.Kinds().NewSprite() - before - 1; took != 1 {
		t.Errorf("closing took %d sprites, want 1", took)
	}
}

func TestAtlas_Close_DrawsEverySpriteInAKindsPlaceAsTheKindAtALevel(t *testing.T) {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 100, Height: 100},
		Entities: EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	p.write.Effects().Define("wet", effect.Spec{})
	wet := p.write.Effects().Named("wet")
	red := render.Solid(color.RGBA{R: 255, A: 255})
	hero, deck := p.Kinds().NewSprite(), p.Kinds().NewSprite()
	a := p.NewAtlas()
	a.Add(hero, 8, red).Facing(4, func(float64) render.SpriteDrawer { return red }).
		Under(wet, red).At(lod.L3, red).Nothing(lod.L5)
	a.Add(deck, 8, red).Under(wet, red).Lying()
	a.Close()
	flat := a.versions[hero][lod.L3].id
	ways, soaked := a.faced[hero], a.twins[wet.Mark()][hero]
	for _, id := range append([]render.SpriteID{hero, soaked}, ways...) {
		for l, want := range []render.SpriteID{id, id, id, flat, flat} {
			if got, ok := a.levels.at(id, lod.Level(l)); !ok || got != want {
				t.Errorf("sprite %d at L%d drawn as %d, %v; want %d", id, l, got, ok, want)
			}
		}
		for _, l := range []lod.Level{lod.L5, lod.L6} {
			if got, ok := a.levels.at(id, l); ok {
				t.Errorf("sprite %d at L%d drawn as %d, want nothing", id, l, got)
			}
		}
	}
	blob := a.versions[deck][lod.L4].id
	if got, ok := a.levels.at(a.twins[wet.Mark()][deck], lod.L4); !ok || got != blob {
		t.Errorf("the wet deck at L4 drawn as %d, %v; want the deck's blob %d", got, ok, blob)
	}
	if !a.lain()[blob] {
		t.Error("the blob of a lying kind stands, want it lying")
	}
}

func TestAtlas_Close_KeepsAKindAlwaysInEverySpriteDrawnInItsPlace(t *testing.T) {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 100, Height: 100},
		Entities: EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	p.write.Effects().Define("wet", effect.Spec{})
	wet := p.write.Effects().Named("wet")
	red := render.Solid(color.RGBA{R: 255, A: 255})
	hero, deck := p.Kinds().NewSprite(), p.Kinds().NewSprite()
	a := p.NewAtlas()
	slot := a.Add(hero, 8, red).Facing(4, func(float64) render.SpriteDrawer { return red }).
		Under(wet, red).At(lod.L3, red).Always()
	a.Add(deck, 8, red).Under(wet, red)
	a.Close()
	kept := append([]render.SpriteID{hero, a.twins[wet.Mark()][hero], a.versions[hero][lod.L3].id},
		a.faced[hero]...)
	for _, id := range kept {
		if !a.isAlways(id) {
			t.Errorf("sprite %d in the hero's place is thinned, want it always kept", id)
		}
	}
	thinned := []render.SpriteID{deck, a.twins[wet.Mark()][deck], a.versions[deck][lod.L4].id,
		p.Kinds().NewSprite()}
	for _, id := range thinned {
		if a.isAlways(id) {
			t.Errorf("sprite %d is always kept, want it thinned as the deck is", id)
		}
	}
	defer func() {
		if recover() == nil {
			t.Error("a kind kept from thinning after the renderer was made was taken")
		}
	}()
	slot.Always()
}

func TestAtlas_RefusesALookAtALevelThatCannotBe(t *testing.T) {
	red := render.Solid(color.RGBA{R: 255, A: 255})
	for name, declare := range map[string]func(s Slot, a *Atlas){
		"at L2":         func(s Slot, _ *Atlas) { s.At(lod.L2, red) },
		"at L6":         func(s Slot, _ *Atlas) { s.At(lod.L6, red) },
		"nothing at L6": func(s Slot, _ *Atlas) { s.Nothing(lod.L6) },
		"twice":         func(s Slot, _ *Atlas) { s.At(lod.L3, red).Nothing(lod.L3) },
		"timed": func(s Slot, _ *Atlas) {
			s.At(lod.L3, render.Phased(2, func(int) render.SpriteDrawer { return red }))
		},
		"after Close": func(s Slot, a *Atlas) { a.Close(); s.Nothing(lod.L4) },
	} {
		t.Run(name, func(t *testing.T) {
			p := NewPlugin(Config{
				Space:    SpaceCfg{Width: 100, Height: 100},
				Entities: EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
			})
			a := p.NewAtlas()
			s := a.Add(p.Kinds().NewSprite(), 8, red)
			defer func() {
				if recover() == nil {
					t.Error("taken without a panic")
				}
			}()
			declare(s, a)
		})
	}
}

func TestAtlas_GivesAKindItsFarColourWhateverSpriteItIsDrawnIn(t *testing.T) {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 100, Height: 100},
		Entities: EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	a := p.NewAtlas()
	walker, flag := p.Kinds().NewSprite(), p.Kinds().NewSprite()
	halves := func(dst *render.Canvas, size int) {
		dst.FillRect(0, 0, float32(size)/2, float32(size), color.RGBA{R: 255, A: 255})
		dst.FillRect(float32(size)/2, 0, float32(size)/2, float32(size),
			color.RGBA{B: 255, A: 255})
	}
	a.Add(walker, 8, halves).Facing(4, func(float64) render.SpriteDrawer {
		return render.Solid(color.RGBA{G: 255, A: 255})
	})
	a.Add(flag, 8, halves).At(lod.L4, render.Solid(color.RGBA{G: 255, A: 255}))
	a.Close()
	want := [4]uint8{128, 0, 128, 255}
	sprites := append([]render.SpriteID{walker, a.versions[walker][lod.L4].id}, a.faced[walker]...)
	for _, id := range sprites {
		if c, ok := a.farColour(id); !ok || c != want {
			t.Errorf("the sprite %d's colour %v, %v; want its kind's blob's %v", id, c, ok, want)
		}
	}
	if c, ok := a.farColour(flag); !ok || c != [4]uint8{0, 255, 0, 255} {
		t.Errorf("a kind declaring its look at L4 has the colour %v, %v; want that look's green",
			c, ok)
	}
}

// recording is a look recording the sprite, the box and the Z of every sprite it is handed
type recording struct {
	Look
	sprites []render.SpriteID
	boxes   []plane.AABB
	zs      []Z
}

func (r *recording) Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, z Z,
	atlas render.AtlasSource, a render.Appearance, lit render.Illumination) {
	r.sprites = append(r.sprites, a.SpriteID)
	r.boxes = append(r.boxes, box)
	r.zs = append(r.zs, z)
	r.Look.Sprite(f, cam, box, z, atlas, a, lit)
}

// composedAt composes an entity of sprite from a over box at z through look, a world 64 by 32
// seen at level everywhere
func composedAt(t *testing.T, a *Atlas, sprite render.SpriteID, level lod.Level,
	box plane.AABB, z Z, look Look) *render.Frame {
	t.Helper()
	cam := cameratest.NewFromSpace(64, 32, 0)
	v := view.NewSeen(cam, everywhere{w: 64, h: 32, level: level})
	r := newRenderer(a, seenBy(cam, v), &Rules{}, func() Look { return look })
	readied(a)(r)
	r.space = SpaceCfg{Width: 64, Height: 32}
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 64, Height: 32, BucketSize: 16})
	if err != nil {
		t.Fatal(err)
	}
	var base goke.Comp[Base]
	var appearance goke.Comp[render.Appearance]
	var zs goke.Comp[Z]
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&base, &appearance, &zs)
		f.Create(1)
		f.Next()
		base.Slice(&f.Cursor)[0] = Base{Pos: Position{AABB: box}}
		appearance.Slice(&f.Cursor)[0] = render.Appearance{SpriteID: sprite}
		zs.Slice(&f.Cursor)[0] = z
		space.Rebuild([]aabbworld.Item{{ID: f.Cursor.IDs[0], Box: box}})
		r.Init(si)
	}})
	views := []*view.View{v}
	view.NewSystem(space, &views, 64, 32).Update(nil, 0)
	return composed(r, cam)
}

// what the seams between levels rest on: each level's version is drawn over the entity's box at
// its Z, through the Look or as a point
func TestAtlas_DrawsEachLevelsVersionOverTheEntitysBoxAtItsZ(t *testing.T) {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 64, Height: 32},
		Entities: EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	plain, atL4, atL5 := p.Kinds().NewSprite(), p.Kinds().NewSprite(), p.Kinds().NewSprite()
	red := render.Solid(color.RGBA{R: 255, A: 255})
	a := p.NewAtlas()
	a.Add(plain, 8, red)
	a.Add(atL4, 8, red).At(lod.L4, render.Solid(color.RGBA{G: 255, A: 255}))
	a.Add(atL5, 8, red).At(lod.L5, render.Solid(color.RGBA{B: 255, A: 255}))
	a.Close()
	box := plane.NewAABB(geom.NewVec(20, 12), 10, 6)
	z := Z{Altitude: 30, Height: 8}
	for _, c := range []struct {
		name   string
		sprite render.SpriteID
		level  lod.Level
		point  bool
	}{
		{"its own at L2", plain, lod.L2, false},
		{"its own at L3", plain, lod.L3, false},
		{"its blob at L4", plain, lod.L4, true},
		{"its blob at L5", plain, lod.L5, true},
		{"declared at L4", atL4, lod.L4, true},
		{"declared at L4, at L5", atL4, lod.L5, false},
		{"its blob at L4, declared at L5", atL5, lod.L4, true},
		{"declared at L5", atL5, lod.L5, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			look := &recording{Look: ilook.NewFlat(64, 32, false, false)}
			f := composedAt(t, a, c.sprite, c.level, box, z, look)
			want, _ := a.levels.at(c.sprite, c.level)
			switch {
			case c.point && len(look.sprites) != 0:
				t.Errorf("the look drew %v, want a point alone", look.sprites)
			case !c.point && (len(look.sprites) != 1 || look.sprites[0] != want ||
				look.boxes[0] != box || look.zs[0] != z):
				t.Errorf("the look drew %v over %v at %v, want sprite %d over %v at %+v",
					look.sprites, look.boxes, look.zs, want, box, z)
			}
			if c.point {
				lowest := z.Altitude + FigureLift
				f.Places(func(_ render.Pass, at render.At) {
					if at.Z() != lowest {
						t.Errorf("the point drawn among the figures at %v, want at %v", at.Z(),
							lowest)
					}
				})
				if places(f) != 1 {
					t.Errorf("%d places asked for, want the point's", places(f))
				}
			}
			cam := cameratest.NewFromSpace(64, 32, 0)
			cam.SetViewport(64, 32)
			pix := pixelsAt(t, cam, 64, 32, 0, c.level, a, c.sprite, box)
			for y := range 32 {
				for x := range 64 {
					in := x >= 20 && x < 30 && y >= 12 && y < 18
					if px := pix(x, y); !in && px[3] != 0 {
						t.Fatalf("pixel (%d, %d) is %v, want nothing drawn off the box", x, y, px)
					}
				}
			}
			if px := pix(25, 15); px[3] == 0 {
				t.Errorf("the box's middle is %v, want its version drawn there", px)
			}
		})
	}
}
