package nativeui

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
)

type preferenceStore struct {
	dir       string
	values    map[string]string
	windows   map[*Window]bool
	loadError error
}

var preferenceStores = map[string]*preferenceStore{}

func preferencesFor(dir string) *preferenceStore {
	if p := preferenceStores[dir]; p != nil {
		return p
	}
	p := &preferenceStore{dir: dir, values: map[string]string{}, windows: map[*Window]bool{}}
	preferenceStores[dir] = p
	if dir == "" {
		return p
	}
	data, err := os.ReadFile(p.path())
	if err == nil {
		p.loadError = json.Unmarshal(data, &p.values)
		if p.values == nil {
			p.values = map[string]string{}
		}
		return p
	}
	if !errors.Is(err, os.ErrNotExist) {
		p.loadError = err
		return p
	}
	if root := legacyStoreRoot(); root != "" {
		p.values, p.loadError = readLegacyPreferences(root)
	}
	if p.loadError == nil {
		p.loadError = p.save()
	}
	return p
}

func (p *preferenceStore) path() string { return filepath.Join(p.dir, "portrait.json") }
func (p *preferenceStore) save() error {
	if p.dir == "" {
		return nil
	}
	data, err := json.Marshal(p.values)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(p.path(), append(data, '\n'))
}
func (w *Window) SetPreference(key, value string) { w.setPreference(key, value) }
func (w *Window) ResetPreferences(keys ...string) {
	w.resetPreferences(func(key string) bool {
		for _, reset := range keys {
			if key == reset {
				return true
			}
		}
		return false
	})
}
func (w *Window) ResetPreferencePrefixes(prefixes ...string) {
	w.resetPreferences(func(key string) bool {
		for _, prefix := range prefixes {
			if strings.HasPrefix(key, prefix) {
				return true
			}
		}
		return false
	})
}
func (w *Window) resetPreferences(match func(string) bool) {
	p := w.store()
	previous := make(map[string]string, len(p.values))
	changed := false
	for key, value := range p.values {
		previous[key] = value
		if match(key) {
			delete(p.values, key)
			changed = true
		}
	}
	if !changed {
		return
	}
	if err := p.save(); err != nil {
		p.values = previous
		w.failure = err.Error()
		if w.win != nil {
			w.win.Invalidate()
		}
		return
	}
	w.failure = ""
	for window := range p.windows {
		if window.win != nil {
			window.win.Invalidate()
		}
		for _, notify := range window.changed {
			notify()
		}
	}
}
func (w *Window) store() *preferenceStore {
	if w.preferences == nil {
		w.preferences = &preferenceStore{values: map[string]string{}, windows: map[*Window]bool{}}
	}
	return w.preferences
}
func (w *Window) setPreference(key, value string) {
	p := w.store()
	if p.values[key] == value {
		return
	}
	previous, existed := p.values[key]
	p.values[key] = value
	if err := p.save(); err != nil {
		if existed {
			p.values[key] = previous
		} else {
			delete(p.values, key)
		}
		w.failure = err.Error()
		if w.win != nil {
			w.win.Invalidate()
		}
		return
	}
	w.failure = ""
	for window := range p.windows {
		if window.win != nil {
			window.win.Invalidate()
		}
	}
}
func (w *Window) setPlugin(name string, value bool) {
	flag := "off"
	if value {
		flag = "on"
	}
	w.setPreference("mgo.plugin."+name, flag)
	for window := range w.preferences.windows {
		for _, notify := range window.changed {
			notify()
		}
	}
}

func legacyStoreRoot() string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	contents := filepath.Dir(filepath.Dir(exe))
	if filepath.Base(contents) != "Contents" {
		return ""
	}
	id, err := exec.Command("/usr/bin/plutil", "-extract", "CFBundleIdentifier", "raw", filepath.Join(contents, "Info.plist")).Output()
	if err != nil {
		return ""
	}
	bundle := strings.TrimSpace(string(id))
	if bundle == "" || filepath.Base(bundle) != bundle {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "WebKit", bundle, "WebsiteData", "Default")
}

func readLegacyPreferences(root string) (map[string]string, error) {
	result := map[string]string{}
	files, err := filepath.Glob(filepath.Join(root, "*", "*", "LocalStorage", "localstorage.sqlite3"))
	if err != nil {
		return result, err
	}
	origin := []byte("\x04\x00\x00\x00\x01mygo\x09\x00\x00\x00\x01localhost\x00")
	for _, file := range files {
		origins, err := os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(file)), "origin"))
		if err != nil || !bytes.Equal(origins, bytes.Repeat(origin, 2)) {
			continue
		}
		query := `SELECT key, hex(value) AS value FROM ItemTable WHERE key IN ('mgo.plugin.feynman','mgo.plugin.git','mgo.researchWidth') OR key GLOB 'mgo.viewMode:*' OR key GLOB 'mgo.folds:*' OR key GLOB 'mgo.table-widths:*'`
		data, err := exec.Command("/usr/bin/sqlite3", "-readonly", "-json", file, query).Output()
		if err != nil {
			return result, err
		}
		if len(bytes.TrimSpace(data)) == 0 {
			continue
		}
		var rows []struct{ Key, Value string }
		if err = json.Unmarshal(data, &rows); err != nil {
			return result, err
		}
		for _, row := range rows {
			raw, err := hex.DecodeString(row.Value)
			if err != nil || len(raw)%2 != 0 {
				continue
			}
			units := make([]uint16, len(raw)/2)
			for i := range units {
				units[i] = binary.LittleEndian.Uint16(raw[2*i:])
			}
			result[row.Key] = string(utf16.Decode(units))
		}
	}
	return result, nil
}
