package engine

import (
	"reflect"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/render"
)

type ecsHost struct {
	ecs       *goke.ECS
	resources *storage

	tracked      []any
	loaded       map[string][]byte // the last Load's groups, for values tracked after it
	pendingSetup []func() []goke.System
	names        map[string]bool
	layers       map[render.Layer]bool // registered, when comparable
}

func newECSHost() *ecsHost {
	return &ecsHost{ecs: goke.New(), resources: newStorage(), layers: map[render.Layer]bool{}}
}

func (h *ecsHost) track(v any) { h.tracked = append(h.tracked, v) }

func (h *ecsHost) loadLate(v any) error {
	s, ok := v.(plugin.Serializable)
	if !ok || h.loaded == nil {
		return nil
	}
	if err := decodeGroup(h.loaded, trackKey(v), s.Persisted()); err != nil {
		return err
	}
	if r, ok := v.(plugin.Restorer); ok {
		r.Restore()
	}
	return nil
}

func (h *ecsHost) addPendingSetup(producer func() []goke.System) {
	h.pendingSetup = append(h.pendingSetup, producer)
}

func (h *ecsHost) useModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(si *goke.SysInit) { m.RegSystems(h.ecs) }}
	h.track(m)
	h.addPendingSetup(func() []goke.System { return append(m.SetupSystems(), regSys) })
}

func (h *ecsHost) setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		h.track(p)
		h.addPendingSetup(p.SetupSystems)
	}
}

func (h *ecsHost) regSys(factory func() goke.System) goke.Runnable {
	return h.ecs.RegSys(factory())
}

func (h *ecsHost) registerLayer(l render.Layer) {
	if reflect.TypeOf(l).Comparable() {
		if h.layers[l] {
			return
		}
		h.layers[l] = true
	}
	sys := goke.SystemFn{OnInit: func(si *goke.SysInit) { l.Init(si) }}
	h.addPendingSetup(func() []goke.System { return []goke.System{sys} })
}

func (h *ecsHost) providedComps() []goke.CompToken {
	all := goke.ProvidedComps(h.tracked...)
	listed := make(map[string]bool, len(all))
	tokens := all[:0]
	for _, token := range all {
		if !listed[token.Name] {
			listed[token.Name] = true
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func (h *ecsHost) postLoadSystems() []goke.System {
	var systems []goke.System
	for _, v := range h.tracked {
		if pl, ok := v.(plugin.PostLoader); ok {
			systems = append(systems, pl.PostLoad())
		}
	}
	return systems
}

func (h *ecsHost) runRestore() {
	for _, v := range h.tracked {
		if r, ok := v.(plugin.Restorer); ok {
			r.Restore()
		}
	}
}

func (h *ecsHost) runPopulate() error {
	for _, v := range h.tracked {
		if p, ok := v.(plugin.Populator); ok {
			if err := p.Populate(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *ecsHost) saveTargets() map[string][]any {
	out := make(map[string][]any)
	for _, v := range h.tracked {
		if s, ok := v.(plugin.Serializable); ok {
			out[trackKey(v)] = s.Persisted()
		}
	}
	return out
}

// by its name too, so several of a type (a stage's ui scenes) stay apart
func trackKey(v any) string {
	key := reflect.TypeOf(v).String()
	if n, ok := v.(interface{ Name() string }); ok {
		key += " " + n.Name()
	}
	return key
}

func (h *ecsHost) persistGroups(extra ...any) map[string][]any {
	groups := h.saveTargets()
	for name, targets := range h.resources.persisted() {
		groups[name] = targets
	}
	for _, r := range extra {
		groups[reflect.TypeOf(r).String()] = []any{r}
	}
	return groups
}

func (h *ecsHost) flushPendingSetup() {
	if len(h.pendingSetup) == 0 {
		return
	}
	var systems []goke.System
	for _, produce := range h.pendingSetup {
		systems = append(systems, produce()...)
	}
	h.ecs.Setup(systems...)
	h.pendingSetup = nil
}
