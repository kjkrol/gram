package world

import (
	"fmt"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// Self is a plugin's own entity in the world, one a plugin, called by the plugin's name: it
// carries the plugin's knobs — components the plugin only reads, which an effect's Alter turns —
// the roles the plugin plays and the effects it is under, any number at once (effect.Wide). A
// plugin embeds the Self NewSelf makes, and so is whom a command may be for
// (rule.Cast(e).On(s.atmosphere)) and plays roles (s.atmosphere.Plays(lunar)). The entity is made
// as the Stage's ECS is set up, or found again in a loaded game by its name.
type Self struct {
	name  string
	w     *Plugin
	knobs []comp.Comp
	roles tag.Tags[rule.Roles]
	id    uid.UID64
}

var _ rule.Router = (*Self)(nil)

// NewSelf is the entity of the plugin called name in w, carrying knobs; call it as the plugin is
// made. One name is one entity: a second of the same name panics.
func NewSelf(w *Plugin, name string, knobs ...comp.Comp) *Self {
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

// Plays has the plugin play roles: the rules of its own moments they obey fire, being of its
// entity. Call it where the Stage defines its rules.
func (s *Self) Plays(roles ...*rule.Part) {
	s.w.must(fmt.Sprintf("the plugin %q given roles", s.name), section.Rules)
	s.w.kinds.Play(roles...)
	for _, r := range roles {
		s.roles = s.roles.With(r.Tag())
	}
}

// Changed reports whether an effect changed the plugin's knobs in the step gone, or one that
// Shows began or ended (effect.Changed): the time to work anew what is costly to.
func (s *Self) Changed() bool { return s.w.module.selves.changed(s.id) }

// Target marks the plugin as whom a command may be for.
func (*Self) Target() {}

// Route is c for the plugin's entity, found by its name: the world carries it out.
func (s *Self) Route(c rule.Command) any {
	c.Whom = entity.Named(s.name)
	return c
}

// selves are the plugins' own entities, made or found by their system, the world's first.
type selves struct {
	list   []*Self
	states *goke.Query
	marks  goke.Comp[tag.Tags[effect.States]]
}

// changed reports whether id's Changed is on.
func (s *selves) changed(id uid.UID64) bool {
	return s.states != nil && s.states.Seek(id) && s.marks.At(s.states.Cursor()).Has(effect.Changed)
}

// tokens name the knobs to a save being loaded.
func (s *selves) tokens() []goke.CompToken {
	var tokens []goke.CompToken
	for _, self := range s.list {
		for _, k := range self.knobs {
			tokens = append(tokens, k.LoadToken())
		}
	}
	return tokens
}

// system finds the entities a loaded game brought by their names, giving each the roles its
// plugin plays now, and makes the rest.
func (s *selves) system() goke.System {
	return goke.SystemFn{OnInit: func(si *goke.SysInit) {
		s.states = si.NewQueryBuilder(&s.marks).Build()
		var label goke.Comp[entity.Label]
		var roles goke.OptComp[tag.Tags[rule.Roles]]
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
		for _, self := range s.list { // in the order the plugins were made
			if _, missing := byName[entity.LabelOf(self.name, "").Name]; missing {
				self.id = self.make(si)
			}
		}
	}}
}

// make spawns the plugin's entity: its name, room for every effect, the markers, its roles, its
// knobs.
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
