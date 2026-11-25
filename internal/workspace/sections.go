package workspace

import (
	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/markdown"
)

type Section = markdown.Section

type Note = markdown.Note

func NormalizeHeadingText(text string) string {
	return markdown.NormalizeHeadingText(text)
}

func ExtractSections(text string) []Section {
	return markdown.ExtractSections(text)
}

func ExtractNotes(text string) []Note {
	return markdown.ExtractNotes(text)
}

func GenerationSections(text string) []genui.SectionRef {
	sections := ExtractSections(text)
	notes := ExtractNotes(text)
	refs := make([]genui.SectionRef, 0, len(sections))
	for _, s := range sections {
		ref := genui.SectionRef{Heading: s.Heading, Occurrence: s.Occurrence, Start: s.Start, End: s.End}
		for _, n := range notes {
			if n.Start >= s.Start && n.End <= s.End {
				ref.Exclude = append(ref.Exclude, genui.TextRange{Start: n.ContentStart(), End: n.End})
			}
		}
		refs = append(refs, ref)
	}
	return refs
}
