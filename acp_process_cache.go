package main

import (
	"encoding/json"
	"errors"
	"hash/fnv"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// defaultAgentIdleTimeout is how long a cached process may sit unused
// before it is stopped and removed from the cache.
const defaultAgentIdleTimeout = 2 * time.Minute

var errAgentCacheClosed = errors.New("the agent process cache is closed")

// acpProcessCache owns at most one idle ACP agent process that a
// documentHost reuses for later generations, under one exclusive lease
// per run. Each lease opens its own session in its own working folder;
// only the process and its initialize handshake are shared.
type acpProcessCache struct {
	mu          sync.Mutex
	idle        *cachedAgentProcess
	timer       *time.Timer
	dir         string
	closed      bool
	idleTimeout time.Duration
}

// cachedAgentProcess is one idle ACP process and what it proved about
// itself: the command it was started with, its initialize answer, and
// the last session ID any run opened on it.
type cachedAgentProcess struct {
	agent          *acpAgent
	executable     string
	args           []string
	cwd            string
	startedAt      time.Time
	envFingerprint string
	initialize     acpInitializeHandshake
	lastSessionID  string
	idleSince      time.Time
}

// acpInitializeHandshake is the initialize answer of a process, kept so
// that a later lease can reuse the handshake instead of repeating it.
type acpInitializeHandshake struct {
	raw               json.RawMessage
	protocolVersion   int
	agentInfo         json.RawMessage
	agentCapabilities json.RawMessage
	authMethods       json.RawMessage
	supportsClose     bool
}

// acpLease is one exclusive use of a cached process. The run opens its
// own session on it; the lease returns the process to the cache only
// when the run proved the session closed and the process is healthy.
// The process metadata and its initialize answer live in the embedded
// cachedAgentProcess, so a reused lease carries them without copying.
type acpLease struct {
	cache *acpProcessCache
	*cachedAgentProcess
	reused bool

	// priorSessionID is the session ID the process already had before
	// this lease; a session/new answer must be a different, non-empty
	// one to be a fresh independent context.
	priorSessionID    string
	acceptedSessionID string

	releaseOnce sync.Once
}

func newACPProcessCache() *acpProcessCache {
	return &acpProcessCache{idleTimeout: defaultAgentIdleTimeout}
}

// envFingerprint identifies an environment without keeping a copy of
// it. A different fingerprint must not reuse an idle process.
func envFingerprint(env []string) string {
	if env == nil {
		env = os.Environ()
	}
	h := fnv.New64a()
	for _, kv := range env {
		_, _ = h.Write([]byte(kv))
		_, _ = h.Write([]byte{0})
	}
	return strconv.FormatUint(h.Sum64(), 16)
}

// lease takes the idle process when it was started with the same
// executable and environment, or starts a new one in the cache's own
// stable working folder. Concurrent leases start their own processes
// when the idle one is taken; at most one is kept afterward.
func (c *acpProcessCache) lease(executable string, env []string) (*acpLease, error) {
	fingerprint := envFingerprint(env)

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, errAgentCacheClosed
	}
	proc := c.idle
	if proc != nil {
		c.idle = nil
		if c.timer != nil {
			c.timer.Stop()
			c.timer = nil
		}
	}
	c.mu.Unlock()

	if proc != nil {
		if proc.executable == executable && proc.envFingerprint == fingerprint && !proc.agent.hasExited() {
			return &acpLease{
				cache:              c,
				cachedAgentProcess: proc,
				reused:             true,
				priorSessionID:     proc.lastSessionID,
			}, nil
		}
		// A different executable or environment, or a process that
		// died while idle, must not be reused.
		proc.agent.kill()
	}
	return c.startLease(executable, env, fingerprint)
}

// startLease starts a new process in the cache's private folder, which
// outlives every lease; each run's own session still gets its own
// temporary working folder.
func (c *acpProcessCache) startLease(executable string, env []string, fingerprint string) (*acpLease, error) {
	dir, err := c.processDir()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(executable, "acp")
	cmd.Dir = dir
	cmd.Env = env
	agent, err := newACPAgent(cmd, newLimitBuffer(4<<10))
	if err != nil {
		return nil, err
	}
	c.watchProcess(agent)
	return &acpLease{
		cache: c,
		cachedAgentProcess: &cachedAgentProcess{
			agent:          agent,
			executable:     executable,
			args:           cmd.Args,
			cwd:            dir,
			startedAt:      time.Now(),
			envFingerprint: fingerprint,
		},
	}, nil
}

// processDir returns the cache's private process folder, making it on
// first use.
func (c *acpProcessCache) processDir() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return "", errAgentCacheClosed
	}
	if c.dir != "" {
		return c.dir, nil
	}
	dir, err := os.MkdirTemp("", "markgogo-acp-")
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	c.dir = dir
	return dir, nil
}

// storeInitialize keeps the real handshake answer of a cold process for
// the leases that reuse it.
func (l *acpLease) storeInitialize(raw json.RawMessage, protocol int, agentInfo, capabilities, authMethods json.RawMessage) {
	l.initialize = acpInitializeHandshake{
		raw:               cloneRaw(raw),
		protocolVersion:   protocol,
		agentInfo:         cloneRaw(agentInfo),
		agentCapabilities: cloneRaw(capabilities),
		authMethods:       cloneRaw(authMethods),
		supportsClose:     agentSupportsSessionClose(capabilities),
	}
}

// acceptSession checks that session/new opened a fresh independent
// context. A repeated or empty ID on a process that already had one is
// not fresh; the lease becomes unusable and the run fails instead of
// silently reusing a conversation.
func (l *acpLease) acceptSession(id string) error {
	if id == "" {
		return agentError("The agent answered session/new without a session ID, so the view was not changed.")
	}
	if id == l.priorSessionID {
		return agentErrorf("The agent answered session/new with the previous session %q, so the view was not changed.", id)
	}
	l.acceptedSessionID = id
	return nil
}

// supportsClose reports whether the agent advertised
// sessionCapabilities.close in its initialize answer.
func (l *acpLease) supportsClose() bool { return l.initialize.supportsClose }

// release ends the lease. keep is true only when the run settled its
// session and proved that session/close succeeded; anything else
// discards the process rather than re-leasing uncertain state.
func (l *acpLease) release(keep bool) {
	l.releaseOnce.Do(func() {
		if !keep || l.acceptedSessionID == "" {
			l.agent.kill()
			return
		}
		c := l.cache
		proc := l.cachedAgentProcess
		proc.lastSessionID = l.acceptedSessionID
		proc.idleSince = time.Now()
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			l.agent.kill()
			return
		}
		previous := c.idle
		c.idle = proc
		if c.timer != nil {
			c.timer.Stop()
			c.timer = nil
		}
		c.timer = time.AfterFunc(c.effectiveIdleTimeout(), func() { c.expire(proc) })
		c.mu.Unlock()
		if previous != nil {
			// At most one idle process is kept.
			previous.agent.kill()
		}
	})
}

// watchProcess drops the idle entry when a process exits on its own,
// so the next lease starts a new one instead of writing into dead
// pipes. One watcher runs per physical process, for its whole life.
func (c *acpProcessCache) watchProcess(agent *acpAgent) {
	go func() {
		<-agent.exited
		c.mu.Lock()
		if c.idle != nil && c.idle.agent == agent {
			c.idle = nil
			if c.timer != nil {
				c.timer.Stop()
				c.timer = nil
			}
		}
		c.mu.Unlock()
	}()
}

// expire stops an idle process that was not leased within the timeout.
func (c *acpProcessCache) expire(proc *cachedAgentProcess) {
	c.mu.Lock()
	if c.idle != proc {
		c.mu.Unlock()
		return
	}
	c.idle = nil
	c.timer = nil
	c.mu.Unlock()
	proc.agent.kill()
}

// effectiveIdleTimeout is the configured timeout, or the default when
// the cache was not constructed by newACPProcessCache.
func (c *acpProcessCache) effectiveIdleTimeout() time.Duration {
	if c.idleTimeout <= 0 {
		return defaultAgentIdleTimeout
	}
	return c.idleTimeout
}

// close stops the idle process, removes the private process folder, and
// makes later leases start no cached process. An active lease that
// releases after close kills its own process.
func (c *acpProcessCache) close() {
	c.mu.Lock()
	c.closed = true
	idle := c.idle
	c.idle = nil
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	dir := c.dir
	c.dir = ""
	c.mu.Unlock()
	if idle != nil {
		idle.agent.kill()
	}
	if dir != "" {
		_ = os.RemoveAll(dir)
	}
}

// agentSupportsSessionClose reads agentCapabilities.sessionCapabilities.close
// from an initialize answer's capabilities.
func agentSupportsSessionClose(capabilities json.RawMessage) bool {
	if len(capabilities) == 0 {
		return false
	}
	var top map[string]json.RawMessage
	if json.Unmarshal(capabilities, &top) != nil {
		return false
	}
	raw, ok := top["sessionCapabilities"]
	if !ok || len(raw) == 0 {
		return false
	}
	var sessionCaps map[string]json.RawMessage
	if json.Unmarshal(raw, &sessionCaps) != nil {
		return false
	}
	_, ok = sessionCaps["close"]
	return ok
}
