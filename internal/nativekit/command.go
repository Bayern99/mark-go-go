package nativekit

import "github.com/egoist/mygo/ui"

type CommandSelection struct {
	Index int
	Moved bool
}

func (s *CommandSelection) Reset() {
	s.Index = 0
	s.Moved = false
}

func (s *CommandSelection) Clamp(count int) {
	s.Index = max(0, min(s.Index, count-1))
}

func (s *CommandSelection) Move(key ui.Key, count int) bool {
	if count == 0 {
		return false
	}
	switch key {
	case ui.KeyDown:
		s.Index, s.Moved = (s.Index+1)%count, true
	case ui.KeyUp:
		s.Index, s.Moved = (s.Index+count-1)%count, true
	default:
		return false
	}
	return true
}

func (s *CommandSelection) Shortcuts(input ui.Element, count int) bool {
	s.Clamp(count)
	if count == 0 {
		return false
	}
	if input.Shortcut(0, ui.KeyDown) {
		s.Move(ui.KeyDown, count)
	}
	if input.Shortcut(0, ui.KeyUp) {
		s.Move(ui.KeyUp, count)
	}
	return input.Submitted()
}
