package world

import (
	"reflect"
	"testing"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
)

func TestFar_CommandsSetHowTheLooseUnitsAreDrawnOverTheConfig(t *testing.T) {
	r := sectorWorld(t, twoLevels)
	r.p.Res.Config.Far, r.p.Res.Config.Counts = Banners, true
	if got := r.p.read.FarLook(); got != Banners || !r.p.read.Counts() {
		t.Errorf("far %v, counts %v as the Config says; want banners, counts", got,
			r.p.read.Counts())
	}
	for _, c := range []struct {
		cmd    command.Command
		far    FarLook
		counts bool
	}{
		{ShowFar{Look: Points}, Points, true},
		{ShowCounts{On: false}, Points, false},
		{ShowFar{Look: AsConfig}, Banners, false},
		{ShowCounts{On: true}, Banners, true},
	} {
		r.p.module.commands.Put(command.Nobody, c.cmd)
		r.run()
		far, counts := r.p.read.FarLook(), r.p.read.Counts()
		if far != c.far || counts != c.counts {
			t.Errorf("after %+v: far %v, counts %v; want %v, %v", c.cmd, far, counts, c.far,
				c.counts)
		}
	}
	r.p.Res.Config.Far = AsConfig
	r.p.write.ShowFar(AsConfig)
	if got := r.p.read.FarLook(); got != Points {
		t.Errorf("far %v with neither the Settings nor the Config saying, want points", got)
	}
	cam := cameratest.NewFromSpace(4096, 512, 0)
	if got := r.p.read.Banners(cam, nil); len(got) != 0 {
		t.Errorf("banners %v published, want none yet", got)
	}
}

func TestFar_SettingsOnlyInAWorldCutIntoSectorsAndNoKeys(t *testing.T) {
	p := sectorWorld(t, twoLevels).p
	labels := map[string]control.Setting{}
	for _, s := range p.Settings() {
		switch s := s.(type) {
		case control.Choice:
			labels[s.Label] = s
		case control.Switch:
			labels[s.Label] = s
		}
	}
	if len(labels) != 5 {
		t.Errorf("settings %v, want the tempo, the sectors drawn, their levels, the far units "+
			"and the counts", labels)
	}
	far, ok := labels["Far units"].(control.Choice)
	if !ok || far.Section != control.Graphics || len(far.Options) != 2 || far.Get() != 0 ||
		far.Set(1) != (ShowFar{Look: Banners}) {
		t.Errorf("the far units' setting %+v, want a choice of two in Graphics, points, setting "+
			"ShowFar banners", far)
	}
	counts, ok := labels["Counts"].(control.Switch)
	if !ok || counts.Section != control.Graphics || counts.Get() ||
		counts.Set(true) != (ShowCounts{On: true}) {
		t.Errorf("the counts' setting %+v, want a switch in Graphics, off, setting ShowCounts on",
			counts)
	}
	for _, k := range p.DefaultBindings() {
		if k.Type() == reflect.TypeFor[ShowFar]() || k.Type() == reflect.TypeFor[ShowCounts]() {
			t.Errorf("%v is bound to %v, want the settings screen alone", k.Type(), k.Trigger)
		}
	}
	for _, s := range sectorWorldOf(t, 0, 0, lod.Ladder{}).p.Settings() {
		if c, ok := s.(control.Choice); ok && c.Label == "Far units" {
			t.Error("a world of no sectors has the far units' setting")
		}
	}
}
