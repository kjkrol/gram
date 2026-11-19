package world_test

import (
	"testing"

	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/world"
)

// the world imports no cameras: its lever is checked against their contract here
var _ cameras.Lever = world.BodiesLever{}

func TestBodies_TheirReachIsSavedWithTheGame(t *testing.T) {
	// the settings w saved, loaded into a world new
	loaded := func(w *world.Plugin) *world.Plugin {
		blob := *w.Serializable().Persisted()[0].(*[]byte)
		l := world.NewPlugin(testWorldConfig())
		*l.Serializable().Persisted()[0].(*[]byte) = blob
		l.Restore()
		return l
	}
	saved := world.NewPlugin(testWorldConfig())
	saved.Write().SetBodies(lod.L2)
	saved.Write().AdaptBodies(false)
	if l := loaded(saved); l.Read(0).BodiesReach() != lod.L2 || !l.Res.Settings.StillBodies {
		t.Errorf("loaded, the bodies reach L%d, still %v; want L2 and still, as saved",
			l.Read(0).BodiesReach(), l.Res.Settings.StillBodies)
	}
	if l := loaded(world.NewPlugin(testWorldConfig())); l.Read(0).BodiesReach() != lod.L3 ||
		l.Res.Settings.StillBodies {
		t.Errorf("loaded of a save saying nothing, the bodies reach L%d, still %v; want L3, "+
			"following the frame budget", l.Read(0).BodiesReach(), l.Res.Settings.StillBodies)
	}
}
