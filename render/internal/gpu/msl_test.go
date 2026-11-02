package gpu

import (
	"regexp"
	"strings"
	"testing"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/msl"
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

// The test asserts naga's bug, which decides whether gram's workaround stays: Metal refuses the
// access left unscoped, so gram's WGSL reads such a parameter from a var or through a pointer.
// It fails once naga scopes the access, and the workaround can go.
func TestMSL_NagaStillLeavesAnIndexedParameterUnscopedBesideAnOperator(t *testing.T) {
	fn := mslFunc(t, mslOf(t, indexedWGSL), "uint byteInNow(")
	if !strings.Contains(fn, ": DefaultConstructible() * 255.0") {
		t.Errorf("naga scopes a parameter indexed at run time now: the vars in byteIn (the "+
			"board's tiles_common.wgsl, the painter's paint_common.wgsl), the word on the "+
			"pointers in the painter's paint_ground.wgsl and the trap in gram-render can go:\n%s",
			fn)
	}
}

// a declaration whose whole value is the bounds check, the access a load of its own
var loadAlone = regexp.MustCompile(
	`^\s*[\w:]+ \w+ = [^?=;]+ \? [^?;]+ : DefaultConstructible\(\);$`)

func TestMSL_AVarAndAPointerIndexedAtRunTimeAreLoadedBeforeTheirOperator(t *testing.T) {
	code := mslOf(t, indexedWGSL)
	for _, head := range []string{"uint byteInVar(", "bool same("} {
		fn := mslFunc(t, code, head)
		loads := 0
		for _, line := range strings.Split(fn, "\n") {
			if !strings.Contains(line, "DefaultConstructible()") {
				continue
			}
			loads++
			if !loadAlone.MatchString(line) {
				t.Errorf("%s: a bounds-checked access inside an expression: %s", head, line)
			}
		}
		if loads == 0 {
			t.Errorf("%s: no bounds-checked access, nothing checked:\n%s", head, fn)
		}
	}
}

// mslOf is source in MSL as wgpu's Metal backend writes it
func mslOf(t *testing.T, source string) string {
	t.Helper()
	ast, err := naga.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	module, err := naga.LowerWithSource(ast, source)
	if err != nil {
		t.Fatal(err)
	}
	code, _, err := msl.Compile(module, msl.DefaultOptions())
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
