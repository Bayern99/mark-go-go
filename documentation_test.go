package main

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestAcceptanceDocumentationLinks(t *testing.T) {
	link := regexp.MustCompile(`\[[^\]\n]+\]\(([^\s)]+)\)`)
	for _, doc := range []string{"DESIGN.md", "docs/adr/0005-workspace-document-window-model.md", "docs/acceptance.md"} {
		t.Run(doc, func(t *testing.T) {
			content, err := os.ReadFile(doc)
			if err != nil {
				t.Fatal(err)
			}
			for _, match := range link.FindAllSubmatch(content, -1) {
				target, err := url.Parse(string(match[1]))
				if err != nil {
					t.Error(err)
					continue
				}
				if target.IsAbs() || target.Host != "" || target.Path == "" {
					continue
				}
				if _, err := os.Stat(filepath.Join(filepath.Dir(doc), filepath.FromSlash(target.Path))); err != nil {
					t.Errorf("%s: %v", match[1], err)
				}
			}
		})
	}
}
