package nativegenui

import (
	"strings"

	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/genuikit"
	"github.com/Bayern99/mark-go-go/internal/openuilang"
)

var streamingSurface = []string{"streaming"}

type statementStream struct {
	library *openuilang.Library
	stream  *openuilang.Stream
	text    string
}

func newStatementStream() (*statementStream, error) {
	library, err := genuikit.Library()
	if err != nil {
		return nil, err
	}
	return &statementStream{library: library, stream: library.NewStream()}, nil
}

func (s *statementStream) Push(preview genui.StatementPreview) (*Surface, error) {
	text := strings.Join(preview.Statements, "\n")
	if text != "" {
		text += "\n"
	}
	if !strings.HasPrefix(text, s.text) {
		s.stream = s.library.NewStream()
		s.text = ""
	}
	chunk := strings.TrimPrefix(text, s.text)
	program := s.stream.Push(chunk)
	s.text = text
	return newStatementSurface(program, text, preview.Sources, "", streamingSurface, false)
}
