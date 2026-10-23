package wgsl

import (
	"strings"
	"testing"

	"github.com/kjkrol/gram/internal/gpu"
)

func TestRegisterImage_RefusesATwiceGivenANameTooManyAndALateOne(t *testing.T) {
	panics := func(what string, f func()) {
		t.Helper()
		defer func() {
			t.Helper()
			if recover() == nil {
				t.Errorf("%s did not panic", what)
			}
		}()
		f()
	}
	was, sealedWas := images, sealed
	defer func() { images, sealed = was, sealedWas }()
	images, sealed = nil, false
	RegisterImage("Probe")
	panics("an image registered twice", func() { RegisterImage("Probe") })
	for len(images) < gpu.FrameImages {
		RegisterImage(strings.Repeat("x", len(images)))
	}
	panics("an image past the frame's", func() { RegisterImage("Past") })
	images = images[:1]
	SourceWith(nil)
	panics("an image registered after a shader was built", func() { RegisterImage("Late") })
}

func TestSourceWith_DeclaresTheImagesAndHandsEachMaterialToItsEntry(t *testing.T) {
	was, sealedWas, entriesWas, sourcesWas := images, sealed, entries, sources
	defer func() { images, sealed, entries, sources = was, sealedWas, entriesWas, sourcesWas }()
	images, sealed, entries, sources = nil, false, nil, nil
	RegisterImage("Probe")
	ids := RegisterMaterials([]byte("fn glow() {}"), nil, "Glow", "Dim")
	src := SourceWith([]byte("fn fs_main() {}"))
	if ids[0] != 0 || ids[1] != 1 {
		t.Errorf("the materials are %v, want 0 and 1", ids)
	}
	for _, want := range []string{"@group(3) @binding(0) var Probe: texture_2d<f32>;", "fn glow() {}",
		"if k < 0.5 {\n        return Glow(", "if k < 1.5 {\n        return Dim(", "fn fs_main() {}"} {
		if !strings.Contains(src, want) {
			t.Errorf("the source lacks %q", want)
		}
	}
}
