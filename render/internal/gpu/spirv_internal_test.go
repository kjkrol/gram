package gpu

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const tinyWGSL = `@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return vec4<f32>(1.0, 0.0, 0.0, 1.0);
}
`

func TestSPIRV_IsKeptAndReadBackAndCompiledAnewWhereTheFileIsNoSPIRV(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GRAM_SHADER_CACHE", dir)
	first, err := spirvOf(tinyWGSL)
	if err != nil {
		t.Fatal(err)
	}
	kept, _ := filepath.Glob(filepath.Join(dir, "*.spv"))
	if len(kept) != 1 {
		t.Fatalf("%d files kept, want the one compiled", len(kept))
	}
	if again, err := spirvOf(tinyWGSL); err != nil || !slices.Equal(again, first) {
		t.Errorf("read back %d words (%v), want the %d compiled", len(again), err, len(first))
	}
	if err := os.WriteFile(kept[0], []byte("not SPIR-V at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	if again, err := spirvOf(tinyWGSL); err != nil || !slices.Equal(again, first) {
		t.Errorf("over a broken file %d words (%v), want the %d compiled anew", len(again), err,
			len(first))
	}
	t.Setenv("GRAM_SHADER_CACHE", "off")
	if cachedAt(tinyWGSL) != "" {
		t.Error("with the cache off a place is named, want none")
	}
}
