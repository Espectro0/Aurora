package mcpclient

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/Espectro0/AuroraProject/config"
	"github.com/Espectro0/AuroraProject/internal/skills"
)

type Status string

const (
	StatusConnecting Status = "connecting"
	StatusReady      Status = "ready"
	StatusError      Status = "error"
	StatusDisabled   Status = "disabled"
)

type ServerStatus struct {
	Name       string    `json:"name"`
	Status     Status    `json:"status"`
	Error      string    `json:"error,omitempty"`
	Tools      []string  `json:"tools,omitempty"`
	Since      time.Time `json:"since"`
	Reconnects int       `json:"reconnects"`
}

type serverState struct {
	cfg        *config.McpServer
	conn       *Conn
	status     Status
	err        error
	since      time.Time
	reconnects int
}

type connectFunc func(ctx context.Context, s *config.McpServer, onToolsChanged func(string)) (*Conn, error)

type Manager struct {
	reg     *skills.Registry
	connect connectFunc

	minBackoff time.Duration
	maxBackoff time.Duration

	mu      sync.RWMutex
	servers map[string]*serverState

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewManager(cfg *config.McpConfig, reg *skills.Registry) *Manager {
	m := &Manager{
		reg:        reg,
		connect:    Connect,
		minBackoff: time.Second,
		maxBackoff: time.Minute,
		servers:    map[string]*serverState{},
	}
	for name, s := range cfg.Servers {
		st := &serverState{cfg: s, status: StatusConnecting, since: time.Now()}
		switch {
		case s.Disabled:
			st.status = StatusDisabled
		case s.Err != nil:
			st.status, st.err = StatusError, s.Err
		}
		m.servers[name] = st
	}
	return m
}

func (m *Manager) Start(ctx context.Context) {
	ctx, m.cancel = context.WithCancel(ctx)

	m.mu.RLock()
	defer m.mu.RUnlock()
	for name, st := range m.servers {
		if st.status == StatusConnecting {
			m.wg.Add(1)
			go m.run(ctx, name)
		}
	}
}

func (m *Manager) run(ctx context.Context, name string) {
	defer m.wg.Done()
	backoff := m.minBackoff

	for ctx.Err() == nil {
		m.setStatus(name, StatusConnecting, nil)

		conn, err := m.connect(ctx, m.cfg(name), m.onToolsChanged)
		if err != nil {
			m.setStatus(name, StatusError, err)
			log.Printf("[mcp] %s: don't connected: %v (retry on %s)", name, err, backoff)
			if !sleep(ctx, backoff) {
				return
			}
			backoff = min(backoff*2, m.maxBackoff)
			continue
		}

		backoff = m.minBackoff
		m.mu.Lock()
		if ctx.Err() != nil {
			m.mu.Unlock()
			conn.Close()
			return
		}
		st := m.servers[name]
		st.conn, st.status, st.err, st.since = conn, StatusReady, nil, time.Now()
		m.mu.Unlock()
		m.register(name, conn)
		log.Printf("[mcp] %s: Ready with %d tools", name, len(conn.Tools()))

		ended := make(chan error, 1)
		go func() { ended <- conn.Session.Wait() }()

		select {
		case <-ctx.Done():
			return
		case err := <-ended:
			m.reg.UnregisterGroup(name)
			m.mu.Lock()
			st.conn = nil
			st.reconnects++
			m.mu.Unlock()
			if err == nil {
				err = errors.New("la sesión terminó")
			}
			m.setStatus(name, StatusError, err)
			log.Printf("[mcp] %s: disconnected: %v (retry on %s)", name, err, backoff)
			if !sleep(ctx, backoff) {
				return
			}
		}
	}
}

func (m *Manager) register(name string, conn *Conn) {
	skipped := m.reg.ReplaceGroup(name, NewToolSkills(name, conn.Tools(), m.connFunc(name), conn.Cfg.Hints))
	if len(skipped) > 0 {
		log.Printf("[mcp] %s: tools skipped because it ain't the same name: %v", name, skipped)
	}
}

func (m *Manager) connFunc(name string) ConnFunc {
	return func() (*Conn, error) {
		m.mu.RLock()
		defer m.mu.RUnlock()
		st, ok := m.servers[name]
		if !ok {
			return nil, fmt.Errorf("servidor %q no existe", name)
		}
		if st.status != StatusReady || st.conn == nil {
			if st.err != nil {
				return nil, fmt.Errorf("%s (%v)", st.status, st.err)
			}
			return nil, errors.New(string(st.status))
		}
		return st.conn, nil
	}
}

func (m *Manager) onToolsChanged(name string) {
	go func() {
		conn, err := m.connFunc(name)()
		if err != nil {
			return
		}
		if err := conn.RefreshTools(context.Background()); err != nil {
			log.Printf("[mcp] %s: Refresh tools: %v", name, err)
			return
		}
		m.register(name, conn)
		log.Printf("[mcp] %s: tools refreshed (%d)", name, len(conn.Tools()))
	}()
}

func (m *Manager) Status() []ServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]ServerStatus, 0, len(m.servers))
	for name, st := range m.servers {
		s := ServerStatus{Name: name, Status: st.status, Since: st.since, Reconnects: st.reconnects}
		if st.err != nil {
			s.Error = st.err.Error()
		}
		if st.conn != nil {
			for _, t := range st.conn.Tools() {
				s.Tools = append(s.Tools, t.Name)
			}
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (m *Manager) Close() {
	if m.cancel != nil {
		m.cancel()
	}

	m.mu.Lock()
	for name, st := range m.servers {
		if st.conn != nil {
			if err := st.conn.Close(); err != nil {
				log.Printf("[mcp] %s: close: %v", name, err)
			}
			st.conn = nil
		}
		m.reg.UnregisterGroup(name)
	}
	m.mu.Unlock()

	m.wg.Wait()
}

func (m *Manager) cfg(name string) *config.McpServer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.servers[name].cfg
}

func (m *Manager) setStatus(name string, s Status, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.servers[name]
	st.status, st.err, st.since = s, err, time.Now()
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
