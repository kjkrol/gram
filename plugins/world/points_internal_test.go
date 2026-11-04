package world

import (
	"image/color"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	ilook "github.com/kjkrol/gram/plugins/world/internal/look"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

// everywhere is a world w by h of four sectors by four, each seen at level
type everywhere struct {
	w, h  float32
	level lod.Level
}

func (everywhere) Len() int  { return 16 }
func (everywhere) Cols() int { return 4 }

func (e everywhere) Box(i int) geom.AABB {
	x, y := float32(i%4)*e.w/4, float32(i/4)*e.h/4
	return geom.NewAABB(geom.NewVec(x, y), geom.NewVec(x+e.w/4, y+e.h/4))
}

func (e everywhere) Of(x, y float32) int {
	return min(max(int(4*y/e.h), 0), 3)*4 + min(max(int(4*x/e.w), 0), 3)
}

func (e everywhere) Level(camera.Camera, int) lod.Level { return e.level }
func (everywhere) Ladder() lod.Ladder                   { return lod.Ladder{} }

// pixelsAt draws entities of sprite over boxes from atlas, a world w by h with edges seen at level
// through cam, onto a screen of cam's viewport, and reads it
func pixelsAt(t *testing.T, cam camera.Camera, w, h uint32, edges aabbworld.Edges,
	level lod.Level, a *Atlas, sprite render.SpriteID, boxes ...plane.AABB) func(x, y int) [4]byte {
	t.Helper()
	gputest.Need(t)
	v := view.NewSeen(cam, everywhere{w: float32(w), h: float32(h), level: level})
	look := ilook.NewFlat(float32(w), float32(h), edges.WrapsX(), edges.WrapsY())
	r := newRenderer(a, seenBy(cam, v), &Rules{}, func() Look { return look })
	readied(a)(r)
	r.space = SpaceCfg{Width: w, Height: h, Edges: edges}
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: w, Height: h, Edges: edges,
		BucketSize: 16})
	if err != nil {
		t.Fatal(err)
	}
	var base goke.Comp[Base]
	var appearance goke.Comp[render.Appearance]
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var items []aabbworld.Item
		f := si.NewFactory(&base, &appearance)
		f.Create(len(boxes))
		for i := 0; f.Next(); {
			for j, id := range f.Cursor.IDs {
				base.Slice(&f.Cursor)[j] = Base{Pos: Position{AABB: boxes[i]}}
				appearance.Slice(&f.Cursor)[j] = render.Appearance{SpriteID: sprite}
				items = append(items, aabbworld.Item{ID: id, Box: boxes[i]})
				i++
			}
		}
		space.Rebuild(items)
		r.Init(si)
	}})
	views := []*view.View{v}
	view.NewSystem(space, &views, w, h).Update(nil, 0)
	r.Publish()
	r.Swap()
	vw, vh := cam.Viewport()
	screen := render.NewImage(int(vw), int(vh))
	render.NewComposer(r).DrawWorld(screen, cam)
	pix := make([]byte, 4*int(vw)*int(vh))
	screen.ReadPixels(pix)
	return func(x, y int) [4]byte { i := 4 * (y*int(vw) + x); return [4]byte(pix[i : i+4]) }
}

// blobSource draws sprite over box through the world's own look, as the renderer did at L2
type blobSource struct {
	look   *ilook.Flat
	atlas  render.AtlasSource
	sprite render.SpriteID
	box    plane.AABB
}

func (s *blobSource) Compose(f *render.Frame, cam camera.Camera) {
	s.look.Begin(f, cam)
	f.DrawAt(f.At(0, 0, 0))
	s.look.Sprite(f, cam, s.box, entity.Z{}, s.atlas, render.Appearance{SpriteID: s.sprite},
		render.WhiteLight)
}

func (s *blobSource) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	s.look.DrawSprites(t, cam, u)
}

func TestRenderer_Compose_AFigureAtL2IsItsBlobAsTheLookDrewIt(t *testing.T) {
	a, own := leveledRed()
	box := plane.NewAABB(geom.NewVec(20, 12), 10, 6)
	cam := cameratest.NewFromSpace(64, 32, 0)
	cam.SetViewport(64, 32)
	point := pixelsAt(t, cam, 64, 32, 0, lod.L2, a, own, box)
	screen := render.NewImage(64, 32)
	render.NewComposer(&blobSource{look: ilook.NewFlat(64, 32, false, false), atlas: a,
		sprite: a.versions[own][lod.L2].id, box: box}).DrawWorld(screen, cam)
	pix := make([]byte, 4*64*32)
	screen.ReadPixels(pix)
	drawn := 0
	for y := range 32 {
		for x := range 64 {
			i := 4 * (y*64 + x)
			if p, want := point(x, y), [4]byte(pix[i:i+4]); p != want {
				t.Fatalf("pixel (%d, %d) is %v, want %v as the look drew it", x, y, p, want)
			}
			if pix[i+3] != 0 {
				drawn++
			}
		}
	}
	if drawn == 0 {
		t.Fatal("nothing drawn")
	}
}

func TestRenderer_Compose_AFigureAtL2UnderAPixelIsAsOpaqueAsItCovers(t *testing.T) {
	a, own := leveledRed()
	cam := cameratest.NewFromSpace(32, 32, 0)
	cam.SetViewport(32, 32)
	pix := pixelsAt(t, cam, 32, 32, 0, lod.L2, a, own, plane.NewAABB(geom.NewVec(10.1, 20.1),
		0.5, 0.5))
	for y := range 32 {
		for x := range 32 {
			p := pix(x, y)
			if x == 10 && y == 20 {
				if p[3] < 62 || p[3] > 66 || p[0] != p[3] || p[1] != 0 {
					t.Errorf("the point is %v, want red covering a quarter of the pixel", p)
				}
			} else if p[3] != 0 {
				t.Errorf("pixel (%d, %d) is %v, want nothing", x, y, p)
			}
		}
	}
}

func TestRenderer_Compose_AFigureAtL2IsDrawnInTheLookDeclaredThere(t *testing.T) {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 32, Height: 32},
		Entities: EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	own := p.Kinds().NewSprite()
	a := p.NewAtlas()
	a.Add(own, 8, render.Solid(color.RGBA{R: 255, A: 255})).
		At(lod.L2, render.Solid(color.RGBA{G: 255, A: 255}))
	a.Close()
	cam := cameratest.NewFromSpace(32, 32, 0)
	cam.SetViewport(32, 32)
	pix := pixelsAt(t, cam, 32, 32, 0, lod.L2, a, own, plane.NewAABB(geom.NewVec(8, 8), 4, 4))
	if got := pix(10, 10); got != [4]byte{0, 255, 0, 255} {
		t.Errorf("the point is %v, want the green declared at L2", got)
	}
}

func TestRenderer_Compose_AFigureAtL2AcrossTheSeamShowsOnce(t *testing.T) {
	a, own := leveledRed()
	cam := cameratest.NewFromSpace(64, 64, aabbworld.Torus)
	cam.SetViewport(32, 32)
	cam.MoveTo(48, 16) // the seam at 16 across the picture
	drawn := func(boxes ...plane.AABB) int {
		pix := pixelsAt(t, cam, 64, 64, aabbworld.Torus, lod.L2, a, own, boxes...)
		n := 0
		for y := range 32 {
			for x := range 32 {
				if pix(x, y)[3] != 0 {
					n++
				}
			}
		}
		return n
	}
	// one with its middle on the seam and one past it, and the two well before it
	across := drawn(plane.NewAABB(geom.NewVec(62, 20), 4, 4),
		plane.NewAABB(geom.NewVec(4, 30), 2, 2))
	before := drawn(plane.NewAABB(geom.NewVec(54, 20), 4, 4),
		plane.NewAABB(geom.NewVec(60, 30), 2, 2))
	if across != before || before == 0 {
		t.Errorf("%d pixels drawn across the seam, %d before it; want as many, some", across,
			before)
	}
}
