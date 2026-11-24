package nativeui

import (
	"strings"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

// SettingsPage is one registered Settings Center destination.
type SettingsPage struct {
	ID, Section, Label string
	Icon               portrait.Icon
	Order              int
	Visible            func() bool
	Keywords           []string
	Draw               func(c *ui.Context, q SettingsQuery)
	Reset              func()
	Dirty              func() bool
}

type settingsSearchState struct{ matches int }

type SettingsQuery struct {
	Text  string
	state *settingsSearchState
}

func (q SettingsQuery) Active() bool { return strings.TrimSpace(q.Text) != "" }

func (q SettingsQuery) Match(title, description string, keywords ...string) bool {
	if !q.Active() {
		return true
	}
	query := strings.ToLower(strings.TrimSpace(q.Text))
	fields := append([]string{title, description}, keywords...)
	for _, field := range fields {
		for _, word := range strings.Fields(query) {
			if !strings.Contains(strings.ToLower(field), word) {
				continue
			}
			if q.state != nil {
				q.state.matches++
			}
			return true
		}
	}
	return false
}

func (q SettingsQuery) PageMatch(page SettingsPage) bool {
	return q.Match(page.Label, page.Section, page.Keywords...)
}

func settingsPages(slots Slots) []SettingsPage {
	return slots.Settings
}

func mergeSettingsPages(pages []SettingsPage) []SettingsPage {
	merged := make([]SettingsPage, 0, len(pages))
	for _, page := range pages {
		if page.ID == "" || page.Draw == nil {
			continue
		}
		found := -1
		for i := range merged {
			if merged[i].ID == page.ID {
				found = i
				break
			}
		}
		if found < 0 {
			merged = append(merged, page)
			continue
		}
		previous := merged[found]
		first, second := previous.Draw, page.Draw
		previous.Draw = func(c *ui.Context, q SettingsQuery) {
			first(c, q)
			second(c, q)
		}
		if previous.Reset == nil {
			previous.Reset = page.Reset
		}
		if previous.Dirty == nil {
			previous.Dirty = page.Dirty
		}
		if previous.Visible == nil {
			previous.Visible = page.Visible
		}
		previous.Keywords = append(previous.Keywords, page.Keywords...)
		merged[found] = previous
	}
	return merged
}
