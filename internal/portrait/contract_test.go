package portrait_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Native surfaces take fonts, sizes and colors from Portrait only.
var outsideTokens = []*regexp.Regexp{
	regexp.MustCompile(`FontSize\(\s*[0-9.]`),
	regexp.MustCompile(`ui\.(Hex|RGBA?)\(`),
	regexp.MustCompile(`\.Font\(\s*"`),
	regexp.MustCompile(`color\.N?RGBA(64)?\{`),
	regexp.MustCompile(`unicode\.Han`),
}

func TestNativeSurfacesTakeDesignValuesFromPortrait(t *testing.T) {
	files, err := filepath.Glob("../native*/*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no native sources found (%v)", err)
	}
	for _, file := range files {
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
