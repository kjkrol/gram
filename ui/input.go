package ui

import (
	"log"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
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
	hit, pressed := topmost(within, c.Pos, nil)
	switch {
	case pressed != nil:
		if c.Action == control.ActionPress {
			s.give(pressed.content.(*button).cmd)
		}
		return false
	case hit == nil:
		return modal == nil
	default:
		p, ok := hit.content.(*picture)
		return ok && p.input != nil
	}
}

// through reports whether the cursor at p is over the world: no element but a picture with an
// Input under it, no modal window held elsewhere.
func (s *Scene) through(modal *Element, p geom.Vec) bool {
	if modal != nil {
		return false
	}
	hit, pressed := topmost(s.root, p, nil)
	if pressed != nil {
		return false
	}
	if hit == nil {
		return true
	}
	pic, ok := hit.content.(*picture)
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
				s.give(cmd)
			}
		}
	}
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

// topmost is the last element drawn under p, none of its own a container's, and the button it lies
// in, if any.
func topmost(e *Element, p geom.Vec, in *Element) (hit, pressed *Element) {
	if e.hidden {
		return nil, nil
	}
	if _, ok := e.content.(*button); ok {
		in = e
	}
	if _, ok := e.content.(container); !ok || e.fill.A > 0 {
		if inside(e.box, p) && (e.mask == nil || e.mask.contains(e.box, p)) {
			hit, pressed = e, in
		}
	}
	for _, c := range e.children {
		if h, b := topmost(c, p, in); h != nil {
			hit, pressed = h, b
		}
	}
	return hit, pressed
}
