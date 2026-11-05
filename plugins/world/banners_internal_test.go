package world

import (
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	ilook "github.com/kjkrol/gram/plugins/world/internal/look"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
	"github.com/kjkrol/uid"
)

// units are n of a kind, drawn in sprite, in rows of four 12 apart from (x, y)
type units struct {
	n      int
	sprite render.SpriteID
	of     kind.ID
	x, y   float32
}

// farRig is a renderer drawing units through cameras, each seeing as halves with a ladder,
// gathering the loose units at the strategic scale as far and counts say
type farRig struct {
	r      *renderer
	look   *counting
	cams   []camera.Camera
	ids    [][]uid.UID64
	far    FarLook
	counts bool
}

// the right sector at L4 through farther, all at L2 through a camera seeing a unit a pixel
var (
	farCam  = farther{Camera: cameratest.NewFromSpace(1024, 1000, 0)}
	nearCam = cameratest.NewFromSpace(1024, 1000, 0)
)

func newFarRig(t *testing.T, k thinnable, ladder lod.Ladder, cams []camera.Camera,
	groups ...units) *farRig {
	t.Helper()
	views := map[camera.Camera]*view.View{}
	var all []*view.View
	for _, cam := range cams {
		v := view.NewSeen(cam, halves{ladder: ladder})
		views[cam] = v
		all = append(all, v)
	}
	look := &counting{Look: ilook.NewFlat(1024, 1000, false, false)}
	g := &farRig{look: look, cams: cams, ids: make([][]uid.UID64, len(groups))}
	g.r = newRenderer(k.a, func() map[camera.Camera]*view.View { return views }, &Rules{},
		func() Look { return look })
	g.r.far.look = func() FarLook { return g.far }
	g.r.far.counted = func() bool { return g.counts }
	g.r.space = SpaceCfg{Width: 1024, Height: 1000}
	readied(k.a)(g.r)
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 1024, Height: 1000, BucketSize: 128})
	if err != nil {
		t.Fatal(err)
	}
	var base goke.Comp[Base]
	var appearance goke.Comp[render.Appearance]
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var items []aabbworld.Item
		f := si.NewFactory(&base, &appearance)
		for n, gr := range groups {
			f.Create(gr.n)
			for i := 0; f.Next(); {
				for j, id := range f.Cursor.IDs {
					box := plane.NewAABB(geom.NewVec(gr.x+float32(i%4)*12, gr.y+float32(i/4)*12),
						4, 4)
					base.Slice(&f.Cursor)[j] = Base{Pos: Position{AABB: box}, TypeID: gr.of}
					appearance.Slice(&f.Cursor)[j] = render.Appearance{SpriteID: gr.sprite}
					items = append(items, aabbworld.Item{ID: id, Box: box})
					g.ids[n] = append(g.ids[n], id)
					i++
				}
			}
		}
		space.Rebuild(items)
		g.r.Init(si)
	}})
	view.NewSystem(space, &all, 1024, 1000).Update(nil, 0)
	return g
}

// publish publishes what the cameras see and hands back the records of each group
func (g *farRig) publish() [][]uid.UID64 {
	g.r.Publish()
	g.r.Swap()
	recorded := make([][]uid.UID64, len(g.ids))
	for _, e := range g.r.figures.Front().all {
		for n := range g.ids {
			if slices.Contains(g.ids[n], e.id) {
				recorded[n] = append(recorded[n], e.id)
			}
		}
	}
	return recorded
}

// compose composes the frame published through cam: the sprites its look drew and the places
// asked, one where anything is drawn
func (g *farRig) compose(cam camera.Camera) (sprites, asked int) {
	g.look.sprites = g.look.sprites[:0]
	var f render.Frame
	f.Reset(cam)
	g.r.Compose(&f, cam)
	return len(g.look.sprites), places(&f)
}

// atL4Right is a ladder drawing the right sector through farCam at L4, nearCam's at L2
var atL4Right = lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 0.5, 0.2}}

func TestFar_GathersAKindDeclaringALookAtL4InABannerASector(t *testing.T) {
	k := newThinnable()
	g := newFarRig(t, k, atL4Right, []camera.Camera{farCam},
		units{n: 10, sprite: k.declared, of: 3, x: 600, y: 100})
	g.far = Banners
	if got := g.publish(); len(got[0]) != 0 {
		t.Errorf("recorded %d of the ten in a banner, want none", len(got[0]))
	}
	banners := g.r.bannersOf(farCam, nil)
	if len(banners) != 1 {
		t.Fatalf("banners %+v, want one", banners)
	}
	b := banners[0]
	var cx, cy float32
	for i := range 10 {
		cx += 600 + float32(i%4)*12 + 2
		cy += 100 + float32(i/4)*12 + 2
	}
	centre := geom.NewVec(cx/10, cy/10)
	if b.Kind != 3 || b.Sector != 1 || b.Count != 10 || !slices.Equal(b.IDs, g.ids[0]) ||
		b.Center != centre || b.Box != geom.NewAABBAt(geom.NewVec(centre.X-2, centre.Y-2), 4, 4) {
		t.Errorf("the banner %+v, want kind 3 in sector 1, the ten at %v over a box of one", b,
			centre)
	}
	if sprites, asked := g.compose(farCam); sprites != 0 || asked != 1 {
		t.Errorf("drew %d sprites through the look and asked %d places, want the banner alone, "+
			"a point", sprites, asked)
	}
	if got := g.r.bannersOf(nearCam, nil); len(got) != 0 {
		t.Errorf("banners %+v of a camera drawing nothing, want none", got)
	}
}

func TestFar_DrawsAKindOfNoLookAtL4AndEveryKindAsPointsWhereTheyAreSo(t *testing.T) {
	k := newThinnable()
	g := newFarRig(t, k, atL4Right, []camera.Camera{farCam},
		units{n: 10, sprite: k.plain, of: 1, x: 600, y: 100},
		units{n: 10, sprite: k.declared, of: 3, x: 600, y: 400})
	g.far = Banners
	got := g.publish()
	if len(got[0]) != 10 || len(got[1]) != 0 || len(g.r.bannersOf(farCam, nil)) != 1 {
		t.Errorf("recorded %d of the plain and %d of the declaring in banners, want ten blobs "+
			"and one banner", len(got[0]), len(got[1]))
	}
	g.far = Points
	got = g.publish()
	if len(got[0]) != 10 || len(got[1]) != 10 || len(g.r.bannersOf(farCam, nil)) != 0 {
		t.Errorf("recorded %d and %d as points, %d banners; want ten each, none", len(got[0]),
			len(got[1]), len(g.r.bannersOf(farCam, nil)))
	}
	if len(g.r.figures.Front().far.groups) != 0 {
		t.Errorf("gathered %v with neither banners nor counts, want nothing",
			g.r.figures.Front().far.groups)
	}
}

func TestFar_CountsEveryUnitOfAKindInASectorThinnedOrNot(t *testing.T) {
	k := newThinnable()
	ladder := atL4Right
	ladder.Keep[lod.L4] = 0.25
	g := newFarRig(t, k, ladder, []camera.Camera{farCam},
		units{n: 10, sprite: k.plain, of: 1, x: 600, y: 100},
		units{n: 10, sprite: k.declared, of: 3, x: 600, y: 400},
		units{n: 1, sprite: k.plain, of: 1, x: 100, y: 100})
	g.counts = true
	got := g.publish()
	f := &g.r.figures.Front().far
	counted := map[kind.ID]int32{}
	for _, gr := range f.groups {
		if gr.banner {
			t.Errorf("a group %+v in a banner drawn as points, want counted alone", gr)
		}
		counted[gr.kind] += gr.count
	}
	if len(got[0]) >= 10 || !f.counts || counted[1] != 10 || counted[3] != 10 ||
		len(f.groups) != 2 {
		t.Errorf("recorded %d of ten thinned, counted %v in %d groups; want fewer, ten of each "+
			"kind in the right sector, the one at L2 none", len(got[0]), counted, len(f.groups))
	}
	g.far = Banners
	g.publish()
	f = &g.r.figures.Front().far
	if b := g.r.bannersOf(farCam, nil); len(b) != 1 || b[0].Count != 10 || len(f.groups) != 2 {
		t.Errorf("banners %+v of %d groups, want one of the ten thinned none, the plain counted",
			b, len(f.groups))
	}
	g.counts = false
	g.publish()
	if f := &g.r.figures.Front().far; f.counts || len(f.groups) != 1 {
		t.Errorf("counts %v, groups %d with the counts off, want the banner alone", f.counts,
			len(f.groups))
	}
}

func TestFar_DrawsAUnitInItsBannerOnlyThroughACameraSeeingItSoFar(t *testing.T) {
	k := newThinnable()
	g := newFarRig(t, k, atL4Right, []camera.Camera{farCam, nearCam},
		units{n: 10, sprite: k.declared, of: 3, x: 600, y: 100})
	g.far = Banners
	if got := g.publish(); len(got[0]) != 10 {
		t.Errorf("recorded %d of the ten another camera sees close, want all", len(got[0]))
	}
	if sprites, _ := g.compose(nearCam); sprites != 10 {
		t.Errorf("drew %d through the near camera, want the ten", sprites)
	}
	if sprites, asked := g.compose(farCam); sprites != 0 || asked != 1 {
		t.Errorf("drew %d sprites and asked %d places through the far camera, want the banner "+
			"alone", sprites, asked)
	}
	if far, near := g.r.bannersOf(farCam, nil), g.r.bannersOf(nearCam, nil); len(far) != 1 ||
		len(near) != 0 {
		t.Errorf("banners %+v far and %+v near, want one far alone", far, near)
	}
}

func TestFar_DrawsTheCountsOnTheGlassInTheKindsColour(t *testing.T) {
	gputest.Need(t)
	k := newThinnable()
	g := newFarRig(t, k, atL4Right, []camera.Camera{farCam},
		units{n: 10, sprite: k.declared, of: 3, x: 600, y: 300},
		units{n: 1, sprite: k.plain, of: 1, x: 800, y: 600})
	g.far, g.counts = Banners, true
	g.publish()
	dst := render.NewImage(1024, 1000)
	g.r.DrawGlass(dst, farCam)
	pix := make([]byte, 1024*1000*4)
	dst.ReadPixels(pix)
	green, other := 0, 0
	for y := range 1000 {
		for x := range 1024 {
			p := pix[(y*1024+x)*4:]
			switch {
			case p[3] == 0:
			case x >= 600 && x < 660 && y >= 270 && y < 312 && p[1] > 200 && p[0] < 60 &&
				p[2] < 60:
				green++
			case x < 600 || x >= 660 || y < 270 || y >= 312:
				other++
			}
		}
	}
	if green == 0 || other != 0 {
		t.Errorf("%d green pixels over the banner, %d drawn elsewhere; want its count in its "+
			"look's green, nothing over the lone unit", green, other)
	}
	g.counts = false
	g.publish()
	clear(pix)
	dst.Clear()
	g.r.DrawGlass(dst, farCam)
	dst.ReadPixels(pix)
	if slices.ContainsFunc(pix, func(b byte) bool { return b != 0 }) {
		t.Error("drew on the glass with the counts off, want nothing")
	}
}
