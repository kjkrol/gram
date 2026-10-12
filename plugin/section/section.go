// Package section names the parts a Stage is defined in, in their order, and lets a plugin
// refuse what is defined in the wrong one. A plugin may name parts of its own between them
// ([Define]) and give a game a [Section] of one to write into its Stage; a Stage written by hand
// is in none, and nothing is refused.
package section

import "fmt"

// Part is a part of a Stage's definition.
type Part uint8

const (
	Anytime Part = iota // a Stage written by hand
	Plugins             // anything may be defined: the plugins define their own
	Players             // the players and the plugins' keys
	Effects
	Rules // the roles and the plans too
	Commands
	Kinds    // of cells and of units
	Controls // the game's own keys
	Spawn
	Scenes
	Done
)

var names = []string{"no section", "Plugins", "Players", "Effects", "Rules", "Commands", "Kinds", "Controls", "Spawn", "Scenes", "the defined Stage"}

var bounds = map[Part][2]Part{}

func (p Part) String() string {
	if int(p) < len(names) {
		return names[p]
	}
	return fmt.Sprintf("part %d", p)
}

// Define is a part of a Stage's definition a plugin names, between the parts after and before.
// Call it once, for a package variable.
func Define(name string, after, before Part) Part {
	if len(names) > 255 {
		panic(fmt.Sprintf("section: %q is one part too many", name))
	}
	if after >= before || before > Controls {
		panic(fmt.Sprintf("section: %q lies after %v and before %v: a plugin's part lies between two of a Stage's up to Controls", name, after, before))
	}
	p := Part(len(names))
	names = append(names, name)
	bounds[p] = [2]Part{after, before}
	return p
}

// Between is the parts a plugin's part lies between; false for a Stage's own.
func (p Part) Between() (after, before Part, ok bool) {
	b, ok := bounds[p]
	return b[0], b[1], ok
}

// Section is a plugin's part of a Stage's definition, run where the game writes it.
type Section struct {
	Part Part
	run  func(used []any) error
}

// Of is the Section of part running f with the plugin of type P the Stage uses.
func Of[P any](part Part, f func(p P) error) Section {
	return Section{Part: part, run: func(used []any) error {
		for _, u := range used {
			if p, ok := u.(P); ok {
				return f(p)
			}
		}
		var none P
		return fmt.Errorf("the Stage uses no %T", none)
	}}
}

// Run runs s with the plugins the Stage uses.
func (s Section) Run(used []any) error { return s.run(used) }

// Reader tells which part of the Stage is being defined.
type Reader interface{ Section() Part }

// Writer is a Reader told which part begins.
type Writer interface {
	Reader
	Enter(p Part)
}

// Check is an error for what, which belongs in one of want, when src says another part of the
// Stage is being defined; nil for a src that tells no parts, for Anytime and for Plugins.
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
