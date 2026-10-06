// Package section names the parts a Stage is defined in, in their order — the plugins, the
// players, the effects, the rules, the commands, the cells' kinds, the units' kinds, the controls,
// the looks, the scenes, then the layout and the units of a fresh game — and lets a plugin refuse
// what is defined in the wrong one. A Stage built with package game/stage goes through them; one
// written by hand is in none, and nothing is refused.
package section

import "fmt"

// Part is a part of a Stage's definition.
type Part uint8

const (
	Anytime  Part = iota // none: a Stage written by hand, a test
	Plugins              // the plugins are made and used: anything may be defined, as they define their own
	Players              // the players and the plugins' default keys
	Effects              // the states
	Rules                // the roles, the rules, the plans
	Commands             // what can be asked for
	Cells                // the kinds of cells, with the roles their cells play
	Kinds                // the kinds of units
	Controls             // the game's own keys
	Looks                // the drawing rules
	Scenes               // the scenes
	Layout               // a fresh game's board
	Units                // a fresh game's units
	Done                 // the Stage is defined
)

var names = [...]string{"no section", "Plugins", "Players", "Effects", "Rules", "Commands", "Cells", "Kinds", "Controls", "Looks", "Scenes", "Layout", "Units", "the defined Stage"}

func (p Part) String() string { return names[p] }

// Reader tells which part of the Stage is being defined: the engine's Initializer is one.
type Reader interface{ Section() Part }

// Writer is a Reader told which part begins: what a Stage built in sections drives.
type Writer interface {
	Reader
	Enter(p Part)
}

// Check is an error for what, which belongs in one of want, when src says another part of the
// Stage is being defined; nil for a src that tells no parts, for no section and for Plugins,
// where the plugins define what is their own.
func Check(src any, what string, want ...Part) error {
	r, ok := src.(Reader)
	if !ok {
		return nil
	}
	now := r.Section()
	if now == Anytime || now == Plugins {
		return nil
	}
	for _, w := range want {
		if now == w {
			return nil
		}
	}
	return fmt.Errorf("%s in %v: it belongs in %v", what, now, want[0])
}

// Must panics with what Check says.
func Must(src any, what string, want ...Part) {
	if err := Check(src, what, want...); err != nil {
		panic("gram: " + err.Error())
	}
}
