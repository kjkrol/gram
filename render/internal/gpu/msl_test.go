package gpu

import (
	"regexp"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/wgsl"
)

const indexedWGSL = `
fn byteInNow(t: vec4<f32>, k: i32) -> u32 {
    return u32(t[k] * 255.0 + 0.5);
}

fn byteInVar(t: vec4<f32>, k: i32) -> u32 {
    var v = t;
    return u32(v[k] * 255.0 + 0.5);
}

fn same(a: ptr<function, array<u32, 9>>, i: i32, k: i32) -> bool {
    return (*a)[i] == (*a)[k];
}

@fragment
fn fs_main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> {
    var a: array<u32, 9>;
    for (var j = 0; j < 9; j++) {
        a[j] = u32(j);
    }
    let k = i32(p.x) % 9;
    let n = byteInNow(p, k % 4) + byteInVar(p, k % 4);
    return vec4<f32>(f32(n), select(0.0, 1.0, same(&a, k, 8 - k)), 0.0, 1.0);
}
`

// a bounds check's zero followed by anything but the end of its own expression
var unscoped = regexp.MustCompile(`DefaultConstructible\(\)\s*[^);\s]`)

// Metal refuses a bounds-checked access left unscoped beside an operator
// (`k < 4 ? t[k] : DefaultConstructible() * 255.0`), which naga wrote before v0.21. gram's WGSL
// indexes parameters at run time (byteIn), so the test says if that comes back.
func TestMSL_ABoundsCheckedAccessIsScopedBesideItsOperator(t *testing.T) {
	policies := map[string]backend.BoundsCheckPolicy{
		"restrict":  backend.BoundsCheckRestrict,
		"read zero": backend.BoundsCheckReadZeroSkipWrite,
	}
	for name, policy := range policies {
		code := mslOf(t, indexedWGSL, policy)
		for _, head := range []string{"uint byteInNow(", "uint byteInVar(", "bool same("} {
			if fn := mslFunc(t, code, head); unscoped.MatchString(fn) {
				t.Errorf("%s, %s: a bounds-checked access unscoped beside an operator:\n%s",
					name, head, fn)
			}
		}
	}
}

// mslOf is source in MSL under one bounds-check policy for indices, buffers and image loads;
// wgpu's Metal backend asks for BoundsCheckRestrict.
func mslOf(t *testing.T, source string, policy backend.BoundsCheckPolicy) string {
	t.Helper()
	module, _, err := wgsl.Parse(source, wgsl.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	checks := backend.BoundsCheckPolicies{
		Index:        policy,
		Buffer:       policy,
		ImageLoad:    policy,
		BindingArray: backend.BoundsCheckUnchecked,
	}
	code, _, err := msl.Compile(module, msl.Options{BoundsCheckPolicies: checks})
	if err != nil {
		t.Fatal(err)
	}
	return code
}

// mslFunc is the function of code from the line starting with head to its closing brace
func mslFunc(t *testing.T, code, head string) string {
	t.Helper()
	from := strings.Index(code, "\n"+head)
	if from < 0 {
		t.Fatalf("no %q in the MSL:\n%s", head, code)
	}
	fn := code[from+1:]
	to := strings.Index(fn, "\n}")
	if to < 0 {
		t.Fatalf("no end of %q in the MSL:\n%s", head, code)
	}
	return fn[:to+2]
}
