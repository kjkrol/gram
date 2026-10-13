package world

import (
	"fmt"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/cond"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// Self is a plugin's own entity in the world, called by the plugin's name, carrying its knobs,
// the roles it plays and the effects it is under. A plugin embeds it, and so is whom a command
// may be for.
type Self struct {
	name  string
	w     *Plugin
	knobs []comp.Comp
	roles tag.Tags[cond.Roles]
	id    uid.UID64
}

var _ rule.Router = (*Self)(nil)

// NewSelf is the entity of the plugin p in w, carrying knobs, and notes p as made over w. Call
// it in the plugin's constructor; a second entity of one name panics.
func NewSelf(w *Plugin, p plugin.Plugin, knobs ...comp.Comp) *Self {
	s := newSelf(w, p.Name(), knobs...)
	w.made.Add(p)
	return s
}

func newSelf(w *Plugin, name string, knobs ...comp.Comp) *Self {
	for _, other := range w.module.selves.list {
		if other.name == name {
			panic(fmt.Sprintf("world: the plugin %q has its entity already", name))
		}
	}
	s := &Self{name: name, w: w, knobs: knobs}
	w.module.selves.list = append(w.module.selves.list, s)
	return s
}

// Entity is the plugin's own entity; 0 before the Stage's ECS is set up.
func (s *Self) Entity() uid.UID64 { return s.id }

// Plays has the plugin play roles. Call it where the Stage defines its rules.
func (s *Self) Plays(roles ...*rule.Part) {
	s.w.must(fmt.Sprintf("the plugin %q given roles", s.name), section.Rules)
	s.w.kinds.Play(roles...)
	for _, r := range roles {
		s.roles = s.roles.With(r.Tag())
	}
}

// Changed reports whether an effect changed the plugin's knobs in the step gone, the time to
// work anew what is costly to.
func (s *Self) Changed() bool { return s.w.module.selves.changed(s.id) }

// Target marks the plugin as whom a command may be for.
func (*Self) Target() {}

// Route is c for the plugin's entity, which the world carries out.
func (s *Self) Route(c rule.Command) control.Command {
	c.Whom = entity.Named(s.name)
	return c
}

type selves struct {
	list   []*Self
	states *goke.Query
	marks  goke.Comp[tag.Tags[effect.States]]
}

func (s *selves) changed(id uid.UID64) bool {
	return s.states != nil && s.states.Seek(id) && s.marks.At(s.states.Cursor()).Has(effect.Changed)
}

func (s *selves) tokens() []goke.CompToken {
	var tokens []goke.CompToken
	for _, self := range s.list {
		for _, k := range self.knobs {
			tokens = append(tokens, k.LoadToken())
		}
	}
	return tokens
}

func (s *selves) system() goke.System {
	return goke.SystemFn{OnInit: func(si *goke.SysInit) {
		s.states = si.NewQueryBuilder(&s.marks).Build()
		var label goke.Comp[entity.Label]
		var roles goke.OptComp[tag.Tags[cond.Roles]]
		named := si.NewQueryBuilder(&label).Optional(&roles).Build()
		byName := make(map[uint64]*Self, len(s.list))
		for _, self := range s.list {
			byName[entity.LabelOf(self.name, "").Name] = self
		}
		for named.All(); named.Next(); {
			cur := named.Cursor()
			labels := label.Slice(cur)
			for i, id := range cur.IDs {
				self, ok := byName[labels[i].Name]
				if !ok {
					continue
				}
				self.id = id
				if roles.Present(cur) {
					roles.Slice(cur)[i] = self.roles
				}
				delete(byName, labels[i].Name)
			}
		}
		for _, self := range s.list {
			if _, missing := byName[entity.LabelOf(self.name, "").Name]; missing {
				self.id = self.make(si)
			}
		}
	}}
}

func (s *Self) make(si *goke.SysInit) uid.UID64 {
	comps := append([]comp.Comp{
		comp.Const(entity.LabelOf(s.name, "")),
		comp.Const(effect.Wide{}),
		comp.Marks[effect.States](),
		comp.Const(s.roles),
	}, s.knobs...)
	spawners := make([]comp.Spawner, len(comps))
	var columns []goke.Addable
	for i, c := range comps {
		spawners[i] = c.Spawner()
		columns = append(columns, spawners[i].Columns()...)
	}
	f := si.NewFactory(columns...)
	f.Create(1)
	var id uid.UID64
	for f.Next() {
		id = f.Cursor.IDs[0]
		for _, sp := range spawners {
			sp.Write(&f.Cursor, 0, nil, id)
		}
	}
	return id
}
