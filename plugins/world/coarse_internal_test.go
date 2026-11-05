package world

import (
	"image/color"
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	ilook "github.com/kjkrol/gram/plugins/world/internal/look"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
)

// two sectors of 512 side by side: the camera sees the left one a unit a pixel, the right one a
// tenth of that, each at the level ladder says at its edge nearest the camera's focus
type halves struct{ ladder lod.Ladder }

func (halves) Len() int  { return 2 }
func (halves) Cols() int { return 2 }
func (halves) Box(i int) geom.AABB {
	return geom.NewAABB(geom.NewVec(float32(i)*512, 0), geom.NewVec(float32(i+1)*512, 1000))
}
func (halves) Of(x, _ float32) int  { return min(max(int(x/512), 0), 1) }
func (h halves) Ladder() lod.Ladder { return h.ladder }

func (h halves) Level(cam camera.Camera, i int) lod.Level {
	x := float32(i) * 512
	return h.ladder.Widened(camera.SightOf(cam)).At(camera.ScaleShown(cam, x, 500, 0), x)
}

type farther struct{ camera.Camera }

func (farther) Visible(camera.AABB) bool { return true }
func (farther) Focus() (x, y, z float32) { return 0, 500, 0 }
func (farther) ScaleAt(x, _, _ float32) float32 {
	return map[bool]float32{true: 1, false: 0.1}[x < 512]
}

// draws what a stander draws, and the fog it is given
type standing struct {
	*counting
	uprights int
	fogs     []float32
}

func (s *standing) Upright(f *render.Frame, cam camera.Camera, box plane.AABB, z Z,
	atlas render.AtlasSource, a render.Appearance, lit render.Illumination) {
	s.uprights++
}

func (s *standing) Fog(amount float32) { s.fogs = append(s.fogs, amount) }

// standing entities at x 100 and 700, and one at 900 of another kind, drawn through a camera
// seeing as ladder says, the sprite standing where stands says
func drawStanding(t *testing.T, ladder lod.Ladder, stands bool, rules ...Rule) *standing {
	t.Helper()
	look := &standing{counting: &counting{Look: ilook.NewFlat(1024, 1000, false, false)}}
	drawSeen(t, ladder, flatAtlas{}, 1, look, func(r *renderer) {
		r.shade(nil, nil)
		r.lie(nil, nil, map[render.SpriteID]bool{1: stands})
	}, rules...)
	return look
}

// drawSeen draws entities of sprite at x 100 and 700, and one at 900 of another kind, from atlas
// through a camera seeing as ladder says, and hands back the composing of them again
func drawSeen(t *testing.T, ladder lod.Ladder, atlas render.AtlasSource, sprite render.SpriteID,
	look Look, ready func(*renderer), rules ...Rule) (again func() *render.Frame) {
	t.Helper()
	cam := farther{Camera: cameratest.NewFromSpace(1024, 1000, 0)}
	return drawSeenAt(t, cam, ladder, []float32{100, 700, 900}, atlas, sprite, look, ready,
		rules...)
}

// drawSeenAt is drawSeen through cam of entities 10 a side at y 500 and each x, the last of
// another kind
func drawSeenAt(t *testing.T, cam camera.Camera, ladder lod.Ladder, xs []float32,
	atlas render.AtlasSource, sprite render.SpriteID, look Look, ready func(*renderer),
	rules ...Rule) (again func() *render.Frame) {
	t.Helper()
	v := view.NewSeen(cam, halves{ladder: ladder})
	var drawing Rules
	if err := drawing.Add(rules...); err != nil {
		t.Fatal(err)
	}
	r := newRenderer(atlas, seenBy(cam, v), &drawing, func() Look { return look })
	ready(r)
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 1024, Height: 1000, BucketSize: 128})
	if err != nil {
		t.Fatal(err)
	}
	var base goke.Comp[Base]
	var appearance goke.Comp[render.Appearance]
	var z goke.Comp[Z]
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var items []aabbworld.Item
		for k, x := range xs {
			f := si.NewFactory(&base, &appearance)
			if k == len(xs)-1 {
				f = si.NewFactory(&base, &appearance, &z)
			}
			f.Create(1)
			f.Next()
			box := plane.NewAABB(geom.NewVec(x, 500), 10, 10)
			base.Slice(&f.Cursor)[0] = Base{Pos: Position{AABB: box}}
			appearance.Slice(&f.Cursor)[0] = render.Appearance{SpriteID: sprite}
			items = append(items, aabbworld.Item{ID: f.Cursor.IDs[0], Box: box})
		}
		space.Rebuild(items)
		r.Init(si)
	}})
	views := []*view.View{v}
	view.NewSystem(space, &views, 1024, 1000).Update(nil, 0)
	composed(r, cam)
	var f render.Frame
	return func() *render.Frame {
		f.Reset(cam)
		r.Compose(&f, cam)
		return &f
	}
}

func TestRenderer_Compose_DrawsACoarseEntityFlat(t *testing.T) {
	look := drawStanding(t, lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 0.5}}, true)
	if look.uprights != 1 || len(look.sprites) != 2 {
		t.Errorf("uprights %d, sprites %d; want the near one standing, the far two flat",
			look.uprights, len(look.sprites))
	}
}

func TestRenderer_Compose_HandsTheLookTheFogOfEachEntity(t *testing.T) {
	look := drawStanding(t, lod.Ladder{Sight: 1000, Fog: 400}, false)
	// the second, 700 off, lies a quarter into the fog from 600 to 1000, the third 900 off three
	// quarters
	f := look.fogs
	if len(f) != 4 || f[0] != 0 || f[1] <= 0 || f[1] >= 0.25 || f[2] <= 0.75 || f[3] != 0 {
		t.Errorf("fogs %v, want none, a little, much, and none again after the entities", f)
	}
}

func TestRenderer_Compose_RunsTheRulesOnAChunkInViewAlone(t *testing.T) {
	var ran []float32
	saw := Show(func(b Base) bool {
		ran = append(ran, b.Pos.TopLeft.X)
		return true
	})
	// the sight ends 500 off: the right sector, and the one at 900 in its own chunk, unseen
	drawStanding(t, lod.Ladder{Sight: 500}, false, saw)
	for _, x := range ran {
		if x == 900 {
			t.Errorf("the rules ran on %v, want not on the chunk out of view", ran)
		}
	}
	if len(ran) == 0 {
		t.Error("the rules ran on nothing, want the chunk in view")
	}
}

// leveledRed is a world's atlas of one kind, red, closed, and the kind's sprite
func leveledRed() (*Atlas, render.SpriteID) {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 1024, Height: 1000},
		Entities: EntitiesCfg{MaxCount: 3, MinSize: 1, MaxSize: 10},
	})
	own := p.Kinds().NewSprite()
	a := p.NewAtlas()
	a.Add(own, 8, render.Solid(color.RGBA{R: 255, A: 255}))
	a.Close()
	return a, own
}

func readied(a *Atlas) func(*renderer) {
	return func(r *renderer) {
		r.shade(a.shaded, a.phased)
		r.lie(a.lain(), a.edged(), a.stood())
	}
}

// places counts the places the Directs composing f asked to draw at: with a look drawing nothing
// itself, one where the renderer draws its points
func places(f *render.Frame) int {
	n := 0
	f.Places(func(render.Pass, render.At) { n++ })
	return n
}

func TestRenderer_Compose_DrawsEachKindAsTheLevelSays(t *testing.T) {
	a, own := leveledRed()
	// the right sector, a tenth of a pixel a unit, at the level the ladder says; the left at L2
	for _, tc := range []struct {
		scales [lod.Levels - 1]float32
		want   []render.SpriteID
		points bool
	}{
		{[...]float32{0, 0, 0.5, 0, 0, 0}, []render.SpriteID{own, own, own}, false},
		{[...]float32{0, 0, 0.5, 0.2, 0, 0}, []render.SpriteID{own}, true},
		{[...]float32{0, 0, 0.5, 0.3, 0.2, 0}, []render.SpriteID{own}, false},
	} {
		look := &counting{Look: ilook.NewFlat(1024, 1000, false, false)}
		again := drawSeen(t, lod.Ladder{Scales: tc.scales}, a, own, look, readied(a))
		slices.Sort(look.sprites)
		if !slices.Equal(look.sprites, tc.want) {
			t.Errorf("with thresholds %v the look drew %v, want %v", tc.scales, look.sprites,
				tc.want)
		}
		if got := places(again()) == 1; got != tc.points {
			t.Errorf("with thresholds %v points drawn: %v, want %v", tc.scales, got, tc.points)
		}
	}
}

func TestRenderer_Compose_DrawsAtL4NothingThroughTheLook(t *testing.T) {
	look := drawStanding(t, lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 0.5, 0.2}}, true)
	if look.uprights != 1 || len(look.sprites) != 0 || len(look.fogs) != 2 {
		t.Errorf("uprights %d, sprites %d, fogs %v; want the near one standing, the far two "+
			"neither drawn nor fogged by the look", look.uprights, len(look.sprites), look.fogs)
	}
}

func TestRenderer_Compose_LaysAMaterialDeclaredAtL4(t *testing.T) {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 1024, Height: 1000},
		Entities: EntitiesCfg{MaxCount: 3, MinSize: 1, MaxSize: 10},
	})
	own := p.Kinds().NewSprite()
	a := p.NewAtlas()
	a.Add(own, 8, render.Solid(color.RGBA{R: 255, A: 255})).At(lod.L4, render.MaterialID(7))
	a.Close()
	look := &counting{Look: ilook.NewFlat(1024, 1000, false, false)}
	again := drawSeen(t, lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 0.5, 0.2}}, a, own, look,
		readied(a))
	look.sprites = nil
	f := again()
	materials := 0
	f.Each(func(_ render.At, verts []render.Vertex) {
		if verts[0].ColorA > 1.5 {
			materials++
		}
	})
	if materials != 2 || places(f) != 0 || len(look.sprites) != 1 {
		t.Errorf("materials %d, points drawn %v, the look's sprites %v; want the far two "+
			"materials, no point, the near one's sprite", materials, places(f) == 1, look.sprites)
	}
}

func TestRenderer_Compose_AllocatesNothingForTheLevels(t *testing.T) {
	a, own := leveledRed()
	look := ilook.NewFlat(1024, 1000, false, false)
	ladder := lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 0.5, 0.2, 0, 0}}
	again := drawSeen(t, ladder, a, own, look, readied(a))
	again()
	if n := testing.AllocsPerRun(10, func() { again() }); n != 0 {
		t.Errorf("a frame of an entity at L2 and two at L4 allocates %v times, want none", n)
	}
}

// sees the world left of x 511 alone, a unit a pixel everywhere
type framed struct{ farther }

func (framed) Bounds() camera.AABB {
	return geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(511, 1000))
}
func (framed) Visible(b camera.AABB) bool      { return b.TopLeft.X < 511 }
func (framed) ScaleAt(_, _, _ float32) float32 { return 1 }

// bounded by the whole world, as a picture turned on it is, yet showing the left sector alone
type turned struct{ framed }

func (turned) Bounds() camera.AABB {
	return geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(1024, 1000))
}
func (turned) Visible(b camera.AABB) bool { return b.TopLeft.X < 512 }

func TestRenderer_Compose_DrawsWhatThePictureCutsByASectorOnIt(t *testing.T) {
	a, own := leveledRed()
	shown := lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 8, 0.5, 0, 0}}
	cut := framed{farther{Camera: cameratest.NewFromSpace(1024, 1000, 0)}}
	// the second's middle at 514 lies in the right sector, off the picture, its box from 509 over
	// the picture's edge or the left sector's
	for _, tc := range []struct {
		name   string
		cam    camera.Camera
		ladder lod.Ladder
		want   []render.SpriteID
	}{
		{"cut by the picture's edge", cut, shown, []render.SpriteID{own, own}},
		{"off a turned picture", turned{cut}, shown, []render.SpriteID{own, own}},
		{"its middle past the sight", turned{cut}, lod.Ladder{Scales: shown.Scales, Sight: 510},
			[]render.SpriteID{own}},
	} {
		look := &counting{Look: ilook.NewFlat(1024, 1000, false, false)}
		drawSeenAt(t, tc.cam, tc.ladder, []float32{100, 509}, a, own, look, readied(a))
		if !slices.Equal(look.sprites, tc.want) {
			t.Errorf("%s: drew %v, want %v", tc.name, look.sprites, tc.want)
		}
	}
}

func TestHeld_IsTheBoundsPointNearestACopyOfTheCoordinate(t *testing.T) {
	for _, tc := range []struct {
		x, lo, hi float32
		wraps     bool
		want      float32
	}{
		{500, 100, 900, false, 500},
		{50, 100, 900, false, 100},
		{950, 100, 900, false, 900},
		// bounds from 900 past the seam to 300 of a world 1000 round
		{950, 900, 1300, true, 950},
		{50, 900, 1300, true, 50},
		{350, 900, 1300, true, 300},
		{800, 900, 1300, true, 900},
	} {
		if got := held(tc.x, tc.lo, tc.hi, 1000, tc.wraps); got != tc.want {
			t.Errorf("%v held within [%v, %v], wrapping %v: %v, want %v", tc.x, tc.lo, tc.hi,
				tc.wraps, got, tc.want)
		}
	}
}
