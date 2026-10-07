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

// Issue is how the scene gives a command that is not its own — a button's, a key's: the players'
// Issue for the player at the keyboard, as a rule.
func (s *Scene) Issue(fn func(cmd any) error) *Scene {
	s.issue = fn
	return s
}

// HandleEvents routes this tick's input. A click goes to the topmost element it hits: a button
// gives its command, a picture with an Input lets it through, any other element keeps it; a shown
// modal window keeps every click and the wheel outside it. The scene's keys are run; what is left
// goes to Input.
func (s *Scene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
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
		s.input(&s.passed, runtime, composition)
	}
}

// click acts on c and reports whether it goes on to Input.
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

// through reports whether the cursor at p is over the world: no element but a picture with an
// Input under it, no modal window held elsewhere.
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

// press gives the command of every key of the scene pressed this tick.
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

// giveAbout gives cmd for the entity the button is pinned to, where it is for It.
func (s *Scene) giveAbout(cmd any, id uid.UID64, pinned bool) {
	cmd, ok := about(cmd, id, pinned)
	if !ok {
		log.Printf("ui: scene %q: a command for ui.It given outside an element pinned to an entity", s.name)
		return
	}
	s.give(cmd)
}

// give carries out cmd if it is the scene's own, else issues it.
func (s *Scene) give(cmd any) {
	if s.carry(cmd) {
		return
	}
	if s.issue == nil {
		log.Printf("ui: scene %q has no Issue for %T", s.name, cmd)
		return
	}
	if err := s.issue(cmd); err != nil {
		log.Printf("ui: scene %q: %v", s.name, err)
	}
}

// modal is the topmost modal window shown, if any.
func (s *Scene) modal() *Element {
	var top *Element
	var visit func(e *Element)
	visit = func(e *Element) {
		if e.hidden {
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

// target is what a point of the screen hits: the last element drawn there, none of its own a
// container's; the button it lies in, if any; and, for a pinned element's Show button, the move of
// the camera it gives.
type target struct {
	hit, button *Element
	look        func()
	about       uid.UID64 // the entity the element hit is pinned to
	pinned      bool
}

// topmost is what p hits under e, in the button in, if any.
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

// topmostHere is topmost where e was last laid.
func topmostHere(e *Element, p geom.Vec, in *Element) target {
	if _, ok := e.content.(*button); ok {
		in = e
	}
	var t target
	if _, ok := e.content.(container); !ok || e.fill.A > 0 {
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
