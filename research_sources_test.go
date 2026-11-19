package main

import (
	"bytes"
	"encoding/base64"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResearchSourceContextUsesOnlySelectedCurrentFiles(t *testing.T) {
	dir := canonicalPath(t.TempDir())
	pic := filepath.Join(dir, "sample image.png")
	notes := filepath.Join(dir, "notes.md")
	const pixel = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGNgSPsPAAHPAWYvlgrbAAAAAElFTkSuQmCC"
	data, _ := base64.StdEncoding.DecodeString(pixel)
	if err := os.WriteFile(pic, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(notes, []byte("read through Feynman"), 0600); err != nil {
		t.Fatal(err)
	}
	approved, err := selectedResearchFiles([]string{pic, notes})
	if err != nil {
		t.Fatal(err)
	}
	selected := []string{canonicalPath(pic), canonicalPath(notes)}
	blocks, warnings, err := researchSourceBlocks(approved, selected, true)
	if err != nil || len(warnings) != 0 || len(blocks) != 3 {
		t.Fatal(len(blocks), warnings, err)
	}
	u, _ := url.Parse(blocks[0]["uri"])
	if blocks[0]["type"] != "resource_link" || u.Path != selected[0] || blocks[1]["type"] != "image" || blocks[1]["mimeType"] != "image/png" || blocks[1]["data"] != pixel || blocks[2]["name"] != "notes.md" {
		t.Fatal(blocks)
	}
	blocks, warnings, err = researchSourceBlocks(approved, selected, false)
	if err != nil || len(blocks) != 2 || len(warnings) != 1 {
		t.Fatal(blocks, warnings, err)
	}
	if _, _, err := researchSourceBlocks(approved, []string{filepath.Join(dir, "private")}, true); err == nil {
		t.Fatal("unselected path accepted")
	}
	if err := os.Rename(pic, filepath.Join(dir, "old.png")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pic, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := researchSourceBlocks(approved, selected, true); err == nil {
		t.Fatal("replaced image accepted")
	}
	if err := os.WriteFile(pic, bytes.Repeat([]byte{'A'}, maxResearchImageBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	approved, err = selectedResearchFiles([]string{pic})
	if err != nil {
		t.Fatal(err)
	}
	blocks, warnings, err = researchSourceBlocks(approved, selected[:1], true)
	if err != nil || len(blocks) != 1 || len(warnings) != 1 || !strings.Contains(warnings[0], "sample image.png") {
		t.Fatal(blocks, warnings, err)
	}
}
