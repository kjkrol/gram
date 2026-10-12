package dialog

import (
	"log"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/gram/ui"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*talkSystem)(nil)

// talkSystem carries the conversations on: it ends those whose TalkingEf was taken off, whose
// listener is gone or whose node is no more, then carries out the step's Begin, Choose and Feel —
// the game's changes made on the game's components. An entity without Talk gets it as it first
// needs it.
type talkSystem struct {
	p *Plugin

	talks   *goke.Query
	talk    goke.Comp[Talk]
	scripts *goke.Query
	script  goke.Comp[Script]
	bodies  *goke.Query
	body    goke.Comp[entity.Base]
	owners  goke.OptComp[tag.Tags[owner.Family]]
	talkID  goke.CompID

	cb        *goke.CmdBuf        // the pass's, while it runs
	freshTalk map[uid.UID64]*Talk // Talks on their way to entities without one, this pass
	offered   []*choice           // offers' scratch
}

func newTalkSystem(p *Plugin) *talkSystem {
	return &talkSystem{p: p, freshTalk: map[uid.UID64]*Talk{}}
}

func (s *talkSystem) Init(si *goke.SysInit) {
	s.talks = si.NewQueryBuilder(&s.talk).Build()
	s.scripts = si.NewQueryBuilder(&s.script).Build()
	s.bodies = si.NewQueryBuilder(&s.body).Optional(&s.owners).Build()
	s.talkID = si.RegComp[Talk]()
	s.p.bind(si)
}

func (s *talkSystem) Update(cb *goke.CmdBuf, _ time.Duration) {
	s.cb = cb
	s.watch()
	s.p.told.begins.Drain(s.begin)
	s.p.told.chooses.Drain(s.choose)
	s.p.told.feels.Drain(s.feel)
	for id, t := range s.freshTalk {
		cb.AddOne(id, s.talkID, *t)
	}
	clear(s.freshTalk)
	s.cb = nil
}

// watch ends every conversation whose TalkingEf was taken off — no TalkedEf then — whose listener
// is gone, or whose node a loaded game no longer has.
func (s *talkSystem) watch() {
	effects := s.p.w.Effects()
	for s.talks.All(); s.talks.Next(); {
		cur := s.talks.Cursor()
		talks := s.talk.Slice(cur)
		for i, id := range cur.IDs {
			t := &talks[i]
			switch {
			case !t.talking():
			case !effects.Has(id, s.p.talking):
				*t = Talk{}
			case s.p.nodes[t.Node] == nil:
				log.Printf("dialog: entity %d talks at a node no longer defined: the conversation ends", id)
				s.end(id, t, false)
			case !s.bodies.Seek(t.With):
				s.end(id, t, false)
			}
		}
	}
}

// begin carries out a Begin: the entity that gave it begins to talk with whom it is about.
func (s *talkSystem) begin(i control.Issued[Begin]) {
	b := i.Command
	speaker, listener := i.Entity, b.with
	if !i.ByEntity || !b.aimed || speaker == listener {
		log.Printf("dialog: a Begin is an entity's own, about another (rule.Order on a moment naming somebody)")
		return
	}
	var start uint64
	switch {
	case b.At != "":
		start = idOf(b.At)
		if s.p.nodes[start] == nil {
			log.Printf("dialog: entity %d begins at %q, which is not defined", speaker, b.At)
			return
		}
	case s.scripts.Seek(speaker):
		start = s.script.At(s.scripts.Cursor()).Start
	default:
		log.Printf("dialog: entity %d begins to talk with no Script and no node", speaker)
		return
	}
	if s.p.nodes[start] == nil || s.busy(speaker) || s.busy(listener) {
		return
	}
	*s.talkOf(speaker) = Talk{Node: start, With: listener}
	s.p.w.Effects().CastFor(s.cb, speaker, s.p.talking, effect.Forever)
}

// busy reports whether id talks now, as a speaker or a listener.
func (s *talkSystem) busy(id uid.UID64) bool {
	if t, ok := s.freshTalk[id]; ok && t.talking() {
		return true
	}
	for _, t := range s.freshTalk {
		if t.talking() && t.With == id {
			return true
		}
	}
	for s.talks.All(); s.talks.Next(); {
		cur := s.talks.Cursor()
		for i, t := range s.talk.Slice(cur) {
			if t.talking() && (cur.IDs[i] == id || t.With == id) {
				return true
			}
		}
	}
	return false
}

// choose carries out a Choose: the answer offered at the Index, chosen by a player the listener
// obeys — the game's changes it makes, the commands it gives, where it leads.
func (s *talkSystem) choose(i control.Issued[Choose]) {
	c := i.Command
	t := s.found(c.Speaker)
	if t == nil || !t.talking() || !s.obeys(t.With, i.Player) {
		return
	}
	n := s.p.nodes[t.Node]
	if n == nil {
		return
	}
	ch := s.offer(n, c.Speaker, t.With, c.Index)
	if ch == nil {
		return
	}
	for _, change := range ch.then {
		change.apply(c.Speaker, t.With)
	}
	for _, cmd := range ch.do {
		if cmd.Whom == ui.It {
			cmd.Whom = entity.ID(c.Speaker)
		}
		s.p.w.Carrier().Put(i.Player, cmd)
	}
	if ch.next == "" {
		s.end(c.Speaker, t, true)
		return
	}
	t.Node = idOf(ch.next)
}

// feel carries out a Feel: the game's change, the entity that gave it the speaker and whom it is
// about the listener.
func (s *talkSystem) feel(i control.Issued[Feel]) {
	f := i.Command
	if !i.ByEntity || !f.aimed {
		log.Printf("dialog: a Feel is an entity's own, about another (rule.Order on a moment naming somebody)")
		return
	}
	change, ok := s.p.changes.byName[f.Change]
	if !ok {
		log.Printf("dialog: entity %d feels %q, which no change is defined as", i.Entity, f.Change)
		return
	}
	change.apply(i.Entity, f.with)
}

// end ends speaker's conversation t: TalkingEf off and, talked to the end, TalkedEf on.
func (s *talkSystem) end(speaker uid.UID64, t *Talk, toTheEnd bool) {
	effects := s.p.w.Effects()
	effects.Dispel(speaker, s.p.talking)
	if toTheEnd {
		effects.Cast(s.cb, speaker, s.p.talked)
	}
	*t = Talk{}
}

// obeys reports whether the entity id obeys the player by: its owner, or nobody's and by nobody.
func (s *talkSystem) obeys(id uid.UID64, by control.PlayerID) bool {
	if !s.bodies.Seek(id) {
		return false
	}
	var owners tag.Tags[owner.Family]
	if o := s.owners.At(s.bodies.Cursor()); o != nil {
		owners = *o
	}
	return owner.Obeys(owners, by)
}

// offer is the answer of n at index among those offered to listener now; nil for none.
func (s *talkSystem) offer(n *node, speaker, listener uid.UID64, index int) *choice {
	s.offered = s.offers(s.offered[:0], n, speaker, listener)
	if index < 0 || index >= len(s.offered) {
		return nil
	}
	return s.offered[index]
}

// offers appends to dst the answers of n whose conditions hold now.
func (s *talkSystem) offers(dst []*choice, n *node, speaker, listener uid.UID64) []*choice {
	for k := range n.choices {
		if ch := &n.choices[k]; s.holds(ch.when, speaker, listener) {
			dst = append(dst, ch)
		}
	}
	return dst
}

// holds reports whether c holds between speaker and listener: the game's condition, an effect on
// the speaker, or none.
func (s *talkSystem) holds(c condition, speaker, listener uid.UID64) bool {
	yes := true
	switch {
	case c.game != nil:
		yes = c.game.holds(speaker, listener)
	case c.isEffect:
		yes = s.p.w.Effects().Has(speaker, c.effect)
	}
	return yes != c.not
}

// found is id's Talk, on its way to it or on it; nil for none.
func (s *talkSystem) found(id uid.UID64) *Talk {
	if t, ok := s.freshTalk[id]; ok {
		return t
	}
	if s.talks.Seek(id) {
		return s.talk.At(s.talks.Cursor())
	}
	return nil
}

// talkOf is id's Talk, given it this pass where it has none.
func (s *talkSystem) talkOf(id uid.UID64) *Talk {
	if t := s.found(id); t != nil {
		return t
	}
	t := &Talk{}
	s.freshTalk[id] = t
	return t
}

// read is id's Talk as it stands: what the window reads every frame.
func (s *talkSystem) read(id uid.UID64) (Talk, bool) {
	if t := s.found(id); t != nil {
		return *t, true
	}
	return Talk{}, false
}
