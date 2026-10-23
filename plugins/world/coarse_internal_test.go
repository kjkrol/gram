package world

import (
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	ilook "github.com/kjkrol/gram/plugins/world/internal/look"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
)

// two sectors of 512 side by side: the camera sees the left one a unit a pixel, the right one a
// tenth of that
type halves struct{}

func (halves) Len() int  { return 2 }
func (halves) Cols() int { return 2 }
func (halves) Box(i int) geom.AABB {
	return geom.NewAABB(geom.NewVec(float64(i)*512, 0), geom.NewVec(float64(i+1)*512, 1000))
}
func (halves) Of(x, _ float64) int { return min(max(int(x/512), 0), 1) }
func (halves) Off(int) bool        { return false }

type farther struct {
	camera.Camera
	d camera.Detail
}

func (f farther) Detail() camera.Detail  { return f.d }
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
// seeing as d says, the sprite standing where stands says
func drawStanding(t *testing.T, d camera.Detail, stands bool, rules ...Rule) *standing {
	t.Helper()
	look := &standing{counting: &counting{Look: ilook.NewFlat(1024, 1000, false, false)}}
	cam := farther{Camera: cameratest.NewFromSpace(1024, 1000, 0), d: d}
	v := view.NewSeen(cam, halves{})
	var drawing Rules
	if err := drawing.Add(rules...); err != nil {
		t.Fatal(err)
	}
	r := newRenderer(flatAtlas{}, seenBy(cam, v), &drawing, func() Look { return look })
	r.shade(nil, nil)
	r.lie(nil, nil, map[render.SpriteID]bool{1: stands})
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
		for _, x := range []float64{100, 700, 900} {
			f := si.NewFactory(&base, &appearance)
			if x == 900 {
				f = si.NewFactory(&base, &appearance, &z)
			}
			f.Create(1)
			f.Next()
			box := plane.NewAABB(geom.NewVec(x, 500), 10, 10)
			base.Slice(&f.Cursor)[0] = Base{Pos: Position{AABB: box}}
			appearance.Slice(&f.Cursor)[0] = render.Appearance{SpriteID: 1}
			items = append(items, aabbworld.Item{ID: f.Cursor.IDs[0], Box: box})
		}
		space.Rebuild(items)
		r.Init(si)
	}})
	views := []*view.View{v}
	view.NewSystem(space, &views, 1024, 1000).Update(nil, 0)
	composed(r, cam)
	return look
}

func TestRenderer_Compose_DrawsACoarseEntityFlat(t *testing.T) {
	look := drawStanding(t, camera.Detail{Coarse: 0.5}, true)
	if look.uprights != 1 || len(look.sprites) != 2 {
		t.Errorf("uprights %d, sprites %d; want the near one standing, the far two flat",
			look.uprights, len(look.sprites))
	}
}

func TestRenderer_Compose_HandsTheLookTheFogOfEachEntity(t *testing.T) {
	look := drawStanding(t, camera.Detail{Sight: 1000, Fog: 400}, false)
	// the second, 700 off, lies a quarter into the fog from 600 to 1000, the third 900 off three
	// quarters
	f := look.fogs
	if len(f) != 4 || f[0] != 0 || f[1] <= 0 || f[1] >= 0.25 || f[2] <= 0.75 || f[3] != 0 {
		t.Errorf("fogs %v, want none, a little, much, and none again after the entities", f)
	}
}

func TestRenderer_Compose_RunsTheRulesOnAChunkInViewAlone(t *testing.T) {
	var ran []float64
	saw := Show(func(b Base) bool {
		ran = append(ran, b.Pos.TopLeft.X)
		return true
	})
	// the sight ends 500 off: the right sector, and the one at 900 in its own chunk, unseen
	drawStanding(t, camera.Detail{Sight: 500}, false, saw)
	for _, x := range ran {
		if x == 900 {
			t.Errorf("the rules ran on %v, want not on the chunk out of view", ran)
		}
	}
	if len(ran) == 0 {
		t.Error("the rules ran on nothing, want the chunk in view")
	}
}
