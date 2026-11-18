package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
)

type baseView struct {
	raw       []byte
	surfaceID string
	sources   map[string]agentSource
	messages  []json.RawMessage
	existing  map[string]bool
	replace   map[string]bool
}

type a2uiMessage struct {
	CreateSurface    *a2uiComponents `json:"createSurface"`
	UpdateComponents *a2uiComponents `json:"updateComponents"`
}

type a2uiComponents struct {
	SurfaceID  string `json:"surfaceId"`
	Components []struct {
		ID string `json:"id"`
	} `json:"components"`
}

func notUpdatable(reason string) *genError {
	return newFailure(failAgentError, "The current view cannot be updated: "+reason, "Regenerate the whole view instead.")
}

func readBaseView(path string, ids []string) (*baseView, error) {
	if len(ids) == 0 {
		return nil, notUpdatable("no components were named to replace.")
	}
	snap := readView(path)
	switch {
	case !snap.Exists:
		return nil, notUpdatable("there is no view file.")
	case snap.Error != "":
		return nil, notUpdatable(snap.Error + ".")
	}
	var file struct {
		Version int               `json:"version"`
		Sources json.RawMessage   `json:"sources"`
		A2UI    []json.RawMessage `json:"a2ui"`
	}
	if err := json.Unmarshal([]byte(snap.Text), &file); err != nil || file.Version != 2 || len(file.A2UI) == 0 {
		return nil, notUpdatable("it is not a version 2 view file.")
	}
	sources, err := parseSources(file.Sources)
	if err != nil {
		return nil, notUpdatable("its sources are not valid.")
	}
	base := &baseView{
		raw:      []byte(snap.Text),
		sources:  sources,
		messages: file.A2UI,
		existing: map[string]bool{},
		replace:  map[string]bool{},
	}
	for _, raw := range file.A2UI {
		var msg a2uiMessage
		if json.Unmarshal(raw, &msg) != nil {
			return nil, notUpdatable("one of its messages is not an object.")
		}
		body := msg.UpdateComponents
		if msg.CreateSurface != nil {
			body = msg.CreateSurface
			if base.surfaceID == "" {
				base.surfaceID = body.SurfaceID
			}
		}
		if body == nil {
			continue
		}
		for _, c := range body.Components {
			if c.ID != "" {
				base.existing[c.ID] = true
			}
		}
	}
	if base.surfaceID == "" {
		return nil, notUpdatable("it creates no surface.")
	}
	for _, id := range ids {
		if !base.existing[id] {
			return nil, notUpdatable(fmt.Sprintf("it has no component %q.", id))
		}
		base.replace[id] = true
	}
	return base, nil
}

func (b *baseView) replaceIDs() []string {
	ids := make([]string, 0, len(b.replace))
	for id := range b.replace {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (b *baseView) merge(a2ui json.RawMessage, added map[string]agentSource) (map[string]agentSource, json.RawMessage, error) {
	var messages []json.RawMessage
	if err := json.Unmarshal(a2ui, &messages); err != nil || len(messages) != 1 {
		return nil, nil, errors.New(`The agent's "a2ui" must hold exactly one updateComponents message.`)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(messages[0], &envelope); err != nil || envelope == nil {
		return nil, nil, errors.New(`The agent's "a2ui" message is not an object.`)
	}
	for key := range envelope {
		if key != "version" && key != "updateComponents" {
			return nil, nil, fmt.Errorf(`The agent's message has %q; an update may only have "version" and "updateComponents".`, key)
		}
	}
	var body a2uiComponents
	if err := json.Unmarshal(envelope["updateComponents"], &body); err != nil || len(body.Components) == 0 {
		return nil, nil, errors.New(`The agent's message is not an updateComponents with components.`)
	}
	if body.SurfaceID != b.surfaceID {
		return nil, nil, fmt.Errorf("The agent's update is for the surface %q; the view's surface is %q.", body.SurfaceID, b.surfaceID)
	}
	seen := map[string]bool{}
	for _, c := range body.Components {
		switch {
		case c.ID == "":
			return nil, nil, errors.New("The agent's update has a component without an id.")
		case seen[c.ID]:
			return nil, nil, fmt.Errorf("The agent's update names the component %q twice.", c.ID)
		case b.existing[c.ID] && !b.replace[c.ID]:
			return nil, nil, fmt.Errorf("The agent's update changes the component %q, which it was not asked to replace.", c.ID)
		}
		seen[c.ID] = true
	}

	sources := make(map[string]agentSource, len(b.sources)+len(added))
	for key, src := range b.sources {
		sources[key] = src
	}
	for key, src := range added {
		if have, ok := b.sources[key]; ok {
			if have != src {
				return nil, nil, fmt.Errorf(`The agent's source %q differs from the view's; an update may only add sources.`, key)
			}
			continue
		}
		sources[key] = src
	}

	merged, err := json.Marshal(append(append([]json.RawMessage(nil), b.messages...), messages[0]))
	if err != nil {
		return nil, nil, errors.New("The agent's update could not be added to the view.")
	}
	return sources, merged, nil
}

func prevViewPath(viewPath string) string {
	return strings.TrimSuffix(viewPath, ".json") + ".prev.json"
}

func replaceView(path string, data, expect []byte) error {
	current, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return agentErrorf("Could not read the current view file: %v.", err)
	}
	hadView := err == nil
	if expect != nil && (!hadView || !bytes.Equal(current, expect)) {
		return newFailure(failAgentError, "The view file changed while the agent worked, so the update was not applied.", "Generate again.")
	}
	prev := prevViewPath(path)
	if hadView {
		oldBackup, backupErr := os.ReadFile(prev)
		mode := os.FileMode(0o644)
		if info, err := os.Stat(path); err == nil {
			mode = info.Mode().Perm()
		}
		if err := writeFileAtomicMode(prev, current, mode); err != nil {
			return agentErrorf("Could not keep the current view to go back to: %v.", err)
		}
		if err := writeFileAtomic(path, data); err != nil {
			if backupErr == nil {
				_ = writeFileAtomic(prev, oldBackup)
			} else {
				_ = os.Remove(prev)
			}
			return agentErrorf("Could not write the view file: %v.", err)
		}
		return nil
	}
	if err := writeFileAtomic(path, data); err != nil {
		return agentErrorf("Could not write the view file: %v.", err)
	}
	return nil
}

func undoView(path string) error {
	prev := prevViewPath(path)
	if fi, err := os.Lstat(prev); err != nil || !fi.Mode().IsRegular() {
		return errors.New("There is no earlier view to go back to.")
	}
	if err := os.Rename(prev, path); err != nil {
		return fmt.Errorf("Could not go back to the earlier view: %v.", err)
	}
	return nil
}
