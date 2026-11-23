package portrait_test

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var dimensionMethods = map[string]bool{
	"Radius":    true,
	"Height":    true,
	"MinHeight": true,
	"MaxHeight": true,
}

var outsideTokens = []*regexp.Regexp{
	regexp.MustCompile(`FontSize\(\s*[0-9.]`),
	regexp.MustCompile(`ui\.(Hex|RGBA?)\(`),
	regexp.MustCompile(`\.Font\(\s*"`),
	regexp.MustCompile(`color\.N?RGBA(64)?\{`),
	regexp.MustCompile(`unicode\.Han`),
	regexp.MustCompile(`\b(Font)?Size:\s*[0-9.]`),
	regexp.MustCompile(`\bSize\s*=\s*(\[\d*\]float(32|64)\{)?[0-9.]`),
}

func nativeSourceFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("../native*/*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no native sources found (%v)", err)
	}
	return files
}

func exprHasNumericLiteral(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if found {
			return false
		}
		if lit, ok := n.(*ast.BasicLit); ok && (lit.Kind == token.INT || lit.Kind == token.FLOAT) {
			found = true
			return false
		}
		return true
	})
	return found
}

func nativeLiteralCounts(t *testing.T) map[string]map[string]int {
	t.Helper()
	counts := make(map[string]map[string]int)
	fset := token.NewFileSet()
	for _, file := range nativeSourceFiles(t) {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !dimensionMethods[sel.Sel.Name] {
				return true
			}
			hasNum := false
			for _, arg := range call.Args {
				if exprHasNumericLiteral(arg) {
					hasNum = true
					break
				}
			}
			if !hasNum {
				return true
			}
			args := make([]string, len(call.Args))
			for i, arg := range call.Args {
				var buf bytes.Buffer
				if err := printer.Fprint(&buf, fset, arg); err != nil {
					t.Fatal(err)
				}
				args[i] = buf.String()
			}
			literal := sel.Sel.Name + "(" + strings.Join(args, ", ") + ")"
			if counts[file] == nil {
				counts[file] = make(map[string]int)
			}
			counts[file][literal]++
			return true
		})
	}
	return counts
}

func TestNativeDimensionLiteralsOnlyRatchetDown(t *testing.T) {
	actual := nativeLiteralCounts(t)
	fileSet := make(map[string]bool, len(nativeLiteralAllowlist)+len(actual))
	for file := range nativeLiteralAllowlist {
		fileSet[file] = true
	}
	for file := range actual {
		fileSet[file] = true
	}
	files := make([]string, 0, len(fileSet))
	for file := range fileSet {
		files = append(files, file)
	}
	sort.Strings(files)
	for _, file := range files {
		allowed := nativeLiteralAllowlist[file]
		found := actual[file]
		if allowed != nil && len(allowed) == 0 {
			t.Errorf("%s has an empty allowlist entry; remove it (把 allowlist 下调到 0)", file)
			continue
		}
		literalSet := make(map[string]bool, len(allowed)+len(found))
		for literal := range allowed {
			literalSet[literal] = true
		}
		for literal := range found {
			literalSet[literal] = true
		}
		literals := make([]string, 0, len(literalSet))
		for literal := range literalSet {
			literals = append(literals, literal)
		}
		sort.Strings(literals)
		for _, literal := range literals {
			want, got := allowed[literal], found[literal]
			switch {
			case got > want:
				t.Errorf("%s %s count increased from %d to %d; use Portrait design tokens instead of adding numeric literals", file, literal, want, got)
			case want <= 0:
				t.Errorf("%s %s has invalid allowlist count %d; remove zero-count entries (把 allowlist 下调到 0)", file, literal, want)
			case got < want:
				t.Errorf("%s %s count decreased from %d to %d; lower the allowlist to %d (把 allowlist 下调到 %d) and remove zero-count entries", file, literal, want, got, got, got)
			}
		}
	}
}

func TestNativeSurfacesTakeDesignValuesFromPortrait(t *testing.T) {
	for _, file := range nativeSourceFiles(t) {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			for _, re := range outsideTokens {
				if re.MatchString(line) {
					t.Errorf("%s:%d defines a design value outside Portrait: %s", file, i+1, strings.TrimSpace(line))
				}
			}
		}
	}
}
