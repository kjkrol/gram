package engine

import (
	"reflect"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/render"
)

// ecsHost owns one *goke.ECS: it queues install work until a single ecs.Setup call
// and keeps the tracked values Save and Load need. One is built per Stage entered.
type ecsHost struct {
	ecs       *goke.ECS
	resources *storage

	tracked      []any
	loaded       map[string][]byte // the groups of the last Load, encoded, for values tracked after it
	pendingSetup []func() []goke.System
	names        map[string]bool
	layers       map[render.Layer]bool // registered, when comparable
}

func newECSHost() *ecsHost {
	return &ecsHost{ecs: goke.New(), resources: newStorage(), layers: map[render.Layer]bool{}}
}

// track records v among the values the host later loads, restores, populates and saves.
func (h *ecsHost) track(v any) { h.tracked = append(h.tracked, v) }

// loadLate gives v, tracked after a Load, the state the save holds for it, and restores it: a
// Stage's scenes, made once the world is loaded.
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

// addPendingSetup queues producer to run once, during flushPendingSetup.
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

// registerLayer has l initialised at Setup, once however many scenes list it.
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

// providedComps collects LoadComps from every tracked goke.CompProvider, each type once.
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

// postLoadSystems collects PostLoad from every tracked value implementing PostLoader.
func (h *ecsHost) postLoadSystems() []goke.System {
	var systems []goke.System
	for _, v := range h.tracked {
		if pl, ok := v.(plugin.PostLoader); ok {
			systems = append(systems, pl.PostLoad())
		}
	}
	return systems
}

// runRestore calls Restore on every tracked Restorer, right after a Load.
func (h *ecsHost) runRestore() {
	for _, v := range h.tracked {
		if r, ok := v.(plugin.Restorer); ok {
			r.Restore()
		}
	}
}

// runPopulate calls Populate on every tracked Populator, stopping at the first error.
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

// saveTargets collects Persisted from every tracked Serializable, keyed by Go type name — and by
// its own name too where it has one, so several of a type (a stage's ui scenes) keep theirs apart.
func (h *ecsHost) saveTargets() map[string][]any {
	out := make(map[string][]any)
	for _, v := range h.tracked {
		if s, ok := v.(plugin.Serializable); ok {
			out[trackKey(v)] = s.Persisted()
		}
	}
	return out
}

// trackKey is the name a tracked value is saved under.
func trackKey(v any) string {
	key := reflect.TypeOf(v).String()
	if n, ok := v.(interface{ Name() string }); ok {
		key += " " + n.Name()
	}
	return key
}

// persistGroups combines tracked and plugin Serializables with extra into one name-keyed map.
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

// flushPendingSetup evaluates every deferred producer once and runs a single ecs.Setup.
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
