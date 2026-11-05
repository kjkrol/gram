package world

import (
	"image/color"
	"math"
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
	"github.com/kjkrol/gram/render/gputest"
	"github.com/kjkrol/uid"
)

// standingKinds is a world's atlas of a red kind standing, a red one lying and a red one with a
// look declared at L4, closed
type standingKinds struct {
	a                    *Atlas
	dome, flat, declared render.SpriteID
}

func newStandingKinds() standingKinds {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 1024, Height: 1000},
		Entities: EntitiesCfg{MaxCount: 100, MinSize: 1, MaxSize: 100},
	})
	k := standingKinds{a: p.NewAtlas()}
	k.dome, k.flat, k.declared = p.Kinds().NewSprite(), p.Kinds().NewSprite(),
		p.Kinds().NewSprite()
	red := render.Solid(color.RGBA{R: 255, A: 255})
	k.a.Add(k.dome, 8, red)
	k.a.Add(k.flat, 8, red).Lying()
	k.a.Add(k.declared, 8, red).At(lod.L4, render.Solid(color.RGBA{G: 255, A: 255}))
	k.a.Close()
	return k
}

// stood is an entity over a box at z, of a sprite, fixed or not
type stood struct {
	box    plane.AABB
	z      Z
	sprite render.SpriteID
	fixed  bool
}

// grid is the sectors a View sees that the sheets lay out
type grid interface {
	view.Sectors
	tiling
}

// sheetRig is a renderer of what stands in a world 1024 by 1000 seen through one camera as its
// grid says, its sheets 8 world units a texel, and the ECS what stands lives in
type sheetRig struct {
	r     *renderer
	look  *counting
	cam   camera.Camera
	views []*view.View
	space *aabbworld.Space
	items []aabbworld.Item
	ids   []uid.UID64
	all   *goke.Query
	base  goke.Comp[Base]
	z     goke.Comp[Z]
	// what makes what stands
	units, fixed  *goke.Factory
	mb            goke.Comp[Base]
	ma            goke.Comp[render.Appearance]
	mz            goke.Comp[Z]
	mf            goke.Comp[Fixed]
	ub            goke.Comp[Base]
	ua            goke.Comp[render.Appearance]
	uz            goke.Comp[Z]
	dense, sparse float32
}

// atL4 draws the right half at L4 and the left at L2 seen as halves through farther
var atL4 = lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 0.5, 0.2}}

func newSheetRig(t *testing.T, k standingKinds, g grid, things ...stood) *sheetRig {
	t.Helper()
	cam := farther{Camera: cameratest.NewFromSpace(1024, 1000, 0)}
	v := view.NewSeen(cam, g)
	look := &counting{Look: ilook.NewFlat(1024, 1000, false, false)}
	s := &sheetRig{r: newRenderer(k.a, seenBy(cam, v), &Rules{}, func() Look { return look }),
		look: look, cam: cam, views: []*view.View{v}}
	readied(k.a)(s.r)
	s.r.space = SpaceCfg{Width: 1024, Height: 1000}
	s.r.sheet = sheetsOver(g, 1.0/8, 128, 125)
	s.r.sheet.open()
	var err error
	s.space, err = aabbworld.NewSpace(aabbworld.Config{Width: 1024, Height: 1000, BucketSize: 128})
	if err != nil {
		t.Fatal(err)
	}
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		s.fixed = si.NewFactory(&s.mb, &s.ma, &s.mz, &s.mf)
		s.units = si.NewFactory(&s.ub, &s.ua, &s.uz)
		s.all = si.NewQueryBuilder(&s.base, &s.z).Build()
		s.add(things...)
		s.r.Init(si)
	}})
	return s
}

// add has things stand in the world
func (s *sheetRig) add(things ...stood) {
	for _, th := range things {
		f, base, look, z := s.units, &s.ub, &s.ua, &s.uz
		if th.fixed {
			f, base, look, z = s.fixed, &s.mb, &s.ma, &s.mz
		}
		f.Create(1)
		f.Next()
		base.Slice(&f.Cursor)[0] = Base{Pos: Position{AABB: th.box}}
		look.Slice(&f.Cursor)[0] = render.Appearance{SpriteID: th.sprite}
		z.Slice(&f.Cursor)[0] = th.z
		id := f.Cursor.IDs[0]
		s.ids = append(s.ids, id)
		s.items = append(s.items, aabbworld.Item{ID: id, Box: th.box})
	}
}

// move has the k-th thing added stand over box at z
func (s *sheetRig) move(k int, box plane.AABB, z Z) {
	if s.all.Seek(s.ids[k]) {
		cur := s.all.Cursor()
		s.base.At(cur).Pos.AABB = box
		*s.z.At(cur) = z
	}
	s.items[k].Box = box
}

// publish has the frame drawn next hold what the camera sees now
func (s *sheetRig) publish() *figures {
	s.space.Rebuild(s.items)
	view.NewSystem(s.space, &s.views, 1024, 1000).Update(nil, 0)
	s.r.Publish()
	s.r.Swap()
	return s.r.figures.Front()
}

// recorded is which of the things added the frame drawn next has records of, by their order
func (s *sheetRig) recorded(f *figures) []int {
	var got []int
	for _, e := range f.all {
		got = append(got, slices.Index(s.ids, e.id))
	}
	slices.Sort(got)
	return got
}

func box(x, y, side float32) plane.AABB { return plane.NewAABB(geom.NewVec(x, y), side, side) }

func TestSheets_PhotographWhatStandsWhereAViewDrawsL4InPlaceOfItsRecords(t *testing.T) {
	k := newStandingKinds()
	s := newSheetRig(t, k, halves{ladder: atL4},
		stood{box: box(600, 400, 88), z: Z{Altitude: 10, Height: 40}, sprite: k.dome, fixed: true},
		stood{box: box(800, 600, 40), z: Z{Altitude: 20, Height: 5}, sprite: k.flat, fixed: true},
		stood{box: box(700, 200, 40), sprite: k.declared, fixed: true},
		stood{box: box(900, 800, 40), sprite: k.dome},
		stood{box: box(100, 100, 40), sprite: k.dome, fixed: true})
	f := s.publish()
	if len(f.sheet.photos) != 1 || f.sheet.photos[0].sector != 1 {
		t.Fatalf("photographs %v, want the right half's alone", f.sheet.photos)
	}
	p := f.sheet.photos[0]
	if n := (p.to - p.from) / photoFloats; n != 2 || p.rect != s.r.sheet.rect(1) {
		t.Errorf("the photograph lays %d things over %v, want the two of no look at L4 over %v", n,
			p.rect, s.r.sheet.rect(1))
	}
	if !f.sheet.held[1] || f.sheet.held[0] || !f.sheet.live {
		t.Errorf("held %v, live %v; want the right half held alone", f.sheet.held, f.sheet.live)
	}
	if got := s.recorded(f); !slices.Equal(got, []int{2, 3, 4}) {
		t.Errorf("records of %v, want of the kind declared at L4, the unit and what stands at L2",
			got)
	}
	if at := f.sheet.instances[p.from:p.to]; at[8] != 10 || at[9] != 40 || at[10] != 0 ||
		at[20] != 20 || at[21] != 5 || at[22] != 1 {
		t.Errorf("laid %v, want the dome 10 up and 40 high standing, then the flat 20 up and 5 "+
			"high lying", at)
	}
}

func TestSheets_PhotographASectorAnewOnlyAsWhatStandsInItChanges(t *testing.T) {
	k := newStandingKinds()
	s := newSheetRig(t, k, halves{ladder: atL4},
		stood{box: box(600, 400, 88), z: Z{Altitude: 10, Height: 40}, sprite: k.dome, fixed: true})
	s.publish()
	v := s.r.sheet.version[1]
	if f := s.publish(); len(f.sheet.photos) != 0 || s.r.sheet.version[1] != v {
		t.Errorf("published again unchanged, %d photographs and version %d; want none, %d",
			len(f.sheet.photos), s.r.sheet.version[1], v)
	}
	s.add(stood{box: box(800, 600, 40), sprite: k.flat, fixed: true})
	if f := s.publish(); len(f.sheet.photos) != 1 || s.r.sheet.version[1] != v+1 ||
		f.sheet.photos[0].to-f.sheet.photos[0].from != 2*photoFloats {
		t.Errorf("one more spawned, %d photographs and version %d; want one of both, %d",
			len(f.sheet.photos), s.r.sheet.version[1], v+1)
	}
	s.move(0, box(600, 400, 88), Z{Altitude: 30, Height: 40})
	if f := s.publish(); len(f.sheet.photos) != 1 || s.r.sheet.version[1] != v+2 {
		t.Errorf("raised, %d photographs and version %d; want one, %d", len(f.sheet.photos),
			s.r.sheet.version[1], v+2)
	}
}

func TestSheets_HoldNothingWhereNothingStandsAndClearAnEmptiedSector(t *testing.T) {
	k := newStandingKinds()
	s := newSheetRig(t, k, halves{ladder: atL4},
		stood{box: box(100, 400, 40), sprite: k.dome, fixed: true})
	f := s.publish()
	if len(f.sheet.photos) != 0 || f.sheet.held[1] || f.sheet.live {
		t.Fatalf("%d photographs, held %v, live %v of nothing standing; want none",
			len(f.sheet.photos), f.sheet.held, f.sheet.live)
	}
	s.move(0, box(600, 400, 40), Z{})
	if f := s.publish(); len(f.sheet.photos) != 1 || !f.sheet.held[1] {
		t.Fatalf("come into the right half, %d photographs, held %v; want one, held",
			len(f.sheet.photos), f.sheet.held)
	}
	s.move(0, box(100, 400, 40), Z{})
	f = s.publish()
	if len(f.sheet.photos) != 1 || f.sheet.photos[0].to != f.sheet.photos[0].from ||
		f.sheet.held[1] || f.sheet.live {
		t.Errorf("gone, %v photographed, held %v, live %v; want the half photographed empty, "+
			"held by none", f.sheet.photos, f.sheet.held, f.sheet.live)
	}
}

func TestSheets_PhotographPerFrameSectorsAFrameKeepingTheRecordsOfTheRest(t *testing.T) {
	k := newStandingKinds()
	var things []stood
	for i := range 16 {
		x, y := float32(i%4)*256+100, float32(i/4)*250+100
		things = append(things, stood{box: box(x, y, 40), sprite: k.dome, fixed: true})
	}
	s := newSheetRig(t, k, everywhere{w: 1024, h: 1000, level: lod.L4}, things...)
	s.r.sheet.perFrame = 5
	var shot []int
	for round, want := range []int{5, 5, 5, 1, 0} {
		f := s.publish()
		for _, p := range f.sheet.photos {
			shot = append(shot, p.sector)
		}
		f.sheet.developed = true
		if len(f.sheet.photos) != want || len(f.all) != 16-len(shot) {
			t.Fatalf("round %d: %d photographs, %d records; want %d, %d", round,
				len(f.sheet.photos), len(f.all), want, 16-len(shot))
		}
	}
	slices.Sort(shot)
	if len(slices.Compact(shot)) != 16 {
		t.Errorf("photographed %v, want each sector once", shot)
	}
}

func TestSheets_PhotographAnewWhatNoFrameDeveloped(t *testing.T) {
	k := newStandingKinds()
	s := newSheetRig(t, k, halves{ladder: atL4},
		stood{box: box(600, 400, 40), sprite: k.dome, fixed: true})
	s.publish()
	s.publish()
	if f := s.publish(); len(f.sheet.photos) != 1 {
		t.Errorf("%d photographs after one no frame developed, want it again", len(f.sheet.photos))
	}
	s.r.figures.Front().sheet.developed = true
	s.publish()
	if f := s.publish(); len(f.sheet.photos) != 0 {
		t.Errorf("%d photographs after one developed, want none", len(f.sheet.photos))
	}
}

func TestSheets_KeepTheRecordOfWhatAnotherViewDrawsShortOfL4(t *testing.T) {
	k := newStandingKinds()
	s := newSheetRig(t, k, halves{ladder: atL4},
		stood{box: box(600, 400, 40), sprite: k.dome, fixed: true})
	near := cameratest.NewFromSpace(1024, 1000, 0)
	v := view.NewSeen(near, halves{})
	s.views = append(s.views, v)
	views := map[camera.Camera]*view.View{s.cam: s.views[0], near: v}
	s.r.views = func() map[camera.Camera]*view.View { return views }
	f := s.publish()
	if len(f.all) != 1 || !f.all[0].fine || f.all[0].sheet != 2 || !f.sheet.held[1] {
		t.Fatalf("records %v, held %v; want one drawn fine by the near view, the right half held",
			f.all, f.sheet.held)
	}
	var fr render.Frame
	fr.Reset(s.cam)
	s.r.Compose(&fr, s.cam)
	if places(&fr) != 0 {
		t.Error("the far camera draws what its sheet stands in the place of")
	}
	fr.Reset(near)
	s.r.Compose(&fr, near)
	if len(s.look.sprites) != 1 {
		t.Errorf("the near camera draws %d sprites of what stands, want its own",
			len(s.look.sprites))
	}
}

// texels reads img's texels
func texels(img *render.Image) func(x, y int) [4]byte {
	b := img.Bounds()
	pix := make([]byte, 4*b.Dx()*b.Dy())
	img.ReadPixels(pix)
	return func(x, y int) [4]byte {
		i := 4 * (y*b.Dx() + x)
		return [4]byte{pix[i], pix[i+1], pix[i+2], pix[i+3]}
	}
}

// heightOf is the top's height a relief texel holds
func heightOf(t [4]byte) float32 { return halfFloat(uint16(t[0]) | uint16(t[1])<<8) }

// halfFloat is the float16 h
func halfFloat(h uint16) float32 {
	sign := float32(1)
	if h&0x8000 != 0 {
		sign = -1
	}
	exp, frac := int(h>>10&0x1f), float64(h&0x3ff)
	if exp == 0 {
		return sign * float32(math.Ldexp(frac, -24))
	}
	return sign * float32(math.Ldexp(1+frac/1024, exp-15))
}

// wayOf is the x and y of the way a relief texel faces
func wayOf(t [4]byte) (float32, float32) {
	return float32(t[2])/255*2 - 1, float32(t[3])/255*2 - 1
}

func TestSheets_DevelopTheirAlbedoTheTopsAndTheHalves(t *testing.T) {
	gputest.Need(t)
	k := newStandingKinds()
	// the dome's middle at the texel (80, 55), the flat from (100, 75) to (105, 80)
	s := newSheetRig(t, k, halves{ladder: atL4},
		stood{box: box(600, 400, 88), z: Z{Altitude: 10, Height: 40}, sprite: k.dome, fixed: true},
		stood{box: box(800, 600, 40), z: Z{Altitude: 20, Height: 5}, sprite: k.flat, fixed: true})
	s.publish()
	s.r.develop(nil)
	sh := &s.r.sheet
	albedo, relief := texels(sh.albedo), texels(sh.relief)
	red := [4]byte{255, 0, 0, 255}
	near := func(a, b [4]byte, by int) bool {
		for c := range a {
			if d := int(a[c]) - int(b[c]); d > by || d < -by {
				return false
			}
		}
		return true
	}
	for y := 50; y < 85; y++ {
		for x := 64; x < 110; x++ {
			in := x >= 75 && x < 86 && y >= 50 && y < 61 || x >= 100 && x < 105 && y >= 75 &&
				y < 80
			if got := albedo(x, y); in && !near(got, red, 1) || !in && got[3] != 0 {
				t.Fatalf("the albedo at (%d, %d) is %v, want red inside a box %v", x, y, got, in)
			}
		}
	}
	if h := heightOf(relief(80, 55)); f32Off(h, 50) > 0.005 {
		t.Errorf("the dome's middle %v high, want 50", h)
	}
	if x, y := wayOf(relief(80, 55)); x*x+y*y > 0.01 {
		t.Errorf("the dome's middle faces (%v, %v), want up", x, y)
	}
	if x, _ := wayOf(relief(84, 55)); x < 0.2 {
		t.Errorf("the dome's slope east of its middle faces %v east, want away from it", x)
	}
	for _, at := range [][2]int{{100, 75}, {104, 79}, {102, 77}} {
		h, tex := heightOf(relief(at[0], at[1])), relief(at[0], at[1])
		if x, y := wayOf(tex); f32Off(h, 25) > 0.005 || x*x+y*y > 0.01 {
			t.Errorf("the flat at %v %v high facing (%v, %v), want 25 facing up", at, h, x, y)
		}
	}
	if h := heightOf(relief(70, 55)); relief(70, 55)[2] != 128 || h != 0 {
		t.Errorf("a texel nothing stands on is %v, want no height facing up", relief(70, 55))
	}
	half, halfRelief := texels(sh.albedoL5), texels(sh.reliefL5)
	for y := 25; y < 41; y++ {
		for x := 32; x < 55; x++ {
			var mean [4]int
			for _, d := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
				a := albedo(2*x+d[0], 2*y+d[1])
				for c := range mean {
					mean[c] += int(a[c])
				}
			}
			want := [4]byte{}
			for c := range mean {
				want[c] = byte((mean[c] + 2) / 4)
			}
			if got := half(x, y); !near(got, want, 1) {
				t.Fatalf("the albedo at L5 at (%d, %d) is %v, want the mean %v", x, y, got, want)
			}
		}
	}
	if h := heightOf(halfRelief(40, 27)); f32Off(h, 50) > 0.03 {
		t.Errorf("the dome's middle at L5 %v high, want about 50", h)
	}
	if h := heightOf(halfRelief(50, 37)); f32Off(h, 25) > 0.005 {
		t.Errorf("the flat at L5 %v high, want 25", h)
	}
}

// f32Off is how far off want got is, a share of want
func f32Off(got, want float32) float32 {
	d := got - want
	if d < 0 {
		d = -d
	}
	return d / want
}

func TestSheets_DevelopEachPhotographOnceAFrame(t *testing.T) {
	gputest.Need(t)
	k := newStandingKinds()
	s := newSheetRig(t, k, halves{ladder: atL4},
		stood{box: box(600, 400, 88), z: Z{Height: 40}, sprite: k.dome, fixed: true})
	s.publish()
	// read as a draw reads it, the albedo has the frame's photograph developed
	dst := render.NewImage(128, 125)
	dst.CopyFrom(s.r.sheet.albedo)
	if !s.r.figures.Front().sheet.developed || texels(dst)(80, 55)[3] == 0 {
		t.Fatal("the albedo read before its photograph was developed")
	}
	s.r.sheet.albedo.SubImage(s.r.sheet.rect(1)).Clear()
	var fr render.Frame
	fr.Reset(s.cam)
	s.r.Compose(&fr, s.cam)
	dst.CopyFrom(s.r.sheet.albedo)
	if texels(dst)(80, 55)[3] != 0 {
		t.Error("the photograph developed again in the frame it was developed in")
	}
}

func TestSheets_PhotographThePartsKeptOfAFieldFoldedTillItUnfolds(t *testing.T) {
	k := newStandingKinds()
	s := newSheetRig(t, k, halves{ladder: atL4})
	s.publish()
	at := place{whole: 7, sector: 1}
	s.r.sheet.fold(at, []shot{{600, 400, 10, 10}, {700, 400, 10, 10}})
	f := s.publish()
	if len(f.sheet.photos) != 1 || f.sheet.photos[0].to-f.sheet.photos[0].from != 2*photoFloats ||
		!f.sheet.held[1] {
		t.Fatalf("photographs %v, held %v; want the right half's of both parts, held",
			f.sheet.photos, f.sheet.held)
	}
	f.sheet.developed = true
	s.r.sheet.unfold(at)
	f = s.publish()
	if len(f.sheet.photos) != 1 || f.sheet.photos[0].to != f.sheet.photos[0].from ||
		f.sheet.held[1] {
		t.Errorf("unfolded, photographs %v, held %v; want the right half's empty, held by none",
			f.sheet.photos, f.sheet.held)
	}
}

// oddHalves is a world 1024 by 1000 cut at x 520, an odd texel of sheets 8 world units a texel
type oddHalves struct{}

func (oddHalves) Len() int  { return 2 }
func (oddHalves) Cols() int { return 2 }
func (oddHalves) Box(i int) geom.AABB {
	x := [3]float32{0, 520, 1024}
	return geom.NewAABB(geom.NewVec(x[i], 0), geom.NewVec(x[i+1], 1000))
}
func (oddHalves) Of(x, _ float32) int                { return min(max(int(x/520), 0), 1) }
func (oddHalves) Neighbours(i int, each func(j int)) { each(1 - i) }

func TestSheets_DevelopARoundOfSectorsAsEachAlone(t *testing.T) {
	gputest.Need(t)
	k := newStandingKinds()
	// the dome covers the texel 64, the last of the first sector, the second sector's thing none
	// by it: the texel at L5 over 64 and 65 both sectors share is half covered
	var f sheetFrame
	for i, th := range []stood{
		{box: box(480, 400, 40), z: Z{Altitude: 10, Height: 40}, sprite: k.dome},
		{box: box(560, 400, 40), z: Z{Altitude: 20, Height: 5}, sprite: k.flat},
	} {
		mid := Position{AABB: th.box}.Center()
		x0, y0, x1, y1 := k.a.UV(th.sprite)
		lies := map[bool]float32{true: 1}[th.sprite == k.flat]
		at := shot{mid.X, mid.Y, th.box.Size.X / 2, th.box.Size.Y / 2, x0, y0, x1, y1,
			th.z.Altitude, th.z.Height, lies}
		p := photo{sector: i, from: len(f.instances), span: 1}
		f.instances = appendShot(f.instances, &at, &p.span)
		p.to = len(f.instances)
		f.photos = append(f.photos, p)
	}
	developed := func(perFrame int) [4][]byte {
		s := sheetsOver(oddHalves{}, 1.0/8, 128, 125)
		s.perFrame = perFrame
		s.open()
		g := f
		g.photos = slices.Clone(f.photos)
		for j := range g.photos {
			g.photos[j].rect = s.rect(g.photos[j].sector)
		}
		s.develop(&g, k.a)
		var pix [4][]byte
		for j, img := range [4]*render.Image{s.albedo, s.relief, s.albedoL5, s.reliefL5} {
			b := img.Bounds()
			pix[j] = make([]byte, 4*b.Dx()*b.Dy())
			img.ReadPixels(pix[j])
		}
		return pix
	}
	round, alone := developed(sheetPerFrame), developed(1)
	for j, name := range []string{"albedo", "relief", "albedo at L5", "relief at L5"} {
		if !slices.Equal(round[j], alone[j]) {
			t.Errorf("the %s developed in a round differs from each sector developed alone", name)
		}
	}
	// the texel (32, 26) at L5, 64 wide, over (64, 52) and (65, 52)
	if a := round[2][4*(26*64+32)+3]; a < 126 || a > 129 {
		t.Errorf("the texel at L5 the sectors share is %d covered, want half", a)
	}
}
