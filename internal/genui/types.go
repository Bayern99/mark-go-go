package genui

import (
	"errors"
	"fmt"
	"github.com/Bayern99/mark-go-go/internal/desktop"
	"strings"
	"unicode/utf8"
)

const MaxViewBytes = 1 << 20

var (
	ErrCancelled = errors.New("generation cancelled")
	ErrTimedOut  = errors.New("generation timed out")
)

type GenerateRequest struct {
	Text       string       `json:"text"`
	Mode       string       `json:"mode"`
	ReplaceIDs []string     `json:"replaceIds,omitempty"`
	Request    string       `json:"request,omitempty"`
	Format     string       `json:"format"`
	Language   string       `json:"language,omitempty"`
	Sections   []SectionRef `json:"sections"`
}

type SectionRef struct {
	Heading    string      `json:"heading"`
	Occurrence int         `json:"occurrence"`
	Start      int         `json:"start"`
	End        int         `json:"end"`
	Exclude    []TextRange `json:"exclude,omitempty"`
}

type TextRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type GenerateProgress struct {
	Phase   string            `json:"phase"`
	Text    string            `json:"text"`
	Failure *GenerateFailure  `json:"failure,omitempty"`
	Preview *StatementPreview `json:"preview,omitempty"`
}

type ViewSnapshot struct {
	Exists  bool   `json:"exists"`
	Text    string `json:"text"`
	Error   string `json:"error"`
	CanUndo bool   `json:"canUndo"`
}

type ViewEvent struct {
	ID      string `json:"id"`
	Exists  bool   `json:"exists"`
	Text    string `json:"text"`
	Error   string `json:"error"`
	CanUndo bool   `json:"canUndo"`
}

var ViewChanged = desktop.Event[ViewEvent]{}

func CutRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max]) + "…"
}

func QuoteAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = fmt.Sprintf("%q", s)
	}
	return out
}

func NormalizeKey(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}
