package hooks

import (
	"log"

	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/uid"
)

// LogFalls writes a line, once for each, for an entity standing where its domain may not be — in
// a hole, a walker in the water — as it falls in.
func LogFalls() plugin.Rule {
	told := map[uid.UID64]bool{}
	return host.Every(func(_ plugin.Tick, st unit.Standing) {
		if st.Fallen() && !told[st.ID] {
			told[st.ID] = true
			log.Printf("entity %d fell into the %s at cell %d", st.ID, st.Kind.Name, st.Cell)
		}
	})
}
