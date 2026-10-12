package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kjkrol/goke/v3"
)

type stateA struct{ N int }
type stateB struct{ S string }

// TestSaveLoad_RoundTrip exercises save/Load against a bare *goke.ECS, no Game involved.
func TestSaveLoad_RoundTrip(t *testing.T) {
	basePath := t.TempDir() + "/save"

	ecs := goke.New()
	a := &stateA{N: 42}
	b := &stateB{S: "hello"}

	if err := save(ecs, basePath, "", map[string][]any{"a": {a}, "b": {b}}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	wantPath := basePath + ".game.save"
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected %s to exist: %v", wantPath, err)
	}
	if tmps, _ := filepath.Glob(filepath.Join(os.TempDir(), "gram-ecs-*.tmp")); len(tmps) != 0 {
		t.Errorf("expected no leftover temp files after Save, got %v", tmps)
	}

	ecs2 := goke.New()
	a2 := &stateA{}
	b2 := &stateB{}
	if _, err := load(ecs2, basePath, "", nil, map[string][]any{"a": {a2}, "b": {b2}}); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if tmps, _ := filepath.Glob(filepath.Join(os.TempDir(), "gram-ecs-*.tmp")); len(tmps) != 0 {
		t.Errorf("expected no leftover temp files after Load, got %v", tmps)
	}

	if a2.N != 42 {
		t.Errorf("a2.N = %d, want 42", a2.N)
	}
	if b2.S != "hello" {
		t.Errorf("b2.S = %q, want %q", b2.S, "hello")
	}
}

func TestSaveLoad_ToleratesResourceAddedAfterSave(t *testing.T) {
	basePath := t.TempDir() + "/save"

	ecs := goke.New()
	if err := save(ecs, basePath, "", map[string][]any{"a": {&stateA{N: 42}}}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	ecs2 := goke.New()
	a2 := &stateA{}
	c2 := &stateB{S: "default"}
	if _, err := load(ecs2, basePath, "", nil, map[string][]any{"a": {a2}, "c": {c2}}); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if a2.N != 42 {
		t.Errorf("a2.N = %d, want 42 (present in the save)", a2.N)
	}
	if c2.S != "default" {
		t.Errorf("c2.S = %q, want unchanged %q (absent from the save)", c2.S, "default")
	}
}

func TestListSaves_QuicksaveAndNamed(t *testing.T) {
	basePath := t.TempDir() + "/save"
	ecs := goke.New()

	if err := save(ecs, basePath, "", map[string][]any{"a": {&stateA{N: 1}}}); err != nil {
		t.Fatalf("Save(quicksave): %v", err)
	}
	if err := save(ecs, basePath, "checkpoint", map[string][]any{"a": {&stateA{N: 2}}}); err != nil {
		t.Fatalf("Save(checkpoint): %v", err)
	}

	labels, err := listSaves(basePath)
	if err != nil {
		t.Fatalf("ListSaves: %v", err)
	}
	want := []string{"", "checkpoint"}
	if len(labels) != len(want) {
		t.Fatalf("ListSaves() = %v, want %v", labels, want)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Errorf("ListSaves()[%d] = %q, want %q", i, labels[i], want[i])
		}
	}
}
