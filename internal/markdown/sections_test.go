package markdown

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type sectionVectorFixture struct {
	Hashes []struct {
		Text string `json:"text"`
		Hash string `json:"hash"`
	} `json:"hashes"`
	Normalize []struct {
		Text string `json:"text"`
		Key  string `json:"key"`
	} `json:"normalize"`
	Document string `json:"document"`
	Sections []struct {
		Heading    string `json:"heading"`
		Occurrence int    `json:"occurrence"`
		Start      int    `json:"start"`
		End        int    `json:"end"`
		Text       string `json:"text"`
		Hash       string `json:"hash"`
	} `json:"sections"`
	Notes struct {
		Document         string `json:"document"`
		TextWithoutNotes string `json:"textWithoutNotes"`
		Sections         []struct {
			Heading    string `json:"heading"`
			Occurrence int    `json:"occurrence"`
			Start      int    `json:"start"`
			End        int    `json:"end"`
			Exclude    []struct {
				Start int `json:"start"`
				End   int `json:"end"`
			} `json:"exclude"`
			Text          string `json:"text"`
			Hash          string `json:"hash"`
			TextAsWritten string `json:"textAsWritten"`
			HashAsWritten string `json:"hashAsWritten"`
		} `json:"sections"`
	} `json:"notes"`
}

func loadSectionVectors(t *testing.T) sectionVectorFixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "testdata", "markdown", "section-vectors.json"))
	if err != nil {
		t.Fatalf("read section-vectors.json: %v", err)
	}
	var fx sectionVectorFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("unmarshal section-vectors.json: %v", err)
	}
	return fx
}

func TestSectionVectorsParity(t *testing.T) {
	fx := loadSectionVectors(t)

	for _, tc := range fx.Hashes {
		if got := SectionHashAsWritten(tc.Text); got != tc.Hash {
			t.Errorf("SectionHashAsWritten(%q) = %q, want %q", tc.Text, got, tc.Hash)
		}
	}

	for _, tc := range fx.Normalize {
		if got := NormalizeHeadingText(tc.Text); got != tc.Key {
			t.Errorf("NormalizeHeadingText(%q) = %q, want %q", tc.Text, got, tc.Key)
		}
	}

	sections := ExtractSections(fx.Document)
	if len(sections) != len(fx.Sections) {
		t.Fatalf("len(ExtractSections) = %d, want %d", len(sections), len(fx.Sections))
	}
	for i, want := range fx.Sections {
		got := sections[i]
		if got.Heading != want.Heading || got.Occurrence != want.Occurrence || got.Start != want.Start || got.End != want.End || got.Hash != want.Hash {
			t.Errorf("section[%d] = %+v, want {Heading:%q Occurrence:%d Start:%d End:%d Hash:%q}",
				i, got, want.Heading, want.Occurrence, want.Start, want.End, want.Hash)
		}
	}

	noteSections := ExtractSections(fx.Notes.Document)
	if len(noteSections) != len(fx.Notes.Sections) {
		t.Fatalf("len(noteSections) = %d, want %d", len(noteSections), len(fx.Notes.Sections))
	}
	for i, want := range fx.Notes.Sections {
		got := noteSections[i]
		if got.Heading != want.Heading || got.Occurrence != want.Occurrence || got.Start != want.Start || got.End != want.End || got.Hash != want.Hash {
			t.Errorf("notes.section[%d] = %+v, want {Heading:%q Occurrence:%d Start:%d End:%d Hash:%q}",
				i, got, want.Heading, want.Occurrence, want.Start, want.End, want.Hash)
		}
	}

	earlierRule := 0
	for i, want := range fx.Notes.Sections {
		got, saved := noteSections[i], want.Hash
		if want.HashAsWritten != "" {
			earlierRule++
			saved = want.HashAsWritten
			if written := SectionHashAsWritten(want.TextAsWritten); written != saved || saved == want.Hash {
				t.Errorf("notes.section[%d]: the text as written hashes to %q, the vector says %q and %q under the current rule", i, written, saved, want.Hash)
			}
		}
		if !got.UnchangedSince(want.Hash) || !got.UnchangedSince(saved) {
			t.Errorf("notes.section[%d] counts as changed since a view saved %q or %q", i, want.Hash, saved)
		}
		if other := fx.Notes.Sections[(i+1)%len(fx.Notes.Sections)]; got.UnchangedSince(other.Hash) {
			t.Errorf("notes.section[%d] counts as unchanged since a view saved %q", i, other.Hash)
		}
	}
	if earlierRule == 0 {
		t.Error("no vector gives a hash under the earlier rule")
	}

	withoutNotesSections := ExtractSections(fx.Notes.TextWithoutNotes)
	if len(withoutNotesSections) != len(fx.Notes.Sections) {
		t.Fatalf("len(withoutNotesSections) = %d, want %d", len(withoutNotesSections), len(fx.Notes.Sections))
	}
	for i, want := range fx.Notes.Sections {
		if withoutNotesSections[i].Hash != want.Hash {
			t.Errorf("withoutNotes[%d].Hash = %q, want %q", i, withoutNotesSections[i].Hash, want.Hash)
		}
	}
}

func TestExtractSectionsInlineAndBlocks(t *testing.T) {
	md := "---\ntitle: \"# Not\"\n---\n\n```md\n# Fenced\n```\n\n<div>\n# InHTML\n</div>\n\n## **Bold** and *italic* `code` [link](https://example.com) <span>x</span>\n\nBody.\n"
	sections := ExtractSections(md)
	if len(sections) != 1 {
		t.Fatalf("got %d sections, want 1: %+v", len(sections), sections)
	}
	if sections[0].Heading != "Bold and italic code link x" {
		t.Fatalf("Heading = %q, want %q", sections[0].Heading, "Bold and italic code link x")
	}
	if sections[0].Level != 2 || sections[0].StartLine != 13 || sections[0].EndLine != 15 {
		t.Fatalf("unexpected section metadata: %+v", sections[0])
	}
}

func TestExtractNotesDetails(t *testing.T) {
	fx := loadSectionVectors(t)
	notes := ExtractNotes(fx.Notes.Document)
	if len(notes) != 3 {
		t.Fatalf("len(ExtractNotes) = %d, want 3: %+v", len(notes), notes)
	}
	if notes[0].Text != "第一条 🎉" || notes[0].SectionHeading != "概要 🚀" || notes[0].SectionOccurrence != 1 {
		t.Errorf("note[0] = %+v", notes[0])
	}
	if notes[1].Text != "第二条 --> end" || notes[1].SectionHeading != "Detail" || notes[1].SectionOccurrence != 1 {
		t.Errorf("note[1] = %+v", notes[1])
	}
	if notes[2].Text != "last" || notes[2].SectionHeading != "End" || notes[2].SectionOccurrence != 1 {
		t.Errorf("note[2] = %+v", notes[2])
	}
}

func TestBlankAtTheEndOfTheDocumentLeavesSavedSectionHashesAlone(t *testing.T) {
	fx := loadSectionVectors(t)
	body := strings.TrimRight(fx.Document, "\n")
	for _, blank := range []string{"", "\n", "\n\n\n", " \t\n", "\n  "} {
		sections := ExtractSections(body + blank)
		if len(sections) != len(fx.Sections) {
			t.Fatalf("ending %q: %d sections, want %d", blank, len(sections), len(fx.Sections))
		}
		for i, saved := range fx.Sections {
			if sections[i].Hash != saved.Hash {
				t.Errorf("ending %q: %s hash = %s, want the saved %s", blank, saved.Heading, sections[i].Hash, saved.Hash)
			}
		}
	}
}

func TestExtractNotesImmediatelyAfterPre(t *testing.T) {
	for name, source := range map[string]string{
		"attributes": "# Guide\n\n<pre>code</pre>\n<!-- note kind=\"reader\": attributed secret -->\n",
		"multiline":  "# Guide\n\n<pre>code</pre>\n<!-- note:\nmultiline secret\n-->\n",
		"crlf":       "# Guide\r\n\r\n<pre>code</pre>\r\n<!-- note:\r\ncrlf secret\r\n-->\r\n",
	} {
		t.Run(name, func(t *testing.T) {
			notes := ExtractNotes(source)
			if len(notes) != 1 || notes[0].SectionHeading != "Guide" || !strings.Contains(notes[0].Text, "secret") {
				t.Fatalf("notes = %+v", notes)
			}
		})
	}
}

func TestBlankAtTheEndOfACRLFDocumentLeavesSavedSectionHashesAlone(t *testing.T) {
	const body = "# One\r\n\r\nFirst.\r\n\r\n## Two\r\n\r\nSecond."
	saved := []string{"fnv1a32:dc8415af", "fnv1a32:b38d3fde"}
	for _, blank := range []string{"", "\r\n", "\r\n\r\n", "\n", " \r\n"} {
		sections := ExtractSections(body + blank)
		if len(sections) != len(saved) {
			t.Fatalf("ending %q: %d sections, want %d", blank, len(sections), len(saved))
		}
		for i, want := range saved {
			if sections[i].Hash != want {
				t.Errorf("ending %q: %s hash = %s, want the saved %s", blank, sections[i].Heading, sections[i].Hash, want)
			}
		}
	}
}
