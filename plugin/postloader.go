package plugin

import "github.com/kjkrol/goke/v3"

// PostLoader is a tracked value needing a one-time system run right after Persistence.Load
// restored the entities.
type PostLoader interface {
	PostLoad() goke.System
}
