package portrait

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// DecisionDocument is one edited document in a close preflight.
type DecisionDocument struct {
	ID, Name string
	Untitled bool
}

// DecisionSpec describes the non-native Save / Don't Save / Cancel choice.
// For multiple documents, Selected contains one save checkbox per document;
// unchecked documents are discarded by Save Selected.
type DecisionSpec struct {
	Title, Body                          string
	Documents                            []DecisionDocument
	Selected                             *[]bool
	SaveLabel, DiscardLabel, CancelLabel string
	Return                               *ui.Handle
	OnSave, OnDiscard, OnCancel          func()
	HideDiscard                          bool
}

// Decision presents an application-drawn modal decision without writing or
// discarding anything itself. All disk behavior belongs to the callbacks.
func Decision(c *ui.Context, open *bool, spec DecisionSpec) ui.Element {
	if spec.SaveLabel == "" {
		spec.SaveLabel = "Save"
	}
	if spec.DiscardLabel == "" {
		spec.DiscardLabel = "Don't Save"
	}
	if spec.CancelLabel == "" {
		spec.CancelLabel = "Cancel"
	}
	owner := ui.Box(c.Key("decision:"+spec.Title)).Absolute().Size(0, 0)
	wasOpen := ui.Local(owner, "was open", func() bool { return false })
	returned := ui.Local(owner, "returned", func() bool { return false })
	if *returned {
		if spec.Return != nil {
			spec.Return.CancelFocus(c)
		}
		*returned = false
	}
	var panel ui.Element
	owner.Children(func() {
		panel = DialogShell(c, "decision dialog", open, DialogSpec{Label: spec.Title, Width: Form.DialogWidth, Role: ui.RoleAlertDialog, Glass: true}, func() {
			hasSelection := len(spec.Documents) > 0 && spec.Selected != nil
			checkedCount, uncheckedCount := 0, 0
			if hasSelection {
				if len(*spec.Selected) != len(spec.Documents) {
					*spec.Selected = make([]bool, len(spec.Documents))
					for i := range *spec.Selected {
						(*spec.Selected)[i] = true
					}
				}
				for _, checked := range *spec.Selected {
					if checked {
						checkedCount++
					} else {
						uncheckedCount++
					}
				}
			}
			saveDisabled := hasSelection && checkedCount == 0
			if hasSelection && checkedCount > 0 && uncheckedCount > 0 {
				spec.SaveLabel = fmt.Sprintf("Save %d, Discard %d", checkedCount, uncheckedCount)
			}
			Type.Title.Apply(ui.Text(c, spec.Title)).FontWeight(600).TextColor(Colors.Ink).MinWidth(0).Wrap().AccessHidden(true)
			if spec.Body != "" {
				Type.Small.Apply(ui.Text(c, spec.Body)).TextColor(Colors.Ink2).MinWidth(0).Wrap().AccessHidden(true)
			}
			if len(spec.Documents) > 1 && spec.Selected != nil {
				// A long list scrolls so the action row below always stays on
				// screen, whatever the window's height.
				ui.Scroll(c).MaxHeight(Form.ListMax).FillWidth().Role(ui.RoleGroup).Label("Documents to save").Children(func() {
					ui.Column(c).FillWidth().Gap(Form.Gap).Children(func() {
						for i, document := range spec.Documents {
							Checkbox(c.Key(document.ID), &(*spec.Selected)[i], document.Name)
						}
					})
				})
			}
			if hasSelection && checkedCount > 0 && uncheckedCount > 0 {
				Type.Meta.Apply(ui.Text(c, "Unchecked documents will be discarded.")).TextColor(Colors.Ink2).Wrap()
			}
			row := ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(Form.DialogGap).FocusGroup(ui.Horizontal)
			row.Children(func() {
				if !spec.HideDiscard {
					discard := ButtonWith(c, spec.DiscardLabel, spec.DiscardLabel, ButtonSpec{Look: DangerOutline})
					if c.Shortcut(ui.Cmd, ui.KeyD) {
						*open = false
						if spec.OnDiscard != nil {
							spec.OnDiscard()
						}
					}
					if discard.Clicked() {
						*open = false
						if spec.OnDiscard != nil {
							spec.OnDiscard()
						}
					}
				}
				ui.Spacer(c)
				cancel := ButtonWith(c, spec.CancelLabel, spec.CancelLabel, ButtonSpec{Look: Quiet})
				if c.Shortcut(ui.Cmd, ui.KeyPeriod) {
					*open = false
					if spec.OnCancel != nil {
						spec.OnCancel()
					}
				}
				if cancel.Clicked() {
					*open = false
					if spec.OnCancel != nil {
						spec.OnCancel()
					}
				}
				save := AccentButton(c, spec.SaveLabel, spec.SaveLabel).Disabled(saveDisabled).AutoFocus()
				commitSave := func() {
					if saveDisabled {
						return
					}
					*open = false
					if spec.OnSave != nil {
						spec.OnSave()
					}
				}
				if c.Shortcut(ui.Cmd, ui.KeyS) {
					commitSave()
				}
				if save.Clicked() {
					commitSave()
				}
			})
		})
		if panel.Valid() {
			panel.Padding(Form.DialogPad).Gap(Form.DialogGap).Description(spec.Body)
			Materials.Sheet.Apply(panel)
		}
	})
	if *wasOpen && !*open && spec.Return != nil {
		if target := c.Resolve(*spec.Return); target.Valid() {
			spec.Return.Focus(c)
			*returned = true
		} else {
			spec.Return.CancelFocus(c)
		}
	}
	*wasOpen = *open
	return panel
}

// PromptSpec describes an application-drawn single-field prompt.
type PromptSpec struct {
	Title, Label, Placeholder string
	Value                     *string
	SelectTo                  int
	SubmitLabel, CancelLabel  string
	Validate                  func(string) string
	Return                    *ui.Handle
	OnSubmit                  func()
}

// Prompt shows a field with inline validation. SelectTo is a rune offset used
// to preselect a filename stem while leaving its extension untouched.
func Prompt(c *ui.Context, open *bool, spec PromptSpec) ui.Element {
	if spec.SubmitLabel == "" {
		spec.SubmitLabel = "OK"
	}
	if spec.CancelLabel == "" {
		spec.CancelLabel = "Cancel"
	}
	owner := ui.Box(c.Key("prompt:"+spec.Title)).Absolute().Size(0, 0)
	initialized := ui.Local(owner, "initialized", func() bool { return false })
	touched := ui.Local(owner, "touched", func() bool { return false })
	submitAttempted := ui.Local(owner, "submit attempted", func() bool { return false })
	wasFocused := ui.Local(owner, "was focused", func() bool { return false })
	lastValue := ui.Local(owner, "last value", func() string { return *spec.Value })
	wasOpen := ui.Local(owner, "was open", func() bool { return false })
	returned := ui.Local(owner, "returned", func() bool { return false })
	if *returned {
		if spec.Return != nil {
			spec.Return.CancelFocus(c)
		}
		*returned = false
	}
	if !*open {
		*initialized = false
		*touched = false
		*submitAttempted = false
		*wasFocused = false
		*lastValue = *spec.Value
	}
	var panel ui.Element
	owner.Children(func() {
		panel = DialogShell(c, "prompt dialog", open, DialogSpec{Label: spec.Title, Width: Form.DialogWidth, Role: ui.RoleDialog, Glass: true}, func() {
			Type.Title.Apply(ui.Text(c, spec.Title)).FontWeight(600).TextColor(Colors.Ink).Wrap().AccessHidden(true)
			problem := ""
			if spec.Validate != nil {
				problem = spec.Validate(*spec.Value)
			}
			showProblem := problem != "" && (*touched || *submitAttempted)
			field := TextField(c, spec.Value, FieldSpec{Label: spec.Label, Placeholder: spec.Placeholder, Invalid: showProblem})
			field.Frame().FillWidth()
			field.Input().TextColor(Colors.Ink)
			focused := field.Frame().FocusWithin()
			if *wasFocused && !focused {
				*touched = true
			}
			*wasFocused = focused
			changed := field.Input().Changed()
			if changed {
				*touched = false
				*submitAttempted = false
			} else if *spec.Value != *lastValue {
				// Values supplied by the caller are already committed edits, not
				// keystrokes whose incomplete extension should stay quiet.
				*touched = true
			}
			*lastValue = *spec.Value
			showProblem = problem != "" && (*touched || *submitAttempted)
			if !*initialized {
				field.Input().Focus().SetTextSelection(0, max(0, min(spec.SelectTo, len([]rune(*spec.Value)))))
				*initialized = true
			}
			if showProblem {
				message := Type.Meta.Apply(ui.Text(c, problem)).TextColor(Colors.Danger).Role(ui.RoleAlert).Wrap()
				field.Input().ErrorMessage(message)
			}
			ui.Row(c).Justify(ui.End).Gap(Form.DialogGap).Children(func() {
				if QuietButton(c, spec.CancelLabel, spec.CancelLabel).Clicked() || c.Shortcut(ui.Cmd, ui.KeyPeriod) {
					*open = false
				}
				submitted := field.Input().Submitted()
				if submitted {
					*submitAttempted = true
				}
				if AccentButton(c, spec.SubmitLabel, spec.SubmitLabel).Disabled(problem != "").Clicked() || submitted && problem == "" {
					*open = false
					if spec.OnSubmit != nil {
						spec.OnSubmit()
					}
				}
			})
		})
		if panel.Valid() {
			panel.Padding(Form.DialogPad).Gap(Form.DialogGap)
			Materials.Sheet.Apply(panel)
		}
	})
	if *wasOpen && !*open && spec.Return != nil {
		if target := c.Resolve(*spec.Return); target.Valid() {
			spec.Return.Focus(c)
			*returned = true
		} else {
			spec.Return.CancelFocus(c)
		}
	}
	*wasOpen = *open
	return panel
}
