package world

import "github.com/kjkrol/gram/plugin/section"

// Figure is a kind of entity in one piece: what it is, defined in the Kinds section, and how
// it looks, declared on the world's atlas in a scene. One with states of its own is Stateful,
// one with roles and rules of its own Ruled; the Stage runs each part in its section.
type Figure interface {
	Define(w *Plugin)
	Look(a *Atlas, w *Plugin)
}

// Stateful is a Figure with effects of its own, defined in the Effects section.
type Stateful interface{ Effects(w *Plugin) }

// Ruled is a Figure with roles and rules of its own, defined in the Rules section.
type Ruled interface{ Rules(w *Plugin) }

// Figures notes the figures the Stage leads through its sections, in this order: their
// Effects after the game's, their Rules after the game's, their Define in Kinds, and their Look
// as the atlas's Looks asks. Called in the Stage's Figures link.
func (p *Plugin) Figures(figures ...Figure) {
	p.must("figures noted", section.Plugins)
	p.figures = append(p.figures, figures...)
}

// FiguresIn runs the figures' part of the section: Effects, Rules or Kinds; nothing elsewhere.
// The Stage calls it after the game's own functions of the section.
func (p *Plugin) FiguresIn(part section.Part) {
	for _, f := range p.figures {
		switch part {
		case section.Effects:
			if s, ok := f.(Stateful); ok {
				s.Effects(p)
			}
		case section.Rules:
			if r, ok := f.(Ruled); ok {
				r.Rules(p)
			}
		case section.Kinds:
			f.Define(p)
		}
	}
}

// Looks declares every figure's look on the atlas, in the order the figures were noted, and
// hands the atlas back to Close.
func (a *Atlas) Looks() *Atlas {
	for _, f := range a.p.figures {
		f.Look(a, a.p)
	}
	return a
}
