package nativefeynman

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/egoist/mygo/ui"
)

func TestDestructiveActionsUseConfirmAndReturnFocus(t *testing.T) {
	tests := []struct {
		name, trigger, title, confirmLabel string
		prepare                            func(*panel)
		draw                               func(*panel, *ui.Context)
	}{
		{
			name: "Delete conversation", trigger: "Delete conversation Older study",
			title: "Delete conversation?", confirmLabel: "Delete",
			prepare: func(p *panel) {
				p.state.sessionRows = []feynman.ResearchConversation{{ID: "older", Title: "Older study"}}
			},
			draw: func(p *panel, c *ui.Context) { p.sessions(c) },
		},
		{
			name: "Clear API key", trigger: "Remove the saved exa API key",
			title: "Clear API key?", confirmLabel: "Clear API key",
			prepare: func(p *panel) {
				p.state.searchStatus = &feynman.CLISearchStatusResult{Parsed: true, SavedAPIKeys: "exa"}
			},
			draw: func(p *panel, c *ui.Context) { p.searchSettings(c) },
		},
		{
			name: "Remove MCP server", trigger: "Remove MCP server docs",
			title: "Remove MCP server?", confirmLabel: "Remove",
			prepare: func(p *panel) {
				p.state.mcpServers = &feynman.CLIMCPListResult{Parsed: true, Servers: []feynman.CLIMCPServer{{Name: "docs", Scope: "project"}}}
			},
			draw: func(p *panel, c *ui.Context) { p.mcpSettings(c) },
		},
	}

	for _, tc := range tests {
		for _, confirmed := range []bool{false, true} {
			outcome := "Escape cancels"
			if confirmed {
				outcome = "Confirm runs"
			}
			t.Run(tc.name+"/"+outcome, func(t *testing.T) {
				p := fixtureReadyPanel()
				tc.prepare(p)
				app := ui.NewTester(func(c *ui.Context) {
					ui.Column(c).Fill().Padding(12).Children(func() { tc.draw(p, c) })
					p.drawConfirmation(c)
				}, 600, 900)
				app.Frame()
				if err := app.Click(tc.trigger); err != nil {
					t.Fatalf("open confirmation: %v", err)
				}
				if !p.confirmationOpen || p.state.cliBusy {
					t.Fatalf("open=%t busy=%t; destructive action did not stop at confirmation", p.confirmationOpen, p.state.cliBusy)
				}
				if _, ok := app.AccessByRole(ui.RoleAlertDialog, tc.title); !ok {
					t.Fatalf("no alert dialog named %q", tc.title)
				}
				confirm, ok := app.AccessByRole(ui.RoleButton, tc.confirmLabel)
				if !ok || !app.Focused("Cancel") || confirm.Focused {
					t.Fatalf("Cancel focused=%t, %s focused=%t", app.Focused("Cancel"), tc.confirmLabel, confirm.Focused)
				}

				calls := 0
				p.confirmationSpec.OnConfirm = func() { calls++ }
				if confirmed {
					if err := app.Click(tc.confirmLabel); err != nil {
						t.Fatalf("confirm action: %v", err)
					}
				} else {
					app.Key(0, ui.KeyEscape)
				}
				app.Frame()
				wantCalls := 0
				if confirmed {
					wantCalls = 1
				}
				if calls != wantCalls {
					t.Fatalf("OnConfirm calls=%d, want %d", calls, wantCalls)
				}
				if p.confirmationOpen || p.state.cliBusy {
					t.Fatalf("closed open=%t busy=%t", p.confirmationOpen, p.state.cliBusy)
				}
				if _, ok := app.AccessByRole(ui.RoleAlertDialog, tc.title); ok {
					t.Fatal("closed confirmation remains accessible")
				}
				if !app.Focused(tc.trigger) {
					t.Fatalf("focus did not return to %q", tc.trigger)
				}
			})
		}
	}
}
