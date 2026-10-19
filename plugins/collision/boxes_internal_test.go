package collision

import (
	"testing"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

func TestBoxes_TheCommandSwitchesFromTheBasesToTheSolidsToNone(t *testing.T) {
	flat := testWorldPlugin()
	raised := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
		Heights:  true,
	})
	for _, tc := range []struct {
		w    *world.Plugin
		want []BoxesShown
	}{
		{raised, []BoxesShown{Bases, Solids, NoBoxes, Bases}},
		{flat, []BoxesShown{Bases, NoBoxes, Bases}},
	} {
		p := newTestPlugin(tc.w)
		if err := tc.w.Carry(p); err != nil {
			t.Fatal(err)
		}
		if p.BoxesShown() != NoBoxes {
			t.Fatal("the boxes are shown before anyone asked")
		}
		key := p.DefaultBindings()[0]
		for i, want := range tc.want {
			cmd, _ := key.Build(control.Context{})
			tc.w.Carrier().Put(command.Nobody, cmd)
			p.switchBoxes()
			if p.BoxesShown() != want {
				t.Errorf("heights %v, Boxes given %d times: %v shown, want %v",
					tc.w.HasHeights(), i+1, p.BoxesShown(), want)
			}
		}
	}
	p := newTestPlugin(testWorldPlugin())
	var keys []string
	for _, b := range p.DefaultBindings() {
		if b.Type().Name() == "Boxes" {
			keys = append(keys, b.Trigger.(control.KeyPress).Key.String())
		}
	}
	if len(keys) != 1 || keys[0] != "X" {
		t.Errorf("Boxes bound to %v, want Shift+X alone", keys)
	}
}

func TestBoxes_TheSettingChoosesAndIsSaved(t *testing.T) {
	w := testWorldPlugin()
	p := newTestPlugin(w)
	if err := w.Carry(p); err != nil {
		t.Fatal(err)
	}
	choice := p.Settings()[0].(control.Choice)
	if len(choice.Options) != 2 || choice.Get() != 0 {
		t.Fatalf("flat, the choice is %v at %d, want none and bases at none", choice.Options,
			choice.Get())
	}
	w.Carrier().Put(command.Nobody, choice.Set(1))
	p.switchBoxes()
	if p.BoxesShown() != Bases || choice.Get() != 1 {
		t.Fatalf("chose the bases: %v shown, the choice at %d", p.BoxesShown(), choice.Get())
	}
	saved := p.Serializable().Persisted()
	q := newTestPlugin(testWorldPlugin())
	loading := q.Serializable().Persisted()
	*loading[0].(*[]byte) = append([]byte(nil), *saved[0].(*[]byte)...)
	q.Restore()
	if q.BoxesShown() != Bases {
		t.Errorf("loaded: %v shown, want the bases saved", q.BoxesShown())
	}
}

type drawsCtx struct {
	plugin.Installer
	drawn []render.Source
}

func (*drawsCtx) UseModule(goke.Module)      {}
func (*drawsCtx) Hosts(...host.Host)         {}
func (*drawsCtx) ECS() *goke.ECS             { return goke.New() }
func (c *drawsCtx) Draws(s ...render.Source) { c.drawn = append(c.drawn, s...) }

func TestPlugin_DrawsTheBoxes(t *testing.T) {
	p := newTestPlugin(testWorldPlugin())
	ctx := &drawsCtx{}
	if err := p.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if len(ctx.drawn) != 1 || ctx.drawn[0] != render.Source(p.boxesView) {
		t.Errorf("collision draws %v, want the boxes", ctx.drawn)
	}
}
