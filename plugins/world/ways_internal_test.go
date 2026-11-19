package world

import (
	"image/color"
	"reflect"
	"testing"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/render"
)

func TestWays_TheSettingsSayBeforeWhichCamerasWhatGivesWayDoes(t *testing.T) {
	r := sectorWorld(t, twoLevels)
	above := cameratest.NewFromSpace(4096, 512, 0)
	loose, behind := fastened{above, camera.Loose}, fastened{above, camera.Behind}
	inside := fastened{above, camera.Inside}
	says := func(when string, tactical, riding bool) {
		t.Helper()
		got := [3]bool{r.p.read.GivesWay(loose), r.p.read.GivesWay(behind),
			r.p.read.GivesWay(inside)}
		if got != [3]bool{tactical, riding, riding} {
			t.Errorf("%s what gives way does before a camera loose %v, behind a unit %v, inside "+
				"one %v; want %v, %v and %v", when, got[0], got[1], got[2], tactical, riding,
				riding)
		}
	}
	says("as the settings start", true, false)
	put := func(c command.Command) {
		t.Helper()
		if !r.p.module.commands.Put(command.Nobody, c) {
			t.Fatalf("no queue takes %T", c)
		}
		r.run()
	}
	put(GiveWayRiding{On: true})
	says("after GiveWayRiding on", true, true)
	put(GiveWay{On: false})
	says("after GiveWay off", false, true)
	// saved with the game, whole
	r.p.Serializable().Persisted()
	put(GiveWay{On: true})
	put(GiveWayRiding{On: false})
	says("set back", true, false)
	r.p.Restore()
	says("restored", false, true)
}

func TestWays_TheirSwitchesAreUnderGameWhereALookGivesWayToOneUncoveredAndNoKeys(t *testing.T) {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 100, Height: 100},
		Entities: EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	switches := func() (found []control.Switch) {
		for _, s := range p.Settings() {
			if sw, ok := s.(control.Switch); ok && sw.Section == control.Game {
				found = append(found, sw)
			}
		}
		return found
	}
	red := render.Solid(color.RGBA{R: 255, A: 255})
	cover, walker := p.Kinds().NewSprite(), p.Kinds().NewSprite()
	a := p.NewAtlas()
	a.Add(cover, 8, red).GivesWay()
	a.Add(walker, 8, red).Uncovered()
	if got := switches(); len(got) != 0 {
		t.Errorf("%d switches under Game before the looks are declared, want none", len(got))
	}
	a.Close()
	got := switches()
	if len(got) != 2 {
		t.Fatalf("%d switches under Game, want the tactical view's and the riding's", len(got))
	}
	if !got[0].Get() || got[0].Set(false) != (GiveWay{On: false}) {
		t.Errorf("the tactical view's switch %+v, want it on, setting GiveWay", got[0])
	}
	if got[1].Get() || got[1].Set(true) != (GiveWayRiding{On: true}) {
		t.Errorf("the riding's switch %+v, want it off, setting GiveWayRiding", got[1])
	}
	for _, k := range p.DefaultBindings() {
		if k.Type() == reflect.TypeFor[GiveWay]() || k.Type() == reflect.TypeFor[GiveWayRiding]() {
			t.Errorf("%v is bound to %v, want the settings screen alone", k.Type(), k.Trigger)
		}
	}
}
