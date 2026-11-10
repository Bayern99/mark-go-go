//go:build readbench

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// ReadBench records only explicitly selected runs in a private directory.
// The production Generate/ACP/commit path remains the one under observation.
func configureReadBench(h *documentHost) {
	root := os.Getenv("MARKGOGO_READBENCH_DIR")
	if root == "" {
		return
	}
	if !filepath.IsAbs(root) {
		log.Fatal("MARKGOGO_READBENCH_DIR must be absolute")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		log.Fatal(err)
	}
	h.prepareJob = func(job *generationJob) {
		var active struct {
			RunID        string `json:"runId"`
			TestID       string `json:"testId"`
			Condition    string `json:"condition"`
			ThoughtLevel string `json:"thoughtLevel"`
			Strategy     string `json:"strategy"`
		}
		data, err := os.ReadFile(filepath.Join(root, "active.json"))
		if err != nil {
			log.Printf("ReadBench requires active.json: %v", err)
			return
		}
		if err := json.Unmarshal(data, &active); err != nil || !readBenchRunID.MatchString(active.RunID) || active.TestID == "" {
			log.Print("ReadBench active.json has no valid runId/testId")
			return
		}
		runDir := filepath.Join(root, "runs", active.RunID)
		if err := os.MkdirAll(runDir, 0o700); err != nil {
			log.Printf("ReadBench directory: %v", err)
			return
		}
		file, err := os.OpenFile(filepath.Join(runDir, "app.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			log.Printf("ReadBench trace: %v", err)
			return
		}
		started := time.Now()
		var mu sync.Mutex
		var traceErr error
		closed := false
		observe := func(event string, payload any) {
			mu.Lock()
			defer mu.Unlock()
			if closed || traceErr != nil {
				return
			}
			raw, err := json.Marshal(payload)
			if err != nil {
				traceErr = err
				_ = file.Close()
				closed = true
				return
			}
			var value any
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.UseNumber()
			if err := decoder.Decode(&value); err != nil {
				traceErr = err
				_ = file.Close()
				closed = true
				return
			}
			redacted := readBenchRedact(value)
			sum := sha256.Sum256(raw)
			record := map[string]any{
				"runId": active.RunID, "testId": active.TestID, "condition": active.Condition,
				"event": event, "at": time.Now().UTC().Format(time.RFC3339Nano),
				"elapsedMs": float64(time.Since(started).Microseconds()) / 1000,
				"data":      value, "rawSha256": hex.EncodeToString(sum[:]), "redacted": redacted,
			}
			if err := json.NewEncoder(file).Encode(record); err != nil {
				traceErr = err
			}
			if event == "run.end" || traceErr != nil {
				if err := file.Sync(); traceErr == nil {
					traceErr = err
				}
				if err := file.Close(); traceErr == nil {
					traceErr = err
				}
				closed = true
			}
		}
		job.Observe = observe
		job.ThoughtLevel = active.ThoughtLevel
		switch active.Strategy {
		case "", "full":
		case "jev-controlled":
			job.Prepare = func(ctx context.Context, req GenerateRequest, dir string) (string, func([]byte) ([]byte, error), error) {
				if req.Mode != "full" {
					return "", nil, fmt.Errorf("jev-controlled experiment supports full generation only")
				}
				return prepareControlledReading(ctx, req, dir, observe)
			}
		default:
			job.Prepare = func(context.Context, GenerateRequest, string) (string, func([]byte) ([]byte, error), error) {
				return "", nil, fmt.Errorf("unknown ReadBench strategy %q", active.Strategy)
			}
		}
		beforeWrite := job.BeforeWrite
		job.BeforeWrite = func() error {
			mu.Lock()
			err := traceErr
			mu.Unlock()
			if err != nil {
				return fmt.Errorf("ReadBench trace was not recorded: %w", err)
			}
			if beforeWrite != nil {
				return beforeWrite()
			}
			return nil
		}
		info := map[string]any{"pid": os.Getpid(), "argv": os.Args}
		if exe, err := os.Executable(); err == nil {
			info["executable"] = exe
			if f, err := os.Open(exe); err == nil {
				hash := sha256.New()
				if _, err := io.Copy(hash, f); err == nil {
					info["executableSha256"] = hex.EncodeToString(hash.Sum(nil))
				}
				_ = f.Close()
			}
		}
		if build, ok := debug.ReadBuildInfo(); ok {
			info["goVersion"] = build.GoVersion
			var vcs []debug.BuildSetting
			for _, setting := range build.Settings {
				if strings.HasPrefix(setting.Key, "vcs.") {
					vcs = append(vcs, setting)
				}
			}
			info["vcs"] = vcs
		}
		observe("app", info)
	}
}

var readBenchRunID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,150}$`)

// Preserve protocol structure and redact credential fields, never the whole
// environment. Benchmark inputs are restricted to the public corpus.
func readBenchRedact(value any) bool {
	changed := false
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			name := strings.ToLower(strings.ReplaceAll(key, "_", ""))
			switch name {
			case "authorization", "password", "apikey", "accesstoken", "refreshtoken", "idtoken", "clientsecret", "cookie", "set-cookie", "credentials":
				v[key] = "[REDACTED]"
				changed = true
			default:
				if readBenchRedact(child) {
					changed = true
				}
			}
		}
	case []any:
		for _, child := range v {
			if readBenchRedact(child) {
				changed = true
			}
		}
	}
	return changed
}
