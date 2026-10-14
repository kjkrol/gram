package world

import (
	"image/color"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
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
	w.Effects().Define("pulling", effect.Spec{effect.Lasts(time.Second)})
	w.Effects().Define("bored", effect.Spec{})
	pulling, bored := w.Effects().Named("pulling"), w.Effects().Named("bored")
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
	w.Effects().Define("blinking", effect.Spec{})
	w.Effects().Define("stamp", effect.Spec{})
	blinking, stamp := w.Effects().Named("blinking"), w.Effects().Named("stamp")
	hero := w.Kinds().NewSprite()
	blank := render.Solid(color.RGBA{})
	a := w.NewAtlas()
	a.Add(hero, 8, blank).Under(blinking, blank).Under(stamp, blank)
	var drawing render.Rules
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
		render.Own(&drawing, &marks)
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
	if p.Styles() != nil {
		t.Error("the world has styles before its renderer is made")
	}
	a.Close()
	p.WithRenderer(a)
	if got, ok := render.StyleOf[outlined](p.Styles(), id); !ok || got.width != 3 {
		t.Errorf("the style of sprite %d is %v, %v; want width 3", id, got, ok)
	}
	defer func() {
		if recover() == nil {
			t.Error("a style declared after the renderer was made was taken")
		}
	}()
	slot.Styled(outlined{width: 5})
}
