package feynman

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/review"
	"io"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

func (h *ResearchManager) PickSources(ctx context.Context, root string) ([]string, error) {
	w, err := h.workspace(ctx, root)
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	session, err := h.activeLocked(ctx, root)
	h.mu.Unlock()
	if err != nil {
		return nil, err
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent: w.Window, Title: "Choose research sources", ButtonLabel: "Attach", DefaultPath: w.Root, Multiple: true,
	})
	if err != nil {
		return nil, err
	}
	files := make([]string, len(paths))
	for i, path := range paths {
		files[i] = fsutil.CanonicalPath(path)
	}
	approved, err := selectedResearchFiles(files)
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	current, err := h.activeLocked(ctx, root)
	if err != nil || current != session {
		return nil, errors.New("The research connection changed. Reattach your sources.")
	}
	if session.sources == nil {
		session.sources = make(map[string]os.FileInfo)
	}
	maps.Copy(session.sources, approved)
	return files, nil
}

const maxResearchImageBytes = 4 << 20

func selectedResearchFiles(paths []string) (map[string]os.FileInfo, error) {
	files := make(map[string]os.FileInfo, len(paths))
	for _, path := range paths {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, errors.New("Choose a regular file as a research source.")
		}
		files[path] = info
	}
	return files, nil
}

func researchSourceBlocks(approved map[string]os.FileInfo, paths []string, images bool) ([]map[string]string, []string, error) {
	var blocks []map[string]string
	var linkedImages []string
	remaining := 8 << 20
	for _, path := range paths {
		picked := approved[path]
		if picked == nil {
			return nil, nil, errors.New("Reattach this research source using the file picker.")
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || !os.SameFile(picked, info) {
			return nil, nil, fmt.Errorf("Research source %q changed. Please reattach it.", picked.Name())
		}
		ref := &url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
		blocks = append(blocks, map[string]string{"type": "resource_link", "uri": ref.String(), "name": info.Name()})
		switch strings.ToLower(filepath.Ext(path)) {
		case ".png", ".jpg", ".jpeg", ".gif":
			if images && info.Size() <= int64(min(maxResearchImageBytes, remaining)) {
				data, mime, err := readResearchImage(path, info, remaining)
				if err != nil {
					return nil, nil, err
				}
				if mime != "" {
					blocks = append(blocks, map[string]string{"type": "image", "mimeType": mime, "data": base64.StdEncoding.EncodeToString(data)})
					remaining -= len(data)
					continue
				}
			}
			linkedImages = append(linkedImages, info.Name())
		}
	}
	return blocks, linkedImages, nil
}

func readResearchImage(path string, expected os.FileInfo, remaining int) ([]byte, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()
	current, err := file.Stat()
	if err != nil || !os.SameFile(expected, current) || !current.Mode().IsRegular() {
		return nil, "", fmt.Errorf("Research source %q changed. Please reattach it.", expected.Name())
	}
	limit := min(maxResearchImageBytes, remaining)
	data, err := io.ReadAll(io.LimitReader(file, int64(limit+1)))
	if err != nil {
		return nil, "", err
	}
	after, err := file.Stat()
	if err != nil || current.Size() != after.Size() || !current.ModTime().Equal(after.ModTime()) {
		return nil, "", fmt.Errorf("Research source %q changed while attaching it. Please retry.", expected.Name())
	}
	if len(data) > limit {
		return nil, "", nil
	}
	return data, researchRasterMIME(data), nil
}

func (h *ResearchManager) OpenCitation(ctx context.Context, root, address string) error {
	if _, err := h.workspace(ctx, root); err != nil {
		return err
	}
	u, err := url.Parse(address)
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return errors.New("Choose an HTTP or HTTPS source link.")
	}
	return mygo.Shell.OpenExternal(u.String())
}

func (h *ResearchManager) RevealOutputs(ctx context.Context, root string) error {
	w, err := h.workspace(ctx, root)
	if err != nil {
		return err
	}
	folder := review.WorkDir(h.dataDir, w.Root)
	if _, err := os.Stat(folder); err != nil {
		return errors.New("Start a research session to create its output folder.")
	}
	return mygo.Shell.OpenPath(folder)
}
