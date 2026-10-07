package cameras_test

import (
	"bytes"
	"encoding/gob"
	"testing"

	"github.com/kjkrol/goke/v3"
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

// Every camera the plugin made is saved, in the order made, and restored by a load.
func TestCameras_AreSavedAndRestoredWithTheGame(t *testing.T) {
	newCameras := func() (*cameras.Plugin, camera.Camera) {
		p := cameras.NewPlugin(newWorld(), cameras.TopDown(), camera.Config{ViewportWidth: 200, ViewportHeight: 200})
		return p, p.New()
	}
	saved, second := newCameras()
	second.Translate(120, 80)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	for _, v := range saved.Serializable().Persisted() {
		if err := enc.Encode(v); err != nil {
			t.Fatal(err)
		}
	}
	loaded, again := newCameras()
	dec := gob.NewDecoder(&buf)
	for _, v := range loaded.Serializable().Persisted() {
		if err := dec.Decode(v); err != nil {
			t.Fatal(err)
		}
	}
	loaded.Restore()
	if got, want := again.Bounds(), second.Bounds(); got != want {
		t.Errorf("the second camera after a load shows %v, want %v", got, want)
	}
	if got := len(loaded.Cameras()); got != 2 || loaded.Main() != loaded.Cameras()[0] {
		t.Errorf("%d cameras, the main one first: want 2", got)
	}
}

// screened is an installer that knows the window's size.
type screened struct{ installCtx }

func (screened) Screen() (int, int) { return 640, 480 }

// Cameras configured with no viewport see the window as the engine says it is, those made before
// Install and after it alike; one with a viewport of its own keeps it.
func TestInstall_SizesTheCamerasToTheScreen(t *testing.T) {
	p := cameras.NewPlugin(newWorld(), cameras.TopDown(), camera.Config{})
	if err := p.Install(&screened{installCtx{ecs: goke.New()}}); err != nil {
		t.Fatal(err)
	}
	for name, cam := range map[string]camera.Camera{"the main": p.Main(), "a later": p.New()} {
		if w, h := cam.Viewport(); w != 640 || h != 480 {
			t.Errorf("%s camera is %v x %v, want the screen's 640 x 480", name, w, h)
		}
	}
	own := cameras.NewPlugin(newWorld(), cameras.TopDown(), camera.Config{ViewportWidth: 200, ViewportHeight: 100})
	if err := own.Install(&screened{installCtx{ecs: goke.New()}}); err != nil {
		t.Fatal(err)
	}
	if w, h := own.Main().Viewport(); w != 200 || h != 100 {
		t.Errorf("a camera configured 200 x 100 is %v x %v after Install", w, h)
	}
}
