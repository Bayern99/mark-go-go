package acp

import (
	"encoding/json"
	"errors"
	"hash/fnv"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/genui"
)

const defaultAgentIdleTimeout = 2 * time.Minute

var errAgentCacheClosed = errors.New("the agent process cache is closed")

type ProcessCache struct {
	mu          sync.Mutex
	idle        *cachedAgentProcess
	timer       *time.Timer
	dir         string
	closed      bool
	idleTimeout time.Duration
}

type cachedAgentProcess struct {
	agent          *Agent
	executable     string
	args           []string
	cwd            string
	startedAt      time.Time
	envFingerprint string
	initialize     acpInitializeHandshake
	lastSessionID  string
	idleSince      time.Time
}

type acpInitializeHandshake struct {
	raw               json.RawMessage
	protocolVersion   int
	agentInfo         json.RawMessage
	agentCapabilities json.RawMessage
	authMethods       json.RawMessage
	supportsClose     bool
}

type acpLease struct {
	cache *ProcessCache
	*cachedAgentProcess
	reused bool

	priorSessionID    string
	acceptedSessionID string

	releaseOnce sync.Once
}

func NewProcessCache() *ProcessCache {
	return &ProcessCache{idleTimeout: defaultAgentIdleTimeout}
}

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

func (c *ProcessCache) lease(executable string, args []string, env []string) (*acpLease, error) {
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
		if proc.executable == executable && slices.Equal(proc.args, args) && proc.envFingerprint == fingerprint && !proc.agent.HasExited() {
			return &acpLease{
				cache:              c,
				cachedAgentProcess: proc,
				reused:             true,
				priorSessionID:     proc.lastSessionID,
			}, nil
		}
		proc.agent.Kill()
	}
	return c.startLease(executable, args, env, fingerprint)
}

func (c *ProcessCache) startLease(executable string, args []string, env []string, fingerprint string) (*acpLease, error) {
	dir, err := c.processDir()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(executable, args...)
	cmd.Dir = dir
	cmd.Env = env
	agent, err := NewAgent(cmd, fsutil.NewLimitBuffer(4<<10))
	if err != nil {
		return nil, err
	}
	c.watchProcess(agent)
	return &acpLease{
		cache: c,
		cachedAgentProcess: &cachedAgentProcess{
			agent:          agent,
			executable:     executable,
			args:           slices.Clone(args),
			cwd:            dir,
			startedAt:      time.Now(),
			envFingerprint: fingerprint,
		},
	}, nil
}

func (c *ProcessCache) processDir() (string, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return "", errAgentCacheClosed
	}
	if c.dir != "" {
		dir := c.dir
		c.mu.Unlock()
		return dir, nil
	}
	c.mu.Unlock()

	dir, err := os.MkdirTemp("", "markgogo-acp-")
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		_ = os.RemoveAll(dir)
		return "", errAgentCacheClosed
	}
	if c.dir != "" {
		_ = os.RemoveAll(dir)
		return c.dir, nil
	}
	c.dir = dir
	return dir, nil
}

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

func (l *acpLease) acceptSession(id string) error {
	if id == "" {
		return genui.AgentError("The agent answered session/new without a session ID, so the view was not changed.")
	}
	if id == l.priorSessionID {
		return genui.AgentErrorf("The agent answered session/new with the previous session %q, so the view was not changed.", id)
	}
	l.acceptedSessionID = id
	return nil
}

func (l *acpLease) supportsClose() bool { return l.initialize.supportsClose }

func (l *acpLease) release(keep bool) {
	l.releaseOnce.Do(func() {
		if !keep || l.acceptedSessionID == "" {
			l.agent.Kill()
			return
		}
		c := l.cache
		proc := l.cachedAgentProcess
		proc.lastSessionID = l.acceptedSessionID
		proc.idleSince = time.Now()
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			l.agent.Kill()
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
			previous.agent.Kill()
		}
	})
}

func (c *ProcessCache) watchProcess(agent *Agent) {
	go func() {
		<-agent.Exited()
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

func (c *ProcessCache) expire(proc *cachedAgentProcess) {
	c.mu.Lock()
	if c.idle != proc {
		c.mu.Unlock()
		return
	}
	c.idle = nil
	c.timer = nil
	c.mu.Unlock()
	proc.agent.Kill()
}

func (c *ProcessCache) effectiveIdleTimeout() time.Duration {
	if c.idleTimeout <= 0 {
		return defaultAgentIdleTimeout
	}
	return c.idleTimeout
}

func (c *ProcessCache) Close() {
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
		idle.agent.Kill()
	}
	if dir != "" {
		_ = os.RemoveAll(dir)
	}
}

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
