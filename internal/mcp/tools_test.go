package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/Bayern99/mark-go-go/internal/workspace"
)

func callTool(t *testing.T, urlStr, token, sid, name string, args map[string]any) mcpCallToolResult {
	t.Helper()
	resp := callMCPJSON(t, urlStr, token, sid, "tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	if resp.Error != nil {
		t.Fatalf("tools/call(%s) protocol error: %+v", name, resp.Error)
	}
	raw, _ := json.Marshal(resp.Result)
	var out mcpCallToolResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode tools/call(%s): %v", name, err)
	}
	return out
}

func TestConcurrentMCPCheckpointCallsReturnTheirOwnSnapshot(t *testing.T) {
	root := fsutil.CanonicalPath(t.TempDir())
	if out, err := exec.Command("git", "init", "--quiet", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %s: %v", out, err)
	}
	if err := os.WriteFile(filepath.Join(root, "paper.md"), []byte("# Paper\n"), 0600); err != nil {
		t.Fatal(err)
	}
	srv := NewServer(Dependencies{Workspace: &testWorkspaceHarness{svc: workspace.NewService(), root: root}})
	t.Cleanup(func() { _ = srv.Close() })
	token, sid := initTestSession(t, srv, root, review.WorkDir(t.TempDir(), root))
	start := make(chan struct{})
	results := make(chan review.Checkpoint, 8)
	var workers sync.WaitGroup
	for i := range 8 {
		workers.Go(func() {
			<-start
			label := fmt.Sprintf("Concurrent snapshot %d", i)
			result := callTool(t, srv.URL(), token, sid, "create_checkpoint", map[string]any{"path": "paper.md", "label": label})
			if result.IsError || len(result.Content) == 0 {
				t.Errorf("create %q failed: %+v", label, result)
				return
			}
			var reply struct{ Checkpoint review.Checkpoint }
			if err := json.Unmarshal([]byte(result.Content[0].Text), &reply); err != nil {
				t.Error(err)
				return
			}
			if reply.Checkpoint.Label != label {
				t.Errorf("request %q received %q", label, reply.Checkpoint.Label)
			}
			results <- reply.Checkpoint
		})
	}
	close(start)
	workers.Wait()
	close(results)
	ids := map[string]bool{}
	for cp := range results {
		ids[cp.ID] = true
	}
	if len(ids) != 8 || ids[""] {
		t.Errorf("expected eight unique Checkpoint IDs, got %v", ids)
	}
}

func TestMCPToolsEndToEnd(t *testing.T) {
	root := fsutil.CanonicalPath(t.TempDir())
	userData := t.TempDir()
	work := review.WorkDir(userData, root)

	livePath := filepath.Join(root, "paper.md")
	initialText := "# Paper\n\n## Intro\n\nFirst paragraph.\n\n## Results\n\nOld numbers.\n"
	if err := os.WriteFile(livePath, []byte(initialText), 0o644); err != nil {
		t.Fatal(err)
	}

	unsavedText := "# Paper\n\n## Intro\n\nLive unsaved paragraph.\n\n## Results\n\nOld numbers.\n"
	wsSvc := workspace.NewService()
	harness := &testWorkspaceHarness{
		svc:      wsSvc,
		root:     root,
		activeID: "doc-paper",
		openDocs: map[string]workspace.DocumentMCPState{
			"doc-paper": {
				ID:      "doc-paper",
				Name:    "paper.md",
				Path:    livePath,
				RelPath: "paper.md",
				Open:    true,
				Edited:  true,
				Version: 3,
				Text:    unsavedText,
			},
		},
	}

	revSvc := review.NewService()
	var reviewEvents []review.ReviewUpdateEvent
	var revMu sync.Mutex
	revSvc.SetEventSink(func(ev review.ReviewUpdateEvent) {
		revMu.Lock()
		reviewEvents = append(reviewEvents, ev)
		revMu.Unlock()
	})

	srv := NewServer(Dependencies{Workspace: harness, Review: revSvc})
	defer func() { _ = srv.Close() }()
	token, sid := initTestSession(t, srv, root, work)

	// 0. tools/list lists all 5 required tools.
	toolsResp := callMCPJSON(t, srv.URL(), token, sid, "tools/list", nil)
	if toolsResp.Error != nil {
		t.Fatalf("tools/list error: %+v", toolsResp.Error)
	}
	rawTools, _ := json.Marshal(toolsResp.Result)
	var listedTools struct {
		Tools []mcpTool `json:"tools"`
	}
	_ = json.Unmarshal(rawTools, &listedTools)
	var toolNames []string
	for _, tl := range listedTools.Tools {
		toolNames = append(toolNames, tl.Name)
	}
	for _, want := range []string{"propose_edit", "create_document", "open_document", "write_genui_view", "create_checkpoint"} {
		if !slices.Contains(toolNames, want) {
			t.Fatalf("tools/list missing %q in %v", want, toolNames)
		}
	}

	// 1. propose_edit queues a Change proposal without mutating open buffer or disk file.
	proposeRes := callTool(t, srv.URL(), token, sid, "propose_edit", map[string]any{
		"documentId": "doc-paper",
		"oldText":    "Old numbers.",
		"newText":    "Updated numbers from experiment.",
		"summary":    "Update results section",
	})
	if proposeRes.IsError {
		t.Fatalf("propose_edit failed: %+v", proposeRes.Content)
	}
	proposals := revSvc.ListProposals(root)
	if len(proposals) != 1 || proposals[0].Source != "mcp" || len(proposals[0].Hunks) != 1 {
		t.Fatalf("unexpected queued proposals: %+v", proposals)
	}
	if proposals[0].BaseText != unsavedText {
		t.Fatalf("proposal BaseText did not use live unsaved buffer")
	}
	diskAfterPropose, _ := os.ReadFile(livePath)
	if string(diskAfterPropose) != initialText {
		t.Fatalf("propose_edit silently overwrote disk file!")
	}
	if harness.openDocs["doc-paper"].Text != unsavedText {
		t.Fatalf("propose_edit silently overwrote open document buffer!")
	}
	revMu.Lock()
	eventCount := len(reviewEvents)
	revMu.Unlock()
	if eventCount == 0 {
		t.Fatal("expected ReviewUpdateEvent to be emitted")
	}
	t.Run("stale line range", func(t *testing.T) {
		result := callTool(t, srv.URL(), token, sid, "propose_edit", map[string]any{
			"documentId": "doc-paper", "baseVersion": 2,
			"startLine": 7, "endLine": 7, "newText": "Replacement from older line numbers.",
		})
		if !result.IsError || len(revSvc.ListProposals(root)) != 1 {
			t.Errorf("stale line range accepted: %+v", result)
		}
	})
	t.Run("unopened document rejects a stale disk version", func(t *testing.T) {
		const rel = "unopened.md"
		path := filepath.Join(root, rel)
		const before = "# Draft\n\nTarget passage.\n"
		const after = "# Draft\n\nInserted line.\nTarget passage.\n"
		if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		read := func() workspace.DocumentMCPState {
			t.Helper()
			res := callMCPJSON(t, srv.URL(), token, sid, "resources/read", map[string]any{"uri": "markgogo://document/" + rel})
			if res.Error != nil {
				t.Fatalf("read disk document: %+v", res.Error)
			}
			var payload struct {
				Contents []mcpTextResourceContents
			}
			raw, err := json.Marshal(res.Result)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, &payload); err != nil || len(payload.Contents) != 1 {
				t.Fatalf("unexpected resource payload: %s (%v)", raw, err)
			}
			var doc workspace.DocumentMCPState
			if err := json.Unmarshal([]byte(payload.Contents[0].Text), &doc); err != nil || doc.Open {
				t.Fatalf("expected unopened document: %+v (%v)", doc, err)
			}
			return doc
		}
		oldDoc := read()
		if err := os.WriteFile(path, []byte(after), 0o644); err != nil {
			t.Fatal(err)
		}
		got := callTool(t, srv.URL(), token, sid, "propose_edit", map[string]any{
			"path": rel, "baseVersion": oldDoc.Version, "startLine": 3, "endLine": 3, "newText": "Updated passage.",
		})
		if !got.IsError || !strings.Contains(got.Content[0].Text, "document version changed") {
			t.Fatalf("stale disk range accepted (version %d): %+v", oldDoc.Version, got)
		}
		current := read()
		if current.Version == 0 || current.Version == oldDoc.Version || current.Version != read().Version {
			t.Fatalf("disk versions are not stable by content: old=%d, current=%d", oldDoc.Version, current.Version)
		}
		fresh := callTool(t, srv.URL(), token, sid, "propose_edit", map[string]any{
			"path": rel, "baseVersion": current.Version, "startLine": 4, "endLine": 4, "newText": "Updated passage.",
		})
		if fresh.IsError {
			t.Fatalf("fresh disk range rejected: %+v", fresh)
		}
	})

	// 2. create_document uses O_EXCL and refuses to overwrite existing files (even under concurrency).
	createRes := callTool(t, srv.URL(), token, sid, "create_document", map[string]any{
		"path":    "outputs/summary.md",
		"content": "# Summary\n\nGenerated findings.\n",
	})
	if createRes.IsError {
		t.Fatalf("create_document failed: %+v", createRes.Content)
	}
	createdBytes, err := os.ReadFile(filepath.Join(root, "outputs", "summary.md"))
	if err != nil || string(createdBytes) != "# Summary\n\nGenerated findings.\n" {
		t.Fatalf("created file mismatch: %q, err=%v", createdBytes, err)
	}
	dupRes := callTool(t, srv.URL(), token, sid, "create_document", map[string]any{
		"path":    "outputs/summary.md",
		"content": "# Overwritten\n",
	})
	if !dupRes.IsError {
		t.Fatalf("expected create_document to refuse overwriting existing file")
	}
	stillBytes, _ := os.ReadFile(filepath.Join(root, "outputs", "summary.md"))
	if string(stillBytes) != "# Summary\n\nGenerated findings.\n" {
		t.Fatalf("existing file was overwritten: %q", stillBytes)
	}
	t.Run("MCP document recovery", func(t *testing.T) {
		files, err := review.CollectArtifacts(work, root)
		if err != nil || len(files) != 1 || files[0].Path != "outputs/summary.md" || files[0].Missing {
			t.Errorf("Review artifacts did not include the created document: %+v (%v)", files, err)
		}
		if err := os.Remove(filepath.Join(root, "outputs", "summary.md")); err != nil {
			t.Fatal(err)
		}
		files, err = review.CollectArtifacts(work, root)
		if err != nil || len(files) != 1 || !files[0].Missing {
			t.Errorf("Review artifacts did not retain the missing document: %+v (%v)", files, err)
		}
		recovered, err := review.RecoverArtifact(work, root, "outputs/summary.md")
		if err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(recovered)
		if err != nil || string(content) != string(createdBytes) {
			t.Errorf("recovered document = %q, %v", content, err)
		}
	})
	badCreate := callTool(t, srv.URL(), token, sid, "create_document", map[string]any{
		"path":    "../escape.md",
		"content": "# Bad\n",
	})
	if !badCreate.IsError {
		t.Fatal("expected path traversal create_document to fail")
	}

	// Concurrent O_EXCL race test: 8 callers race to create the same new path; exactly 1 succeeds.
	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res := callTool(t, srv.URL(), token, sid, "create_document", map[string]any{
				"path":    "outputs/race.md",
				"content": fmt.Sprintf("# Race %d\n", i),
			})
			if !res.IsError {
				wins.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("concurrent create_document wins = %d, want 1", wins.Load())
	}

	// 3. open_document focuses document and optional heading.
	openRes := callTool(t, srv.URL(), token, sid, "open_document", map[string]any{
		"path":    "outputs/summary.md",
		"heading": "Summary",
	})
	if openRes.IsError {
		t.Fatalf("open_document failed: %+v", openRes.Content)
	}
	if len(harness.opened) == 0 || !harness.opened[len(harness.opened)-1].HasReveal || harness.opened[len(harness.opened)-1].RevealOffset != 0 {
		t.Fatalf("expected open_document to record reveal offset 0, got %+v", harness.opened)
	}
	badHeadingOpen := callTool(t, srv.URL(), token, sid, "open_document", map[string]any{
		"path":    "outputs/summary.md",
		"heading": "Does Not Exist",
	})
	if !badHeadingOpen.IsError {
		t.Fatal("expected open_document with nonexistent heading to fail")
	}

	// 4. write_genui_view validates via pure-Go headless validator and atomically writes .genui.json.
	invalidView := callTool(t, srv.URL(), token, sid, "write_genui_view", map[string]any{
		"path": "outputs/summary.md",
		"a2ui": []map[string]any{
			{
				"version": genui.TargetA2UIVersion,
				"createSurface": map[string]any{
					"surfaceId": "s1",
					"catalogId": genui.CatalogID,
					"components": []map[string]any{
						{"id": "root", "component": "UnknownWidget"},
					},
				},
			},
		},
		"sources": map[string]any{
			"s": map[string]any{"heading": "Summary", "occurrence": 1},
		},
	})
	if !invalidView.IsError {
		t.Fatal("expected write_genui_view to reject invalid catalog component")
	}
	viewPath := genui.ViewPath(filepath.Join(root, "outputs", "summary.md"))
	if _, err := os.Stat(viewPath); !os.IsNotExist(err) {
		t.Fatalf("invalid GenUI view must not be written to %s", viewPath)
	}

	validView := callTool(t, srv.URL(), token, sid, "write_genui_view", map[string]any{
		"path":    "outputs/summary.md",
		"request": "Summarize findings",
		"sources": map[string]any{
			"summary": map[string]any{"heading": "Summary", "occurrence": 1},
		},
		"a2ui": []map[string]any{
			{
				"version": genui.TargetA2UIVersion,
				"createSurface": map[string]any{
					"surfaceId": "summary-view",
					"catalogId": genui.CatalogID,
					"components": []map[string]any{
						{
							"id":        "root",
							"component": "Column",
							"children":  []string{"kp"},
						},
						{
							"id":        "kp",
							"component": "KeyPoints",
							"items":     []string{"Generated findings."},
						},
					},
				},
			},
		},
	})
	if validView.IsError {
		t.Fatalf("valid write_genui_view failed: %+v", validView.Content)
	}
	snap := genui.ReadView(viewPath)
	if !snap.Exists || snap.Error != "" {
		t.Fatalf("unexpected written GenUI view snapshot: %+v", snap)
	}
	if err := genui.ValidateGeneratedView([]byte(snap.Text)); err != nil {
		t.Fatalf("written GenUI view failed validation: %v", err)
	}
	var writtenView struct {
		Sources map[string]struct {
			Hash string `json:"hash"`
		} `json:"sources"`
	}
	if err := json.Unmarshal([]byte(snap.Text), &writtenView); err != nil {
		t.Fatal(err)
	}
	summarySecs := workspace.ExtractSections("# Summary\n\nGenerated findings.\n")
	if len(summarySecs) != 1 || writtenView.Sources["summary"].Hash != summarySecs[0].Hash {
		t.Fatalf("stamped source hash = %q, want %q", writtenView.Sources["summary"].Hash, summarySecs[0].Hash)
	}

	// Second write_genui_view creates .prev.json for one-step undo.
	secondView := callTool(t, srv.URL(), token, sid, "write_genui_view", map[string]any{
		"path":    "outputs/summary.md",
		"request": "Summarize findings v2",
		"sources": map[string]any{
			"summary": map[string]any{"heading": "Summary", "occurrence": 1},
		},
		"a2ui": []map[string]any{
			{
				"version": genui.TargetA2UIVersion,
				"createSurface": map[string]any{
					"surfaceId": "summary-view",
					"catalogId": genui.CatalogID,
					"components": []map[string]any{
						{"id": "root", "component": "Column", "children": []string{"kp"}},
						{"id": "kp", "component": "KeyPoints", "items": []string{"Updated findings."}},
					},
				},
			},
		},
	})
	if secondView.IsError {
		t.Fatalf("second write_genui_view failed: %+v", secondView.Content)
	}
	if !genui.ReadView(viewPath).CanUndo {
		t.Fatal("expected CanUndo=true after second write_genui_view")
	}

	// 5. create_checkpoint records labeled provenance checkpoint (refusing unsaved open doc, succeeding on saved doc).
	unsavedCP := callTool(t, srv.URL(), token, sid, "create_checkpoint", map[string]any{
		"documentId": "doc-paper",
		"label":      "Should refuse unsaved",
	})
	if !unsavedCP.IsError {
		t.Fatal("expected create_checkpoint on unsaved edited document to fail")
	}

	cpRes := callTool(t, srv.URL(), token, sid, "create_checkpoint", map[string]any{
		"path":  "outputs/summary.md",
		"label": "Initial research summary",
	})
	if cpRes.IsError {
		t.Fatalf("create_checkpoint failed: %+v", cpRes.Content)
	}
	cps, err := review.ListCheckpoints(context.Background(), root, filepath.Join(root, "outputs", "summary.md"), wsSvc.UserDataDir())
	if err != nil {
		t.Fatalf("ListCheckpoints failed: %v", err)
	}
	if len(cps) != 1 || cps[0].Label != "Initial research summary" || cps[0].Source != "mcp" {
		t.Fatalf("unexpected checkpoints: %+v", cps)
	}

	// Verify ResearchManager hook wiring compiles and registers cleanly.
	rm := feynman.NewResearchManager(nil)
	rm.SetBeforeStart(srv.BeforeResearchStart)
	rm.SetAfterStop(srv.AfterResearchStop)
	rm.Close()
}
