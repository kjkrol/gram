package engine_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

// a source publishing what it draws, noting each call
type publishing struct {
	calls []string
	drawn render.Buffered[int]
}

func (*publishing) Compose(*render.Frame, camera.Camera) {}
func (p *publishing) Publish() {
	p.calls = append(p.calls, "publish")
	*p.drawn.Back() = len(p.calls)
}
func (p *publishing) Swap() {
	p.calls = append(p.calls, "swap")
	p.drawn.Swap()
}

func TestEngine_AFramePublishesThenSwapsOnceForEveryPublisher(t *testing.T) {
	src := &publishing{}
	var w *world.Plugin
	cam := cameratest.NewFromSpace(100, 100, 0)
	st := stage.New("meadow").
		World(testWorldConfig()).
		Plugins(func(made *world.Plugin) {
			w = made
			w.Made().Add(&drawing{src: src})
		}).
		Scenes(func() []game.Scene {
			return []game.Scene{ui.NewScene("main",
				ui.Image(render.NewFeed(cam, w.Read(0).Picture())))}
		}).
		Shows("main").
		Update(func(goke.RunCtx, time.Duration) {})
	eng := engine.NewEngine(oneStageGame{stage: st})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	for range 2 {
		if err := eng.Frame(); err != nil {
			t.Fatalf("Frame: %v", err)
		}
	}
	want := []string{"publish", "swap", "publish", "swap"}
	if len(src.calls) != len(want) {
		t.Fatalf("two frames called %v, want %v", src.calls, want)
	}
	for i := range want {
		if src.calls[i] != want[i] {
			t.Fatalf("two frames called %v, want %v", src.calls, want)
		}
	}
	if got := *src.drawn.Front(); got != 3 {
		t.Errorf("after two frames the source draws what it published %d calls in, want 3", got)
	}
}
