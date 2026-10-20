package cameras_test

import (
	"bytes"
	"encoding/gob"
	"log"
	"math"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
)

func newWorld() *world.Plugin {
	return world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
}

func TestCameras_AreSavedAndRestoredWithTheGame(t *testing.T) {
	newCamera := func(p *cameras.Plugin, w, h float32) camera.Camera {
		cam := p.New(cameras.FromAbove, camera.Config{})
		cam.SetViewport(w, h)
		return cam
	}
	saved := cameras.NewPlugin(newWorld(), cameras.Config{})
	newCamera(saved, 200, 200)
	second := newCamera(saved, 300, 100)
	second.Translate(120, 80)
	second.(camera.Fastenable).Fasten(camera.Fastening{Entity: 7, How: camera.Centred})
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	for _, v := range saved.Serializable().Persisted() {
		if err := enc.Encode(v); err != nil {
			t.Fatal(err)
		}
	}

	loaded := cameras.NewPlugin(newWorld(), cameras.Config{})
	first := newCamera(loaded, 200, 200) // made before the load
	dec := gob.NewDecoder(&buf)
	for _, v := range loaded.Serializable().Persisted() {
		if err := dec.Decode(v); err != nil {
			t.Fatal(err)
		}
	}
	loaded.Restore()
	again := newCamera(loaded, 300, 100) // made after it, as a scene makes its cameras
	if got, want := again.Bounds(), second.Bounds(); got != want {
		t.Errorf("the second camera after a load shows %v, want %v", got, want)
	}
	if got := again.(camera.Fastenable).Fastening(); got != (camera.Fastening{Entity: 7, How: camera.Centred}) {
		t.Errorf("the second camera after a load is fastened %+v, want to entity 7, Centred", got)
	}
	if got := len(loaded.Cameras()); got != 2 || loaded.Cameras()[0] != first || loaded.Cameras()[1] != again {
		t.Errorf("%d cameras in the order made: want 2", got)
	}
}

func TestNew_EveryCameraHasAConfigOfItsOwn(t *testing.T) {
	p := cameras.NewPlugin(newWorld(), cameras.Config{})
	red := p.New(cameras.FromAbove, camera.Config{Zoom: 2})
	blue := p.New(cameras.FromAbove, camera.Config{})
	if red.Zoom() != 2 || blue.Zoom() != 1 {
		t.Errorf("red's camera at zoom %v, blue's at %v; want each its own: 2 and 1", red.Zoom(), blue.Zoom())
	}
}

func reliefCameras() *cameras.Plugin {
	return cameras.NewPlugin(newWorld(), cameras.Config{Turn: true, Tilt: true, Cell: 32})
}

// what a save is made of: the cameras, or a save of theirs from before
type saving interface{ Serializable() plugin.Serializable }

func carried(t *testing.T, from saving, to *cameras.Plugin) {
	t.Helper()
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	for _, v := range from.Serializable().Persisted() {
		if err := enc.Encode(v); err != nil {
			t.Fatal(err)
		}
	}
	dec := gob.NewDecoder(&buf)
	for _, v := range to.Serializable().Persisted() {
		if err := dec.Decode(v); err != nil {
			t.Fatal(err)
		}
	}
	to.Restore()
}

func TestCameras_InReliefAreSavedAsTheirStick(t *testing.T) {
	saved := reliefCameras()
	cam := saved.New(cameras.Isometrically, camera.Config{})
	cam.SetViewport(300, 200)
	cam.CenterOn(400, 500, 30)
	cameratest.Turn(cam, 0.4)
	cameratest.Tilt(cam, 0.3)
	cam.ZoomIn(2, 400, 500)
	loaded := reliefCameras()
	again := loaded.New(cameras.FromAbove, camera.Config{})
	again.SetViewport(300, 200)
	carried(t, saved, loaded)
	for _, q := range [][3]float32{{400, 500, 30}, {420, 470, 0}} {
		bx, by := cam.Project(q[0], q[1], q[2])
		if ax, ay := again.Project(q[0], q[1], q[2]); math.Abs(float64(ax-bx)) > 1e-3 ||
			math.Abs(float64(ay-by)) > 1e-3 {
			t.Errorf("loaded, %v is drawn at (%v, %v), want (%v, %v) as saved", q, ax, ay, bx, by)
		}
	}
}

func TestCameras_InReliefSavedBeforeTheStickBeginAfresh(t *testing.T) {
	var old bytes.Buffer
	enc := gob.NewEncoder(&old)
	// the views' save before the stick: the window's corner, the zoom, the heading and the pitch
	for _, v := range []any{geom.NewVec(10, 20), float32(2), float32(0.5), float32(0.7)} {
		if err := enc.Encode(v); err != nil {
			t.Fatal(err)
		}
	}
	var save bytes.Buffer
	if err := gob.NewEncoder(&save).Encode([][]byte{old.Bytes()}); err != nil {
		t.Fatal(err)
	}
	var logged bytes.Buffer
	log.SetOutput(&logged)
	defer log.SetOutput(os.Stderr)
	p := reliefCameras()
	cam := p.New(cameras.Isometrically, camera.Config{})
	cam.SetViewport(300, 200)
	bx, by := cam.Project(500, 500, 0)
	// the save holds the cameras alone, from before the Settings were kept after them
	if err := gob.NewDecoder(&save).Decode(p.Serializable().Persisted()[0]); err != nil {
		t.Fatal(err)
	}
	p.Restore()
	if ax, ay := cam.Project(500, 500, 0); ax != bx || ay != by {
		t.Errorf("an old save moved the camera: (500, 500) from (%v, %v) to (%v, %v)",
			bx, by, ax, ay)
	}
	if !strings.Contains(logged.String(), "camera 0 is not as it was saved") {
		t.Errorf("an old save logged %q, want camera 0 named as not as it was saved",
			logged.String())
	}
}

func TestNew_ACameraIsDrawnAtTheConfigsResolutionAMinimapsAtAll(t *testing.T) {
	p := cameras.NewPlugin(newWorld(), cameras.Config{Tactical: cameras.Knobs{Resolution: 0.7}})
	cam := p.New(cameras.FromAbove, camera.Config{})
	whole := p.New(cameras.FromAbove, camera.Config{Whole: true})
	if got := camera.ResolutionOf(cam); got != 0.7 {
		t.Errorf("the camera is drawn at %v of its pixels, want the Config's 0.7", got)
	}
	if got := camera.ResolutionOf(whole); got != 1 {
		t.Errorf("a camera keeping the whole world in view is drawn at %v, want all its pixels",
			got)
	}
	relief := reliefCameras()
	if got := camera.ResolutionOf(relief.New(cameras.Isometrically, camera.Config{})); got != 1 {
		t.Errorf("a camera of a zero Resolution is drawn at %v, want all its pixels", got)
	}
	least := cameras.NewPlugin(newWorld(), cameras.Config{Tactical: cameras.Knobs{Resolution: 0.1}})
	lowest := least.New(cameras.FromAbove, camera.Config{})
	if got := camera.ResolutionOf(lowest); got != cameras.LeastResolution {
		t.Errorf("a Resolution of 0.1 draws at %v, want the least, %v", got, cameras.LeastResolution)
	}
}

func TestSettings_AreSavedWithTheCamerasAndAZeroSavedComesBackSo(t *testing.T) {
	saved := cameras.NewPlugin(newWorld(), cameras.Config{})
	saved.ShowZones(true)
	loaded := cameras.NewPlugin(newWorld(),
		cameras.Config{Tactical: cameras.Knobs{Strategy: cameras.Uniform}})
	carried(t, saved, loaded)
	if !loaded.ZonesShown() {
		t.Error("the zones shown were not saved with the cameras")
	}
	if loaded.Strategy(cameras.Tactical) != cameras.Still {
		t.Error("the resolution follows the budget after a load that saved it still")
	}
}

var defaultLayers = []camera.Layer{
	{Share: 1, Resolution: 0.6},
	{Share: 0.6, Resolution: 0.8},
	{Share: 0.35, Resolution: 1},
}

func TestNew_UnderLayersACameraIsDrawnInTheDefaultLayersAMinimapsWhole(t *testing.T) {
	layered := cameras.Config{Tactical: cameras.Knobs{Layers: true}}
	p := cameras.NewPlugin(newWorld(), layered)
	cam := p.New(cameras.FromAbove, camera.Config{})
	whole := p.New(cameras.FromAbove, camera.Config{Whole: true})
	if got := camera.LayersOf(cam); !slices.Equal(got, defaultLayers) ||
		camera.ResolutionOf(cam) != 1 {
		t.Errorf("the camera is drawn in %v at %v, want the default %v at all its centre's pixels",
			got, camera.ResolutionOf(cam), defaultLayers)
	}
	if got := camera.LayersOf(whole); len(got) != 0 {
		t.Errorf("a camera keeping the whole world in view is drawn in %v, want whole", got)
	}
	relief := cameras.NewPlugin(newWorld(), cameras.Config{
		Turn:     true,
		Tilt:     true,
		Cell:     32,
		Tactical: cameras.Knobs{Layers: true},
	})
	if got := camera.LayersOf(relief.New(cameras.Isometrically, camera.Config{})); !slices.Equal(
		got, defaultLayers) {
		t.Errorf("a camera in relief is drawn in %v, want the default", got)
	}
	centre := cameras.NewPlugin(newWorld(),
		cameras.Config{Tactical: cameras.Knobs{Layers: true, Resolution: 0.7}})
	cam = centre.New(cameras.FromAbove, camera.Config{})
	if got := camera.LayersOf(cam); got[2].Resolution != 0.7 || camera.ResolutionOf(cam) != 0.7 {
		t.Errorf("at a Resolution of 0.7 the camera is drawn in %v at %v, want its centre layer "+
			"at 0.7", got, camera.ResolutionOf(cam))
	}
}

// a setting's label and section
type listed struct{ label, section string }

func labelOf(st control.Setting) string {
	switch st := st.(type) {
	case control.Switch:
		return st.Label
	case control.Choice:
		return st.Label
	case control.Range:
		return st.Label
	}
	return ""
}

// the eight settings of a profile's knobs, the layers' under layers, the rest under section
func knobsListed(section, layers string) []listed {
	return []listed{
		{"Follows the frame budget", section},
		{"Resolution", section},
		{"Figures at every pixel", section},
		{"Drawn in layers", layers},
		{"Inner layer", layers},
		{"Middle layer", layers},
		{"Outer layer resolution", layers},
		{"Middle layer resolution", layers},
		{"Show the layers", layers},
	}
}

func TestSettings_AreTheZonesTheBudgetAndEachProfilesKnobsInTheirSectionsEachBuildingItsCommand(
	t *testing.T) {
	p := cameras.NewPlugin(newWorld(), cameras.Config{
		Tactical: cameras.Knobs{Strategy: cameras.Uniform},
		Riding:   cameras.Knobs{Strategy: cameras.InLayers, Resolution: 0.8},
	})
	settings := p.Settings()
	want := []listed{{"Scroll zones", ""}, {"Frame budget", control.Picture},
		{"Samples the frame rate over", control.Picture}}
	want = append(want, knobsListed(control.PictureTactical, control.PictureTacticalLayers)...)
	want = append(want, knobsListed(control.PictureRiding, control.PictureRidingLayers)...)
	if len(settings) != len(want) {
		t.Fatalf("%d settings, want %d: the zones, the budget, the sampling, a profile's nine twice", len(settings), len(want))
	}
	for i, want := range want {
		if got := (listed{labelOf(settings[i]), control.SectionOf(settings[i])}); got != want {
			t.Errorf("setting %d is %q in %q, want %q in %q", i, got.label, got.section,
				want.label, want.section)
		}
	}
	budget := settings[1].(control.Range)
	if budget.Min != 30 || budget.Max != 120 || budget.Get() != 60 || budget.Shows(60) != "60 FPS" {
		t.Errorf("the budget runs %v to %v at %v shown %q, want 30 to 120 at 60, \"60 FPS\"",
			budget.Min, budget.Max, budget.Get(), budget.Shows(60))
	}
	if got := budget.Set(30); got != (cameras.Budget{Frame: time.Second / 30}) {
		t.Errorf("the budget at 30 builds %v, want a frame of a thirtieth of a second", got)
	}
	sampling := settings[2].(control.Range)
	if sampling.Min != 1 || sampling.Max != 20 || sampling.Get() != 2 || sampling.Shows(5) != "5 s" ||
		sampling.Set(5) != (cameras.Sampling{Over: 5 * time.Second}) {
		t.Errorf("the sampling runs %v to %v at %v shown %q building %v, want 1 to 20 at 2, \"5 s\", "+
			"Sampling{5s}", sampling.Min, sampling.Max, sampling.Get(), sampling.Shows(5),
			sampling.Set(5))
	}
	for _, c := range []struct {
		profile    cameras.Profile
		at         int
		strategy   cameras.Strategy
		resolution float32
		layers     bool
	}{
		{cameras.Tactical, 3, cameras.Uniform, 1, false},
		{cameras.Riding, 12, cameras.InLayers, 0.8, true},
	} {
		knobs := settings[c.at:]
		strategy := knobs[0].(control.Choice)
		if strategy.Get() != int(c.strategy) || len(strategy.Options) != 3 {
			t.Errorf("%v: the strategy shows %d of %v, want %v of three", c.profile,
				strategy.Get(), strategy.Options, c.strategy)
		}
		if got := strategy.Set(2); got != (cameras.Adapt{Profile: c.profile,
			Strategy: cameras.InLayers}) {
			t.Errorf("%v: the strategy's third choice builds %v, want Adapt{InLayers}", c.profile,
				got)
		}
		resolution := knobs[1].(control.Range)
		if resolution.Min != cameras.LeastResolution || resolution.Max != 1 ||
			float32(resolution.Get()) != c.resolution || resolution.Shows(0.85) != "0.85" {
			t.Errorf("%v: the resolution runs %v to %v at %v shown %q, want 0.4 to 1 at %v, 0.85",
				c.profile, resolution.Min, resolution.Max, resolution.Get(),
				resolution.Shows(0.85), c.resolution)
		}
		if got := resolution.Set(0.7); got != (cameras.Picture{Profile: c.profile,
			Resolution: 0.7}) {
			t.Errorf("%v: the resolution at 0.7 builds %v, want Picture{0.7}", c.profile, got)
		}
		sharp := knobs[2].(control.Switch)
		if !sharp.Get() || sharp.Set(false) != (cameras.SharpFigures{Profile: c.profile}) {
			t.Errorf("%v: the figures at every pixel are %v building %v, want on, "+
				"SharpFigures{On: false}", c.profile, sharp.Get(), sharp.Set(false))
		}
		layers := knobs[3].(control.Switch)
		if layers.Get() != c.layers ||
			layers.Set(true) != (cameras.Layers{Profile: c.profile, On: true}) {
			t.Errorf("%v: the layers are %v building %v, want %v, Layers{On: true}", c.profile,
				layers.Get(), layers.Set(true), c.layers)
		}
		inner, middle := knobs[4].(control.Range), knobs[5].(control.Range)
		if inner.Get() != 35 || middle.Get() != 60 || inner.Shows(35) != "35%" {
			t.Errorf("%v: the shares show %v and %v (%q), want 35%% and 60%%", c.profile,
				inner.Get(), middle.Get(), inner.Shows(35))
		}
		if got := inner.Set(55); got != (cameras.LayerShares{Profile: c.profile, Inner: 0.55,
			Middle: 0.65}) {
			t.Errorf("%v: the inner layer at 55%% builds %v, want the middle pushed to 65%%",
				c.profile, got)
		}
		if got := middle.Set(40); got != (cameras.LayerShares{Profile: c.profile, Inner: 0.3,
			Middle: 0.4}) {
			t.Errorf("%v: the middle layer at 40%% builds %v, want the inner pushed to 30%%",
				c.profile, got)
		}
		outer, middleRes := knobs[6].(control.Range), knobs[7].(control.Range)
		if float32(outer.Get()) != 0.6 || float32(middleRes.Get()) != 0.8 {
			t.Errorf("%v: the layers' resolutions show %v and %v, want 0.6 and 0.8", c.profile,
				outer.Get(), middleRes.Get())
		}
		if got := outer.Set(0.5); got != (cameras.LayerResolutions{Profile: c.profile,
			Outer: 0.5, Middle: 0.8}) {
			t.Errorf("%v: the outer layer at 0.5 builds %v, want LayerResolutions{0.5, 0.8}",
				c.profile, got)
		}
		if got := middleRes.Set(0.9); got != (cameras.LayerResolutions{Profile: c.profile,
			Outer: 0.6, Middle: 0.9}) {
			t.Errorf("%v: the middle layer at 0.9 builds %v, want LayerResolutions{0.6, 0.9}",
				c.profile, got)
		}
	}
	p.SetResolution(cameras.Riding, 0.5)
	if tactical, riding := settings[4].(control.Range).Get(),
		settings[13].(control.Range).Get(); tactical != 1 || riding != 0.5 {
		t.Errorf("the riding resolution set to 0.5, the settings show %v tactical and %v riding, "+
			"want the tactical untouched at 1", tactical, riding)
	}
	shown := settings[11].(control.Switch)
	if shown.Get() || shown.Set(true) != (cameras.ShowLayers{Profile: cameras.Tactical, Shown: true}) {
		t.Errorf("the layers shown are %v building %v, want hidden, ShowLayers{Tactical, true}",
			shown.Get(), shown.Set(true))
	}
}

func TestRestore_DrawsTheCamerasInLayersAsTheSettingsLoadedSay(t *testing.T) {
	saved := cameras.NewPlugin(newWorld(), cameras.Config{Tactical: cameras.Knobs{Layers: true}})
	saved.SetLayerShares(cameras.Tactical, 0.3, 0.5)
	saved.SetLayerResolutions(cameras.Tactical, 0.5, 0.7)
	loaded := cameras.NewPlugin(newWorld(), cameras.Config{})
	before := loaded.New(cameras.FromAbove, camera.Config{})
	before.(camera.Resolved).SetResolution(0.7)
	carried(t, saved, loaded)
	want := []camera.Layer{
		{Share: 1, Resolution: 0.5},
		{Share: 0.5, Resolution: 0.7},
		{Share: 0.3, Resolution: 1},
	}
	if got := camera.LayersOf(before); !slices.Equal(got, want) ||
		camera.ResolutionOf(before) != 1 {
		t.Errorf("after the load the camera made before it is drawn in %v at %v, want the layers "+
			"saved at all its centre's pixels", got, camera.ResolutionOf(before))
	}
	after := loaded.New(cameras.FromAbove, camera.Config{})
	if got := camera.LayersOf(after); !slices.Equal(got, want) {
		t.Errorf("a camera made after the load is drawn in %v, want the layers saved", got)
	}
	whole := cameras.NewPlugin(newWorld(), cameras.Config{})
	layered := cameras.NewPlugin(newWorld(),
		cameras.Config{Tactical: cameras.Knobs{Layers: true}})
	cam := layered.New(cameras.FromAbove, camera.Config{})
	carried(t, whole, layered)
	if got := camera.LayersOf(cam); len(got) != 0 || layered.Layers(cameras.Tactical) {
		t.Errorf("after a load saved whole the camera is drawn in %v, want whole", got)
	}
}

func TestNew_ACameraDrawsItsFiguresAtEveryPixelUnlessTheConfigSaysSoft(t *testing.T) {
	p := cameras.NewPlugin(newWorld(), cameras.Config{})
	if cam := p.New(cameras.FromAbove, camera.Config{}); !camera.SharpFiguresOf(cam) ||
		!p.SharpFigures(cameras.Tactical) {
		t.Error("a camera of the zero Config draws its figures at the ground's resolution, want " +
			"at every pixel")
	}
	soft := cameras.NewPlugin(newWorld(),
		cameras.Config{Tactical: cameras.Knobs{SoftFigures: true}})
	if cam := soft.New(cameras.FromAbove, camera.Config{}); camera.SharpFiguresOf(cam) ||
		soft.SharpFigures(cameras.Tactical) {
		t.Error("under Config.SoftFigures a camera draws its figures at every pixel, want the " +
			"ground's resolution")
	}
	relief := cameras.NewPlugin(newWorld(), cameras.Config{
		Turn:     true,
		Tilt:     true,
		Cell:     32,
		Tactical: cameras.Knobs{SoftFigures: true},
	})
	if cam := relief.New(cameras.Isometrically, camera.Config{}); camera.SharpFiguresOf(cam) {
		t.Error("under Config.SoftFigures a camera in relief draws its figures at every pixel, " +
			"want the ground's resolution")
	}
}

func TestSharpFigures_ReachesEveryCameraAndTheTelemetry(t *testing.T) {
	w, p, start, tick := emptyWorld(t, cameras.Config{Tactical: cameras.Knobs{Resolution: 0.7}})
	cam := p.New(cameras.FromAbove, camera.Config{})
	start()
	if got := report(p); got != "Picture: tactical\n0.70 of the pixels\nthe figures sharp\nstill" {
		t.Fatalf("the telemetry says %q, want the figures sharp at 0.70", got)
	}
	w.Carrier().Put(command.Nobody, cameras.SharpFigures{Profile: cameras.Tactical, On: false})
	tick()
	if camera.SharpFiguresOf(cam) || p.SharpFigures(cameras.Tactical) {
		t.Error("after SharpFigures{On: false} the figures are drawn at every pixel, want the " +
			"ground's resolution")
	}
	if got := report(p); got != "Picture: tactical\n0.70 of the pixels\nstill" {
		t.Errorf("the telemetry says %q, want nothing of the figures", got)
	}
	w.Carrier().Put(command.Nobody, cameras.SharpFigures{Profile: cameras.Tactical, On: true})
	tick()
	if !camera.SharpFiguresOf(cam) || !p.SharpFigures(cameras.Tactical) {
		t.Error("after SharpFigures{On: true} the figures are drawn at the ground's resolution, " +
			"want every pixel")
	}
	w.Carrier().Put(command.Nobody, cameras.Picture{Profile: cameras.Tactical, Resolution: 1})
	tick()
	if got := report(p); got != "Picture: tactical\n1.00 of the pixels\nstill" {
		t.Errorf("at all the pixels the telemetry says %q, want nothing of the figures", got)
	}
}

func TestRestore_DrawsTheFiguresSoftAsTheSettingsLoadedSay(t *testing.T) {
	saved := cameras.NewPlugin(newWorld(), cameras.Config{})
	saved.SetSharpFigures(cameras.Tactical, false)
	loaded := cameras.NewPlugin(newWorld(), cameras.Config{})
	before := loaded.New(cameras.FromAbove, camera.Config{})
	carried(t, saved, loaded)
	after := loaded.New(cameras.FromAbove, camera.Config{})
	if sharp := loaded.SharpFigures(cameras.Tactical); sharp || camera.SharpFiguresOf(before) ||
		camera.SharpFiguresOf(after) {
		t.Errorf("after a load saved soft the figures are drawn at every pixel: the plugin %v, "+
			"the camera made before the load %v, after it %v; want none", sharp,
			camera.SharpFiguresOf(before), camera.SharpFiguresOf(after))
	}
	// a save of sharp figures holds no field for them, as a save from before the setting
	sharp := cameras.NewPlugin(newWorld(), cameras.Config{})
	soft := cameras.NewPlugin(newWorld(),
		cameras.Config{Tactical: cameras.Knobs{SoftFigures: true}})
	cam := soft.New(cameras.FromAbove, camera.Config{})
	carried(t, sharp, soft)
	if !soft.SharpFigures(cameras.Tactical) || !camera.SharpFiguresOf(cam) {
		t.Error("after a load saved sharp the figures stay at the ground's resolution, want " +
			"every pixel")
	}
}

func TestProfileOf_IsRidingInsideAnEntityElseTactical(t *testing.T) {
	cam := cameratest.NewFromSpace(100, 100, 0)
	for how, want := range map[camera.How]cameras.Profile{
		0:              cameras.Tactical,
		camera.Loose:   cameras.Tactical,
		camera.Centred: cameras.Tactical,
		camera.Behind:  cameras.Tactical,
		camera.Inside:  cameras.Riding,
	} {
		cam.(camera.Fastenable).Fasten(camera.Fastening{Entity: 7, How: how})
		if got := cameras.ProfileOf(cam); got != want {
			t.Errorf("fastened %v, the camera's profile is %v, want %v", how, got, want)
		}
	}
	if got := cameras.ProfileOf(struct{ camera.Camera }{cam}); got != cameras.Tactical {
		t.Errorf("a camera that cannot be fastened has the profile %v, want tactical", got)
	}
}

// the cameras' Settings as a save from before the profiles holds them
type settingsBeforeProfiles struct {
	Zones       bool
	Strategy    cameras.Strategy
	Resolution  float32
	SoftFigures bool
	Layers      bool
	Inner       float32
	Middle      float32
	OuterRes    float32
	MiddleRes   float32
	FrameBudget time.Duration
	ShowLayers  bool
}

func (s *settingsBeforeProfiles) Serializable() plugin.Serializable {
	return saveBeforeProfiles{s}
}

// no camera and the Settings of the time
type saveBeforeProfiles struct{ settings *settingsBeforeProfiles }

func (s saveBeforeProfiles) Persisted() []any {
	return append([]any{&[][]byte{}}, plugin.Keep(s.settings).Persisted()...)
}

func TestKnobs_FilledGivesAZeroSaveTheDefaults(t *testing.T) {
	old := &settingsBeforeProfiles{
		Zones:       true,
		Strategy:    cameras.InLayers,
		Resolution:  0.7,
		SoftFigures: true,
		Layers:      true,
		Inner:       0.3,
		Middle:      0.5,
		OuterRes:    0.5,
		MiddleRes:   0.7,
		FrameBudget: time.Second / 30,
		ShowLayers:  true,
	}
	p := cameras.NewPlugin(newWorld(), cameras.Config{
		Tactical: cameras.Knobs{Strategy: cameras.Uniform, Resolution: 0.5, Inner: 0.2},
		Riding:   cameras.Knobs{Strategy: cameras.InLayers, Layers: true, OuterRes: 0.4},
	})
	cam := p.New(cameras.FromAbove, camera.Config{})
	carried(t, old, p)
	if !p.ZonesShown() || p.Budget() != time.Second/30 {
		t.Errorf("an old save loaded shows the zones %v at a budget of %v, want them shown at a "+
			"thirtieth of a second, as saved", p.ZonesShown(), p.Budget())
	}
	for _, profile := range []cameras.Profile{cameras.Tactical, cameras.Riding} {
		inner, middle := p.LayerShares(profile)
		outer, middleRes := p.LayerResolutions(profile)
		if p.Strategy(profile) != cameras.Still || p.Resolution(profile) != 1 ||
			!p.SharpFigures(profile) || p.Layers(profile) || inner != 0.35 || middle != 0.6 ||
			outer != 0.6 || middleRes != 0.8 {
			t.Errorf("%v after an old save: %v at %v, sharp %v, in layers %v of %v/%v at %v/%v; "+
				"want the defaults: still at 1, sharp, whole, of 0.35/0.6 at 0.6/0.8", profile,
				p.Strategy(profile), p.Resolution(profile), p.SharpFigures(profile),
				p.Layers(profile), inner, middle, outer, middleRes)
		}
	}
	if got := camera.ResolutionOf(cam); got != 1 || len(camera.LayersOf(cam)) != 0 {
		t.Errorf("after an old save the camera is drawn at %v in %v, want whole at all its pixels",
			got, camera.LayersOf(cam))
	}
}

// a camera in relief, isometric, fastened over the one entity of an empty world, its plugin
// reaching the perspective and drawing by tactical and riding: give carries a command out in a
// tick
func ridden(t *testing.T, tactical, riding cameras.Knobs) (p *cameras.Plugin, cam camera.Camera,
	give func(command.Command)) {
	t.Helper()
	w, p, start, tick := emptyWorld(t, cameras.Config{
		Turn:        true,
		Tilt:        true,
		Perspective: true,
		Cell:        32,
		Tactical:    tactical,
		Riding:      riding,
	})
	cam = p.New(cameras.Isometrically, camera.Config{})
	cam.SetViewport(400, 300)
	kind.Define[float64](w.Kinds(), "rider", kind.Spec{
		comp.Load(func(x float64) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(x, x), 10, 10)}
		}),
		comp.Const(world.Velocity{}),
	})
	rider := kind.Named[float64](w.Kinds(), "rider")
	w.Seed(rider.Entry(500).Told(cameras.Follow{Camera: cam}))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	start()
	tick()
	if how := camera.HowOf(cam); how != camera.Centred {
		t.Fatalf("the camera is fastened %v, want Centred over the entity", how)
	}
	return p, cam, func(cmd command.Command) {
		w.Carrier().Put(command.Nobody, cmd)
		tick()
	}
}

var laid = map[camera.Layout]string{camera.Inset: "inset", camera.Footed: "footed"}

func TestDress_ACameraRidingTakesTheRidingProfileAndComesBackTactical(t *testing.T) {
	_, cam, give := ridden(t, cameras.Knobs{Layers: true}, cameras.Knobs{Resolution: 0.6})
	for _, step := range []struct {
		when       string
		give       command.Command
		profile    cameras.Profile
		layers     int
		layout     camera.Layout
		resolution float32
	}{
		{"over the entity isometrically", nil, cameras.Tactical, 3, camera.Inset, 1},
		{"over it in perspective", cameras.View{Camera: cam}, cameras.Tactical, 3, camera.Footed,
			1},
		{"behind it", cameras.Ride{Camera: cam}, cameras.Tactical, 3, camera.Footed, 1},
		{"riding in it", cameras.Ride{Camera: cam}, cameras.Riding, 0, camera.Inset, 0.6},
		{"over it again", cameras.Ride{Camera: cam}, cameras.Tactical, 3, camera.Footed, 1},
	} {
		if step.give != nil {
			give(step.give)
		}
		profile, layers := cameras.ProfileOf(cam), len(camera.LayersOf(cam))
		layout, resolution := camera.LayoutOf(cam), camera.ResolutionOf(cam)
		if profile != step.profile || layers != step.layers || layout != step.layout ||
			resolution != step.resolution {
			t.Errorf("%s the camera is %v in %d layers %s at %v, want %v in %d %s at %v",
				step.when, profile, layers, laid[layout], resolution, step.profile, step.layers,
				laid[step.layout], step.resolution)
		}
	}
}

func TestReporter_SaysTheLeadCamerasProfile(t *testing.T) {
	p, cam, give := ridden(t, cameras.Knobs{Strategy: cameras.Uniform, Resolution: 0.7},
		cameras.Knobs{Layers: true})
	if got := report(p); got != "Picture: tactical\n0.70 of the pixels\nthe figures sharp\n"+
		"following a budget of 60 FPS" {
		t.Errorf("over the entity the telemetry says %q, want the tactical profile's", got)
	}
	give(cameras.Ride{Camera: cam})
	give(cameras.Ride{Camera: cam})
	if got := report(p); got != "Picture: riding\n0.60/0.80/1.00 of the pixels\nin layers at 60% "+
		"and 35%\nstill" {
		t.Errorf("riding in it the telemetry says %q, want the riding profile's", got)
	}
}

// a lever of the frame budget: a dial of 0 to 3, following or not
type dial struct {
	at        int
	following bool
	turns     []string
}

func (d *dial) Following() bool { return d.following }

func (d *dial) Coarser() bool {
	d.turns = append(d.turns, "coarser")
	if d.at == 0 {
		return false
	}
	d.at--
	return true
}

func (d *dial) Finer() bool {
	d.turns = append(d.turns, "finer")
	if d.at == 3 {
		return false
	}
	d.at++
	return true
}

func TestLevers_GoCoarserBeforeThePicturesAndFinerAfterThemWhileFollowing(t *testing.T) {
	d := &dial{at: 1, following: true}
	p := cameras.NewPlugin(newWorld(), cameras.Config{Levers: []cameras.Lever{d},
		Tactical: cameras.Knobs{Strategy: cameras.Uniform}})
	cam := p.New(cameras.FromAbove, camera.Config{})
	p.Redraw(cameras.ResolutionStep)
	if d.at != 0 || camera.ResolutionOf(cam) != 1 {
		t.Fatalf("coarser: the dial at %d, the picture at %v; want the dial turned first, the "+
			"picture at every pixel still", d.at, camera.ResolutionOf(cam))
	}
	p.Redraw(cameras.ResolutionStep)
	if d.at != 0 || camera.ResolutionOf(cam) != cameras.ResolutionStep {
		t.Fatalf("coarser with the dial at its end: the dial at %d, the picture at %v; want the "+
			"picture stepped", d.at, camera.ResolutionOf(cam))
	}
	p.Redraw(1 / cameras.ResolutionStep)
	if d.at != 0 || camera.ResolutionOf(cam) != 1 {
		t.Fatalf("finer: the dial at %d, the picture at %v; want the picture first, back at every "+
			"pixel", d.at, camera.ResolutionOf(cam))
	}
	p.Redraw(1 / cameras.ResolutionStep)
	if d.at != 1 {
		t.Errorf("finer with the picture at every pixel: the dial at %d, want it turned up", d.at)
	}
	d.following, d.turns = false, nil
	p.Redraw(cameras.ResolutionStep)
	if len(d.turns) != 0 || camera.ResolutionOf(cam) != cameras.ResolutionStep {
		t.Errorf("not following, the dial was turned %v and the picture is at %v; want it left "+
			"alone, the picture stepped", d.turns, camera.ResolutionOf(cam))
	}
}
