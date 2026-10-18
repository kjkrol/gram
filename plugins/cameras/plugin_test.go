package cameras_test

import (
	"bytes"
	"encoding/gob"
	"log"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
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
	for _, v := range p.Serializable().Persisted() {
		if err := gob.NewDecoder(&save).Decode(v); err != nil {
			t.Fatal(err)
		}
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
