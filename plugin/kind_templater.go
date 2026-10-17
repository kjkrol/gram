package plugin

import "github.com/kjkrol/gram/entity/kind"

// KindTemplater is a plugin asking something of the kinds a game defines: in KindTemplates, and
// nowhere else, it writes what it requires of a template's kinds and what it brings them.
type KindTemplater interface {
	KindTemplates(r *kind.Roster)
}
