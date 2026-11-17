package gpu

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/spirv"
)

// compiled is how naga writes a program's SPIR-V: indices held in range, loops bounded, buffers
// and image loads left to the device, which wgpu's Vulkan backend has check them wherever it
// can, asking naga the same of its own compile there. naga's check of every image load took
// 3 % of a frame of the island.
var compiled = naga.CompileOptions{SPIRV: spirv.Options{
	BoundsCheckPolicies: backend.BoundsCheckPolicies{
		Index:        backend.BoundsCheckRestrict,
		Buffer:       backend.BoundsCheckUnchecked,
		ImageLoad:    backend.BoundsCheckUnchecked,
		BindingArray: backend.BoundsCheckUnchecked,
	},
}}

// spirvOf is source compiled to SPIR-V by naga, kept in the shader cache and read back from it
// the next time: GRAM_SHADER_CACHE "off" keeps none, a folder keeps it there, the user's cache
// folder's gram/shaders by default. A file that is no SPIR-V is compiled anew.
func spirvOf(source string) ([]uint32, error) {
	path := cachedAt(source)
	if path != "" {
		if b, err := os.ReadFile(path); err == nil && isSPIRV(b) {
			return wordsOf(b), nil
		}
	}
	b, err := naga.CompileWithOptions(source, compiled)
	if err != nil {
		return nil, err
	}
	if !isSPIRV(b) {
		return nil, fmt.Errorf("naga's output is no SPIR-V: %d bytes", len(b))
	}
	if path != "" {
		keep(path, b)
	}
	return wordsOf(b), nil
}

// cachedAt is where source's SPIR-V is kept, named by it, naga's version and how it is compiled;
// "" for none
func cachedAt(source string) string {
	dir := os.Getenv("GRAM_SHADER_CACHE")
	switch dir {
	case "off":
		return ""
	case "":
		base, err := os.UserCacheDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(base, "gram", "shaders")
	}
	how := fmt.Sprintf("%+v", compiled)
	sum := sha256.Sum256([]byte(nagaVersion() + "\n" + how + "\n" + source))
	return filepath.Join(dir, hex.EncodeToString(sum[:])+".spv")
}

// nagaVersion is the version of naga the binary is built with, which a kept SPIR-V is of
var nagaVersion = sync.OnceValue(func() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, m := range info.Deps {
			if m.Path == "github.com/gogpu/naga" {
				if m.Replace != nil {
					m = m.Replace
				}
				return m.Version + " " + m.Sum
			}
		}
	}
	return "unknown"
})

// keep writes b to path whole or not at all; a cache that cannot be written is no cache
func keep(path string, b []byte) {
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	f, err := os.CreateTemp(filepath.Dir(path), "*.tmp")
	if err != nil {
		return
	}
	_, werr := f.Write(b)
	cerr := f.Close()
	if werr != nil || cerr != nil || os.Rename(f.Name(), path) != nil {
		os.Remove(f.Name())
	}
}

const spirvMagic = 0x07230203

func isSPIRV(b []byte) bool {
	return len(b) >= 20 && len(b)%4 == 0 && binary.LittleEndian.Uint32(b) == spirvMagic
}

func wordsOf(b []byte) []uint32 {
	w := make([]uint32, len(b)/4)
	for i := range w {
		w[i] = binary.LittleEndian.Uint32(b[4*i:])
	}
	return w
}
