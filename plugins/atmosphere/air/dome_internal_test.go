package air

import (
	"math"
	"reflect"
	"testing"

	"github.com/kjkrol/gram/internal/gpu/gputest"
	"github.com/kjkrol/gram/render"
)

var (
	clearMass  = Mass{}
	cloudyMass = Mass{Cover: 0.8, Billow: 0.5}
	rainyMass  = Mass{Cover: 0.9, Billow: 0.2, Falls: 0.6}
)

func near(a, b Mass) bool {
	d := func(x, y float32) bool { return math.Abs(float64(x-y)) < 1e-4 }
	return d(a.Cover, b.Cover) && d(a.Billow, b.Billow) && d(a.Falls, b.Falls) && d(a.Damp, b.Damp)
}

func bornOf(m Mass, s int16) func(x, y float32) (Mass, int16) {
	return func(float32, float32) (Mass, int16) { return m, s }
}

func laidDome(width, height, reach float32, m Mass) *Dome {
	d := NewDome(width, height, reach)
	d.Own()
	d.Carry([2]float32{}, bornOf(m, 0), true)
	return d
}

func TestDome_LevelsNestOutToTheHorizon(t *testing.T) {
	d := NewDome(3584, 2688, 38400)
	if n := len(d.levels); n < 3 || n > maxLevels {
		t.Fatalf("a small world with a far horizon has %d levels", n)
	}
	for k := 1; k < len(d.levels); k++ {
		was, l := d.levels[k-1], d.levels[k]
		if l.cell < 2*was.cell {
			t.Errorf("level %d's cells are %v, want at least twice level %d's %v", k, l.cell, k-1,
				was.cell)
		}
		for i := range 2 {
			if l.inside[i] > was.inside[i]-domeBand*l.cell ||
				l.inside[i+2] < was.inside[i+2]+domeBand*l.cell {
				t.Errorf("level %d's rect %v does not take level %d's %v and a band", k, l.inside,
					k-1, was.inside)
			}
		}
	}
	last := d.levels[len(d.levels)-1]
	if last.inside[2]-3584 < 38400 {
		t.Errorf("the dome reaches %v past the world, want the horizon's 38400",
			last.inside[2]-3584)
	}
	if patches := len(d.Slots()); patches > 100 {
		t.Errorf("the dome takes %d patches, want few", patches)
	}
}

func TestDome_ANewMassComesFromTheHorizonWithTheWind(t *testing.T) {
	d := laidDome(3584, 2688, 38400, clearMass)
	coarse := d.levels[len(d.levels)-1].cell
	// the wind blowing along x carries the clouds on; at the horizon, where it blows from, the
	// cloudy air is born
	drift := float32(0)
	for range 6 {
		drift += coarse
		d.Carry([2]float32{drift, 0}, bornOf(cloudyMass, 1), false)
	}
	edge := d.levels[len(d.levels)-1].inside[0]
	if m, s := d.At(edge+coarse, 1344); !near(m, cloudyMass) || s != 1 {
		t.Errorf("by the horizon the wind blows from the mass is %v of %d, want the cloudy one",
			m, s)
	}
	if m, _ := d.At(1792, 1344); !near(m, clearMass) {
		t.Errorf("over the world the mass is %v, want the clear one till the cloudy one comes", m)
	}
	// on till the cloudy air has crossed the dome to beyond the world
	for drift < 1792-edge+4*coarse {
		drift += BoardCell
		d.Carry([2]float32{drift, 0}, bornOf(cloudyMass, 1), false)
	}
	if m, s := d.At(1792, 1344); !near(m, cloudyMass) || s != 1 {
		t.Errorf("once the cloudy air came over the world the mass there is %v of %d", m, s)
	}
}

func TestDome_AMassSailsWithItsClouds(t *testing.T) {
	d := laidDome(640, 480, 2000, clearMass)
	drift := float32(0)
	for _, in := range []Mass{cloudyMass, rainyMass, cloudyMass, rainyMass} {
		for range 3 {
			drift += BoardCell
			d.Carry([2]float32{drift, 0}, bornOf(in, 0), false)
		}
	}
	for range 40 {
		drift += BoardCell
		d.Carry([2]float32{drift, 0}, bornOf(clearMass, 0), false)
	}
	x := float32(200)
	was, _ := d.At(x, 240)
	for _, step := range []float32{37, 90, 128} {
		drift += step
		x += step
		d.Carry([2]float32{drift, 0}, bornOf(clearMass, 0), false)
		if got, _ := d.At(x, 240); !near(got, was) {
			t.Errorf("drifted %v on, the mass under the same cloud is %v, want %v", step, got, was)
		}
	}
}

func TestDome_SettlesWhereItIsTold(t *testing.T) {
	d := laidDome(640, 480, 2000, clearMass)
	d.Settle(bornOf(cloudyMass, 2), func(x, _ float32) float32 {
		if x < 320 {
			return 0.5
		}
		return 0
	}, 0, 1)
	if m, s := d.At(100, 240); math.Abs(float64(m.Cover-0.4)) > 1e-4 || s != 2 {
		t.Errorf("half settled towards the cloudy mass, the west has %v of %d", m, s)
	}
	if m, _ := d.At(600, 240); !near(m, clearMass) {
		t.Errorf("the east, not told, has %v", m)
	}
	if d.Most().Cover != 0.4 {
		t.Errorf("the most cover is %v, want 0.4", d.Most().Cover)
	}
}

func TestPatch_IsAComponentOfFixedSizeWithoutPointers(t *testing.T) {
	var check func(t reflect.Type, path string)
	check = func(ty reflect.Type, path string) {
		switch ty.Kind() {
		case reflect.Struct:
			for i := range ty.NumField() {
				f := ty.Field(i)
				if !f.IsExported() {
					t.Errorf("%s.%s is unexported", path, f.Name)
				}
				check(f.Type, path+"."+f.Name)
			}
		case reflect.Array:
			check(ty.Elem(), path+"[]")
		case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Interface, reflect.String,
			reflect.Chan, reflect.Func:
			t.Errorf("%s is a %v", path, ty.Kind())
		}
	}
	check(reflect.TypeFor[Patch](), "Patch")
}

var airProbe = render.NewMeshShaderWith("air probe", []byte(`
struct Out { @builtin(position) clip: vec4<f32>, @location(0) px: vec2<f32> }
@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Out {
    var corners = array<vec2<f32>, 3>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>(3.0, -1.0),
        vec2<f32>(-1.0, 3.0),
    );
    var o: Out;
    o.clip = vec4<f32>(corners[vid], 0.0, 1.0);
    o.px = vec2<f32>(corners[vid].x + 1.0, 1.0 - corners[vid].y) * 0.5 * U.Pixels;
    return o;
}
@fragment
fn fs_main(o: Out) -> @location(0) vec4<f32> {
    return airAt(U.From + floor(o.px) * U.Step);
}
`), []render.Uniform{{Name: "Pixels", Size: 2}, {Name: "From", Size: 2}, {Name: "Step", Size: 2}})

func TestDome_TheShadersReadTheDomeAsTheCPUDoes(t *testing.T) {
	gputest.Need(t)
	d := laidDome(640, 480, 6000, clearMass)
	drift := [2]float32{}
	carry := func(masses ...Mass) {
		for k, in := range masses {
			for range 4 {
				drift[0] += 7.3 * BoardCell
				drift[1] -= float32(k) * 1.1 * BoardCell
				d.Carry(drift, bornOf(in, int16(k)), false)
			}
		}
	}
	var image DomeImage
	carry(cloudyMass, rainyMass, Mass{Cover: 0.3, Damp: 1}, cloudyMass, clearMass)
	compare(t, d, &image, "carried")
	// the windows moved on and a part settled: the picture written anew where it changed
	carry(rainyMass, cloudyMass)
	d.Settle(bornOf(Mass{Cover: 0.1, Billow: 1}, 9), func(x, _ float32) float32 {
		if x < 0 {
			return 0.7
		}
		return 0
	}, 0, 1)
	compare(t, d, &image, "carried on and settled")
}

func compare(t *testing.T, d *Dome, image *DomeImage, when string) {
	t.Helper()
	const w, h = 96, 72
	// across the whole dome and past it, through every level and its band
	from, step := [2]float32{-7500, -7500}, [2]float32{15640 / w, 15480 / h}
	u := map[string]any{
		"Pixels":    []float32{w, h},
		"From":      from[:],
		"Step":      step[:],
		"AirMasses": image.Of(d),
	}
	for k, names := range levelNames {
		a, c := d.frame(k)
		u[names[0]], u[names[1]] = a[:], c[:]
	}
	screen := render.NewImage(w, h)
	defer screen.Deallocate()
	screen.DrawMesh(nil, airProbe, &render.DrawMeshOptions{Vertices: 3, Uniforms: u})
	pix := make([]byte, 4*w*h)
	screen.ReadPixels(pix)
	worst, at := 0.0, [2]int{}
	for y := range h {
		for x := range w {
			want, _ := d.At(from[0]+float32(x)*step[0], from[1]+float32(y)*step[1])
			i := 4 * (y*w + x)
			for k, v := range []float32{want.Cover, want.Billow, want.Falls, want.Damp} {
				if e := math.Abs(float64(pix[i+k])/255 - float64(v)); e > worst {
					worst, at = e, [2]int{x, y}
				}
			}
		}
	}
	if worst > 2.5/255 {
		t.Errorf("%s, the shaders' dome differs from the CPU's by %v at %v, want within 2.5/255",
			when, worst*255, at)
	}
}
