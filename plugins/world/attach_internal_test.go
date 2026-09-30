package world

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/uid"
)

// attachInstaller is the least of a plugin.Installer that world needs.
type attachInstaller struct {
	ecs     *goke.ECS
	pending []func() []goke.System
}

func (c *attachInstaller) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}

func (c *attachInstaller) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *attachInstaller) RegSys(f func() goke.System) goke.Runnable { return c.ecs.RegSys(f()) }
func (c *attachInstaller) ECS() *goke.ECS                            { return c.ecs }

// attachWorld is an installed world holding one entity that carries a
// spawnerStat of 3 HP, and a way to run one edit against it per tick.
type attachWorld struct {
	p      *Plugin
	ecs    *goke.ECS
	id     uid.UID64
	edit   func(cb *goke.CmdBuf)
	stats  goke.Comp[spawnerStat]
	statQ  *goke.Query
	tagged *goke.Query
}

func newAttachWorld(t *testing.T) *attachWorld {
	t.Helper()
	w := &attachWorld{p: testPlugin(), ecs: goke.New()}
	ctx := &attachInstaller{ecs: w.ecs}
	if err := w.p.Install(ctx); err != nil {
		t.Fatalf("Install: %v", err)
	}
	stat := kind.Define[int](w.p.Kinds(), "stat", statSpec())
	w.p.Seed(stat.Entry(3))
	if err := w.p.Populate(); err != nil {
		t.Fatalf("Populate: %v", err)
	}

	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		w.statQ = si.NewQueryBuilder(&w.stats).Build()
		w.tagged = si.NewQueryBuilder().Include(goke.Include[spawnerTag]()).Build()
		w.statQ.All()
		w.statQ.Next()
		w.id = w.statQ.Cursor().IDs[0]
	}})
	w.ecs.Setup(systems...)

	editor := w.ecs.RegSys(goke.SystemFn{OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
		if w.edit != nil {
			w.edit(cb)
			w.edit = nil
		}
	}})
	w.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		rc.Run(editor, d)
		rc.Sync()
	})
	return w
}

func (w *attachWorld) apply(edit func(cb *goke.CmdBuf)) {
	w.edit = edit
	w.ecs.Tick(time.Millisecond)
}

func (w *attachWorld) hp() (hp []int) {
	for w.statQ.All(); w.statQ.Next(); {
		for _, s := range w.stats.Slice(w.statQ.Cursor()) {
			hp = append(hp, s.HP)
		}
	}
	return hp
}

func (w *attachWorld) tagCount() (n int) {
	for w.tagged.All(); w.tagged.Next(); {
		n += len(w.tagged.Cursor().IDs)
	}
	return n
}

func TestAttach_ReplacesTheValueAnEntityAlreadyCarries(t *testing.T) {
	w := newAttachWorld(t)

	w.apply(func(cb *goke.CmdBuf) { w.p.Attach(cb, w.id, spawnerStat{HP: 9}) })

	if got := w.hp(); len(got) != 1 || got[0] != 9 {
		t.Errorf("HP after Attach = %v, want the one entity at 9", got)
	}
}

func TestAttachAndDetach_GiveAndTakeATag(t *testing.T) {
	w := newAttachWorld(t)

	w.apply(func(cb *goke.CmdBuf) { w.p.Attach(cb, w.id, spawnerTag{}) })
	if got := w.tagCount(); got != 1 {
		t.Fatalf("%d entities carry the tag after Attach, want 1", got)
	}
	if got := w.hp(); len(got) != 1 || got[0] != 3 {
		t.Errorf("HP after attaching a tag = %v, want the entity's 3 untouched", got)
	}

	w.apply(func(cb *goke.CmdBuf) { w.p.Detach[spawnerTag](cb, w.id) })
	if got := w.tagCount(); got != 0 {
		t.Errorf("%d entities carry the tag after Detach, want 0", got)
	}

	w.apply(func(cb *goke.CmdBuf) { w.p.Detach[spawnerTag](cb, w.id) })
	if got := w.hp(); len(got) != 1 {
		t.Errorf("detaching what the entity does not carry left %d entities, want it alone and alive", len(got))
	}
}

func TestDetach_Base_IsRefused(t *testing.T) {
	w := newAttachWorld(t)
	defer func() {
		if recover() == nil {
			t.Error("Detach[Base] went through — every entity has to keep its Base")
		}
	}()

	w.apply(func(cb *goke.CmdBuf) { w.p.Detach[Base](cb, w.id) })
}

func TestAttach_BeforeInstall_Panics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Attach on a Plugin that was never installed went through")
		}
	}()

	testPlugin().Attach(nil, uid.UID64(0), spawnerTag{})
}

func TestDeclare_TellsSaveFilesAboutTheType(t *testing.T) {
	p := testPlugin()
	before := len(p.module.LoadComps())

	p.Declare[spawnerTag]()

	tokens := p.module.LoadComps()
	if len(tokens) != before+1 || tokens[before].Name != goke.LoadComp[spawnerTag]().Name {
		t.Errorf("LoadComps after Declare lists %d types, want %d with the declared one last", len(tokens), before+1)
	}
}
