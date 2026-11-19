package world

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	ilook "github.com/kjkrol/gram/plugins/world/internal/look"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

type counting struct {
	Look
	sprites []render.SpriteID
	angles  []float32
	sways   []float32
	boxes   []plane.AABB
	frame   *render.Frame
}

func (c *counting) Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, z Z,
	atlas render.AtlasSource, a render.Appearance, lit render.Illumination) {
	c.sprites = append(c.sprites, a.SpriteID)
	c.angles = append(c.angles, a.Angle)
	c.sways = append(c.sways, a.Sway)
	c.boxes = append(c.boxes, box)
	c.Look.Sprite(f, cam, box, z, atlas, a, lit)
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
	_ render.AtlasSource, a render.Appearance, _ render.Illumination) {
	c.lain = append(c.lain, a.SpriteID)
}

func (c *lyingCounting) Lay(_ *render.Frame, _ camera.Camera, _ plane.AABB, dir geom.Vec,
	lie Lie, _ Z, _ render.AtlasSource, a render.Appearance, edge render.SpriteID,
	_ render.Illumination) {
	c.along = append(c.along, laid{a.SpriteID, edge, dir, lie})
}

type flatAtlas struct{}

func (flatAtlas) Atlas() *render.Image                            { return nil }
func (flatAtlas) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 1, 1 }
func (flatAtlas) White() (u, v float32)                           { return 0, 0 }

func drawThrough(t *testing.T, pick func(ids []uid.UID64, v *view.View), rules []Rule, dir geom.Vec, at ...geom.Vec) *counting {
	return drawShaded(t, nil, pick, rules, dir, at...)
}

func drawShaded(t *testing.T, shaded map[render.SpriteID]render.MaterialID, pick func(ids []uid.UID64, v *view.View), rules []Rule, dir geom.Vec, at ...geom.Vec) *counting {
	look := &counting{Look: ilook.NewFlat(1000, 1000, false, false)}
	drawAs(t, look, func(r *renderer) { r.shade(shaded, nil) }, pick, rules, dir, at...)
	return look
}

func drawAs(t *testing.T, look Look, ready func(*renderer), pick func(ids []uid.UID64,
	v *view.View), rules []Rule, dir geom.Vec, at ...geom.Vec) *render.Frame {
	t.Helper()
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	return drawAsThrough(t, cam, look, ready, pick, rules, dir, at...)
}

func drawAsThrough(t *testing.T, cam camera.Camera, look Look, ready func(*renderer),
	pick func(ids []uid.UID64, v *view.View), rules []Rule, dir geom.Vec,
	at ...geom.Vec) *render.Frame {
	t.Helper()
	v := &view.View{}
	var drawing Rules
	if err := drawing.Add(rules...); err != nil {
		t.Fatal(err)
	}
	r := newRenderer(flatAtlas{}, seenBy(cam, v), &drawing, func() Look { return look })
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
				bases[j] = Base{Pos: Position{AABB: plane.NewAABB(at[i], 10, 10)},
					Vel: Velocity{Dir: dir, Value: drawSpeed}}
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

	f := composed(r, cam)
	if c, ok := look.(*counting); ok {
		c.frame = f
	}
	return f
}

// seenBy is the views of a renderer drawing through cam alone, as v sees
func seenBy(cam camera.Camera, v *view.View) func() map[camera.Camera]*view.View {
	return func() map[camera.Camera]*view.View { return map[camera.Camera]*view.View{cam: v} }
}

// composed publishes what r draws and composes it through cam
func composed(r *renderer, cam camera.Camera) *render.Frame {
	r.Publish()
	r.Swap()
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	return &f
}

// the speed the entities drawAs makes walk at, 0 standing
var drawSpeed float32

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
	const east, halo render.SpriteID = 5, 9
	rules := []Rule{
		With(func(a render.Appearance, b Base) render.Appearance { a.SpriteID = east; return a }),
		Over[render.Appearance](render.Appearance{SpriteID: halo}),
		Show(func(b Base) bool { return b.Pos.AABB.TopLeft.X < 500 }),
	}
	drawn := drawThrough(t, nil, rules, geom.NewVec(1, 0), quarters...).sprites
	if len(drawn) != 4 {
		t.Fatalf("drew %v, want the two on the left, each turned east under a halo", drawn)
	}
	for i := 0; i < len(drawn); i += 2 {
		if drawn[i] != east || drawn[i+1] != halo {
			t.Errorf("drew %v, want %v under %v for each", drawn, east, halo)
		}
	}
}

func TestSetAtlas_TakesTheEffectsLooks(t *testing.T) {
	p := testPlugin()
	p.write.Effects().Define("frozen", effect.Spec{})
	frozen := p.write.Effects().Named("frozen")
	p.write.Effects().Define("wet", effect.Spec{})
	wet := p.write.Effects().Named("wet")
	free := p.Kinds().NewSprite()
	a := p.NewAtlas()
	if got := a.twin(frozen, 0); got != free+1 {
		t.Errorf("frozen's look = %d, want the world's next slot %d", got, free+1)
	}
	if again := a.twin(frozen, 0); again != free+1 {
		t.Errorf("frozen's look asked again = %d, want the same %d", again, free+1)
	}
	p.write.SetAtlas(a)
	p.write.SetAtlas(a)
	defer func() {
		if recover() == nil {
			t.Error("the first look of wet after SetAtlas did not panic")
		}
	}()
	a.twin(wet, 0)
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

func TestRenderer_Compose_ASwayingSpritesEntitySwaysAsItsLookSays(t *testing.T) {
	sway := func(of map[render.SpriteID]float32) []float32 {
		look := &counting{Look: ilook.NewFlat(1000, 1000, false, false)}
		drawAs(t, look, func(r *renderer) { r.sways(of) }, nil, nil, geom.Vec{}, quarters[0])
		return look.sways
	}
	if got := sway(nil); len(got) != 1 || got[0] != 0 {
		t.Errorf("an entity of a sprite that does not sway is drawn swaying %v, want [0]", got)
	}
	if got := sway(map[render.SpriteID]float32{1: 0.6}); len(got) != 1 || got[0] != 0.6 {
		t.Errorf("an entity of a sprite swaying 0.6 is drawn swaying %v, want [0.6]", got)
	}
}

func TestAtlas_Swaying_HoldsForTheSpritesKin(t *testing.T) {
	a := &Atlas{
		sway:   map[render.SpriteID]float32{1: 0.4},
		unders: map[render.SpriteID][]render.SpriteID{1: {7}},
	}
	if got := a.swaying(); got[1] != 0.4 || got[7] != 0.4 || len(got) != 2 {
		t.Errorf("a sprite swaying 0.4 and its twin under an effect sway %v, want 0.4 both", got)
	}
}

func TestRenderer_Compose_AShadedSpriteIsOneMaterialQuad(t *testing.T) {
	look := drawShaded(t, map[render.SpriteID]render.MaterialID{1: 7}, nil, nil, geom.NewVec(0, -1), quarters[0])
	if len(look.sprites) != 0 {
		t.Fatalf("a shaded entity still drew sprites %v, want none", look.sprites)
	}
	quads := 0
	look.frame.Each(func(_ render.At, verts []render.Vertex) {
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
		a := &Atlas{animated: map[render.SpriteID]animation{1: {frames: frames,
			loops: render.Loops{N: len(frames), Period: period}}}}
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
	under := With(func(ap render.Appearance, b Base) render.Appearance {
		if b.Pos.AABB.TopLeft.X > 500 {
			ap.SpriteID = wet
		}
		return ap
	})
	lies := &lyingCounting{counting: &counting{Look: ilook.NewFlat(1000, 1000, false, false)}}
	drawAs(t, lies, func(r *renderer) { r.lie(a.lain(), nil, nil) }, nil, []Rule{under},
		geom.Vec{},
		quarters[0], quarters[1])
	if len(lies.lain) != 2 || len(lies.sprites) != 0 {
		t.Errorf("laid %v and stood %v, want the deck and its wet look laid", lies.lain,
			lies.sprites)
	}
	stands := &counting{Look: ilook.NewFlat(1000, 1000, false, false)}
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
	_ render.AtlasSource, a render.Appearance, _ render.Illumination) {
	c.stood = append(c.stood, a.SpriteID)
}

func TestRenderer_Compose_AnUprightSpriteStandsTrue(t *testing.T) {
	standing := map[render.SpriteID]bool{1: true}
	stands := &uprightCounting{counting: &counting{Look: ilook.NewFlat(1000, 1000, false, false)}}
	drawAs(t, stands, func(r *renderer) { r.lie(nil, nil, standing) }, nil, nil, geom.Vec{},
		quarters[0])
	if len(stands.stood) != 1 || len(stands.sprites) != 0 {
		t.Errorf("stood %v and drew %v, want the one stood true", stands.stood, stands.sprites)
	}
	flat := &counting{Look: ilook.NewFlat(1000, 1000, false, false)}
	drawAs(t, flat, func(r *renderer) { r.lie(nil, nil, standing) }, nil, nil, geom.Vec{},
		quarters[0])
	if len(flat.sprites) != 1 {
		t.Errorf("a look standing nothing true drew %v, want it as a sprite", flat.sprites)
	}
}

func TestRenderer_Compose_ACameraInsideAnEntityDrawsAllButIt(t *testing.T) {
	for _, how := range []camera.How{camera.Inside, camera.Behind} {
		cam := cameratest.NewFromSpace(1000, 1000, 0)
		ride := func(ids []uid.UID64, _ *view.View) {
			cam.(camera.Fastenable).Fasten(camera.Fastening{Entity: ids[1], How: how})
		}
		look := &counting{Look: ilook.NewFlat(1000, 1000, false, false)}
		drawAsThrough(t, cam, look, func(*renderer) {}, ride, nil, geom.Vec{}, quarters...)
		drew := slices.ContainsFunc(look.boxes, func(b plane.AABB) bool {
			return b.TopLeft == quarters[1]
		})
		inside, want := how == camera.Inside, len(quarters)
		if inside {
			want--
		}
		if drew == inside || len(look.boxes) != want {
			t.Errorf("fastened %v to the entity at %v, drew %v", how, quarters[1], look.boxes)
		}
	}
}

func TestRenderer_Compose_AnEntityWithALieIsLaidAlongIt(t *testing.T) {
	const deck, plank, rail render.SpriteID = 1, 2, 9
	lain := map[render.SpriteID]bool{deck: true, plank: true}
	edges := map[render.SpriteID]render.SpriteID{deck: rail}
	look := &lyingCounting{counting: &counting{Look: ilook.NewFlat(1000, 1000, false, false)}}
	v := &view.View{}
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	var drawing Rules
	r := newRenderer(flatAtlas{}, seenBy(cam, v), &drawing, func() Look { return look })
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
	composed(r, cam)
	slices.SortFunc(look.along, func(a, b laid) int { return int(a.sprite) - int(b.sprite) })
	plain := lie
	plain.Edged = false
	want := []laid{{deck, rail, south, lie}, {plank, 0, south, plain}}
	if !slices.Equal(look.along, want) || len(look.lain) != 0 {
		t.Errorf("laid along %+v and flat %v, want %+v", look.along, look.lain, want)
	}
}

func TestRenderer_Compose_TheFiguresComeOverWhatLiesBeneathTheLowestOfThem(t *testing.T) {
	f := drawAs(t, ilook.NewFlat(1000, 1000, false, false), func(*renderer) {}, nil, nil,
		geom.NewVec(1, 0), quarters...)
	var places []render.At
	f.Places(func(_ render.Pass, at render.At) { places = append(places, at) })
	if len(places) != 1 || places[0].Z() != FigureLift {
		t.Errorf("the figures are drawn at %v, want once, the figures' lift over the ground",
			places)
	}
	none := drawAs(t, ilook.NewFlat(1000, 1000, false, false), func(*renderer) {}, nil, nil,
		geom.NewVec(1, 0))
	none.Places(func(render.Pass, render.At) {
		t.Error("no figure in sight, yet a place for them")
	})
}

func ptr[T any](v T) *T { return &v }

func TestRenderer_Compose_DrawsNothingAsleepEvenUnculled(t *testing.T) {
	look := &counting{Look: ilook.NewFlat(1000, 1000, false, false)}
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	r := newRenderer(flatAtlas{}, seenBy(cam, &view.View{}), &Rules{}, func() Look { return look })
	r.shade(nil, nil)
	var base goke.Comp[Base]
	var appearance goke.Comp[render.Appearance]
	var off goke.Comp[sector.L6]
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		for k, f := range []*goke.Factory{
			si.NewFactory(&base, &appearance),
			si.NewFactory(&base, &appearance, &off),
		} {
			f.Create(1)
			f.Next()
			base.Slice(&f.Cursor)[0] = Base{Pos: Position{AABB: plane.NewAABB(quarters[k], 10, 10)}}
			appearance.Slice(&f.Cursor)[0] = render.Appearance{SpriteID: 1}
		}
		r.Init(si)
	}})
	composed(r, cam)
	if len(look.sprites) != 1 {
		t.Errorf("the zero View drew %d sprites, want the one awake",
			len(look.sprites))
	}
}

// a look noting what each sprite laid was told: uncovered, giving way
type waying struct {
	*counting
	bare, gives bool
	bared, gave map[render.SpriteID]bool
}

func (w *waying) Uncover(on bool) { w.bare = on }

func (w *waying) GiveWay(on bool) { w.gives = on }

func (w *waying) Uncovers() bool { return false }

func (w *waying) Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, z Z,
	atlas render.AtlasSource, a render.Appearance, lit render.Illumination) {
	w.bared[a.SpriteID], w.gave[a.SpriteID] = w.bare, w.gives
	w.counting.Sprite(f, cam, box, z, atlas, a, lit)
}

type reliefCamera struct{ camera.Camera }

func (reliefCamera) Projection() camera.Projection { return reliefProjection{} }

type reliefProjection struct{ camera.TopDown }

func (reliefProjection) Sorts() bool { return true }

// fastened is a camera from above fastened as how says, to an entity that is none of a test's
type fastened struct {
	camera.Camera
	how camera.How
}

func (fastened) Fasten(camera.Fastening) {}

func (c fastened) Fastening() camera.Fastening {
	return camera.Fastening{How: c.how, Entity: 1 << 40}
}

// covered draws a cover of the sprite 2, giving way, standing over a walker of the sprite 1,
// uncovered, through look and a camera fastened as how says, the renderer readied by ready
func covered(t *testing.T, look Look, how camera.How, ready func(*renderer)) *render.Frame {
	t.Helper()
	cam := camera.Camera(fastened{cameratest.NewFromSpace(1000, 1000, 0), how})
	r := newRenderer(flatAtlas{}, seenBy(cam, &view.View{}), &Rules{}, func() Look { return look })
	r.shade(nil, nil)
	r.ways(map[render.SpriteID]bool{1: true}, map[render.SpriteID]bool{2: true})
	if ready != nil {
		ready(r)
	}
	var base goke.Comp[Base]
	var appearance goke.Comp[render.Appearance]
	var fixed goke.Comp[Fixed]
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		cover := si.NewFactory(&base, &appearance, &fixed)
		cover.Create(1)
		cover.Next()
		box := plane.NewAABB(geom.NewVec(100, 100), 20, 20)
		base.Slice(&cover.Cursor)[0] = Base{Pos: Position{AABB: box}}
		appearance.Slice(&cover.Cursor)[0] = render.Appearance{SpriteID: 2}
		walker := si.NewFactory(&base, &appearance)
		walker.Create(1)
		walker.Next()
		under := plane.NewAABB(geom.NewVec(108, 108), 4, 4)
		base.Slice(&walker.Cursor)[0] = Base{Pos: Position{AABB: under}}
		appearance.Slice(&walker.Cursor)[0] = render.Appearance{SpriteID: 1}
		r.Init(si)
	}})
	return composed(r, cam)
}

func TestRenderer_Compose_WhatGivesWayDoesBeforeTheCamerasTheSettingsSay(t *testing.T) {
	ways := func(loose, riding bool) func(*renderer) {
		return func(r *renderer) { r.givesWay = func() (bool, bool) { return loose, riding } }
	}
	for name, c := range map[string]struct {
		how   camera.How
		ready func(*renderer)
		gives bool
	}{
		"loose, as the settings start":           {camera.Loose, nil, true},
		"centred on a unit":                      {camera.Centred, nil, true},
		"behind a unit, as the settings start":   {camera.Behind, nil, false},
		"inside a unit, as the settings start":   {camera.Inside, nil, false},
		"loose, the covers kept":                 {camera.Loose, ways(false, true), false},
		"behind a unit, giving way riding":       {camera.Behind, ways(true, true), true},
		"inside a unit, giving way riding alone": {camera.Inside, ways(false, true), true},
	} {
		look := &waying{counting: &counting{Look: ilook.NewFlat(1000, 1000, false, false)},
			bared: map[render.SpriteID]bool{}, gave: map[render.SpriteID]bool{}}
		covered(t, look, c.how, c.ready)
		if len(look.sprites) != 2 || look.bared[1] != c.gives || look.gave[1] ||
			look.bared[2] || look.gave[2] != c.gives || look.bare || look.gives {
			t.Errorf("%s: of %d sprites the walker laid uncovered %v, giving way %v, the cover "+
				"uncovered %v, giving way %v, the look left uncovering %v, giving way %v; want "+
				"the walker uncovered and the cover giving way %v alone, neither left on", name,
				len(look.sprites), look.bared[1], look.gave[1], look.bared[2], look.gave[2],
				look.bare, look.gives, c.gives)
		}
	}
}

func TestRenderer_Compose_AsksForThePassOfTheUncoveredOnlyWhereItLaysBoth(t *testing.T) {
	passes := func(how camera.How, ready func(*renderer)) []render.Pass {
		var got []render.Pass
		covered(t, ilook.NewFlat(1000, 1000, false, false), how, ready).Places(
			func(p render.Pass, _ render.At) { got = append(got, p) })
		return got
	}
	if got := passes(camera.Loose, nil); !slices.Contains(got, render.Uncovered) {
		t.Errorf("a walker uncovered under a cover giving way take the places %v, want the "+
			"pass of the uncovered among them", got)
	}
	walker := map[render.SpriteID]bool{1: true}
	cover := map[render.SpriteID]bool{2: true}
	for name, c := range map[string]struct {
		how   camera.How
		ready func(*renderer)
	}{
		"a cover giving way with nothing uncovered": {camera.Loose, func(r *renderer) {
			r.ways(nil, cover)
		}},
		"a walker uncovered with nothing giving way": {camera.Loose, func(r *renderer) {
			r.ways(walker, nil)
		}},
		"both before a camera riding a unit": {camera.Behind, nil},
	} {
		if got := passes(c.how, c.ready); slices.Contains(got, render.Uncovered) {
			t.Errorf("%s take the places %v, want no pass of the uncovered", name, got)
		}
	}
}

func TestRenderer_Compose_ACutoutTakesNoPlaceAmongWhatIsSeenThrough(t *testing.T) {
	look := ilook.NewFlat(1000, 1000, false, false)
	places := func(ready func(*renderer)) []render.Pass {
		f := drawAs(t, look, ready, nil, nil, geom.NewVec(1, 0), quarters...)
		var got []render.Pass
		f.Places(func(p render.Pass, _ render.At) { got = append(got, p) })
		return got
	}
	cut := func(r *renderer) { r.mark(map[render.SpriteID]bool{1: true}) }
	if got := places(cut); !slices.Equal(got, []render.Pass{render.Cutout}) {
		t.Errorf("figures all cutouts take the places %v, want the cutout pass alone", got)
	}
	if got := places(func(*renderer) {}); !slices.Equal(got, []render.Pass{render.SeeThrough}) {
		t.Errorf("figures seen through take the places %v, want one among what is seen through",
			got)
	}
}
