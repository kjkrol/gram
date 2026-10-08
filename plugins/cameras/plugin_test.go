package cameras_test

import (
	"bytes"
	"encoding/gob"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/world"
)

func newWorld() *world.Plugin {
	return world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
}

// Every camera the plugin made is saved, in the order made, each with its window, zoom and
// fastening, and restored by a load — those made before it and those made after alike.
func TestCameras_AreSavedAndRestoredWithTheGame(t *testing.T) {
	newCamera := func(p *cameras.Plugin, w, h float32) camera.Camera {
		cam := p.New(cameras.TopDown(), camera.Config{})
		cam.SetViewport(w, h)
		return cam
	}
	saved := cameras.NewPlugin(newWorld())
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

	loaded := cameras.NewPlugin(newWorld())
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

// Each camera is its own: two of one plugin, made for two players, keep a maker and a config each.
func TestNew_EveryCameraHasAConfigOfItsOwn(t *testing.T) {
	p := cameras.NewPlugin(newWorld())
	looking := func(width, height uint32, edges aabbworld.Edges, cfg camera.Config) camera.Camera {
		return &looker{Camera: cameras.TopDown()(width, height, edges, cfg), on: cfg.MouseLook}
	}
	red := p.New(cameras.TopDown(), camera.Config{Zoom: 2})
	blue := p.New(looking, camera.Config{MouseLook: true})
	if red.Zoom() != 2 || blue.Zoom() != 1 {
		t.Errorf("red's camera at zoom %v, blue's at %v; want each its own: 2 and 1", red.Zoom(), blue.Zoom())
	}
	if camera.MouseLooks(red) || !camera.MouseLooks(blue) {
		t.Errorf("looking round with the mouse: red %v, blue %v; want blue's alone", camera.MouseLooks(red), camera.MouseLooks(blue))
	}
}
