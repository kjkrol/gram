package cell

import (
	"bytes"
	"fmt"
)

// MaxNameLen is the longest Kind name, in bytes.
const MaxNameLen = 16

// Name is a Kind's name as fixed-size bytes, so a Ground component stays contiguous in memory.
type Name [MaxNameLen]byte

// Named is the Name for s; it panics past MaxNameLen.
func Named(s string) Name {
	n, ok := nameOf(s)
	if !ok {
		panic(fmt.Sprintf("cell: kind name %q is longer than %d bytes", s, MaxNameLen))
	}
	return n
}

// nameOf is Named that reports a name too long instead of panicking.
func nameOf(s string) (Name, bool) {
	var n Name
	if len(s) > MaxNameLen {
		return n, false
	}
	copy(n[:], s)
	return n, true
}

func (n Name) String() string { return string(bytes.TrimRight(n[:], "\x00")) }
