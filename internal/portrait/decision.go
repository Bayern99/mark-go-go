package portrait

import (
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
			Type.Title.Apply(ui.Text(c, spec.Title)).FontWeight(600).TextColor(Colors.Ink).MinWidth(0).Wrap().AccessHidden(true)
			if spec.Body != "" {
				Type.Small.Apply(ui.Text(c, spec.Body)).TextColor(Colors.Ink2).MinWidth(0).Wrap().AccessHidden(true)
			}
			if len(spec.Documents) > 1 && spec.Selected != nil {
				if len(*spec.Selected) != len(spec.Documents) {
					*spec.Selected = make([]bool, len(spec.Documents))
					for i := range *spec.Selected {
						(*spec.Selected)[i] = true
					}
				}
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
				save := AccentButton(c, spec.SaveLabel, spec.SaveLabel).AutoFocus()
				if c.Shortcut(ui.Cmd, ui.KeyS) {
					*open = false
					if spec.OnSave != nil {
						spec.OnSave()
					}
				}
				if save.Clicked() {
					*open = false
					if spec.OnSave != nil {
						spec.OnSave()
					}
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
	}
	var panel ui.Element
	owner.Children(func() {
		panel = DialogShell(c, "prompt dialog", open, DialogSpec{Label: spec.Title, Width: Form.DialogWidth, Role: ui.RoleDialog, Glass: true}, func() {
			Type.Title.Apply(ui.Text(c, spec.Title)).FontWeight(600).TextColor(Colors.Ink).Wrap().AccessHidden(true)
			problem := ""
			if spec.Validate != nil {
				problem = spec.Validate(*spec.Value)
			}
			field := TextField(c, spec.Value, FieldSpec{Label: spec.Label, Placeholder: spec.Placeholder, Invalid: problem != ""})
			field.Frame().FillWidth()
			field.Input().TextColor(Colors.Ink)
			if !*initialized {
				field.Input().Focus().SetTextSelection(0, max(0, min(spec.SelectTo, len([]rune(*spec.Value)))))
				*initialized = true
			}
			if problem != "" {
				Type.Meta.Apply(ui.Text(c, problem)).TextColor(Colors.Danger).Role(ui.RoleAlert).Wrap()
			}
			ui.Row(c).Justify(ui.End).Gap(Form.DialogGap).Children(func() {
				if QuietButton(c, spec.CancelLabel, spec.CancelLabel).Clicked() || c.Shortcut(ui.Cmd, ui.KeyPeriod) {
					*open = false
				}
				if AccentButton(c, spec.SubmitLabel, spec.SubmitLabel).Disabled(problem != "").Clicked() || field.Input().Submitted() && problem == "" {
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
