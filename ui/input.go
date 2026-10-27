package ui

import (
	"log"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
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
func (s *Scene) Issue(fn func(cmd command.Command) error) *Scene {
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
			switch {
			case s.held != nil:
				// a drag ends wherever the button is let go
				if c.Action == control.ActionRelease {
					s.held.content.(dragged).drag(s, s.held, c.Pos)
					s.drop()
				}
			case s.overlay != nil:
				s.clickOverlay(c)
			case s.click(modal, c):
				s.passed.ClickQueue = append(s.passed.ClickQueue, c)
			}
		}
		if s.held != nil {
			s.held.content.(dragged).drag(s, s.held, events.MousePos)
		}
		if s.overlay != nil || !s.through(modal, events.MousePos) {
			s.passed.ScrollDelta = 0
			if s.overlay == nil && events.ScrollDelta != 0 {
				s.wheel(modal, events.MousePos, events.ScrollDelta)
			}
		}
	}
	s.press(events)
	if s.input != nil {
		s.wire(events.MousePos)
		s.input(&s.passed, runtime, composition)
	}
}

func (s *Scene) wire(cursor geom.Vec) {
	if s.root == nil {
		return
	}
	within := s.root
	if modal := s.modal(); modal != nil {
		within = modal
	}
	under := topmost(within, cursor, nil, false).hit
	var visit func(e *Element)
	visit = func(e *Element) {
		if e.shut() {
			return
		}
		if p, ok := e.content.(*picture); ok && p.input != nil {
			p.input.Over(shrink(e.box, e.padding), p.src)
			if h, ok := p.input.(Hovered); ok {
				h.Hover(under == e)
			}
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
	t := topmost(within, c.Pos, nil, false)
	switch {
	case t.pressed != nil:
		if c.Action == control.ActionPress {
			if t.look != nil {
				t.look()
			} else {
				t.pressed.content.(pressable).press(s, t.pressed, c.Pos, t.about, t.pinned)
			}
		}
		return false
	case t.hit == nil:
		return modal == nil
	default:
		p, ok := t.hit.content.(*picture)
		if !ok || p.input == nil {
			return false
		}
		clicked, ok := p.input.(Clicked)
		if !ok {
			return true
		}
		// its own press alone: a drag begun elsewhere still ends where it is let go
		if c.Action == control.ActionPress {
			clicked.Click(c.Pos.Sub(shrink(t.hit.box, t.hit.padding).TopLeft))
			return false
		}
		return true
	}
}

func (s *Scene) through(modal *Element, p geom.Vec) bool {
	if modal != nil {
		return false
	}
	t := topmost(s.root, p, nil, false)
	if t.pressed != nil {
		return false
	}
	if t.hit == nil {
		return true
	}
	pic, ok := t.hit.content.(*picture)
	if !ok || pic.input == nil {
		return false
	}
	_, clicked := pic.input.(Clicked)
	return !clicked
}

// open shows list over everything, hanging from field, till an option is pressed or a click
// falls outside it.
func (s *Scene) open(list *Element, field geom.AABB) {
	s.overlay, s.field = list, field
	s.dress()
}

func (s *Scene) close() { s.overlay = nil }

func (s *Scene) clickOverlay(c control.ClickEvent) {
	if c.Action != control.ActionPress {
		return
	}
	t := topmost(s.overlay, c.Pos, nil, false)
	if t.pressed == nil {
		s.close()
		return
	}
	t.pressed.content.(pressable).press(s, t.pressed, c.Pos, t.about, t.pinned)
}

// dragged is a content a press takes hold of (Scene.held) till the button is let go: a slider's
// knob, a scroll's thumb
type dragged interface {
	drag(s *Scene, e *Element, at geom.Vec)
	drop()
}

func (s *Scene) drop() {
	s.held.content.(dragged).drop()
	s.held = nil
}

func (s *Scene) wheel(modal *Element, p geom.Vec, delta float32) {
	within := s.root
	if modal != nil {
		within = modal
	}
	if t := topmost(within, p, nil, false); t.pressed != nil {
		if sl, ok := t.pressed.content.(*slider); ok {
			sl.wheel(s, t.pressed, delta)
			return
		}
	}
	if e := scrollAt(within, p); e != nil {
		e.content.(*scroll).wheel(s, e, delta)
	}
}

// scrollAt is the innermost Scroll under p, nil for none
func scrollAt(e *Element, p geom.Vec) *Element {
	if e.shut() || e.absent() {
		return nil
	}
	var found *Element
	if _, ok := e.content.(*scroll); ok {
		if !inside(e.box, p) {
			return nil
		}
		found = e
	}
	for _, c := range e.children {
		if in := scrollAt(c, p); in != nil {
			found = in
		}
	}
	return found
}

func (s *Scene) press(events *control.InputEvents) {
	mods := control.Mods{Shift: events.Modifiers.Shift, Ctrl: events.Modifiers.Ctrl, Alt: events.Modifiers.Alt}
	for _, k := range events.KeyEvents {
		if k.Action != control.ActionPress {
			continue
		}
		if s.overlay != nil && k.Key == control.KeyEscape {
			s.close()
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

func (s *Scene) giveAbout(cmd command.Command, id uid.UID64, pinned bool) {
	cmd, ok := about(cmd, id, pinned)
	if !ok {
		log.Printf("ui: scene %q: a command for ui.It (or an About) given outside an element pinned to an entity", s.name)
		return
	}
	s.give(cmd)
}

func (s *Scene) give(cmd command.Command) {
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

// target is what a point of the screen hits: the element, the pressable one it lies in, and
// the entity that one is pinned to.
type target struct {
	hit, pressed *Element
	look         func()
	about        uid.UID64
	pinned       bool
}

// topmost is what p hits in e: in is the pressable e lies in, passing the word of a Passing
// element over it (a pressable takes the pointer back for itself and what is in it)
func topmost(e *Element, p geom.Vec, in *Element, passing bool) target {
	if e.shut() {
		return target{}
	}
	if e.pin == nil {
		return topmostHere(e, p, in, passing)
	}
	var t target
	e.pin.each(e, e.parent, func(at *instance) {
		if h := topmostHere(e, p, in, passing); h.hit != nil {
			t = h
			t.about, t.pinned = at.id, true
		}
		if at.show && e.pin.show.hitsHere(p, false) {
			id, looker := at.id, at.looker
			t = target{hit: e.pin.show, pressed: e.pin.show, look: func() { looker.LookAt(id) }}
		}
	})
	return t
}

func topmostHere(e *Element, p geom.Vec, in *Element, passing bool) target {
	if e.absent() {
		return target{}
	}
	if _, ok := e.content.(clipping); ok && !inside(e.box, p) {
		return target{}
	}
	if _, ok := e.content.(pressable); ok {
		in, passing = e, false
	} else {
		passing = passing || e.passing
	}
	var t target
	if _, ok := e.content.(container); !passing && (!ok || e.fillColor().A > 0) {
		if inside(e.box, p) && (e.mask == nil || e.mask.contains(e.box, p)) {
			t = target{hit: e, pressed: in}
		}
	}
	for _, c := range e.children {
		if h := topmost(c, p, in, passing); h.hit != nil {
			t = h
		}
	}
	return t
}
