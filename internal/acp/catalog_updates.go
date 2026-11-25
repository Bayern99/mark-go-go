package acp

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

const catalogUpdateGrace = 250 * time.Millisecond

type catalogUpdate struct {
	sequence uint64
	options  json.RawMessage
}

type catalogUpdates struct {
	mu       sync.Mutex
	sequence uint64
	byID     map[string]catalogUpdate
	changed  chan struct{}
}

func newCatalogUpdates() *catalogUpdates {
	return &catalogUpdates{byID: make(map[string]catalogUpdate), changed: make(chan struct{}, 1)}
}

func (u *catalogUpdates) notify(method string, raw json.RawMessage) {
	if method != "session/update" && method != "session/config_option_update" {
		return
	}
	var payload struct {
		SessionID     string          `json:"sessionId"`
		ConfigOptions json.RawMessage `json:"configOptions"`
		Update        struct {
			Kind          string          `json:"sessionUpdate"`
			ConfigOptions json.RawMessage `json:"configOptions"`
		} `json:"update"`
	}
	if json.Unmarshal(raw, &payload) != nil || payload.SessionID == "" {
		return
	}
	options := payload.ConfigOptions
	if method == "session/update" {
		if payload.Update.Kind != "config_option_update" {
			return
		}
	}
	if !hasOptions(options) {
		options = payload.Update.ConfigOptions
	}
	if !hasOptions(options) {
		return
	}
	u.mu.Lock()
	u.sequence++
	u.byID[payload.SessionID] = catalogUpdate{u.sequence, options}
	u.mu.Unlock()
	select {
	case u.changed <- struct{}{}:
	default:
	}
}

func hasOptions(raw json.RawMessage) bool {
	return len(raw) != 0 && string(raw) != "null"
}

func (u *catalogUpdates) latest(session string) catalogUpdate {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.byID[session]
}

func (u *catalogUpdates) after(ctx context.Context, session string, previous uint64, wait bool) catalogUpdate {
	if !wait {
		return u.latest(session)
	}
	timer := time.NewTimer(catalogUpdateGrace)
	defer timer.Stop()
	for {
		if current := u.latest(session); current.sequence > previous {
			return current
		}
		select {
		case <-u.changed:
		case <-timer.C:
			return u.latest(session)
		case <-ctx.Done():
			return u.latest(session)
		}
	}
}
