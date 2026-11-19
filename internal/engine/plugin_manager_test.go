package engine

import (
	"errors"
	"testing"

	"github.com/kjkrol/goke/v3"
)

type stubPostLoader struct{ ran bool }

func (s *stubPostLoader) SetupSystems() []goke.System { return nil }
func (s *stubPostLoader) PostLoad() goke.System {
	return goke.SystemFn{OnInit: func(si *goke.SysInit) { s.ran = true }}
}

func TestEcsHost_PostLoadSystems_RunsTrackedPostLoader(t *testing.T) {
	host := newECSHost()
	stub := &stubPostLoader{}
	host.track(stub)

	systems := host.postLoadSystems()
	if len(systems) != 1 {
		t.Fatalf("postLoadSystems() returned %d systems, want 1", len(systems))
	}

	goke.New().Setup(systems...)
	if !stub.ran {
		t.Error("expected the tracked PostLoader's system to have run")
	}
}

func TestEcsHost_ProvidedComps_SkipsValuesWithoutCompProvider(t *testing.T) {
	host := newECSHost()
	host.track(&stubPostLoader{})

	if got := host.providedComps(); len(got) != 0 {
		t.Errorf("providedComps() = %v, want empty (stubPostLoader isn't a CompProvider)", got)
	}
}

type stubSerializable struct{ targets []any }

func (s *stubSerializable) Persisted() []any { return s.targets }

type saveTargetPayload struct{ N int }

func TestEcsHost_SaveTargets_CollectsTrackedSerializable(t *testing.T) {
	host := newECSHost()
	a, b := &saveTargetPayload{N: 1}, &saveTargetPayload{N: 2}
	host.track(&stubSerializable{targets: []any{a, b}})
	host.track(&stubPostLoader{})

	got := host.saveTargets()
	if len(got) != 1 {
		t.Fatalf("saveTargets() returned %d groups, want 1", len(got))
	}
	for _, targets := range got {
		if len(targets) != 2 || targets[0] != any(a) || targets[1] != any(b) {
			t.Errorf("saveTargets() targets = %v, want [%v %v]", targets, a, b)
		}
	}
}

type stubPopulator struct {
	ran int
	err error
}

func (s *stubPopulator) Populate() error {
	s.ran++
	return s.err
}

func TestEcsHost_RunPopulate_CallsTrackedPopulators(t *testing.T) {
	host := newECSHost()
	a, b := &stubPopulator{}, &stubPopulator{}
	host.track(a)
	host.track(&stubPostLoader{})
	host.track(b)

	if err := host.runPopulate(); err != nil {
		t.Fatalf("runPopulate: %v", err)
	}
	if a.ran != 1 || b.ran != 1 {
		t.Errorf("Populate calls = %d, %d, want 1, 1", a.ran, b.ran)
	}
}

func TestEcsHost_RunPopulate_StopsAtFirstError(t *testing.T) {
	host := newECSHost()
	wantErr := errors.New("test: populate failed")
	failing, after := &stubPopulator{err: wantErr}, &stubPopulator{}
	host.track(failing)
	host.track(after)

	if err := host.runPopulate(); err != wantErr {
		t.Fatalf("runPopulate() = %v, want %v", err, wantErr)
	}
	if after.ran != 0 {
		t.Errorf("Populate after the failing one ran %d times, want 0", after.ran)
	}
}

type stubCompProvider struct{}

type stubSharedComp struct{ N int }

func (stubCompProvider) LoadComps() []goke.CompToken {
	return []goke.CompToken{goke.LoadComp[stubSharedComp]()}
}

// A kind and a module may both name a type, and goke refuses to be told twice.
func TestEcsHost_ProvidedComps_ListsASharedTypeOnce(t *testing.T) {
	host := newECSHost()
	host.track(stubCompProvider{})
	host.track(stubCompProvider{})

	if got := host.providedComps(); len(got) != 1 {
		t.Errorf(
			"providedComps() returned %d tokens, want 1 for a type two providers name",
			len(got),
		)
	}
}
