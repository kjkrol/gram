package ui

import (
	"log"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/uid"
)

// Keys are the scene's own keys: each gives its command when its key is pressed with the modifiers
// it asks for — one of the scene's own (Show, Hide, Toggle), or any other through Issue.
func (s *Scene) Keys(bindings ...control.Binding) *Scene {
	s.keys = append(s.keys, bindings...)
	return s
}

// Issue is how the scene gives a command that is not its own, a button's or a key's: the
// players' Issue, as a rule. Without one the scene gives it as nobody through the Runtime.
func (s *Scene) Issue(fn func(cmd control.Command) error) *Scene {
	s.issue = fn
	return s
}

// HandleEvents routes this tick's input: a click to the topmost element it hits, the scene's
// keys to their commands, what is left to Input.
func (s *Scene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	s.runtime = runtime
	s.passed = *events
	s.passed.ClickQueue = s.passed.ClickQueue[:0:0]
	if s.root != nil {
		modal := s.modal()
		for _, c := range events.ClickQueue {
			if s.click(modal, c) {
				s.passed.ClickQueue = append(s.passed.ClickQueue, c)
			}
		}
		if !s.through(modal, events.MousePos) {
			s.passed.ScrollDelta = 0
		}
	}
	s.press(events)
	if s.input != nil {
		s.wire()
		s.input(&s.passed, runtime, composition)
	}
}

func (s *Scene) wire() {
	if s.root == nil {
		return
	}
	var visit func(e *Element)
	visit = func(e *Element) {
		if e.hidden {
			return
		}
		if p, ok := e.content.(*picture); ok && p.input != nil {
			p.input.Over(shrink(e.box, e.padding), p.src)
		}
		for _, c := range e.children {
			visit(c)
		}
	}
	visit(s.root)
}

func (s *Scene) click(modal *Element, c control.ClickEvent) bool {
	within := s.root
	if modal != nil {
		if !modal.Hits(c.Pos) {
			return false
		}
		within = modal
	}
	t := topmost(within, c.Pos, nil)
	switch {
	case t.button != nil:
		if c.Action == control.ActionPress {
			if t.look != nil {
				t.look()
			} else {
				for _, cmd := range t.button.content.(*button).cmds {
					s.giveAbout(cmd, t.about, t.pinned)
				}
			}
		}
		return false
	case t.hit == nil:
		return modal == nil
	default:
		p, ok := t.hit.content.(*picture)
		return ok && p.input != nil
	}
}

func (s *Scene) through(modal *Element, p geom.Vec) bool {
	if modal != nil {
		return false
	}
	t := topmost(s.root, p, nil)
	if t.button != nil {
		return false
	}
	if t.hit == nil {
		return true
	}
	pic, ok := t.hit.content.(*picture)
	return ok && pic.input != nil
}

func (s *Scene) press(events *control.InputEvents) {
	mods := control.Mods{Shift: events.Modifiers.Shift, Ctrl: events.Modifiers.Ctrl, Alt: events.Modifiers.Alt}
	for _, k := range events.KeyEvents {
		if k.Action != control.ActionPress {
			continue
		}
		for _, b := range s.keys {
			if b.Trigger != (control.KeyPress{Key: k.Key, Mods: mods}) {
				continue
			}
			if cmd, ok := b.Build(control.Context{Mods: mods}); ok {
				s.giveAbout(cmd, 0, false)
			}
		}
	}
}

func (s *Scene) giveAbout(cmd control.Command, id uid.UID64, pinned bool) {
	cmd, ok := about(cmd, id, pinned)
	if !ok {
		log.Printf("ui: scene %q: a command for ui.It (or an About) given outside an element pinned to an entity", s.name)
		return
	}
	s.give(cmd)
}

func (s *Scene) give(cmd control.Command) {
	if s.carry(cmd) {
		return
	}
	issue := s.issue
	if issue == nil && s.runtime != nil {
		issue = s.runtime.Issue
	}
	if issue == nil {
		log.Printf("ui: scene %q has nobody to give %T to", s.name, cmd)
		return
	}
	if err := issue(cmd); err != nil {
		log.Printf("ui: scene %q: %v", s.name, err)
	}
}

func (s *Scene) modal() *Element {
	var top *Element
	var visit func(e *Element)
	visit = func(e *Element) {
		if e.gone() {
			return
		}
		if e.modal() {
			top = e
		}
		for _, c := range e.children {
			visit(c)
		}
	}
	visit(s.root)
	return top
}

type target struct {
	hit, button *Element
	look        func()
	about       uid.UID64
	pinned      bool
}

func topmost(e *Element, p geom.Vec, in *Element) target {
	if e.hidden {
		return target{}
	}
	if e.pin == nil {
		return topmostHere(e, p, in)
	}
	var t target
	e.pin.each(e, e.parent, func(at *instance) {
		if h := topmostHere(e, p, in); h.hit != nil {
			t = h
			t.about, t.pinned = at.id, true
		}
		if at.show && e.pin.show.hitsHere(p) {
			id, looker := at.id, at.looker
			t = target{hit: e.pin.show, button: e.pin.show, look: func() { looker.LookAt(id) }}
		}
	})
	return t
}

func topmostHere(e *Element, p geom.Vec, in *Element) target {
	if e.absent() {
		return target{}
	}
	if _, ok := e.content.(*button); ok {
		in = e
	}
	var t target
	if _, ok := e.content.(container); !ok || e.fillColor().A > 0 {
		if inside(e.box, p) && (e.mask == nil || e.mask.contains(e.box, p)) {
			t = target{hit: e, button: in}
		}
	}
	for _, c := range e.children {
		if h := topmost(c, p, in); h.hit != nil {
			t = h
		}
	}
	return t
}
