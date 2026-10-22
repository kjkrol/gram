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

func TestBoxes_TheCommandShowsTheBasesTheSolidsOrNone(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
		Heights:  true,
	})
	p := newTestPlugin(w)
	if err := w.Carry(p); err != nil {
		t.Fatal(err)
	}
	if p.read.BoxesShown() != NoBoxes {
		t.Fatal("the boxes are shown before anyone asked")
	}
	if options := p.Settings()[0].(control.Choice).Options; len(options) != 3 {
		t.Errorf("with heights the choice is %v, want none, bases and solids", options)
	}
	for _, want := range []BoxesShown{Bases, Solids, NoBoxes} {
		w.Write().Carrier().Put(command.Nobody, Boxes{Shown: want})
		p.switchBoxes()
		if p.read.BoxesShown() != want {
			t.Errorf("Boxes of %v: %v shown", want, p.read.BoxesShown())
		}
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
	w.Write().Carrier().Put(command.Nobody, choice.Set(1))
	p.switchBoxes()
	if p.read.BoxesShown() != Bases || choice.Get() != 1 {
		t.Fatalf("chose the bases: %v shown, the choice at %d", p.read.BoxesShown(), choice.Get())
	}
	saved := p.Serializable().Persisted()
	q := newTestPlugin(testWorldPlugin())
	loading := q.Serializable().Persisted()
	*loading[0].(*[]byte) = append([]byte(nil), *saved[0].(*[]byte)...)
	q.Restore()
	if q.read.BoxesShown() != Bases {
		t.Errorf("loaded: %v shown, want the bases saved", q.read.BoxesShown())
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
