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

func (h *ResearchHost) Abort(ctx context.Context, root string) error {
	return h.m.Abort(ctx, root)
}

func (h *ResearchHost) Answer(ctx context.Context, root, requestID, optionID string) error {
	return h.m.Answer(ctx, root, requestID, optionID)
}

func (h *ResearchHost) AnswerConfirm(ctx context.Context, root, requestID string, confirmed, cancelled bool) error {
	return h.m.AnswerConfirm(ctx, root, requestID, confirmed, cancelled)
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

func (h *ResearchHost) ClearQueue(ctx context.Context, root string) (ResearchQueueState, error) {
	return h.m.ClearQueue(ctx, root)
}

func (h *ResearchHost) Clone(ctx context.Context, root string) (bool, error) {
	return h.m.Clone(ctx, root)
}

func (h *ResearchHost) Commands(ctx context.Context, root string) ([]ResearchCommand, error) {
	return h.m.Commands(ctx, root)
}

func (h *ResearchHost) Compact(ctx context.Context, root, customInstructions string) (ResearchCompactionResult, error) {
	return h.m.Compact(ctx, root, customInstructions)
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

func (h *ResearchHost) FollowUp(ctx context.Context, root, text string, sources []string) error {
	return h.m.FollowUp(ctx, root, text, sources)
}

func (h *ResearchHost) Fork(ctx context.Context, root, entryID string) (ResearchForkResult, error) {
	return h.m.Fork(ctx, root, entryID)
}

func (h *ResearchHost) ForkMessages(ctx context.Context, root string) ([]ResearchForkMessage, error) {
	return h.m.ForkMessages(ctx, root)
}

func (h *ResearchHost) NewSession(ctx context.Context, root, parentSession string) error {
	return h.m.NewSession(ctx, root, parentSession)
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

func (h *ResearchHost) SessionStats(ctx context.Context, root string) (ResearchSessionStats, error) {
	return h.m.SessionStats(ctx, root)
}

func (h *ResearchHost) Sessions(ctx context.Context, root string) ([]ResearchConversation, error) {
	return h.m.Sessions(ctx, root)
}

func (h *ResearchHost) SetConfig(ctx context.Context, root, id, value string) error {
	return h.m.SetConfig(ctx, root, id, value)
}

func (h *ResearchHost) SetModel(ctx context.Context, root, provider, modelID string) (ResearchModel, error) {
	return h.m.SetModel(ctx, root, provider, modelID)
}

func (h *ResearchHost) SetSessionName(ctx context.Context, root, name string) error {
	return h.m.SetSessionName(ctx, root, name)
}

func (h *ResearchHost) SetThinkingLevel(ctx context.Context, root, level string) error {
	return h.m.SetThinkingLevel(ctx, root, level)
}

func (h *ResearchHost) State(ctx context.Context, root string) (ResearchSessionState, error) {
	return h.m.State(ctx, root)
}

func (h *ResearchHost) Steer(ctx context.Context, root, text string, sources []string) error {
	return h.m.Steer(ctx, root, text, sources)
}

func (h *ResearchHost) SwitchSession(ctx context.Context, root, target string) error {
	return h.m.SwitchSession(ctx, root, target)
}

func (h *ResearchHost) Tree(ctx context.Context, root string) (ResearchTree, error) {
	return h.m.Tree(ctx, root)
}

func (h *ResearchHost) CLIAlphaAnnotate(ctx context.Context, root, action, paperID, note string) (CLIAlphaAnnotateResult, error) {
	return h.m.CLIAlphaAnnotate(ctx, root, action, paperID, note)
}

func (h *ResearchHost) CLIAlphaAsk(ctx context.Context, root, paperID, question string) (CLICommandResult, error) {
	return h.m.CLIAlphaAsk(ctx, root, paperID, question)
}

func (h *ResearchHost) CLIAlphaCode(ctx context.Context, root, repoURL, filePath string) (CLICommandResult, error) {
	return h.m.CLIAlphaCode(ctx, root, repoURL, filePath)
}

func (h *ResearchHost) CLIAlphaGet(ctx context.Context, root, paperID string, fullText bool) (CLIAlphaPaperResult, error) {
	return h.m.CLIAlphaGet(ctx, root, paperID, fullText)
}

func (h *ResearchHost) CLIAlphaLogout(ctx context.Context, root string) (CLICommandResult, error) {
	return h.m.CLIAlphaLogout(ctx, root)
}

func (h *ResearchHost) CLIAlphaSearch(ctx context.Context, root, query, mode string) (CLIAlphaSearchResult, error) {
	return h.m.CLIAlphaSearch(ctx, root, query, mode)
}

func (h *ResearchHost) CLIAlphaStatus(ctx context.Context, root string) (CLIAlphaStatusResult, error) {
	return h.m.CLIAlphaStatus(ctx, root)
}

func (h *ResearchHost) CLIDoctor(ctx context.Context, root string) (CLIDoctorResult, error) {
	return h.m.CLIDoctor(ctx, root)
}

func (h *ResearchHost) CLIMCPAdd(ctx context.Context, root string, input CLIMCPAddInput) (CLICommandResult, error) {
	return h.m.CLIMCPAdd(ctx, root, input)
}

func (h *ResearchHost) CLIMCPList(ctx context.Context, root string) (CLIMCPListResult, error) {
	return h.m.CLIMCPList(ctx, root)
}

func (h *ResearchHost) CLIMCPRemove(ctx context.Context, root, name string, local bool) (CLICommandResult, error) {
	return h.m.CLIMCPRemove(ctx, root, name, local)
}

func (h *ResearchHost) CLIModelAction(ctx context.Context, root, action, value string) (CLICommandResult, error) {
	return h.m.CLIModelAction(ctx, root, action, value)
}

func (h *ResearchHost) CLIModelList(ctx context.Context, root string) (CLIModelListResult, error) {
	return h.m.CLIModelList(ctx, root)
}

func (h *ResearchHost) CLIPackageAction(ctx context.Context, root, action, target string) (CLICommandResult, error) {
	return h.m.CLIPackageAction(ctx, root, action, target)
}

func (h *ResearchHost) CLIPackagesList(ctx context.Context, root string) (CLIPackageListResult, error) {
	return h.m.CLIPackagesList(ctx, root)
}

func (h *ResearchHost) CLISearchAction(ctx context.Context, root, action, provider, apiKey string) (CLICommandResult, error) {
	return h.m.CLISearchAction(ctx, root, action, provider, apiKey)
}

func (h *ResearchHost) CLISearchStatus(ctx context.Context, root string) (CLISearchStatusResult, error) {
	return h.m.CLISearchStatus(ctx, root)
}

func (h *ResearchHost) CLISetupPreview(ctx context.Context, root string) (CLICommandResult, error) {
	return h.m.CLISetupPreview(ctx, root)
}

func (h *ResearchHost) CLIStatus(ctx context.Context, root string) (CLIStatusResult, error) {
	return h.m.CLIStatus(ctx, root)
}

func (h *ResearchHost) LaunchTerminalWizard(ctx context.Context, root, wizard, target string) (CLITerminalLaunchResult, error) {
	return h.m.LaunchTerminalWizard(ctx, root, wizard, target)
}

func (h *ResearchHost) RefreshModels(ctx context.Context, root string) ([]ResearchModel, error) {
	return h.m.RefreshModels(ctx, root)
}

func (h *ResearchHost) WatchFocus(ctx context.Context, root string) error {
	return h.m.WatchFocus(ctx, root)
}
