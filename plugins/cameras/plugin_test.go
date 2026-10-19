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
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
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

func carried(t *testing.T, from, to *cameras.Plugin) {
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
	p := cameras.NewPlugin(newWorld(), cameras.Config{Resolution: 0.7})
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
	least := cameras.NewPlugin(newWorld(), cameras.Config{Resolution: 0.1})
	lowest := least.New(cameras.FromAbove, camera.Config{})
	if got := camera.ResolutionOf(lowest); got != cameras.LeastResolution {
		t.Errorf("a Resolution of 0.1 draws at %v, want the least, %v", got, cameras.LeastResolution)
	}
}

func TestSettings_AreSavedWithTheCamerasAndAZeroSavedComesBackSo(t *testing.T) {
	saved := cameras.NewPlugin(newWorld(), cameras.Config{})
	saved.ShowZones(true)
	loaded := cameras.NewPlugin(newWorld(), cameras.Config{Strategy: cameras.Uniform})
	carried(t, saved, loaded)
	if !loaded.ZonesShown() {
		t.Error("the zones shown were not saved with the cameras")
	}
	if loaded.Strategy() != cameras.Still {
		t.Error("the resolution follows the budget after a load that saved it still")
	}
}

var defaultRings = []camera.Ring{
	{Share: 1, Resolution: 0.6},
	{Share: 0.6, Resolution: 0.8},
	{Share: 0.35, Resolution: 1},
}

func TestNew_UnderRingsACameraIsDrawnInTheDefaultRingsAMinimapsWhole(t *testing.T) {
	p := cameras.NewPlugin(newWorld(), cameras.Config{Rings: true})
	cam := p.New(cameras.FromAbove, camera.Config{})
	whole := p.New(cameras.FromAbove, camera.Config{Whole: true})
	if got := camera.RingsOf(cam); !slices.Equal(got, defaultRings) ||
		camera.ResolutionOf(cam) != 1 {
		t.Errorf("the camera is drawn in %v at %v, want the default %v at all its centre's pixels",
			got, camera.ResolutionOf(cam), defaultRings)
	}
	if got := camera.RingsOf(whole); len(got) != 0 {
		t.Errorf("a camera keeping the whole world in view is drawn in %v, want whole", got)
	}
	relief := cameras.NewPlugin(newWorld(),
		cameras.Config{Turn: true, Tilt: true, Cell: 32, Rings: true})
	if got := camera.RingsOf(relief.New(cameras.Isometrically, camera.Config{})); !slices.Equal(
		got, defaultRings) {
		t.Errorf("a camera in relief is drawn in %v, want the default", got)
	}
	centre := cameras.NewPlugin(newWorld(), cameras.Config{Rings: true, Resolution: 0.7})
	cam = centre.New(cameras.FromAbove, camera.Config{})
	if got := camera.RingsOf(cam); got[2].Resolution != 0.7 || camera.ResolutionOf(cam) != 0.7 {
		t.Errorf("at a Resolution of 0.7 the camera is drawn in %v at %v, want its centre ring "+
			"at 0.7", got, camera.ResolutionOf(cam))
	}
}

func TestSettings_AreTheZonesAndThePicturesInTheirSectionsEachBuildingItsCommand(t *testing.T) {
	p := cameras.NewPlugin(newWorld(), cameras.Config{Strategy: cameras.Uniform})
	settings := p.Settings()
	if len(settings) != 10 {
		t.Fatalf("%d settings, want the zones and the picture's nine", len(settings))
	}
	for i, want := range []struct{ label, section string }{
		{"Scroll zones", ""},
		{"Resolution follows the frame budget", control.PictureResolution},
		{"Resolution", control.PictureResolution},
		{"Frame budget", control.PictureAdaptive},
		{"Drawn in rings", control.PictureRings},
		{"Inner ring", control.PictureRings},
		{"Middle ring", control.PictureRings},
		{"Outer ring resolution", control.PictureRings},
		{"Middle ring resolution", control.PictureRings},
		{"Show the rings", control.PictureAreas},
	} {
		var label string
		switch st := settings[i].(type) {
		case control.Switch:
			label = st.Label
		case control.Choice:
			label = st.Label
		case control.Range:
			label = st.Label
		}
		if label != want.label || control.SectionOf(settings[i]) != want.section {
			t.Errorf("setting %d is %q in %q, want %q in %q", i, label,
				control.SectionOf(settings[i]), want.label, want.section)
		}
	}
	strategy := settings[1].(control.Choice)
	if strategy.Get() != int(cameras.Uniform) || len(strategy.Options) != 3 {
		t.Errorf("the strategy shows %d of %v, want the whole picture's of three",
			strategy.Get(), strategy.Options)
	}
	if got := strategy.Set(2); got != (cameras.Adapt{Strategy: cameras.InRings}) {
		t.Errorf("the strategy's third choice builds %v, want Adapt{InRings}", got)
	}
	resolution := settings[2].(control.Range)
	if resolution.Min != cameras.LeastResolution || resolution.Max != 1 ||
		resolution.Get() != 1 || resolution.Shows(0.85) != "0.85" {
		t.Errorf("the resolution runs %v to %v at %v shown %q, want 0.4 to 1 at 1, 0.85",
			resolution.Min, resolution.Max, resolution.Get(), resolution.Shows(0.85))
	}
	if got := resolution.Set(0.7); got != (cameras.Picture{Resolution: 0.7}) {
		t.Errorf("the resolution at 0.7 builds %v, want Picture{0.7}", got)
	}
	budget := settings[3].(control.Range)
	if budget.Min != 30 || budget.Max != 120 || budget.Get() != 60 || budget.Shows(60) != "60 FPS" {
		t.Errorf("the budget runs %v to %v at %v shown %q, want 30 to 120 at 60, \"60 FPS\"",
			budget.Min, budget.Max, budget.Get(), budget.Shows(60))
	}
	if got := budget.Set(30); got != (cameras.Budget{Frame: time.Second / 30}) {
		t.Errorf("the budget at 30 builds %v, want a frame of a thirtieth of a second", got)
	}
	rings := settings[4].(control.Switch)
	if rings.Get() || rings.Set(true) != (cameras.Rings{On: true}) {
		t.Errorf("the rings are %v building %v, want off, Rings{On: true}", rings.Get(),
			rings.Set(true))
	}
	inner, middle := settings[5].(control.Range), settings[6].(control.Range)
	if inner.Get() != 35 || middle.Get() != 60 || inner.Shows(35) != "35%" {
		t.Errorf("the shares show %v and %v (%q), want 35%% and 60%%", inner.Get(), middle.Get(),
			inner.Shows(35))
	}
	if got := inner.Set(55); got != (cameras.RingShares{Inner: 0.55, Middle: 0.65}) {
		t.Errorf("the inner ring at 55%% builds %v, want the middle pushed to 65%%", got)
	}
	if got := middle.Set(40); got != (cameras.RingShares{Inner: 0.3, Middle: 0.4}) {
		t.Errorf("the middle ring at 40%% builds %v, want the inner pushed to 30%%", got)
	}
	outer, middleRes := settings[7].(control.Range), settings[8].(control.Range)
	if float32(outer.Get()) != 0.6 || float32(middleRes.Get()) != 0.8 {
		t.Errorf("the rings' resolutions show %v and %v, want 0.6 and 0.8", outer.Get(),
			middleRes.Get())
	}
	if got := outer.Set(0.5); got != (cameras.RingResolutions{Outer: 0.5, Middle: 0.8}) {
		t.Errorf("the outer ring at 0.5 builds %v, want RingResolutions{0.5, 0.8}", got)
	}
	if got := middleRes.Set(0.9); got != (cameras.RingResolutions{Outer: 0.6, Middle: 0.9}) {
		t.Errorf("the middle ring at 0.9 builds %v, want RingResolutions{0.6, 0.9}", got)
	}
	shown := settings[9].(control.Switch)
	if shown.Get() || shown.Set(true) != (cameras.ShowRings{Shown: true}) {
		t.Errorf("the rings shown are %v building %v, want hidden, ShowRings{Shown: true}",
			shown.Get(), shown.Set(true))
	}
}

func TestRestore_DrawsTheCamerasInRingsAsTheSettingsLoadedSay(t *testing.T) {
	saved := cameras.NewPlugin(newWorld(), cameras.Config{Rings: true})
	saved.SetRingShares(0.3, 0.5)
	saved.SetRingResolutions(0.5, 0.7)
	loaded := cameras.NewPlugin(newWorld(), cameras.Config{})
	before := loaded.New(cameras.FromAbove, camera.Config{})
	before.(camera.Resolved).SetResolution(0.7)
	carried(t, saved, loaded)
	want := []camera.Ring{
		{Share: 1, Resolution: 0.5},
		{Share: 0.5, Resolution: 0.7},
		{Share: 0.3, Resolution: 1},
	}
	if got := camera.RingsOf(before); !slices.Equal(got, want) ||
		camera.ResolutionOf(before) != 1 {
		t.Errorf("after the load the camera made before it is drawn in %v at %v, want the rings "+
			"saved at all its centre's pixels", got, camera.ResolutionOf(before))
	}
	after := loaded.New(cameras.FromAbove, camera.Config{})
	if got := camera.RingsOf(after); !slices.Equal(got, want) {
		t.Errorf("a camera made after the load is drawn in %v, want the rings saved", got)
	}
	whole := cameras.NewPlugin(newWorld(), cameras.Config{})
	ringed := cameras.NewPlugin(newWorld(), cameras.Config{Rings: true})
	cam := ringed.New(cameras.FromAbove, camera.Config{})
	carried(t, whole, ringed)
	if got := camera.RingsOf(cam); len(got) != 0 || ringed.Rings() {
		t.Errorf("after a load saved whole the camera is drawn in %v, want whole", got)
	}
}
