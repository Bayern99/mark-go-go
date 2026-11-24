package workspace

import (
	"context"
	"testing"
)

func TestClosePreflightIncludesEveryEditedDocument(t *testing.T) {
	h := NewService()
	defer h.CloseAgentCache()
	ids := []string{"untitled", "readonly", "conflict", "deleted", "autosave-failed", "clean"}
	for i, id := range ids {
		h.byID[id] = &docSession{host: h, id: id, edited: i < len(ids)-1}
	}
	docs, err := h.ClosePreflight(context.Background(), ids)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != len(ids)-1 {
		t.Fatalf("preflight returned %d documents, want every edited document: %+v", len(docs), docs)
	}
	for i, doc := range docs {
		if doc.ID != ids[i] || !doc.Edited {
			t.Fatalf("preflight document %d = %+v", i, doc)
		}
	}
}

func TestRenameValidation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		valid bool
	}{
		{"Notes.md", true}, {"Notes.markdown", true}, {"", false}, {"   ", false},
		{"../Notes.md", false}, {"folder/Notes.md", false}, {`folder\\Notes.md`, false},
		{"Notes.txt", false}, {".", false}, {"..", false},
	} {
		if err := ValidateDocumentName(tc.name); (err == nil) != tc.valid {
			t.Errorf("ValidateDocumentName(%q) error=%v, valid=%v", tc.name, err, tc.valid)
		}
	}
}
