package feynman

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
)

func removeSearchAPIKey(path, provider string) error {
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("locate Web Search settings: %w", err)
	}
	info, err := os.Lstat(target)
	if err != nil {
		return fmt.Errorf("inspect Web Search settings: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("The Web Search settings are not a regular file.")
	}
	before, err := os.ReadFile(target)
	if err != nil {
		return fmt.Errorf("read Web Search settings: %w", err)
	}
	const utf8BOM = "\ufeff"
	jsonData, hadBOM := bytes.CutPrefix(before, []byte(utf8BOM))
	var config map[string]json.RawMessage
	if json.Unmarshal(jsonData, &config) != nil || config == nil {
		return errors.New("The Web Search settings are not a JSON object.")
	}
	field := provider + "ApiKey"
	var key string
	if json.Unmarshal(config[field], &key) != nil || strings.TrimSpace(key) == "" {
		return errors.New("Could not verify the saved Web Search API key.")
	}
	delete(config, field)
	after, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if hadBOM {
		after = append([]byte(utf8BOM), after...)
	}
	currentTarget, err := filepath.EvalSymlinks(path)
	if err != nil || currentTarget != target {
		return errors.New("The Web Search settings changed. Refresh Settings and retry.")
	}
	currentInfo, err := os.Lstat(target)
	if err != nil || !os.SameFile(info, currentInfo) || info.Mode() != currentInfo.Mode() {
		return errors.New("The Web Search settings changed. Refresh Settings and retry.")
	}
	current, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(before, current) {
		return errors.New("The Web Search settings changed. Refresh Settings and retry.")
	}
	return fsutil.WriteFileAtomicMode(target, append(after, '\n'), info.Mode().Perm())
}
