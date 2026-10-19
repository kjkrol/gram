package plugin_test

import (
	"bytes"
	"encoding/gob"
	"testing"

	"github.com/kjkrol/gram/plugin"
)

type settings struct {
	Shown bool
	Count int
}

// persistence encodes what the saving side gives and decodes it into the loading side's
// pointers, as the engine does with every Serializable.
func persistence(t *testing.T, saved, loading []any) {
	t.Helper()
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	for _, v := range saved {
		if err := enc.Encode(v); err != nil {
			t.Fatal(err)
		}
	}
	dec := gob.NewDecoder(&buf)
	for _, v := range loading {
		if err := dec.Decode(v); err != nil {
			t.Fatal(err)
		}
	}
}

func TestKeptZeroValuesComeBackOverTheStart(t *testing.T) {
	saving := settings{Shown: false, Count: 0}
	loaded := settings{Shown: true, Count: 3}
	kept := plugin.Keep(&loaded)
	persistence(t, plugin.Keep(&saving).Persisted(), kept.Persisted())
	kept.Restore()
	if loaded != saving {
		t.Fatalf("restored %+v, want %+v", loaded, saving)
	}
}

func TestKeptWithoutASaveKeepsTheStart(t *testing.T) {
	start := settings{Shown: true, Count: 3}
	kept := plugin.Keep(&start)
	kept.Persisted()
	kept.Restore()
	if (start != settings{Shown: true, Count: 3}) {
		t.Fatalf("start changed to %+v", start)
	}
}
