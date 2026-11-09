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
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	ilook "github.com/kjkrol/gram/plugins/world/internal/look"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
	"github.com/kjkrol/uid"
)

// units are n of a kind, drawn in sprite, in rows of four 12 apart from (x, y), each an aggregate
// of strength where that is set
type units struct {
	n        int
	sprite   render.SpriteID
	of       kind.ID
	x, y     float32
	strength int
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
	var aggregate goke.Comp[Aggregate]
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var items []aabbworld.Item
		loose := si.NewFactory(&base, &appearance)
		folded := si.NewFactory(&base, &appearance, &aggregate)
		for n, gr := range groups {
			f := loose
			if gr.strength > 0 {
				f = folded
			}
			f.Create(gr.n)
			for i := 0; f.Next(); {
				for j, id := range f.Cursor.IDs {
					box := plane.NewAABB(geom.NewVec(gr.x+float32(i%4)*12, gr.y+float32(i/4)*12),
						4, 4)
					base.Slice(&f.Cursor)[j] = Base{Pos: Position{AABB: box}, TypeID: gr.of}
					appearance.Slice(&f.Cursor)[j] = render.Appearance{SpriteID: gr.sprite}
					if gr.strength > 0 {
						aggregate.Slice(&f.Cursor)[j] = Aggregate{Strength: gr.strength}
					}
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

func TestFar_CountsAFoldedAggregateForItsStrength(t *testing.T) {
	k := newThinnable()
	g := newFarRig(t, k, atL4Right, []camera.Camera{farCam},
		units{n: 1, sprite: k.declared, of: 3, x: 600, y: 100, strength: 30},
		units{n: 1, sprite: k.declared, of: 3, x: 612, y: 100},
		units{n: 2, sprite: k.plain, of: 1, x: 600, y: 400, strength: 12})
	g.far, g.counts = Banners, true
	g.publish()
	b := g.r.bannersOf(farCam, nil)
	if len(b) != 1 || b[0].Count != 31 || len(b[0].IDs) != 2 {
		t.Errorf("banners %+v, want one of two entities standing for 31", b)
	}
	f := &g.r.figures.Front().far
	for _, gr := range f.groups {
		switch {
		case gr.banner && gr.width != render.BadgeWidth(31, gr.style.Shape, gr.style.Size):
			t.Errorf("the banner's badge %v wide, want room for 31", gr.width)
		case !gr.banner && (gr.count != 2 || gr.heads != 24):
			t.Errorf("the points %+v, want two entities counted 24", gr)
		}
	}
	if len(f.groups) != 2 {
		t.Errorf("%d groups, want the banner and the points", len(f.groups))
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
	if _, asked := g.compose(farCam); asked != 1 {
		t.Errorf("asked %d places for the banner, the points and their counts, want one", asked)
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

// farSeen is farCam drawing what it sees, through its camera's lines of sight
type farSeen struct{ farther }

func (c farSeen) Rays() (camera.RayField, bool) { return c.Camera.(camera.Rays).Rays() }

// flat boxes over the world in grey, each at its height, written into the depth
var blockShader = render.NewMeshShaderWith("test block", []byte(`
struct Out {
    @builtin(position) clip: vec4<f32>,
}
@vertex
fn vs_main(@builtin(vertex_index) vid: u32, @location(0) box: vec4<f32>,
    @location(1) high: vec4<f32>) -> Out {
    var k = vid % 6u;
    if k >= 3u {
        k -= 2u;
    }
    let at = vec2<f32>(select(box.x, box.z, k % 2u == 1u), select(box.y, box.w, k / 2u == 1u));
    var o: Out;
    o.clip = U.BlockViewProj * vec4<f32>(at, high.x, 1.0);
    return o;
}
@fragment
fn fs_main(o: Out) -> @location(0) vec4<f32> {
    return vec4<f32>(0.25, 0.25, 0.25, 1.0);
}
`), []render.Uniform{{Name: "BlockViewProj", Size: 16}}).Instanced(2)

// block is an opaque ground of the box from (x0, y0) to (x1, y1) at height z
type block struct{ x0, y0, x1, y1, z float32 }

func (block) Compose(f *render.Frame, _ camera.Camera) { f.DrawOpaque() }

func (b block) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	if t.Pass != render.Opaque {
		return
	}
	rays, _ := cam.(camera.Rays).Rays()
	w, h := cam.Viewport()
	tr, _ := camera.SceneTransform(rays, w, h)
	opts := render.DrawMeshOptions{Uniforms: map[string]any{"BlockViewProj": tr.M[:]},
		Instances: []float32{b.x0, b.y0, b.x1, b.y1, b.z, 0, 0, 0}, Vertices: 6,
		Depth: t.Depth, WriteDepth: true}
	u.Into(opts.Uniforms)
	t.Screen.DrawMesh(nil, blockShader, &opts)
}

func TestFar_DrawsABannerABadgeInItsKindsColourHoldingItsCountAsDeepAsItLies(t *testing.T) {
	gputest.Need(t)
	k := newThinnable()
	cam := farSeen{farCam}
	// ten round (617.6, 311.6) in a banner, its badge's middle 9 over, on the pixels' corner
	// (618, 303); one of another kind a banner round (802, 452)
	g := newFarRig(t, k, atL4Right, []camera.Camera{cam},
		units{n: 10, sprite: k.declared, of: 3, x: 600, y: 300},
		units{n: 1, sprite: k.plain, of: 1, x: 800, y: 600},
		units{n: 1, sprite: k.declared, of: 4, x: 800, y: 450})
	g.far = Banners
	g.publish()
	draw := func(sources ...render.Source) func(x, y int) [4]byte {
		screen := render.NewImage(1024, 1000)
		render.NewComposer(append(sources, g.r)...).DrawWorld(screen, cam)
		pix := make([]byte, 1024*1000*4)
		screen.ReadPixels(pix)
		return func(x, y int) [4]byte { return [4]byte(pix[(y*1024+x)*4:]) }
	}
	green := func(p [4]byte) bool { return p[1] > 200 && p[0] < 60 && p[2] < 60 }
	ink := func(p [4]byte) bool { return p[0] < 60 && p[1] < 60 && p[2] < 60 && p[3] > 200 }
	drawn := func(p [4]byte) bool { return p[3] != 0 }
	count := func(at func(x, y int) [4]byte, x0, y0, x1, y1 int, is func([4]byte) bool) int {
		n := 0
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				if is(at(x, y)) {
					n++
				}
			}
		}
		return n
	}
	at := draw()
	reach := g.r.bannersOf(cam, nil)[0].Reach
	if p := at(617, 302); !green(p) || reach != bannerSize/2 {
		t.Errorf("the badge's middle %v, reaching %v; want its kind's green, 7 round", p, reach)
	}
	x0, y0, x1, y1 := 1024, 1000, 0, 0
	for y := 285; y < 318; y++ {
		for x := 600; x < 636; x++ {
			if green(at(x, y)) {
				x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x+1), max(y1, y+1)
			}
		}
	}
	bx, by, _ := g.r.bannersOf(cam, nil)[0].BadgeAt(cam)
	if g.r.bannersOf(cam, nil)[0].Count != 10 {
		t.Fatalf("banners %+v, want the ten's first", g.r.bannersOf(cam, nil))
	}
	if mx, my := float32(x0+x1)/2, float32(y0+y1)/2; f32.Abs(mx-bx) > 1 || f32.Abs(my-by) > 1 {
		t.Errorf("the badge filled round (%v, %v), its banner says (%v, %v); want them within "+
			"a pixel", mx, my, bx, by)
	}
	if n, out := count(at, 600, 285, 636, 318, green), count(at, 560, 250, 600, 290,
		drawn); n < 60 || out != 0 {
		t.Errorf("%d green pixels round the banner, %d beyond its badge; want a badge 14 "+
			"across, nothing beyond", n, out)
	}
	if n := count(at, 600, 285, 636, 318, ink); n != 0 {
		t.Errorf("%d pixels of a number in the badge with the counts off, want none", n)
	}
	g.counts = true
	g.publish()
	at = draw()
	if n, lone := count(at, 610, 293, 626, 309, ink), count(at, 770, 560, 840, 600,
		drawn); n == 0 || lone != 0 {
		t.Errorf("%d pixels of the count in the badge, %d over the lone unit; want its count "+
			"inside, nothing over the lone unit", n, lone)
	}
	if one, filled := count(at, 790, 432, 814, 452, ink), count(at, 790, 432, 814, 452,
		green); one != 0 || filled < 60 {
		t.Errorf("%d pixels of a count in the badge of a banner of one, %d of its fill; want "+
			"a badge and no count", one, filled)
	}
	// a block over the badge, standing higher than the banner lies
	at = draw(block{x0: 590, y0: 270, x1: 646, y1: 330, z: 5})
	if n := count(at, 600, 285, 636, 318, green); n != 0 {
		t.Errorf("%d green pixels behind a block higher than the banner, want the badge hidden",
			n)
	}
	at = draw(block{x0: 590, y0: 270, x1: 646, y1: 330, z: 1})
	if n := count(at, 600, 285, 636, 318, green); n < 60 {
		t.Errorf("%d green pixels over a block lower than the banner, want the badge before it", n)
	}
}

func TestFar_DrawsAKindDeclaringABannerStyleAloneInBannersOfThatStyle(t *testing.T) {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 1024, Height: 1000},
		Entities: EntitiesCfg{MaxCount: 1000, MinSize: 1, MaxSize: 10},
	})
	k := thinnable{a: p.NewAtlas()}
	k.plain, k.declared = p.Kinds().NewSprite(), p.Kinds().NewSprite()
	red := render.Solid(color.RGBA{R: 255, A: 255})
	k.a.Add(k.plain, 8, red).Styled(BannerStyle{Shape: render.BadgeDiamond, Size: 20})
	k.a.Add(k.declared, 8, red).At(lod.L4, render.Solid(color.RGBA{G: 255, A: 255}))
	k.a.Close()
	g := newFarRig(t, k, atL4Right, []camera.Camera{farCam},
		units{n: 10, sprite: k.plain, of: 1, x: 600, y: 100},
		units{n: 10, sprite: k.declared, of: 3, x: 600, y: 400})
	g.far = Banners
	if got := g.publish(); len(got[0]) != 0 || len(got[1]) != 0 {
		t.Errorf("recorded %d of the styled and %d of the declaring, want both in banners",
			len(got[0]), len(got[1]))
	}
	reaches := func() map[kind.ID]float32 {
		by := map[kind.ID]float32{}
		for _, b := range g.r.bannersOf(farCam, nil) {
			by[b.Kind] = b.Reach
		}
		return by
	}
	if by := reaches(); len(by) != 2 || by[1] != 10 || by[3] != bannerSize/2 {
		t.Errorf("banners reaching %v by kind, want the styled 10, the declaring 7", by)
	}
	want := BannerStyle{
		Shape: render.BadgeDiamond,
		Size:  20,
		Fill:  color.RGBA{R: 255, A: 255},
		Rim:   color.RGBA{R: 85, A: 255},
	}
	for _, gr := range g.r.figures.Front().far.groups {
		if gr.kind == 1 && gr.style != want {
			t.Errorf("the styled kind's banners drawn in %+v, want %+v", gr.style, want)
		}
	}
	g.counts = true
	g.publish()
	by := reaches()
	diamond, disc := render.BadgeWidth(10, render.BadgeDiamond, 20)/2,
		render.BadgeWidth(10, render.BadgeDisc, bannerSize)/2
	if by[1] != diamond || by[3] != disc || disc <= bannerSize/2 {
		t.Errorf("banners of ten counted reaching %v by kind, want %v and %v, as wide as they "+
			"hold the count", by, diamond, disc)
	}
}

func TestFar_ABadgeAFrameAllocatesNothingOnceWarm(t *testing.T) {
	k := newThinnable()
	g := newFarRig(t, k, atL4Right, []camera.Camera{farCam},
		units{n: 10, sprite: k.declared, of: 3, x: 600, y: 100},
		units{n: 10, sprite: k.plain, of: 1, x: 600, y: 400})
	g.far, g.counts = Banners, true
	var f render.Frame
	var cam camera.Camera = farCam
	frame := func() {
		g.r.Publish()
		g.r.Swap()
		f.Reset(cam)
		g.r.Compose(&f, cam)
	}
	frame()
	if n := testing.AllocsPerRun(100, frame); n != 0 {
		t.Errorf("a frame of a badge and counted points allocates %v times, want none", n)
	}
}

// joining is a renderer over a world 2048 a side cut into sectors 512 a side, gathering units at
// L4 as the walk of a Publish does, through no view
type joining struct {
	r    *renderer
	b    figures
	grid *sector.Grid
	next uid.UID64
}

func newJoining(edges aabbworld.Edges) *joining {
	grid := sector.NewGrid(2048, 2048, edges.WrapsX(), edges.WrapsY(), 512)
	r := &renderer{space: SpaceCfg{Width: 2048, Height: 2048, Edges: edges}}
	r.sheet.grid, r.sheet.small = grid, grid.Side()/4
	return &joining{r: r, grid: grid}
}

// gather begins a walk anew and has it meet the crowds given
func (j *joining) gather(crowds ...units) {
	j.r.farBegin(&j.b)
	j.next = 0
	for _, c := range crowds {
		j.meet(c)
	}
	j.r.farSettle()
}

// meet has the walk meet n units of a kind, 4 a side, in rows of six 12 apart from (x, y), each in
// the sector of its middle, wrapped into the world
func (j *joining) meet(c units) {
	for i := range c.n {
		x := c.x + float32(i%6)*12
		y := c.y + float32(i/6)*12
		x, y = farWrapped(x, 2048), farWrapped(y, 2048)
		box := plane.NewAABB(geom.NewVec(x, y), 4, 4)
		key := farKey{sector: int32(j.grid.Of(x+2, y+2)), kind: c.of, level: lod.L4}
		j.next++
		j.r.join(key, nil, true, box, Z{}, c.sprite, nil, j.next, 1)
	}
}

func TestFar_JoinsACrowdStandingAcrossTheSeamsOfFourSectorsInOneBanner(t *testing.T) {
	j := newJoining(0)
	// 30 in six files astride the corner of sectors 0, 1, 4 and 5 at (512, 512)
	j.gather(units{n: 30, of: 3, x: 482, y: 488})
	groups := j.b.far.groups
	if len(groups) != 1 || groups[0].count != 30 || len(j.b.far.ids) != 30 {
		t.Fatalf("gathered %+v, want one group of the 30", groups)
	}
	g := groups[0]
	want := geom.NewVec(482+2.5*12+2, 488+2*12+2)
	if !slices.IsSorted(j.b.far.ids) || g.centre != want || g.size != geom.NewVec(4, 4) {
		t.Errorf("the banner %+v of %v, want every one of the 30 in the order met, at %v over "+
			"a box of one", g, j.b.far.ids, want)
	}
	if b := j.b.far.banner(&g); b.Count != 30 || len(b.IDs) != 30 || b.Sector != 5 {
		t.Errorf("the banner %+v, want 30 in sector 5, its centre's", b)
	}
}

func TestFar_JoinsNoKindsNorCrowdsLyingASectorApart(t *testing.T) {
	j := newJoining(0)
	j.gather(units{n: 30, of: 3, x: 482, y: 488}, units{n: 30, of: 4, x: 482, y: 488})
	if groups := j.b.far.groups; len(groups) != 2 || groups[0].count != 30 ||
		groups[1].count != 30 || groups[0].kind == groups[1].kind {
		t.Errorf("gathered %+v of two kinds astride a corner, want a group of each", groups)
	}
	// six in sector 0 and six in sector 1 a sector to the east, the Small apart and more
	j.gather(units{n: 6, of: 3, x: 300, y: 200}, units{n: 6, of: 3, x: 812, y: 200})
	if groups := j.b.far.groups; len(groups) != 2 || groups[0].count != 6 ||
		groups[1].count != 6 {
		t.Errorf("gathered %+v of a kind a sector apart, want two groups of six", groups)
	}
	// and the Small apart at most, across the seam
	j.gather(units{n: 6, of: 3, x: 440, y: 200}, units{n: 6, of: 3, x: 576, y: 200})
	if groups := j.b.far.groups; len(groups) != 1 || groups[0].count != 12 {
		t.Errorf("gathered %+v of a kind the Small apart across a seam, want one of 12", groups)
	}
}

func TestFar_JoinsACrowdAcrossTheSeamWhereTheWorldWraps(t *testing.T) {
	j := newJoining(aabbworld.Torus)
	// six files from 34 short of the right edge, the last three past it at the left; then the
	// first three past it
	j.gather(units{n: 30, of: 3, x: 2048 - 34, y: 488})
	groups := j.b.far.groups
	if len(groups) != 1 || groups[0].count != 30 {
		t.Fatalf("gathered %+v astride the seam, want one group of the 30", groups)
	}
	if g := groups[0]; g.centre != geom.NewVec(2046, 514) || g.sector != 7 {
		t.Errorf("the banner at %v in sector %d, want at (2046, 514) by the seam, in 7",
			g.centre, g.sector)
	}
	j.gather(units{n: 30, of: 3, x: 2048 - 22, y: 488})
	if groups := j.b.far.groups; len(groups) != 1 || groups[0].centre != geom.NewVec(10, 514) ||
		groups[0].sector != 4 {
		t.Errorf("gathered %+v astride the seam, its middle past it, want one at (10, 514) in "+
			"sector 4", groups)
	}
	j = newJoining(0)
	j.gather(units{n: 30, of: 3, x: 2048 - 34, y: 488})
	if groups := j.b.far.groups; len(groups) != 2 {
		t.Errorf("gathered %+v astride the edge of a world not wrapping, want two", groups)
	}
}

func TestFar_JoiningAllocatesNothingOnceWarm(t *testing.T) {
	j := newJoining(aabbworld.Torus)
	crowds := []units{{n: 30, of: 3, x: 482, y: 488}, {n: 30, of: 4, x: 2048 - 34, y: 1000},
		{n: 6, of: 3, x: 300, y: 1500}}
	j.gather(crowds...)
	if n := testing.AllocsPerRun(100, func() { j.gather(crowds...) }); n != 0 {
		t.Errorf("a gathering joining crowds allocates %v times, want none", n)
	}
}
