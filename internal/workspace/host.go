package workspace

import (
	"context"

	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/egoist/mygo"
)

type Host struct {
	s *Service
}

func NewHost(s *Service) *Host {
	return &Host{s: s}
}

func (h *Host) Activate(ctx context.Context, documentID string) error {
	return h.s.Activate(ctx, documentID)
}

func (h *Host) ActivateWorkspace(ctx context.Context, workspaceID string) error {
	return h.s.ActivateWorkspace(ctx, workspaceID)
}

func (h *Host) CloseDocument(ctx context.Context, documentID string) (CloseResult, error) {
	return h.s.CloseDocument(ctx, documentID)
}

func (h *Host) CloseWorkspace(ctx context.Context) (CloseResult, error) {
	return h.s.CloseWorkspace(ctx)
}

func (h *Host) CloseWorkspaceRoot(ctx context.Context, workspaceID string) (CloseResult, error) {
	return h.s.CloseWorkspaceRoot(ctx, workspaceID)
}

func (h *Host) Duplicate(ctx context.Context, documentID string) (DocumentInfo, error) {
	return h.s.Duplicate(ctx, documentID)
}

func (h *Host) Flush(ctx context.Context, documentID, text string) error {
	return h.s.Flush(ctx, documentID, text)
}

func (h *Host) FlushQuit(ctx context.Context, documentID, text string, attempt int) error {
	return h.s.FlushQuit(ctx, documentID, text, attempt)
}

func (h *Host) Generate(ctx context.Context, documentID string, req genui.GenerateRequest, progress *mygo.Channel[genui.GenerateProgress]) error {
	return h.s.Generate(ctx, documentID, req, progress)
}

func (h *Host) GetSettings() (Settings, error) {
	return h.s.GetSettings()
}

func (h *Host) Load(ctx context.Context, documentID string) (Snapshot, error) {
	return h.s.Load(ctx, documentID)
}

func (h *Host) NavSnapshot(ctx context.Context) (NavSnapshot, error) {
	return h.s.NavSnapshot(ctx)
}

func (h *Host) New(ctx context.Context) (DocumentInfo, error) {
	return h.s.New(ctx)
}

func (h *Host) Open(ctx context.Context, path string) (DocumentInfo, error) {
	return h.s.Open(ctx, path)
}

func (h *Host) OpenWorkspace(ctx context.Context, root string) error {
	return h.s.OpenWorkspace(ctx, root)
}

func (h *Host) PickFolder(ctx context.Context) (string, error) {
	return h.s.PickFolder(ctx)
}

func (h *Host) PrepareQuit(ctx context.Context, attempt int, failure string) error {
	return h.s.PrepareQuit(ctx, attempt, failure)
}

func (h *Host) RecentWorkspaces(ctx context.Context) ([]RecentWorkspace, error) {
	return h.s.RecentWorkspaces(ctx)
}

func (h *Host) RevealPath(ctx context.Context, path string) error {
	return h.s.RevealPath(ctx, path)
}

func (h *Host) Revert(ctx context.Context, documentID string) error {
	return h.s.Revert(ctx, documentID)
}

func (h *Host) Save(ctx context.Context, documentID string) error {
	return h.s.Save(ctx, documentID)
}

func (h *Host) SaveAs(ctx context.Context, documentID string) error {
	return h.s.SaveAs(ctx, documentID)
}

func (h *Host) SetFilter(ctx context.Context, query string) (NavSearchResult, error) {
	return h.s.SetFilter(ctx, query)
}

func (h *Host) SetNativeEditState(ctx context.Context, documentID string, canUndo, canRedo bool) error {
	return h.s.SetNativeEditState(ctx, documentID, canUndo, canRedo)
}

func (h *Host) SetSettings(p SettingsPatch) (Settings, error) {
	return h.s.SetSettings(p)
}

func (h *Host) SetViewMode(ctx context.Context, documentID, mode string) error {
	return h.s.SetViewMode(ctx, documentID, mode)
}

func (h *Host) Snapshot(ctx context.Context) (WorkspaceSnapshot, error) {
	return h.s.Snapshot(ctx)
}

func (h *Host) ToggleFolder(ctx context.Context, rel string) error {
	return h.s.ToggleFolder(ctx, rel)
}

func (h *Host) UndoView(ctx context.Context, documentID string) error {
	return h.s.UndoView(ctx, documentID)
}

func (h *Host) UnwatchView(ctx context.Context, documentID string) error {
	return h.s.UnwatchView(ctx, documentID)
}

func (h *Host) Update(ctx context.Context, documentID, text string) error {
	return h.s.Update(ctx, documentID, text)
}

func (h *Host) WatchView(ctx context.Context, documentID string) error {
	return h.s.WatchView(ctx, documentID)
}
