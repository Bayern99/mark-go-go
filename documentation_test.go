package main

import (
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestAcceptanceDocumentationLinks(t *testing.T) {
	link := regexp.MustCompile(`\[[^\]\n]+\]\(([^\s)]+)\)`)
	docs := []string{"ARCHITECTURE.md", "DESIGN.md"}
	err := filepath.WalkDir("docs", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Ext(path) == ".md" {
			docs = append(docs, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range docs {
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
