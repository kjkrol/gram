package plugin_test

import (
	"strings"
	"testing"

	"github.com/kjkrol/gram/plugin"
)

type keptGame struct {
	Gold  float32
	Names []string
}

func TestKeepOf_SavesAStructWholeAndPutsItBack(t *testing.T) {
	res := &keptGame{Gold: 12.5, Names: []string{"a", "b"}}
	kept := plugin.KeepOf(res)
	blob := kept.Persisted()
	res.Gold, res.Names = 0, nil
	loaded := plugin.KeepOf(res)
	*loaded.Persisted()[0].(*[]byte) = *blob[0].(*[]byte)
	loaded.(plugin.Restorer).Restore()
	if res.Gold != 12.5 || len(res.Names) != 2 {
		t.Errorf("restored %+v, want 12.5 gold and two names", *res)
	}
}

func TestKeepOf_WantsAPointerToAStruct(t *testing.T) {
	for _, v := range []any{keptGame{}, 7, (*keptGame)(nil), new(int)} {
		func() {
			defer func() {
				if r := recover(); r == nil || !strings.Contains(r.(string), "pointer to a struct") {
					t.Errorf("KeepOf(%T): %v, want a panic naming a pointer to a struct", v, r)
				}
			}()
			plugin.KeepOf(v)
		}()
	}
}
