package main

import "testing"

func TestDocumentInfoReadsOneSessionState(t *testing.T) {
	host := &documentHost{}
	session := &docSession{host: host, viewMode: "preview"}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 2000 {
			host.mu.Lock()
			if i%2 == 0 {
				session.generating = true
				session.viewMode = "markdown"
				session.actionError = "save failed"
				session.restoreError = "could not restore"
			} else {
				session.generating = false
				session.viewMode = "preview"
				session.actionError = ""
				session.restoreError = ""
			}
			host.mu.Unlock()
		}
	}()
	defer func() { <-done }()
	for range 2000 {
		info := session.info()
		if info.Generating {
			if info.ViewMode != "markdown" || info.ActionError != "save failed" || info.RestoreError != "could not restore" {
				t.Fatalf("mixed session state while generating: %+v", info)
			}
		} else if info.ViewMode != "preview" || info.ActionError != "" || info.RestoreError != "" {
			t.Fatalf("mixed session state while idle: %+v", info)
		}
	}
}
