package players_test

import (
	"image"
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
)

func TestViewports_OneLocalPlayerSeesTheWholeScreen(t *testing.T) {
	r := newRig(t)
	screen := image.Rect(0, 0, 800, 600)
	vps := r.p.Viewports(screen)
	if len(vps) != 1 || vps[0].Area != screen || vps[0].Camera != r.w.Camera() {
		t.Errorf("viewports %+v, want the world's camera over the whole screen", vps)
	}
}

func TestViewports_NoLocalPlayerFallsBackToTheWorldsCamera(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	p := players.NewPlugin(w)
	p.Add("ai")
	screen := image.Rect(0, 0, 800, 600)
	if vps := p.Viewports(screen); len(vps) != 1 || vps[0].Camera != w.Camera() || vps[0].Area != screen {
		t.Errorf("viewports %+v, want the world's camera over the whole screen", vps)
	}
}

func TestViewports_PlayersWithTheirOwnCamerasShareTheScreenInColumns(t *testing.T) {
	r := newRig(t)
	second := r.p.Local("second")
	second.Camera = camera.NewFromSpace(1000, 1000, 0)
	third := r.p.Local("third") // looks through the world's camera, as the first does

	vps := r.p.Viewports(image.Rect(0, 0, 801, 600))
	if len(vps) != 2 {
		t.Fatalf("%d viewports, want one per camera: 2", len(vps))
	}
	if vps[0].Camera != third.Camera || vps[0].Area != image.Rect(0, 0, 400, 600) {
		t.Errorf("first viewport %v through %p, want the left half through the world's camera", vps[0].Area, vps[0].Camera)
	}
	if vps[1].Camera != second.Camera || vps[1].Area != image.Rect(400, 0, 801, 600) {
		t.Errorf("second viewport %v, want the rest of the screen through the second player's camera", vps[1].Area)
	}
}
