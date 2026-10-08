package dialog

import (
	"testing"

	"github.com/kjkrol/uid"
)

func TestMemory_FullItForgetsTheOneItFeelsLeastAbout(t *testing.T) {
	var m Memory
	for i := range MaxMemory {
		m.feel(uid.UID64(i), int8(10*(i+1))) // 10 for entity 0, up to 80
	}
	m.feel(100, -50)
	if _, met := m.of(0); m.slot(0) >= 0 || met {
		t.Fatal("entity 0, felt least about, is remembered still")
	}
	if mood, _ := m.of(100); mood != -50 {
		t.Fatalf("the newcomer's mood is %d, want -50", mood)
	}
	if mood, _ := m.of(1); mood != 20 {
		t.Errorf("entity 1's mood is %d, want 20 kept", mood)
	}
	m.feel(1, 100)
	if mood, _ := m.of(1); mood != 100 {
		t.Errorf("a mood past 100 is %d, want held at 100", mood)
	}
}
