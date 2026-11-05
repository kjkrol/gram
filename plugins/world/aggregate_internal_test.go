package world

import (
	"image"
	"image/color"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	ilook "github.com/kjkrol/gram/plugins/world/internal/look"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
	"github.com/kjkrol/uid"
)

// summable is a world's atlas of a red kind, a blue one, one never thinned, one with a look
// declared at L3, one drawn as nothing from L3 and a material, closed
type summable struct {
	a                                     *Atlas
	red, blue, always, declared, nothing  render.SpriteID
	material                              render.SpriteID
	redColour, blueColour, declaredColour color.RGBA
}

func newSummable() summable {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 1024, Height: 1000},
		Entities: EntitiesCfg{MaxCount: 1000, MinSize: 1, MaxSize: 40},
	})
	k := summable{a: p.NewAtlas(), redColour: color.RGBA{R: 255, A: 255},
		blueColour: color.RGBA{B: 255, A: 255}, declaredColour: color.RGBA{G: 255, A: 255}}
	for _, id := range []*render.SpriteID{&k.red, &k.blue, &k.always, &k.declared, &k.nothing,
		&k.material} {
		*id = p.Kinds().NewSprite()
	}
	k.a.Add(k.red, 8, render.Solid(k.redColour))
	k.a.Add(k.blue, 8, render.Solid(k.blueColour))
	k.a.Add(k.always, 8, render.Solid(k.redColour)).Always()
	k.a.Add(k.declared, 8, render.Solid(k.redColour)).At(lod.L3, render.Solid(k.declaredColour))
	k.a.Add(k.nothing, 8, render.Solid(k.redColour)).Nothing(lod.L3)
	k.a.Add(k.material, 8, render.MaterialID(0))
	k.a.Close()
	return k
}

// placed is an entity of a kind over a box, keen or not
type placed struct {
	box    plane.AABB
	sprite render.SpriteID
	keen   bool
}

func at(x, y, side float32, sprite render.SpriteID) placed {
	return placed{box: plane.NewAABB(geom.NewVec(x, y), side, side), sprite: sprite}
}

// summing is a renderer of entities over a world 1024 by 1000 seen as halves, the left at L0,
// the right as ladder says, its aggregate, where a renderer reads one, 8 world units a texel
type summing struct {
	t     *testing.T
	r     *renderer
	space *aabbworld.Space
	sys   *view.System
	base  goke.Comp[Base]
	all   *goke.Query
	ids   []uid.UID64
	boxes []plane.AABB
}

func newSumming(t *testing.T, k summable, ladder lod.Ladder, ps ...placed) *summing {
	t.Helper()
	return newSummingFor(t, k, ladder, true, ps...)
}

func newSummingFor(t *testing.T, k summable, ladder lod.Ladder, read bool,
	ps ...placed) *summing {
	t.Helper()
	cam := farther{Camera: cameratest.NewFromSpace(1024, 1000, 0)}
	v := view.NewSeen(cam, halves{ladder: ladder})
	look := ilook.NewFlat(1024, 1000, false, false)
	seen := map[camera.Camera]*view.View{cam: v}
	s := &summing{t: t, r: newRenderer(k.a, func() map[camera.Camera]*view.View { return seen },
		&Rules{}, func() Look { return look })}
	readied(k.a)(s.r)
	s.r.space = SpaceCfg{Width: 1024, Height: 1000}
	if read {
		s.r.aggregate = newAggregate(Config{Space: s.r.space, LOD: lod.Ladder{Texel: 8}}, 512)
	}
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 1024, Height: 1000, BucketSize: 128})
	if err != nil {
		t.Fatal(err)
	}
	s.space = space
	var appearance goke.Comp[render.Appearance]
	var keen goke.Comp[sector.Keen]
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		for _, p := range ps {
			f := si.NewFactory(&s.base, &appearance)
			if p.keen {
				f = si.NewFactory(&s.base, &appearance, &keen)
			}
			f.Create(1)
			f.Next()
			s.base.Slice(&f.Cursor)[0] = Base{Pos: Position{AABB: p.box}}
			appearance.Slice(&f.Cursor)[0] = render.Appearance{SpriteID: p.sprite}
			s.ids = append(s.ids, f.Cursor.IDs[0])
			s.boxes = append(s.boxes, p.box)
		}
		s.all = si.NewQueryBuilder(&s.base).Build()
		s.r.Init(si)
	}})
	views := []*view.View{v}
	s.sys = view.NewSystem(space, &views, 1024, 1000)
	s.publish()
	return s
}

// publish has the views see the entities where they stand, then the renderer publish and swap
func (s *summing) publish() {
	items := make([]aabbworld.Item, len(s.ids))
	for i, id := range s.ids {
		items[i] = aabbworld.Item{ID: id, Box: s.boxes[i]}
	}
	s.space.Rebuild(items)
	s.sys.Update(nil, 0)
	s.r.Publish()
	s.r.Swap()
}

// move has the i-th entity stand over box, then publishes
func (s *summing) move(i int, box plane.AABB) {
	s.boxes[i] = box
	for s.all.All(); s.all.Next(); {
		cur := s.all.Cursor()
		for j, id := range cur.IDs {
			if id == s.ids[i] {
				s.base.Slice(cur)[j].Pos = Position{AABB: box}
			}
		}
	}
	s.publish()
}

// recorded reports whether the i-th entity was published as a record
func (s *summing) recorded(i int) bool {
	for _, e := range s.r.figures.Front().all {
		if e.id == s.ids[i] {
			return true
		}
	}
	return false
}

// texel is the texel (x, y) of the sums published last, premultiplied
func texel(g *aggregate, x, y int) [4]uint8 {
	f := g.sums.Front()
	tx, ty := x/aggregateTile, y/aggregateTile
	k := f.at[ty*g.across+tx]
	if k == 0 {
		return [4]uint8{}
	}
	i := 4 * ((y-ty*aggregateTile)*aggregateTile + x - tx*aggregateTile)
	px := f.blocks[k-1][i : i+4]
	return [4]uint8{px[0], px[1], px[2], px[3]}
}

// the texels of the sums published last that are not clear
func written(g *aggregate) int {
	n := 0
	for y := range g.rows {
		for x := range g.cols {
			if texel(g, x, y) != ([4]uint8{}) {
				n++
			}
		}
	}
	return n
}

// the right half at L3, the left at L0
var atL3 = lod.Ladder{Scales: [lod.Levels - 1]float32{0.5, 0.2, 0.15}}

func TestAggregate_SumsAFigureAtL3IntoTheTexelUnderItsMiddleRecordingItNot(t *testing.T) {
	k := newSummable()
	// a quarter of a texel each, the second across the texels 75 and 76, its middle in 76
	s := newSumming(t, k, atL3, at(600, 100, 4, k.red), at(606, 100, 4, k.blue),
		at(100, 100, 4, k.red))
	g := &s.r.aggregate
	if got, want := texel(g, 75, 12), ([4]uint8{64, 0, 0, 64}); got != want {
		t.Errorf("the texel under the red one is %v, want a quarter of red %v", got, want)
	}
	if got, want := texel(g, 76, 12), ([4]uint8{0, 0, 64, 64}); got != want {
		t.Errorf("the texel under the blue one's middle is %v, want a quarter of blue %v", got,
			want)
	}
	if n := written(g); n != 2 {
		t.Errorf("%d texels summed, want the two under the figures at L3", n)
	}
	if s.recorded(0) || s.recorded(1) || !s.recorded(2) {
		t.Errorf("recorded at L3 %v and %v, at L0 %v; want those at L3 summed alone",
			s.recorded(0), s.recorded(1), s.recorded(2))
	}
}

func TestAggregate_SumsNothingInAWorldNoRendererAskedItOf(t *testing.T) {
	k := newSummable()
	s := newSummingFor(t, k, atL3, false, at(600, 100, 4, k.red), at(100, 100, 4, k.red))
	if g := s.r.aggregate; g.img != nil || g.summing || len(g.sums.Front().tiles) != 0 {
		t.Errorf("an aggregate %v, summing %v, %d tiles summed; want none", g.img, g.summing,
			len(g.sums.Front().tiles))
	}
	// published as before the aggregate: the figure at L3 drawn by nothing
	if !s.recorded(0) || !s.recorded(1) {
		t.Errorf("recorded at L3 %v, at L0 %v; want both, as without an aggregate",
			s.recorded(0), s.recorded(1))
	}
}

func TestAggregate_ReadsNoTexelsAUnitWhileItHoldsNoColour(t *testing.T) {
	k := newSummable()
	s := newSumming(t, k, atL3, at(100, 100, 4, k.red))
	read := &Read{p: &Plugin{renderer: s.r}}
	if img, perUnit := read.Aggregate(); img == nil || perUnit != 0 {
		t.Errorf("no figure at L3: an aggregate %v, %v texels a unit; want one, 0", img, perUnit)
	}
	s.move(0, plane.NewAABB(geom.NewVec(600, 100), 4, 4))
	if _, perUnit := read.Aggregate(); perUnit != 1.0/8 {
		t.Errorf("a figure summed: %v texels a unit, want 1/8", perUnit)
	}
	s.move(0, plane.NewAABB(geom.NewVec(100, 100), 4, 4))
	if _, perUnit := read.Aggregate(); perUnit != 0 {
		t.Errorf("the figure gone back to L0: %v texels a unit, want 0", perUnit)
	}
}

func TestAggregate_RecordsAKindDeclaringALookAtL3SummingNoneOfItsOwn(t *testing.T) {
	k := newSummable()
	s := newSumming(t, k, atL3, at(600, 100, 4, k.declared), at(700, 100, 4, k.nothing),
		at(800, 100, 4, k.material))
	if n := written(&s.r.aggregate); n != 0 {
		t.Errorf("%d texels summed of a kind declared at L3, one drawn as nothing and a "+
			"material, want none", n)
	}
	if !s.recorded(0) || !s.recorded(1) || !s.recorded(2) {
		t.Errorf("recorded %v, %v, %v; want each published as without the aggregate",
			s.recorded(0), s.recorded(1), s.recorded(2))
	}
}

func TestAggregate_SumsNothingShortOfL3(t *testing.T) {
	k := newSummable()
	s := newSumming(t, k, lod.Ladder{Scales: [lod.Levels - 1]float32{0.5, 0.2}},
		at(600, 100, 4, k.red))
	if n := written(&s.r.aggregate); n != 0 || !s.recorded(0) {
		t.Errorf("at L2 %d texels summed, recorded %v; want none, recorded", n, s.recorded(0))
	}
	if s.r.aggregate.summing {
		t.Error("summing where no sector is seen at L3")
	}
}

func TestAggregate_ClearsWhatAFigureLeftTheFrameAfter(t *testing.T) {
	k := newSummable()
	s := newSumming(t, k, atL3, at(600, 100, 4, k.red))
	g := &s.r.aggregate
	s.move(0, plane.NewAABB(geom.NewVec(900, 500), 4, 4))
	if texel(g, 75, 12) != ([4]uint8{}) || texel(g, 112, 62) == ([4]uint8{}) {
		t.Errorf("moved within L3: the texel left %v, the one reached %v; want it clear, the "+
			"other summed", texel(g, 75, 12), texel(g, 112, 62))
	}
	s.move(0, plane.NewAABB(geom.NewVec(100, 500), 4, 4))
	if n := written(g); n != 0 || len(g.sums.Front().tiles) != 0 || !s.recorded(0) {
		t.Errorf("moved to L0: %d texels summed in %d tiles, recorded %v; want none, recorded",
			n, len(g.sums.Front().tiles), s.recorded(0))
	}
}

func TestAggregate_SumsFiguresInATexelHoldingItAtAWhole(t *testing.T) {
	k := newSummable()
	// a quarter each, then over a half each
	s := newSumming(t, k, atL3, at(600, 100, 4, k.red), at(601, 101, 4, k.blue),
		at(616, 200, 6, k.red), at(617, 201, 6, k.blue))
	g := &s.r.aggregate
	if got, want := texel(g, 75, 12), ([4]uint8{64, 0, 64, 128}); got != want {
		t.Errorf("two quarters in a texel sum to %v, want %v", got, want)
	}
	if got, want := texel(g, 77, 25), ([4]uint8{143, 0, 143, 255}); got != want {
		t.Errorf("two halves and more in a texel sum to %v, want %v held at a whole", got, want)
	}
}

func TestAggregate_AddsABoxByTheShareOfEachTexelItCovers(t *testing.T) {
	red := [4]uint8{255, 0, 0, 255}
	cover := func(g *aggregate) map[[2]int]uint8 {
		out := map[[2]int]uint8{}
		f := g.sums.Front()
		for y := range g.rows {
			for x := range g.cols {
				if f.at[y/aggregateTile*g.across+x/aggregateTile] != 0 {
					if a := texel(g, x, y)[3]; a != 0 {
						out[[2]int{x, y}] = a
					}
				}
			}
		}
		return out
	}
	for _, c := range []struct {
		name  string
		edges aabbworld.Edges
		box   plane.AABB
		want  map[[2]int]uint8
	}{
		{"under a texel", aabbworld.Edges(0), plane.NewAABB(geom.NewVec(6, 3), 4, 2),
			map[[2]int]uint8{{1, 0}: 32}},
		{"a texel long, two across", aabbworld.Edges(0), plane.NewAABB(geom.NewVec(4, 4), 16, 8),
			map[[2]int]uint8{{0, 1}: 128, {1, 1}: 255, {2, 1}: 128}},
		{"bigger both ways", aabbworld.Edges(0), plane.NewAABB(geom.NewVec(0, 0), 20, 12),
			map[[2]int]uint8{{0, 0}: 255, {1, 0}: 255, {2, 0}: 128, {0, 1}: 128, {1, 1}: 128,
				{2, 1}: 64}},
		{"across a torus's seam", aabbworld.Torus, plane.NewAABB(geom.NewVec(1020, 0), 12, 8),
			map[[2]int]uint8{{127, 0}: 128, {0, 0}: 255}},
		{"past a bounded world's edge", aabbworld.Edges(0),
			plane.NewAABB(geom.NewVec(1020, 0), 12, 8), map[[2]int]uint8{{127, 0}: 255}},
	} {
		t.Run(c.name, func(t *testing.T) {
			g := newAggregate(Config{Space: SpaceCfg{Width: 1024, Height: 1000, Edges: c.edges},
				LOD: lod.Ladder{Texel: 8}}, 512)
			g.begin()
			g.add(c.box, red)
			g.swap()
			got := cover(&g)
			if len(got) != len(c.want) {
				t.Fatalf("covered %v, want %v", got, c.want)
			}
			for p, a := range c.want {
				if got[p] != a {
					t.Errorf("covered %v, want %v", got, c.want)
					break
				}
			}
		})
	}
}

func TestAggregate_AddsAWideBoxAsWeighingEachTexelOnTheImage(t *testing.T) {
	green := [4]uint8{40, 160, 60, 255}
	for _, c := range []struct {
		name  string
		edges aabbworld.Edges
		box   plane.AABB
	}{
		{"on texels", aabbworld.Edges(0), plane.NewAABB(geom.NewVec(96, 104), 200, 160)},
		{"across tiles", aabbworld.Edges(0),
			plane.NewAABB(geom.NewVec(430.3, 470.7), 333.3, 120.9)},
		{"past a bounded world's corner", aabbworld.Edges(0),
			plane.NewAABB(geom.NewVec(-50.5, -30.2), 200, 150)},
		{"past its far corner", aabbworld.Edges(0),
			plane.NewAABB(geom.NewVec(900.1, 900.6), 300, 300)},
		{"across a torus's seams", aabbworld.Torus,
			plane.NewAABB(geom.NewVec(900.1, 900.6), 300, 300)},
	} {
		t.Run(c.name, func(t *testing.T) {
			cfg := Config{Space: SpaceCfg{Width: 1024, Height: 1000, Edges: c.edges},
				LOD: lod.Ladder{Texel: 8}}
			wide, weighed := newAggregate(cfg, 512), newAggregate(cfg, 512)
			wide.begin()
			wide.add(c.box, green)
			wide.swap()
			u0, v0 := c.box.TopLeft.X/8, c.box.TopLeft.Y/8
			cv := cover{u0: u0, v0: v0, u1: u0 + c.box.Size.X/8, v1: v0 + c.box.Size.Y/8}
			cv.x0, cv.nx = span(cv.u0, cv.u1)
			cv.y0, cv.ny = span(cv.v0, cv.v1)
			i0, i1 := inside(cv.x0, cv.nx, weighed.cols, weighed.wrapX)
			j0, j1 := inside(cv.y0, cv.ny, weighed.rows, weighed.wrapY)
			weighed.begin()
			weighed.weighed(weighed.sums.Back(), &cv, i0, i1, j0, j1, green, 0)
			weighed.swap()
			if n := written(&wide); n == 0 || n != written(&weighed) {
				t.Fatalf("%d texels written, want %d, some", n, written(&weighed))
			}
			for y := range wide.rows {
				for x := range wide.cols {
					if got, want := texel(&wide, x, y), texel(&weighed, x, y); got != want {
						t.Fatalf("texel (%d, %d) is %v, want %v", x, y, got, want)
					}
				}
			}
		})
	}
}

func TestAggregate_ThinsAtL3AsTheKeepSays(t *testing.T) {
	k := newSummable()
	ladder := atL3
	ladder.Keep[lod.L3] = 0.01
	var ps []placed
	for i := range 300 {
		sprite := []render.SpriteID{k.red, k.red, k.always}[i/100]
		p := at(520+float32(i%40)*12, float32(i/40)*12, 4, sprite)
		p.keen = i >= 100 && i < 200
		ps = append(ps, p)
	}
	s := newSumming(t, k, ladder, ps...)
	g := &s.r.aggregate
	summed := [3]int{}
	for i, p := range ps {
		mid := Position{AABB: p.box}.Center()
		if texel(g, int(mid.X/8), int(mid.Y/8)) != ([4]uint8{}) {
			summed[i/100]++
		}
		if s.recorded(i) {
			t.Fatalf("the entity %d at L3 recorded, want it summed or thinned", i)
		}
	}
	if summed[0] > 10 || summed[1] != 100 || summed[2] != 100 {
		t.Errorf("summed %d plain, %d keen, %d never thinned of 100 each; want few, then all",
			summed[0], summed[1], summed[2])
	}
}

func TestAggregate_PublishesFiguresAtL3WithoutAllocating(t *testing.T) {
	k := newSummable()
	var ps []placed
	for i := range 200 {
		ps = append(ps, at(520+float32(i%40)*12, float32(i/40)*12, 4, k.red))
	}
	s := newSumming(t, k, atL3, ps...)
	frame := func() {
		s.r.Publish()
		s.r.Swap()
		s.r.aggregate.upload()
	}
	frame()
	frame()
	if n := testing.AllocsPerRun(10, frame); n != 0 {
		t.Errorf("a frame of 200 figures at L3 allocates %v times, want none", n)
	}
	if n := written(&s.r.aggregate); n != 200 {
		t.Errorf("%d texels summed, want one a figure", n)
	}
}

func TestAggregate_PutsTheSumsOnTheGPUAndClearsThemAfter(t *testing.T) {
	gputest.Need(t)
	k := newSummable()
	s := newSumming(t, k, atL3, at(600, 100, 4, k.red))
	g := &s.r.aggregate
	read := func() func(x, y int) [4]uint8 {
		g.upload()
		pix := make([]byte, 4*g.cols*g.rows)
		g.img.ReadPixels(pix)
		return func(x, y int) [4]uint8 {
			i := 4 * (y*g.cols + x)
			return [4]uint8{pix[i], pix[i+1], pix[i+2], pix[i+3]}
		}
	}
	if got, want := read()(75, 12), ([4]uint8{64, 0, 0, 64}); got != want {
		t.Errorf("the image under the figure holds %v, want %v", got, want)
	}
	// a mark in the figure's tile stays while the figure stands, goes as it moves
	mark := []byte{1, 2, 3, 4}
	g.img.SubImage(image.Rect(70, 10, 71, 11)).WritePixels(mark)
	s.publish()
	if got := read()(70, 10); got != [4]uint8(mark) {
		t.Errorf("the tile of a figure standing still holds %v where it was marked, want it "+
			"written once", got)
	}
	s.move(0, plane.NewAABB(geom.NewVec(608, 100), 4, 4))
	if got := read()(70, 10); got != ([4]uint8{}) {
		t.Errorf("the tile of a figure moved holds %v where it was marked, want it written "+
			"anew", got)
	}
	s.move(0, plane.NewAABB(geom.NewVec(100, 500), 4, 4))
	if got := read()(75, 12); got != ([4]uint8{}) {
		t.Errorf("the image the figure left holds %v, want it clear", got)
	}
}

func TestAtlas_SumsAKindInItsBlobsColourWhateverSpriteItIsDrawnIn(t *testing.T) {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 100, Height: 100},
		Entities: EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	a := p.NewAtlas()
	walker := p.Kinds().NewSprite()
	halves := func(dst *render.Canvas, size int) {
		dst.FillRect(0, 0, float32(size)/2, float32(size), color.RGBA{R: 255, A: 255})
		dst.FillRect(float32(size)/2, 0, float32(size)/2, float32(size),
			color.RGBA{B: 255, A: 255})
	}
	a.Add(walker, 8, halves).Facing(4, func(float64) render.SpriteDrawer {
		return render.Solid(color.RGBA{G: 255, A: 255})
	})
	a.Close()
	want := [4]uint8{128, 0, 128, 255}
	sprites := append([]render.SpriteID{walker, a.versions[walker][lod.L2].id}, a.faced[walker]...)
	for _, id := range sprites {
		if c, ok := a.blobColour(id); !ok || c != want {
			t.Errorf("the sprite %d is summed in %v, %v; want its kind's blob's %v", id, c, ok,
				want)
		}
	}
	if m := render.MeanColour(halves, 8); [4]uint8{m.R, m.G, m.B, m.A} != want {
		t.Errorf("the blob's colour %v, want %v", m, want)
	}
}

func TestAggregate_PutsATileCutAtTheImagesEdgeOnTheGPU(t *testing.T) {
	gputest.Need(t)
	// 125 texels a side: the last tiles 61 across and down
	g := newAggregate(Config{Space: SpaceCfg{Width: 1000, Height: 1000},
		LOD: lod.Ladder{Texel: 8}}, 512)
	g.begin()
	g.add(plane.NewAABB(geom.NewVec(984, 992), 8, 8), [4]uint8{0, 255, 0, 255})
	g.swap()
	g.upload()
	pix := make([]byte, 4*125*125)
	g.img.ReadPixels(pix)
	for _, c := range []struct {
		x, y int
		want [4]uint8
	}{{123, 124, [4]uint8{0, 255, 0, 255}}, {124, 124, [4]uint8{}}, {123, 123, [4]uint8{}}} {
		i := 4 * (c.y*125 + c.x)
		if got := [4]uint8(pix[i : i+4]); got != c.want {
			t.Errorf("the texel (%d, %d) holds %v, want %v", c.x, c.y, got, c.want)
		}
	}
}
