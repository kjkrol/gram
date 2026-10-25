package light

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

func TestDistant_LightsBySquareOnTheSurfaceFacesIt(t *testing.T) {
	sun := Distant{Dir: [3]float32{0, 0, 2}, Strength: 0.6, Ambient: 0.3}
	if got := sun.Light(0, 0, 1)[0]; !near(got, 0.9) {
		t.Errorf("ground under a sun overhead is lit %v, want ambient 0.3 plus all 0.6", got)
	}
	if got := sun.Light(1, 0, 1)[0]; !near(got, 0.3+0.6*float32(math.Sqrt(0.5))) {
		t.Errorf("a slope at 45° is lit %v, want ambient plus 0.6·cos 45°", got)
	}
	if got := sun.Light(0, 0, -1)[0]; got != 0.3 {
		t.Errorf("a surface turned away is lit %v, want the ambient alone", got)
	}
}

func shadowsOf(alt, height float32, sun Distant) (middles []geom.Vec) {
	if p, ok := sun.ShadowOf(geom.NewAABBAt(geom.NewVec(100, 100), 10, 10), alt, height,
		nil); ok {
		middles = append(middles, geom.NewVec(p.X, p.Y))
	}
	return middles
}

func TestDistant_LaysAShadowAwayFromItPushedOffByHowHighTheThingStands(t *testing.T) {
	// 45° up in the west
	west := Distant{Dir: [3]float32{-1, 0, 1}, Strength: 0.6, Ambient: 0.3}
	walker, hawk := shadowsOf(0, 4, west), shadowsOf(40, 4, west)
	if len(walker) != 1 || len(hawk) != 1 {
		t.Fatalf("shadows %v and %v, want one each", walker, hawk)
	}
	// the walker's centre is at (105, 105): its shadow reaches east by its height, 4 at 45°
	if walker[0].X != 107 || walker[0].Y != 105 {
		t.Errorf("the walker's shadow lies round %v, want (107, 105): east of it by half its "+
			"height", walker[0])
	}
	if hawk[0].X != 147 || hawk[0].Y != 105 {
		t.Errorf("the hawk's shadow lies round %v, want (147, 105): pushed 40 further east",
			hawk[0])
	}
	if down := shadowsOf(0, 4, Distant{Dir: [3]float32{-1, 0, -0.1}}); len(down) != 0 {
		t.Errorf("%d shadows with the light down, want none", len(down))
	}
}

func TestLamp_LaysAShadowAwayFromItLongerTheNearerAndTheTallerTheThing(t *testing.T) {
	lamp := Lamp{X: 100, Y: 100, Z: 20, Reach: 200, Color: Light{1, 1, 1}}
	box := geom.NewAABBAt(geom.NewVec(145, 95), 10, 10) // round (150, 100)
	// 50 east of the lamp, 4 tall under a lamp 20 up: the shadow runs east 50·4/16 past its feet
	walker, ok := lamp.ShadowOf(box, 0, 4, false)
	if !ok || walker.UX != 1 || walker.UY != 0 || !near(walker.X, 150+12.5/2) ||
		!near(walker.Along, 12.5/2+5) {
		t.Errorf("the walker's shadow is %+v, want it east, 12.5 long past its feet", walker)
	}
	if tower, ok := lamp.ShadowOf(box, 0, 30, false); !ok || !near(tower.Along, 30*6/2+5) {
		t.Errorf("a tower taller than the lamp casts %+v, want a shadow as long as the sun's "+
			"longest, six times its height", tower)
	}
	if shape, _ := lamp.ShadowOf(box, 0, 4, true); shape.Fade != 0 ||
		!near(shape.Along, 12.5/2) || shape.Wide != 5 {
		t.Errorf("the shape cast is %+v, want it unfaded, as long as the shadow, as wide as the "+
			"box", shape)
	}
	far := geom.NewAABBAt(geom.NewVec(395, 95), 10, 10)
	if _, ok := lamp.ShadowOf(far, 0, 4, false); ok {
		t.Error("past the lamp's reach it casts a shadow")
	}
}
