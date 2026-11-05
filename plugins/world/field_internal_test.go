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
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
	"github.com/kjkrol/uid"
)

// spot is an entity over a box, a field of size where size is not zero, asleep at L4 where
// asleep, an aggregate Unfolded where unfolded with a stretch standing in each sector of in
type spot struct {
	box      plane.AABB
	size     geom.Vec
	asleep   bool
	unfolded bool
	in       []int
}

// spreading is a renderer of entities of one kind over a world 1024 by 1000 seen as halves
// through cam, its aggregate 8 world units a texel
type spreading struct {
	r   *renderer
	cam camera.Camera
	ids []uid.UID64
}

// spread publishes entities of sprite at spots from k's atlas, seen by cam as ladder says, drawn
// through look
func spread(t *testing.T, cam camera.Camera, k summable, sprite render.SpriteID,
	ladder lod.Ladder, look Look, spots ...spot) *spreading {
	t.Helper()
	v := view.NewSeen(cam, halves{ladder: ladder})
	s := &spreading{r: newRenderer(k.a, seenBy(cam, v), &Rules{}, func() Look { return look }),
		cam: cam}
	readied(k.a)(s.r)
	s.r.space = SpaceCfg{Width: 1024, Height: 1000}
	s.r.covers = halves{}.cover
	s.r.aggregate = newAggregate(Config{Space: s.r.space, LOD: lod.Ladder{Texel: 8}}, 512)
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 1024, Height: 1000, BucketSize: 128})
	if err != nil {
		t.Fatal(err)
	}
	var base goke.Comp[Base]
	var appearance goke.Comp[render.Appearance]
	var field goke.Comp[Field]
	var asleep goke.Comp[sector.L4]
	var unfolded goke.Comp[Unfolded]
	var st goke.Comp[stretch]
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var items []aabbworld.Item
		for _, p := range spots {
			comps := []goke.Addable{&base, &appearance}
			if p.size != (geom.Vec{}) {
				comps = append(comps, &field)
			}
			if p.asleep {
				comps = append(comps, &asleep)
			}
			if p.unfolded {
				comps = append(comps, &unfolded)
			}
			f := si.NewFactory(comps...)
			f.Create(1)
			f.Next()
			base.Slice(&f.Cursor)[0] = Base{Pos: Position{AABB: p.box}}
			appearance.Slice(&f.Cursor)[0] = render.Appearance{SpriteID: sprite}
			if p.size != (geom.Vec{}) {
				field.Slice(&f.Cursor)[0] = Field{Size: p.size}
			}
			id := f.Cursor.IDs[0]
			s.ids = append(s.ids, id)
			if !p.asleep {
				items = append(items, aabbworld.Item{ID: id, Box: p.box})
			}
			for _, in := range p.in {
				f := si.NewFactory(&st)
				f.Create(1)
				f.Next()
				st.Slice(&f.Cursor)[0] = stretch{Whole: id, Sector: in}
			}
		}
		space.Rebuild(items)
		s.r.Init(si)
	}})
	views := []*view.View{v}
	view.NewSystem(space, &views, 1024, 1000).Update(nil, 0)
	s.r.Publish()
	s.r.Swap()
	return s
}

// frame is what s composes
func (s *spreading) frame() *render.Frame {
	var f render.Frame
	f.Reset(s.cam)
	s.r.Compose(&f, s.cam)
	return &f
}

// record is the i-th entity's first record as published
func (s *spreading) record(i int) (figure, bool) {
	for _, e := range s.r.figures.Front().all {
		if e.id == s.ids[i] {
			return e, true
		}
	}
	return figure{}, false
}

// pieces are the boxes of the i-th entity's records, a field's part in a sector each
func (s *spreading) pieces(i int) []plane.AABB {
	var boxes []plane.AABB
	for _, e := range s.r.figures.Front().all {
		if e.id == s.ids[i] && e.piece {
			boxes = append(boxes, e.box)
		}
	}
	return boxes
}

// the halves' sectors box covers, with the part of it in each
func (halves) cover(box geom.AABB, dst []sector.Part) []sector.Part {
	for i := range 2 {
		x0 := max(box.TopLeft.X, float32(i)*512)
		x1 := min(box.BottomRight.X, float32(i+1)*512)
		if x0 < x1 {
			dst = append(dst, sector.Part{Sector: i, Box: geom.NewAABB(
				geom.NewVec(x0, box.TopLeft.Y), geom.NewVec(x1, box.BottomRight.Y))})
		}
	}
	return dst
}

// the world boxes the materials composed lay over
func materialBoxes(f *render.Frame) []geom.AABB {
	var boxes []geom.AABB
	f.Each(func(_ render.At, verts []render.Vertex) {
		if verts[0].ColorA > 1.5 {
			boxes = append(boxes, geom.NewAABB(geom.NewVec(verts[0].ColorG, verts[0].ColorB),
				geom.NewVec(verts[3].ColorG, verts[3].ColorB)))
		}
	})
	return boxes
}

// a field centred at (772, 508) in the right half, its own box 20 a side, over 200 a side, lying
// on the texels 84 to 108 across and 51 to 75 down
var (
	ownBox    = plane.NewAABB(geom.NewVec(762, 498), 20, 20)
	fieldSize = geom.NewVec(200, 200)
	fieldAt   = geom.NewAABB(geom.NewVec(672, 408), geom.NewVec(872, 608))
)

func TestRenderer_Compose_DrawsAFieldFoldedOverItsFieldAtEveryLevel(t *testing.T) {
	k := newSummable()
	cam := farther{Camera: cameratest.NewFromSpace(1024, 1000, 0)}
	// the right half at L2, a tenth of a pixel a unit
	at := spread(t, cam, k, k.material, lod.Ladder{Scales: [lod.Levels - 1]float32{0.5, 0.2}},
		ilook.NewFlat(1024, 1000, false, false), spot{box: ownBox, size: fieldSize})
	wide := plane.NewAABB(fieldAt.TopLeft, fieldSize.X, fieldSize.Y)
	if got := at.pieces(0); !slices.Equal(got, []plane.AABB{wide}) {
		t.Fatalf("published over %v, want over its field %v", got, wide)
	}
	if got := materialBoxes(at.frame()); !slices.Equal(got, []geom.AABB{fieldAt}) {
		t.Errorf("at L2 its material lies over %v, want over its field %v", got, fieldAt)
	}

	// the left half at L1, a pixel a unit
	look := &counting{Look: ilook.NewFlat(1024, 1000, false, false)}
	own := plane.NewAABB(geom.NewVec(240, 490), 20, 20)
	spread(t, cam, k, k.red, lod.Ladder{Scales: [lod.Levels - 1]float32{8, 0.5}}, look,
		spot{box: own, size: fieldSize}).frame()
	left := plane.NewAABB(geom.NewVec(150, 400), fieldSize.X, fieldSize.Y)
	if !slices.Equal(look.boxes, []plane.AABB{left}) {
		t.Errorf("at L1 the look drew it over %v, want over its field %v", look.boxes, left)
	}
}

// a field over both halves, the left at L1, the right at L2
var (
	acrossOwn  = plane.NewAABB(geom.NewVec(502, 490), 20, 20)
	acrossSize = geom.NewVec(400, 200)
	leftPart   = plane.NewAABB(geom.NewVec(312, 400), 200, 200)
	rightPart  = plane.NewAABB(geom.NewVec(512, 400), 200, 200)
)

func TestRenderer_Compose_DrawsEachSectorOfAFieldAtItsOwnLevel(t *testing.T) {
	k := newSummable()
	cam := farther{Camera: cameratest.NewFromSpace(1024, 1000, 0)}
	look := &counting{Look: ilook.NewFlat(1024, 1000, false, false)}
	at := spread(t, cam, k, k.red, lod.Ladder{Scales: [lod.Levels - 1]float32{0.5, 0.2}}, look,
		spot{box: acrossOwn, size: acrossSize})
	if got := at.pieces(0); !slices.Equal(got, []plane.AABB{leftPart, rightPart}) {
		t.Fatalf("published over %v, want over its part in each sector %v and %v", got,
			leftPart, rightPart)
	}
	f := at.frame()
	if !slices.Equal(look.boxes, []plane.AABB{leftPart}) || places(f) != 1 {
		t.Errorf("the look drew it over %v, %d points drawn; want the left part through the "+
			"look at L1 and the right one a point at L2", look.boxes, places(f))
	}
}

func TestRenderer_Publish_LeavesASectorAFieldIsUnfoldedInToItsParts(t *testing.T) {
	k := newSummable()
	cam := farther{Camera: cameratest.NewFromSpace(1024, 1000, 0)}
	look := &counting{Look: ilook.NewFlat(1024, 1000, false, false)}
	at := spread(t, cam, k, k.red, lod.Ladder{Scales: [lod.Levels - 1]float32{0.5, 0.2}}, look,
		spot{box: acrossOwn, size: acrossSize, unfolded: true, in: []int{0}})
	if got := at.pieces(0); !slices.Equal(got, []plane.AABB{rightPart}) {
		t.Fatalf("unfolded in the left sector, published over %v, want over its right part %v "+
			"alone", got, rightPart)
	}
	if f := at.frame(); len(look.boxes) != 0 || places(f) != 1 {
		t.Errorf("the look drew it over %v, %d points drawn; want the right part alone, a point",
			look.boxes, places(f))
	}
}

func TestRenderer_Publish_RecordsNoPointAggregateUnfolded(t *testing.T) {
	k := newSummable()
	cam := farther{Camera: cameratest.NewFromSpace(1024, 1000, 0)}
	for _, c := range []struct {
		name   string
		ladder lod.Ladder
		x      float32
	}{
		{"at L0", lod.Ladder{Scales: [lod.Levels - 1]float32{0.5, 0.2}}, 100},
		{"at L1", lod.Ladder{Scales: [lod.Levels - 1]float32{8, 0.5}}, 100},
		{"at L2", lod.Ladder{Scales: [lod.Levels - 1]float32{0.5, 0.2}}, 700},
	} {
		box := plane.NewAABB(geom.NewVec(c.x, 500), 20, 20)
		at := spread(t, cam, k, k.red, c.ladder, ilook.NewFlat(1024, 1000, false, false),
			spot{box: box, unfolded: true, in: []int{0}}, spot{box: box})
		if _, ok := at.record(0); ok {
			t.Errorf("%s: an aggregate unfolded is published", c.name)
		}
		if _, ok := at.record(1); !ok {
			t.Errorf("%s: an entity beside it is not published", c.name)
		}
	}
}

func TestRenderer_Publish_WalksNoStretchesWithoutAField(t *testing.T) {
	k := newSummable()
	cam := farther{Camera: cameratest.NewFromSpace(1024, 1000, 0)}
	box := plane.NewAABB(geom.NewVec(100, 500), 20, 20)
	at := spread(t, cam, k, k.red, lod.Ladder{Scales: [lod.Levels - 1]float32{0.5, 0.2}},
		ilook.NewFlat(1024, 1000, false, false), spot{box: box, unfolded: true, in: []int{0}},
		spot{box: box})
	if at.r.found || len(at.r.unfoldedIn) != 0 {
		t.Errorf("the stretches %v are walked with no field to spread", at.r.unfoldedIn)
	}
}

func TestRenderer_Compose_DrawsAFieldAtL2AsAPointNotThroughTheLook(t *testing.T) {
	k := newSummable()
	cam := farther{Camera: cameratest.NewFromSpace(1024, 1000, 0)}
	look := &counting{Look: ilook.NewFlat(1024, 1000, false, false)}
	f := spread(t, cam, k, k.red, lod.Ladder{Scales: [lod.Levels - 1]float32{0.5, 0.2}}, look,
		spot{box: ownBox, size: fieldSize}).frame()
	if places(f) != 1 || len(look.sprites) != 0 {
		t.Errorf("points drawn %v, the look's sprites %v; want a point, none through the look",
			places(f) == 1, look.sprites)
	}
}

func TestRenderer_Compose_AFieldsLookAtL2FillsItsField(t *testing.T) {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 64, Height: 32},
		Entities: EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	own := p.Kinds().NewSprite()
	a := p.NewAtlas()
	a.Add(own, 8, render.Solid(color.RGBA{R: 255, A: 255})).
		At(lod.L2, render.Solid(color.RGBA{G: 255, A: 255}))
	a.Close()
	cam := cameratest.NewFromSpace(64, 32, 0)
	cam.SetViewport(64, 32)
	// its own box 4 a side round (32, 16), its field from (12, 6) to (52, 26)
	pix := pixelsOf(t, cam, 64, 32, 0, lod.L2, a, own, geom.NewVec(40, 20),
		plane.NewAABB(geom.NewVec(30, 14), 4, 4))
	for y := range 32 {
		for x := range 64 {
			in := x >= 12 && x < 52 && y >= 6 && y < 26
			if got := pix(x, y); in && got != [4]byte{0, 255, 0, 255} || !in && got[3] != 0 {
				t.Fatalf("pixel (%d, %d) is %v, want the green declared at L2 over the field "+
					"alone", x, y, got)
			}
		}
	}
}

// opaque asks for the opaque pass and draws nothing in it: what follows is tested against a depth
type opaque struct{}

func (opaque) Compose(f *render.Frame, _ camera.Camera)           { f.DrawOpaque() }
func (opaque) Draw(render.Target, camera.Camera, render.Uniforms) {}

// beneath draws a field of canopy standing over own, of size, and a figure of banner over box,
// both at L2 over an opaque ground of a world 64 by 32, and reads the screen
func beneath(t *testing.T, a *Atlas, canopy, banner render.SpriteID, own plane.AABB,
	size geom.Vec, box plane.AABB) func(x, y int) [4]byte {
	t.Helper()
	gputest.Need(t)
	cam := cameratest.NewFromSpace(64, 32, 0)
	cam.SetViewport(64, 32)
	v := view.NewSeen(cam, everywhere{w: 64, h: 32, level: lod.L2})
	look := ilook.NewFlat(64, 32, false, false)
	r := newRenderer(a, seenBy(cam, v), &Rules{}, func() Look { return look })
	readied(a)(r)
	r.space = SpaceCfg{Width: 64, Height: 32}
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 64, Height: 32, BucketSize: 16})
	if err != nil {
		t.Fatal(err)
	}
	var base goke.Comp[Base]
	var appearance goke.Comp[render.Appearance]
	var field goke.Comp[Field]
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var items []aabbworld.Item
		made := func(box plane.AABB, sprite render.SpriteID, wide bool) {
			f := si.NewFactory(&base, &appearance)
			if wide {
				f = si.NewFactory(&base, &appearance, &field)
			}
			f.Create(1)
			f.Next()
			base.Slice(&f.Cursor)[0] = Base{Pos: Position{AABB: box}}
			appearance.Slice(&f.Cursor)[0] = render.Appearance{SpriteID: sprite}
			if wide {
				field.Slice(&f.Cursor)[0] = Field{Size: size}
			}
			items = append(items, aabbworld.Item{ID: f.Cursor.IDs[0], Box: box})
		}
		made(box, banner, false)
		made(own, canopy, true)
		space.Rebuild(items)
		r.Init(si)
	}})
	views := []*view.View{v}
	view.NewSystem(space, &views, 64, 32).Update(nil, 0)
	r.Publish()
	r.Swap()
	screen := render.NewImage(64, 32)
	render.NewComposer(opaque{}, r).DrawWorld(screen, cam)
	pix := make([]byte, 4*64*32)
	screen.ReadPixels(pix)
	return func(x, y int) [4]byte { i := 4 * (y*64 + x); return [4]byte(pix[i : i+4]) }
}

func TestRenderer_Compose_AFigureAtL2StandsOverAFieldsPiece(t *testing.T) {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 64, Height: 32},
		Entities: EntitiesCfg{MaxCount: 2, MinSize: 1, MaxSize: 10},
	})
	canopy, banner := p.Kinds().NewSprite(), p.Kinds().NewSprite()
	a := p.NewAtlas()
	a.Add(canopy, 8, render.Solid(color.RGBA{G: 255, A: 255})).
		At(lod.L2, render.Solid(color.RGBA{G: 255, A: 255}))
	a.Add(banner, 8, render.Solid(color.RGBA{R: 255, A: 255})).
		At(lod.L2, render.Solid(color.RGBA{R: 255, A: 255}))
	a.Close()
	// the field from (12, 6) to (52, 26), the banner 4 a side round (40, 16) within it
	own := plane.NewAABB(geom.NewVec(30, 14), 4, 4)
	box := plane.NewAABB(geom.NewVec(38, 14), 4, 4)
	pix := beneath(t, a, canopy, banner, own, geom.NewVec(40, 20), box)
	if got := pix(40, 16); got != [4]byte{255, 0, 0, 255} {
		t.Errorf("the banner's pixel is %v, want its red over the field", got)
	}
	if got := pix(20, 10); got != [4]byte{0, 255, 0, 255} {
		t.Errorf("the field's pixel is %v, want its green", got)
	}
}

func TestRenderer_Compose_LaysAFieldsPieceUnderTheFigures(t *testing.T) {
	k := newSummable()
	cam := farther{Camera: cameratest.NewFromSpace(1024, 1000, 0)}
	// the left half at L1, through a look laying what lies
	look := &lyingCounting{counting: &counting{Look: ilook.NewFlat(1024, 1000, false, false)}}
	own := plane.NewAABB(geom.NewVec(240, 490), 20, 20)
	spread(t, cam, k, k.red, lod.Ladder{Scales: [lod.Levels - 1]float32{8, 0.5}}, look,
		spot{box: own, size: fieldSize}).frame()
	if len(look.lain) != 1 || len(look.sprites) != 0 {
		t.Errorf("at L1 the look laid %v and stood %v, want the field laid alone", look.lain,
			look.sprites)
	}

	// the right half at L2, its material
	f := spread(t, cam, k, k.material, lod.Ladder{Scales: [lod.Levels - 1]float32{0.5, 0.2}},
		ilook.NewFlat(1024, 1000, false, false), spot{box: ownBox, size: fieldSize}).frame()
	f.Each(func(at render.At, verts []render.Vertex) {
		if verts[0].ColorA > 1.5 && at.Z() != ilook.PieceLift {
			t.Errorf("at L2 its material lies at %v, want at %v", at.Z(), ilook.PieceLift)
		}
	})
}

func TestRenderer_Publish_SumsAFieldAtL3OverEachTexelOfItsField(t *testing.T) {
	k := newSummable()
	cam := farther{Camera: cameratest.NewFromSpace(1024, 1000, 0)}
	at := spread(t, cam, k, k.red, atL3, ilook.NewFlat(1024, 1000, false, false),
		spot{box: ownBox, size: fieldSize})
	if _, ok := at.record(0); ok {
		t.Error("the field summed at L3 is recorded too")
	}
	g := &at.r.aggregate
	if n := written(g); n != 25*25 {
		t.Errorf("%d texels written, want the 625 of its field", n)
	}
	for _, xy := range [][2]int{{84, 51}, {108, 51}, {84, 75}, {108, 75}, {96, 63}} {
		if got := texel(g, xy[0], xy[1]); got != [4]uint8{255, 0, 0, 255} {
			t.Errorf("texel %v is %v, want red wholly", xy, got)
		}
	}
}

func TestRenderer_Publish_HasAFieldSeenWhereverItsFieldIsAsleepOrNot(t *testing.T) {
	k := newSummable()
	// the left half alone in the picture, at L3
	cam := framed{farther{Camera: cameratest.NewFromSpace(1024, 1000, 0)}}
	ladder := lod.Ladder{Scales: [lod.Levels - 1]float32{8, 4, 2}}
	// centred at (700, 500) in the right half, off the picture, its field from 400 on it, its
	// part in the left sector from 400 to 512
	own := plane.NewAABB(geom.NewVec(690, 490), 20, 20)
	size := geom.NewVec(600, 200)
	wide := plane.NewAABB(geom.NewVec(400, 400), 112, 200)
	for _, asleep := range []bool{false, true} {
		look := &counting{Look: ilook.NewFlat(1024, 1000, false, false)}
		at := spread(t, cam, k, k.declared, ladder, look, spot{box: own, size: size,
			asleep: asleep}, spot{box: own})
		at.frame()
		if _, ok := at.record(1); ok {
			t.Errorf("asleep %v: an entity off the picture is published", asleep)
		}
		if _, ok := at.record(0); !ok || !slices.Equal(look.boxes, []plane.AABB{wide}) {
			t.Errorf("asleep %v: the field is published %v, drawn over %v; want it drawn over "+
				"its part on the picture %v", asleep, ok, look.boxes, wide)
		}
	}
}
