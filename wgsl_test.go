package gram_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Metal refuses an entry point given one builtin or location twice (a parameter and a field of
// another's struct, two fields, two parameters), which WGSL forbids and naga and Vulkan let by.
func TestWGSL_EntryPointsTakeEachBuiltinOnce(t *testing.T) {
	units := wgslUnits(t)
	fragments := 0
	for _, u := range units {
		for _, e := range entryPoints(u.src) {
			if e.stage == "fragment" {
				fragments++
			}
			check := func(verb string, decls []string) {
				sets, err := ioSets(units, u, decls)
				switch k := repeated(sets, map[string]bool{}); {
				case err != nil:
					t.Errorf("%s: %s: %v", u.name, e.name, err)
				case k != "":
					t.Errorf("%s: the %s entry %s %s @%s twice", u.name, e.stage, e.name, verb, k)
				}
			}
			check("takes", e.params)
			check("gives", []string{e.result})
		}
	}
	if fragments == 0 {
		t.Fatal("no fragment entry found")
	}
}

// the directories whose WGSL is composed into other packages' shaders
var wgslLibraries = map[string]bool{"render/internal/wgsl": true, "internal/gpu/shaders": true}

type wgslUnit struct {
	name, dir string // dir is "" for a Go literal
	src       string
	structs   map[string][]string // each member's declaration
}

type wgslEntry struct {
	stage, name string
	params      []string
	result      string
}

// every .wgsl in the repository and every Go string literal holding an entry point
func wgslUnits(t *testing.T) []*wgslUnit {
	var units []*wgslUnit
	add := func(name, dir, src string) {
		u := &wgslUnit{
			name:    name,
			dir:     dir,
			src:     stripWGSLComments(src),
			structs: map[string][]string{},
		}
		for _, m := range wgslStruct.FindAllStringSubmatch(u.src, -1) {
			u.structs[m[1]] = splitTop(m[2])
		}
		units = append(units, u)
	}
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && path != "." && strings.HasPrefix(d.Name(), "."):
			return filepath.SkipDir
		case strings.HasSuffix(path, ".wgsl"):
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			add(path, filepath.ToSlash(filepath.Dir(path)), string(src))
		case strings.HasSuffix(path, ".go"):
			src, err := os.ReadFile(path)
			if err != nil || !wgslEntryAttr.Match(src) {
				return err
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			ast.Inspect(f, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING || !wgslEntryAttr.MatchString(lit.Value) {
					return true
				}
				s, err := strconv.Unquote(lit.Value)
				if err == nil {
					add(fset.Position(lit.Pos()).String(), "", s)
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return units
}

var (
	wgslStruct    = regexp.MustCompile(`\bstruct\s+(\w+)\s*\{([^}]*)\}`)
	wgslEntryAttr = regexp.MustCompile(`@(vertex|fragment|compute)\b`)
	wgslFn        = regexp.MustCompile(`\bfn\s+(\w+)\s*\(`)
	wgslAttr      = regexp.MustCompile(`^@\s*(\w+)\s*(?:\(([^)]*)\))?\s*`)
	wgslNamed     = regexp.MustCompile(`^(\w+)\s*:\s*(.+)$`)
	wgslIdent     = regexp.MustCompile(`^[A-Za-z_]\w*$`)
	wgslScalar    = regexp.MustCompile(`^(bool|[fiu]32|f16|(vec[234]|mat[234]x[234])[fhiu]?)$`)
)

func entryPoints(src string) []wgslEntry {
	var es []wgslEntry
	for _, m := range wgslEntryAttr.FindAllStringSubmatchIndex(src, -1) {
		fn := wgslFn.FindStringSubmatchIndex(src[m[1]:])
		if fn == nil {
			continue
		}
		open := m[1] + fn[1] - 1
		end := closing(src, open)
		if end < 0 {
			continue
		}
		body := strings.IndexByte(src[end:], '{')
		if body < 0 {
			continue
		}
		result := strings.TrimPrefix(strings.TrimSpace(src[end+1:end+body]), "->")
		es = append(es, wgslEntry{
			stage:  src[m[2]:m[3]],
			name:   src[m[1]+fn[2] : m[1]+fn[3]],
			params: splitTop(src[open+1 : end]),
			result: strings.TrimSpace(result),
		})
	}
	return es
}

// the index of the parenthesis closing the one at open, -1 for none
func closing(src string, open int) int {
	depth := 0
	for i := open; i < len(src); i++ {
		switch src[i] {
		case '(':
			depth++
		case ')':
			if depth--; depth == 0 {
				return i
			}
		}
	}
	return -1
}

// ioSets has, for each declaration of decls, the builtins and locations it takes, once for each
// struct its type may be in the shaders u is composed into
func ioSets(units []*wgslUnit, u *wgslUnit, decls []string) ([][][]string, error) {
	var sets [][][]string
	for _, d := range decls {
		keys, typ := ioOf(d)
		if len(keys) > 0 || !wgslIdent.MatchString(typ) || wgslScalar.MatchString(typ) {
			sets = append(sets, [][]string{keys})
			continue
		}
		var found [][]string
		for _, members := range structsNamed(units, u, typ) {
			var ks []string
			for _, m := range members {
				k, _ := ioOf(m)
				ks = append(ks, k...)
			}
			found = append(found, ks)
		}
		if len(found) == 0 {
			return nil, fmt.Errorf("no struct %s for %q", typ, d)
		}
		sets = append(sets, found)
	}
	return sets, nil
}

// the members of each struct name in the nearest of u's own source, its directory, the libraries
// and the whole repository holding one
func structsNamed(units []*wgslUnit, u *wgslUnit, name string) [][]string {
	near := []func(o *wgslUnit) bool{
		func(o *wgslUnit) bool { return o == u },
		func(o *wgslUnit) bool { return u.dir != "" && o.dir == u.dir },
		func(o *wgslUnit) bool { return wgslLibraries[o.dir] },
		func(o *wgslUnit) bool { return true },
	}
	for _, in := range near {
		var found [][]string
		for _, o := range units {
			if members, ok := o.structs[name]; ok && in(o) {
				found = append(found, members)
			}
		}
		if len(found) > 0 {
			return found
		}
	}
	return nil
}

// the builtins and locations a declaration (a parameter, a member, a result) carries, and its type
func ioOf(decl string) (keys []string, typ string) {
	rest := strings.TrimSpace(decl)
	for {
		m := wgslAttr.FindStringSubmatch(rest)
		if m == nil {
			break
		}
		if m[1] == "builtin" || m[1] == "location" {
			keys = append(keys, m[1]+"("+strings.Join(strings.Fields(m[2]), "")+")")
		}
		rest = rest[len(m[0]):]
	}
	if m := wgslNamed.FindStringSubmatch(rest); m != nil {
		rest = m[2]
	}
	return keys, strings.TrimSpace(rest)
}

// the first key repeated across one choice of struct for each declaration, "" for none
func repeated(sets [][][]string, seen map[string]bool) string {
	if len(sets) == 0 {
		return ""
	}
	for _, keys := range sets[0] {
		took := map[string]bool{}
		for k := range seen {
			took[k] = true
		}
		for _, k := range keys {
			if took[k] {
				return k
			}
			took[k] = true
		}
		if k := repeated(sets[1:], took); k != "" {
			return k
		}
	}
	return ""
}

// s split at its commas and semicolons outside parentheses and angle brackets, the parts trimmed
// and the empty ones left out
func splitTop(s string) []string {
	var parts []string
	depth, from := 0, 0
	for i := 0; i <= len(s); i++ {
		if i < len(s) {
			switch s[i] {
			case '(', '<':
				depth++
				continue
			case ')', '>':
				depth--
				continue
			case ',', ';':
				if depth > 0 {
					continue
				}
			default:
				continue
			}
		}
		if p := strings.TrimSpace(s[from:i]); p != "" {
			parts = append(parts, p)
		}
		from = i + 1
	}
	return parts
}

// src with its line and (nested) block comments blanked out
func stripWGSLComments(src string) string {
	b := []byte(src)
	for i := 0; i < len(b); i++ {
		switch {
		case b[i] == '/' && i+1 < len(b) && b[i+1] == '/':
			for ; i < len(b) && b[i] != '\n'; i++ {
				b[i] = ' '
			}
		case b[i] == '/' && i+1 < len(b) && b[i+1] == '*':
			depth := 0
			for ; i < len(b); i++ {
				switch {
				case b[i] == '/' && i+1 < len(b) && b[i+1] == '*':
					depth++
					b[i], b[i+1] = ' ', ' '
					i++
				case b[i] == '*' && i+1 < len(b) && b[i+1] == '/':
					depth--
					b[i], b[i+1] = ' ', ' '
					i++
				case b[i] != '\n':
					b[i] = ' '
				}
				if depth == 0 {
					break
				}
			}
		}
	}
	return string(b)
}
