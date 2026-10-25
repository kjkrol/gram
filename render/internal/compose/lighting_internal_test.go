package compose

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/render/gputest"
	"github.com/kjkrol/gram/render/internal/light"
)

// the sun 45° up in the west
var west = light.Distant{Dir: [3]float32{-1, 0, 1}, Strength: 1}

// what a world's renderer says casts: a walker's patch, a hero's silhouette of sheet's, a ghost
// casting none and a tower Baked, 4 tall, every 20 east of 500
func casters() items {
	return items(func(f *Frame) {
		for i, c := range []light.Casting{light.Patch, light.Silhouette, light.NoShadow,
			light.Baked} {
			box := geom.NewAABBAt(geom.NewVec(498+20*float32(i), 498), 4, 4)
			f.Cast(box, 0, 4, light.Shadowing{Cast: c}, sheet{}, 1)
		}
	})
}

// the casts a frame from above lit by lt lays, and whether each has a sprite's shape
func castsOf(lt *light.Lights, sources ...Source) ([]light.Umbra, []bool) {
	c := NewComposer(sources...)
	c.Light(func() *light.Lights { return lt })
	c.DrawWorld(nil, topDown())
	var casts []light.Umbra
	var shaped []bool
	for k := 0; k+castFloats <= len(c.lit.casts); k += castFloats {
		v := c.lit.casts[k : k+castFloats]
		casts = append(casts, light.Umbra{X: v[0], Y: v[1], UX: v[2], UY: v[3], Along: v[4],
			Wide: v[5], Fade: v[6], Veil: v[7]})
		shaped = append(shaped, [4]float32(v[8:12]) != [4]float32{})
	}
	return casts, shaped
}

func TestLighting_CastsWhatTheSourcesSayAsThePolicyTheLampsAndTheReliefSay(t *testing.T) {
	white := light.Distant{Dir: [3]float32{0, 0, 1}, Ambient: 1}
	if casts, _ := castsOf(&light.Lights{Sun: white}, casters()); len(casts) != 0 {
		t.Errorf("without a sun %d shadows are cast, want none", len(casts))
	}
	lt := &light.Lights{Sun: west}
	casts, shaped := castsOf(lt, casters())
	if len(casts) != 2 || casts[0].UX != 1 || casts[0].Y != 500 || shaped[0] || !shaped[1] {
		t.Fatalf("the sun casts %v, shaped %v; want the walker's patch and the hero's shape, "+
			"east of them", casts, shaped)
	}
	lt.Policy = func(declared light.Casting, _ float32) light.Casting {
		if declared == light.Silhouette {
			return light.Patch
		}
		return light.NoShadow
	}
	if casts, shaped := castsOf(lt, casters()); len(casts) != 1 || shaped[0] {
		t.Errorf("under the policy %v are cast, shaped %v; want the hero's patch alone", casts,
			shaped)
	}
	lt.Policy = nil
	// a torch west of the walker, 10 up, reaching the hero too: their shadows fall east of it
	lt.Lamps = []light.Lamp{{X: 490, Y: 500, Z: 10, Reach: 40, Color: light.Light{1, 1, 1}}}
	lt.LampShadows = 1
	casts, _ = castsOf(lt, casters())
	if len(casts) != 4 || casts[1].UX != 1 || casts[1].Along <= casts[0].Along {
		t.Fatalf("with a torch %v are cast, want the sun's two and the torch's two, the walker's "+
			"east and longer than the sun's", casts)
	}
	lt.LampShadows = 0
	if casts, _ := castsOf(lt, casters()); len(casts) != 2 {
		t.Errorf("the lamps' shadows off, %d are cast, want the sun's two", len(casts))
	}
	lt.LampShadows, lt.GroundAt = 1, func(x, y float32) float32 { return 0 }
	if casts, _ := castsOf(lt, casters()); len(casts) != 2 {
		t.Errorf("on a relief %d are cast, want the sun's two: the lamps cast on level ground "+
			"alone", len(casts))
	}
}

// how many of the frame's places are the light's own
func lightPlaces(c *Composer) int {
	n := 0
	for _, it := range c.frame.items {
		if it.direct == c.lit.index && it.count == 0 {
			n++
		}
	}
	return n
}

func TestLighting_LaysTheCastsOnceTheSourcesComposedInThePartTheyCameIn(t *testing.T) {
	lt := &light.Lights{Sun: west}
	c := NewComposer(casters())
	c.Light(func() *light.Lights { return lt })
	c.compose(topDown(), wholePart)
	if lightPlaces(c) != 1 || !c.frame.ground {
		t.Errorf("a whole frame: %d places of the casts, the ground read %v; want one over the "+
			"ground's depth", lightPlaces(c), c.frame.ground)
	}
	// the figures over the ground drawn apart, sharp: their shadows go with them
	c = NewComposer(overlyingSource{casters()})
	c.Light(func() *light.Lights { return lt })
	c.Lay(topDown(), true)
	if n := lightPlaces(c); n != 0 {
		t.Errorf("the ground's part lays %d places of the casts, want none: no figure in it", n)
	}
	c.DrawOver(nil, topDown())
	if n := lightPlaces(c); n != 1 || !c.frame.ground {
		t.Errorf("the part over the ground lays %d places of the casts, the ground read %v; want "+
			"the figures' shadows there", n, c.frame.ground)
	}
}

func TestLighting_HandsEveryPartTheLampsNearestTheCamera(t *testing.T) {
	lt := &light.Lights{Sun: west}
	for i := range 10 {
		lt.Lamps = append(lt.Lamps, light.Lamp{X: float32(50 + 90*i), Y: 500, Reach: 50})
	}
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	cam.SetViewport(200, 200)
	cam.CenterOn(900, 500, 0)
	c := NewComposer(items(func(*Frame) {}))
	c.Light(func() *light.Lights { return lt })
	c.Lay(cam, true) // the ground's part, the figures composed apart
	c.setUniforms()
	got := c.Uniforms()
	if got["LampCount"][0] != 8 || got["Lamp0At"][0] != 860 || got["Lamp1At"][0] != 770 ||
		got["Lamp7At"][0] != 230 || got["LightStrength"][0] != 1 {
		t.Errorf("the ground is handed %v lamps, the first at %v, the second %v, the last %v, the "+
			"sun %v strong; want 8 nearest 900: 860, 770 .. 230, and the sun", got["LampCount"],
			got["Lamp0At"], got["Lamp1At"], got["Lamp7At"], got["LightStrength"])
	}
}

func TestLighting_BakesTheGroundsShadeFirstOfTheOpaquePass(t *testing.T) {
	gputest.Need(t)
	lt := &light.Lights{Sun: west,
		Blocks: []light.Block{{X: 500, Y: 500, UX: 1, Along: 10, Wide: 10, Top: 20}},
		Ground: light.Lattice{Cols: 9, Rows: 9, Cell: 128}}
	c := NewComposer(items(func(f *Frame) { f.DrawOpaque() }))
	c.Light(func() *light.Lights { return lt })
	c.compose(topDown(), wholePart)
	if o := c.frame.opaque; len(o) != 2 || o[0] != c.lit.index {
		t.Errorf("the opaque pass draws %v, want the shade baked first, %d, then the ground", o,
			c.lit.index)
	}
	lt.Ground = light.Lattice{}
	c.compose(topDown(), wholePart)
	if o := c.frame.opaque; len(o) != 1 || o[0] == c.lit.index {
		t.Errorf("with no ground to bake over the opaque pass draws %v, want the ground alone", o)
	}
}
