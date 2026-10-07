package players_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
)

func TestViewports_OneLocalPlayerSeesTheWholeScreen(t *testing.T) {
	r := newRig(t)
	screen := geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(800, 600))
	vps := r.p.Viewports(screen)
	if len(vps) != 1 || vps[0].Area != screen || vps[0].Camera != r.cam {
		t.Errorf("viewports %+v, want the player's camera over the whole screen", vps)
	}
}

func TestViewports_NoneWithoutALocalPlayer(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	p := players.NewPlugin(w)
	p.Add("ai")
	screen := geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(800, 600))
	if vps := p.Viewports(screen); len(vps) != 0 {
		t.Errorf("viewports %+v, want none: nobody looks at this screen", vps)
	}
}

func TestViewports_PlayersWithTheirOwnCamerasShareTheScreenInColumns(t *testing.T) {
	r := newRig(t)
	second := r.p.Local("second", r.cams.New(cameras.TopDown(), camera.Config{}))
	third := r.p.Local("third", r.cam) // looks through the main camera, as the first does

	vps := r.p.Viewports(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(801, 600)))
	if len(vps) != 2 {
		t.Fatalf("%d viewports, want one per camera: 2", len(vps))
	}
	if vps[0].Camera != third.Camera || vps[0].Area != geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(400, 600)) {
		t.Errorf("first viewport %v through %p, want the left half through the main camera", vps[0].Area, vps[0].Camera)
	}
	if vps[1].Camera != second.Camera || vps[1].Area != geom.NewAABB(geom.NewVec(400, 0), geom.NewVec(801, 600)) {
		t.Errorf("second viewport %v, want the rest of the screen through the second player's camera", vps[1].Area)
	}
}
