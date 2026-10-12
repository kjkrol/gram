package world

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	ilook "github.com/kjkrol/gram/plugins/world/internal/look"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

type counting struct {
	Look
	sprites []render.SpriteID
	angles  []float32
	frame   *render.Frame
}

func (c *counting) Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, z Z, atlas render.AtlasSource, a render.Appearance, light render.Light) {
	c.sprites = append(c.sprites, a.SpriteID)
	c.angles = append(c.angles, a.Angle)
	c.Look.Sprite(f, cam, box, z, atlas, a, light)
}

type lyingCounting struct {
	*counting
	lain  []render.SpriteID
	along []laid
}

type laid struct {
	sprite, edge render.SpriteID
	dir          geom.Vec
	lie          Lie
}

func (c *lyingCounting) Lie(_ *render.Frame, _ camera.Camera, _ plane.AABB, _ Z,
	_ render.AtlasSource, a render.Appearance, _ render.Light) {
	c.lain = append(c.lain, a.SpriteID)
}

func (c *lyingCounting) Lay(_ *render.Frame, _ camera.Camera, _ plane.AABB, dir geom.Vec,
	lie Lie, _ Z, _ render.AtlasSource, a render.Appearance, edge render.SpriteID,
	_ render.Light) {
	c.along = append(c.along, laid{a.SpriteID, edge, dir, lie})
}

type flatAtlas struct{}

func (flatAtlas) Atlas() *render.Image                            { return nil }
func (flatAtlas) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 1, 1 }
func (flatAtlas) White() (u, v float32)                           { return 0, 0 }

func drawThrough(t *testing.T, pick func(ids []uid.UID64, v *view.View), rules []render.Rule, dir geom.Vec, at ...geom.Vec) *counting {
	return drawShaded(t, nil, pick, rules, dir, at...)
}

func drawShaded(t *testing.T, shaded map[render.SpriteID]render.MaterialID, pick func(ids []uid.UID64, v *view.View), rules []render.Rule, dir geom.Vec, at ...geom.Vec) *counting {
	look := &counting{Look: ilook.NewFlat(1000, 1000)}
	drawAs(t, look, func(r *renderer) { r.shade(shaded) }, pick, rules, dir, at...)
	return look
}

func drawAs(t *testing.T, look Look, ready func(*renderer), pick func(ids []uid.UID64,
	v *view.View), rules []render.Rule, dir geom.Vec, at ...geom.Vec) *render.Frame {
	t.Helper()
	v := &view.View{}
	cam := icamera.NewFromSpace(1000, 1000, 0)
	var drawing render.Rules
	if err := drawing.Add(rules...); err != nil {
		t.Fatal(err)
	}
	r := newRenderer(flatAtlas{}, func(camera.Camera) *view.View { return v }, &drawing, func() Look { return look })
	ready(r)

	var base goke.Comp[Base]
	var appearance goke.Comp[render.Appearance]
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&base, &appearance)
		f.Create(len(at))
		var ids []uid.UID64
		i := 0
		for f.Next() {
			bases, looks := base.Slice(&f.Cursor), appearance.Slice(&f.Cursor)
			for j, id := range f.Cursor.IDs {
				bases[j] = Base{Pos: Position{AABB: plane.NewAABB(at[i], 10, 10)}, Vel: Velocity{Dir: dir}}
				looks[j] = render.Appearance{SpriteID: 1}
				ids = append(ids, id)
				i++
			}
		}
		if pick != nil {
			pick(ids, v)
		}
		r.Init(si)
	}})

	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	if c, ok := look.(*counting); ok {
		c.frame = &f
	}
	return &f
}

var quarters = []geom.Vec{geom.NewVec(100, 100), geom.NewVec(700, 100), geom.NewVec(100, 700), geom.NewVec(700, 700)}

func TestRenderer_Compose_DrawsOnlyWhatTheViewContains(t *testing.T) {
	firstOnly := func(ids []uid.UID64, v *view.View) {
		v.Culled = true
		v.In.Add(ids[0])
	}
	if drawn := drawThrough(t, firstOnly, nil, geom.NewVec(1, 0), quarters...).sprites; len(drawn) != 1 {
		t.Errorf("a View holding one entity drew %d sprites, want 1", len(drawn))
	}
	if drawn := drawThrough(t, nil, nil, geom.NewVec(1, 0), quarters...).sprites; len(drawn) != 4 {
		t.Errorf("the zero View drew %d sprites, want all 4", len(drawn))
	}
}

func TestRenderer_Compose_DrawsAsTheRulesSay(t *testing.T) {
	const east, crown render.SpriteID = 5, 9
	rules := []render.Rule{
		render.With(func(a render.Appearance, b Base) render.Appearance { a.SpriteID = east; return a }),
		render.Over[render.Appearance](render.Appearance{SpriteID: crown}),
		render.Show(func(b Base) bool { return b.Pos.AABB.TopLeft.X < 500 }),
	}
	drawn := drawThrough(t, nil, rules, geom.NewVec(1, 0), quarters...).sprites
	if len(drawn) != 4 {
		t.Fatalf("drew %v, want the two on the left, each turned east under a crown", drawn)
	}
	for i := 0; i < len(drawn); i += 2 {
		if drawn[i] != east || drawn[i+1] != crown {
			t.Errorf("drew %v, want %v under %v for each", drawn, east, crown)
		}
	}
}

func TestWithRenderer_TakesTheEffectsLooks(t *testing.T) {
	p := testPlugin()
	p.Effects().Define("frozen", effect.Spec{})
	frozen := p.Effects().Named("frozen")
	p.Effects().Define("wet", effect.Spec{})
	wet := p.Effects().Named("wet")
	free := p.Kinds().NewSprite()
	if got := frozen.Look(0); got != free+1 {
		t.Errorf("frozen's look = %d, want the world's next slot %d", got, free+1)
	}
	p.WithRenderer(flatAtlas{})
	p.WithRenderer(flatAtlas{})
	defer func() {
		if recover() == nil {
			t.Error("the first look of wet after WithRenderer did not panic")
		}
	}()
	wet.Look(0)
}

func TestRenderer_Compose_ATurnedSpriteFollowsItsWay(t *testing.T) {
	for _, tc := range []struct {
		name string
		dir  geom.Vec
		want float32
	}{
		{"east", geom.NewVec(1, 0), 0},
		{"north, up the screen", geom.NewVec(0, -1), 90},
		{"west", geom.NewVec(-1, 0), -180}, // atan2's range: the same turn as 180
		{"still: the Appearance's own angle holds", geom.Vec{}, 0},
	} {
		a := &Atlas{turned: map[render.SpriteID]bool{1: true}, unders: map[render.SpriteID][]render.SpriteID{}}
		look := drawThrough(t, nil, a.rules(func() time.Duration { return 0 }), tc.dir, quarters[0])
		if len(look.angles) != 1 || look.angles[0] != tc.want {
			t.Errorf("%s: drew angles %v, want [%v]", tc.name, look.angles, tc.want)
		}
	}
}

func TestRenderer_Compose_AShadedSpriteIsOneMaterialQuad(t *testing.T) {
	look := drawShaded(t, map[render.SpriteID]render.MaterialID{1: 7}, nil, nil, geom.NewVec(0, -1), quarters[0])
	if len(look.sprites) != 0 {
		t.Fatalf("a shaded entity still drew sprites %v, want none", look.sprites)
	}
	quads := 0
	look.frame.Each(func(_ render.Tier, _ float32, verts []render.Vertex) {
		quads++
		v := verts[0]
		if v.ColorA <= 1.5 {
			t.Fatalf("the piece is no material: alpha %v", v.ColorA)
		}
		cx, cy := float32(quarters[0].X+5), float32(quarters[0].Y+5)
		if v.Custom0 != cx || v.Custom1 != cy || v.Custom2 != 5 {
			t.Errorf("Custom = (%v, %v, %v), want the middle (%v, %v) and half the side 5", v.Custom0, v.Custom1, v.Custom2, cx, cy)
		}
		if north := float32(math.Pi / 2); v.Custom3 != north {
			t.Errorf("Custom3 = %v, want %v — headed up the screen", v.Custom3, north)
		}
	})
	if quads != 1 {
		t.Fatalf("laid %d pieces, want the one quad", quads)
	}
}

func TestRenderer_Compose_AnAnimatedSpriteShowsItsFrameOfTheClock(t *testing.T) {
	const period = 180 * time.Millisecond
	frames := []render.SpriteID{10, 11, 12}
	for now, want := range map[time.Duration]render.SpriteID{
		0:          10,
		period:     11,
		2 * period: 12,
		3 * period: 10, // round again
	} {
		a := &Atlas{animated: map[render.SpriteID]animation{1: {frames: frames, period: period}}}
		look := drawThrough(t, nil, a.rules(func() time.Duration { return now }), geom.NewVec(1, 0), quarters[0])
		if len(look.sprites) != 1 || look.sprites[0] != want {
			t.Errorf("at %v drew %v, want frame %v", now, look.sprites, want)
		}
	}
}

func TestRenderer_Compose_ALyingSpriteLiesWhereTheLookLaysIt(t *testing.T) {
	const deck, wet render.SpriteID = 1, 4
	a := &Atlas{
		lying:  map[render.SpriteID]bool{deck: true},
		unders: map[render.SpriteID][]render.SpriteID{deck: {wet}},
	}
	under := render.With(func(ap render.Appearance, b Base) render.Appearance {
		if b.Pos.AABB.TopLeft.X > 500 {
			ap.SpriteID = wet
		}
		return ap
	})
	lies := &lyingCounting{counting: &counting{Look: ilook.NewFlat(1000, 1000)}}
	drawAs(t, lies, func(r *renderer) { r.lie(a.lain(), nil, nil) }, nil, []render.Rule{under},
		geom.Vec{},
		quarters[0], quarters[1])
	if len(lies.lain) != 2 || len(lies.sprites) != 0 {
		t.Errorf("laid %v and stood %v, want the deck and its wet look laid", lies.lain,
			lies.sprites)
	}
	stands := &counting{Look: ilook.NewFlat(1000, 1000)}
	drawAs(t, stands, func(r *renderer) { r.lie(a.lain(), nil, nil) }, nil, nil, geom.Vec{}, quarters[0])
	if len(stands.sprites) != 1 {
		t.Errorf("a look laying nothing drew %v, want the deck as a sprite", stands.sprites)
	}
}

type uprightCounting struct {
	*counting
	stood []render.SpriteID
}

func (c *uprightCounting) Upright(_ *render.Frame, _ camera.Camera, _ plane.AABB, _ Z,
	_ render.AtlasSource, a render.Appearance, _ render.Light) {
	c.stood = append(c.stood, a.SpriteID)
}

func TestRenderer_Compose_AnUprightSpriteStandsTrue(t *testing.T) {
	standing := map[render.SpriteID]bool{1: true}
	stands := &uprightCounting{counting: &counting{Look: ilook.NewFlat(1000, 1000)}}
	drawAs(t, stands, func(r *renderer) { r.lie(nil, nil, standing) }, nil, nil, geom.Vec{},
		quarters[0])
	if len(stands.stood) != 1 || len(stands.sprites) != 0 {
		t.Errorf("stood %v and drew %v, want the one stood true", stands.stood, stands.sprites)
	}
	flat := &counting{Look: ilook.NewFlat(1000, 1000)}
	drawAs(t, flat, func(r *renderer) { r.lie(nil, nil, standing) }, nil, nil, geom.Vec{},
		quarters[0])
	if len(flat.sprites) != 1 {
		t.Errorf("a look standing nothing true drew %v, want it as a sprite", flat.sprites)
	}
}

func TestRenderer_Compose_AnEntityWithALieIsLaidAlongIt(t *testing.T) {
	const deck, plank, rail render.SpriteID = 1, 2, 9
	lain := map[render.SpriteID]bool{deck: true, plank: true}
	edges := map[render.SpriteID]render.SpriteID{deck: rail}
	look := &lyingCounting{counting: &counting{Look: ilook.NewFlat(1000, 1000)}}
	v := &view.View{}
	cam := icamera.NewFromSpace(1000, 1000, 0)
	var drawing render.Rules
	r := newRenderer(flatAtlas{}, func(camera.Camera) *view.View { return v }, &drawing,
		func() Look { return look })
	r.lie(lain, edges, nil)
	lie := Lie{Length: 40, Width: 10, Edged: true, Edge: 2}
	south := geom.NewVec(0, 1)
	var base goke.Comp[Base]
	var appearance goke.Comp[render.Appearance]
	var lies goke.Comp[Lie]
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&base, &appearance, &lies)
		f.Create(2)
		for f.Next() {
			bases, looks := base.Slice(&f.Cursor), appearance.Slice(&f.Cursor)
			ls := lies.Slice(&f.Cursor)
			for j := range f.Cursor.IDs {
				bases[j] = Base{Pos: Position{AABB: plane.NewAABB(quarters[j], 10, 10)},
					Vel: Velocity{Dir: south}}
				looks[j] = render.Appearance{SpriteID: deck + render.SpriteID(j)}
				ls[j] = lie
			}
		}
		r.Init(si)
	}})
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	slices.SortFunc(look.along, func(a, b laid) int { return int(a.sprite) - int(b.sprite) })
	plain := lie
	plain.Edged = false
	want := []laid{{deck, rail, south, lie}, {plank, 0, south, plain}}
	if !slices.Equal(look.along, want) || len(look.lain) != 0 {
		t.Errorf("laid along %+v and flat %v, want %+v", look.along, look.lain, want)
	}
}
