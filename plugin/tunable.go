package plugin

import "github.com/kjkrol/gram/control"

// Tunable is a plugin with settings the players adjust on the settings screen, a
// control.Setting each, listed in the order shown.
type Tunable interface {
	Settings() []control.Setting
}
