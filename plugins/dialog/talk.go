package dialog

import "github.com/kjkrol/uid"

// Script is where an entity's conversations begin: a kind's component (Plugin.Script), the same
// for all its entities with comp.Const or each one's own with comp.Load.
type Script struct{ Start uint64 }

// Talk is the conversation an entity holds now as its speaker: the node it stands at and the
// listener; Node 0 for none. The plugin gives it where missing.
type Talk struct {
	Node uint64
	With uid.UID64
}

// talking reports whether the entity holds a conversation.
func (t Talk) talking() bool { return t.Node != 0 }

// MaxMemory is how many others an entity remembers what it makes of.
const MaxMemory = 8

// Memory is whom an entity has talked with and what it makes of them: a mood from -100 to 100
// each, and whether they talked to the end. Full, it forgets the one it feels least about. The
// plugin gives it where missing.
type Memory struct {
	Of   [MaxMemory]uid.UID64
	Mood [MaxMemory]int8
	Met  [MaxMemory]bool // talked to the end
	Used [MaxMemory]bool // the slot remembers somebody: entity 0 is an entity
}

// slot is where m remembers id; -1 for nowhere.
func (m *Memory) slot(id uid.UID64) int {
	for i := range m.Of {
		if m.Used[i] && m.Of[i] == id {
			return i
		}
	}
	return -1
}

// of is the mood towards id and whether they talked to the end; nothing for one not remembered.
func (m *Memory) of(id uid.UID64) (mood int8, met bool) {
	if i := m.slot(id); i >= 0 {
		return m.Mood[i], m.Met[i]
	}
	return 0, false
}

// keep is the slot remembering id, taken now if it was not: a free one, else the one with the
// mildest mood.
func (m *Memory) keep(id uid.UID64) int {
	if i := m.slot(id); i >= 0 {
		return i
	}
	at := -1
	for i := range m.Of {
		if !m.Used[i] {
			at = i
			break
		}
		if at < 0 || abs(m.Mood[i]) < abs(m.Mood[at]) {
			at = i
		}
	}
	m.Of[at], m.Mood[at], m.Met[at], m.Used[at] = id, 0, false, true
	return at
}

// feel adds by to the mood towards id, held within -100 and 100.
func (m *Memory) feel(id uid.UID64, by int8) {
	i := m.keep(id)
	m.Mood[i] = int8(max(-100, min(100, int(m.Mood[i])+int(by))))
}

// met notes that the entity talked with id to the end.
func (m *Memory) met(id uid.UID64) { m.Met[m.keep(id)] = true }

func abs(v int8) int {
	if v < 0 {
		return -int(v)
	}
	return int(v)
}
