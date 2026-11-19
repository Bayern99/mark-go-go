package feynman

import (
	"context"

	"github.com/Bayern99/mark-go-go/internal/review"
)

type ResearchHost struct {
	m *ResearchManager
}

func (m *ResearchManager) Host() *ResearchHost {
	return &ResearchHost{m: m}
}

func (h *ResearchHost) Answer(ctx context.Context, root, requestID, optionID string) error {
	return h.m.Answer(ctx, root, requestID, optionID)
}

func (h *ResearchHost) AnswerText(ctx context.Context, root, requestID, text string, cancelled bool) error {
	return h.m.AnswerText(ctx, root, requestID, text, cancelled)
}

func (h *ResearchHost) Artifacts(ctx context.Context, root string) ([]review.ResearchArtifact, error) {
	return h.m.Artifacts(ctx, root)
}

func (h *ResearchHost) Cancel(ctx context.Context, root string) error {
	return h.m.Cancel(ctx, root)
}

func (h *ResearchHost) Connect(ctx context.Context, root string) error {
	return h.m.Connect(ctx, root)
}

func (h *ResearchHost) DeleteSession(ctx context.Context, root, id string) error {
	return h.m.DeleteSession(ctx, root, id)
}

func (h *ResearchHost) Disconnect(ctx context.Context, root string) error {
	return h.m.Disconnect(ctx, root)
}

func (h *ResearchHost) OpenCitation(ctx context.Context, root, address string) error {
	return h.m.OpenCitation(ctx, root, address)
}

func (h *ResearchHost) PickSources(ctx context.Context, root string) ([]string, error) {
	return h.m.PickSources(ctx, root)
}

func (h *ResearchHost) Prompt(ctx context.Context, root, text, documentID, buffer string) (string, error) {
	return h.m.Prompt(ctx, root, text, documentID, buffer)
}

func (h *ResearchHost) PromptWithSources(ctx context.Context, root, text, documentID, buffer string, sources []string) (string, error) {
	return h.m.PromptWithSources(ctx, root, text, documentID, buffer, sources)
}

func (h *ResearchHost) ReadArtifact(ctx context.Context, root, relative string) (string, error) {
	return h.m.ReadArtifact(ctx, root, relative)
}

func (h *ResearchHost) RecoverArtifact(ctx context.Context, root, relative string) (string, error) {
	return h.m.RecoverArtifact(ctx, root, relative)
}

func (h *ResearchHost) RevealOutputs(ctx context.Context, root string) error {
	return h.m.RevealOutputs(ctx, root)
}

func (h *ResearchHost) SelectSession(ctx context.Context, root, id string) error {
	return h.m.SelectSession(ctx, root, id)
}

func (h *ResearchHost) Sessions(ctx context.Context, root string) ([]ResearchConversation, error) {
	return h.m.Sessions(ctx, root)
}

func (h *ResearchHost) SetConfig(ctx context.Context, root, id, value string) error {
	return h.m.SetConfig(ctx, root, id, value)
}

type ResearchConsoleHost struct {
	m *ConsoleManager
}

func (m *ConsoleManager) Host() *ResearchConsoleHost {
	return &ResearchConsoleHost{m: m}
}

func (h *ResearchConsoleHost) Connect(ctx context.Context, root string, args []string) error {
	return h.m.Connect(ctx, root, args)
}

func (h *ResearchConsoleHost) Disconnect(ctx context.Context, root string) error {
	return h.m.Disconnect(ctx, root)
}

func (h *ResearchConsoleHost) Input(ctx context.Context, root, data string) error {
	return h.m.Input(ctx, root, data)
}

func (h *ResearchConsoleHost) Resize(ctx context.Context, root string, cols, rows int) error {
	return h.m.Resize(ctx, root, cols, rows)
}

func (h *ResearchConsoleHost) Snapshot(ctx context.Context, root string) (ResearchConsoleSnapshot, error) {
	return h.m.Snapshot(ctx, root)
}
