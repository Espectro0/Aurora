package localai

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultContext     = 4096
	defaultIdleTimeout = 10 * time.Minute
	healthTimeout      = 180 * time.Second
	healthPollInterval = 500 * time.Millisecond
	idleCheckInterval  = 30 * time.Second
)

type Options struct {
	BinPath     string
	ChatModel   string
	EmbedModel  string
	ChatPort    int
	EmbedPort   int
	Context     int
	IdleTimeout time.Duration
}

type ServerManager struct {
	mu          sync.Mutex
	binPath     string
	context     int
	chatModel   string
	embedModel  string
	chatPort    int
	embedPort   int
	idleTimeout time.Duration

	chat   *instance
	embed  *instance
	closed bool
}

type instance struct {
	cmd      *exec.Cmd
	baseURL  string
	lastUsed time.Time
	done     chan struct{}
}

func New(o Options) *ServerManager {
	if o.Context <= 0 {
		o.Context = defaultContext
	}
	if o.IdleTimeout <= 0 {
		o.IdleTimeout = defaultIdleTimeout
	}
	return &ServerManager{
		binPath:     o.BinPath,
		context:     o.Context,
		chatModel:   o.ChatModel,
		embedModel:  o.EmbedModel,
		chatPort:    o.ChatPort,
		embedPort:   o.EmbedPort,
		idleTimeout: o.IdleTimeout,
	}
}

func (m *ServerManager) EnsureChat(ctx context.Context) (string, error) {
	return m.ensure(ctx, "chat", m.chatModel, m.chatPort, false)
}

func (m *ServerManager) EnsureEmbed(ctx context.Context) (string, error) {
	return m.ensure(ctx, "embed", m.embedModel, m.embedPort, true)
}

func (m *ServerManager) ensure(ctx context.Context, kind, model string, port int, embeddings bool) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return "", fmt.Errorf("localai: manager closed")
	}

	inst := m.instanceFor(kind)
	if inst != nil && inst.alive() {
		inst.lastUsed = time.Now()
		baseURL := inst.baseURL
		m.mu.Unlock()
		return baseURL, nil
	}

	if inst != nil {
		m.setInstance(kind, nil)
		go inst.kill()
	}

	if m.binPath == "" {
		m.mu.Unlock()
		return "", fmt.Errorf("localai: llama-server binary not configured (set LLAMA_BIN_PATH)")
	}
	if model == "" {
		m.mu.Unlock()
		return "", fmt.Errorf("localai: %s model not configured (set LLAMA_CHAT_MODEL_PATH/LLAMA_EMBED_MODEL_PATH)", kind)
	}
	if _, err := os.Stat(model); err != nil {
		m.mu.Unlock()
		return "", fmt.Errorf("localai: %s model file not found: %w", kind, err)
	}

	args := []string{
		"-m", model,
		"--host", "127.0.0.1",
		"--port", strconv.Itoa(port),
		"-c", strconv.Itoa(m.context),
		"--no-webui",
	}
	if embeddings {
		args = append(args, "--embeddings")
	}

	cmd := exec.Command(m.binPath, args...)
	inst = &instance{
		cmd:     cmd,
		baseURL: fmt.Sprintf("http://127.0.0.1:%d/v1", port),
		done:    make(chan struct{}),
	}
	inst.lastUsed = time.Now()
	m.setInstance(kind, inst)

	if err := cmd.Start(); err != nil {
		m.setInstance(kind, nil)
		m.mu.Unlock()
		return "", fmt.Errorf("localai: %s start: %w", kind, err)
	}
	log.Printf("[localai] %s server starting (model=%s port=%d)", kind, model, port)

	go func() {
		err := cmd.Wait()
		close(inst.done)
		if err != nil {
			log.Printf("[localai] %s server exited: %v", kind, err)
		} else {
			log.Printf("[localai] %s server exited", kind)
		}
		m.clearInstance(kind, inst)
	}()

	m.mu.Unlock()

	if err := m.waitHealthy(inst); err != nil {
		inst.kill()
		return "", fmt.Errorf("localai: %s health: %w", kind, err)
	}
	log.Printf("[localai] %s server ready at %s", kind, inst.baseURL)

	m.startIdleWatcher(kind, inst)

	return inst.baseURL, nil
}

func (m *ServerManager) waitHealthy(inst *instance) error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(healthTimeout)
	var lastErr error

	for time.Now().Before(deadline) {
		select {
		case <-inst.done:
			if lastErr == nil {
				lastErr = fmt.Errorf("process exited during startup")
			}
			return lastErr
		default:
		}

		resp, err := client.Get(inst.baseURL + "/health")
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		} else {
			lastErr = err
		}

		time.Sleep(healthPollInterval)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("timeout waiting for %s", inst.baseURL)
	}
	return lastErr
}

func (m *ServerManager) startIdleWatcher(kind string, inst *instance) {
	if m.idleTimeout <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(idleCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-inst.done:
				return
			case <-ticker.C:
				m.mu.Lock()
				stillCurrent := m.instanceFor(kind) == inst
				idle := time.Since(inst.lastUsed)
				m.mu.Unlock()

				if stillCurrent && idle >= m.idleTimeout {
					log.Printf("[localai] %s server idle %.0fm, stopping", kind, idle.Minutes())
					m.killInstance(kind, inst)
					return
				}
			}
		}
	}()
}

func (m *ServerManager) killInstance(kind string, inst *instance) {
	m.mu.Lock()
	if m.instanceFor(kind) == inst {
		m.setInstance(kind, nil)
	}
	m.mu.Unlock()
	inst.kill()
}

func (i *instance) alive() bool {
	select {
	case <-i.done:
		return false
	default:
		return true
	}
}

func (i *instance) kill() {
	if i.cmd.Process != nil {
		_ = i.cmd.Process.Kill()
	}
	select {
	case <-i.done:
	case <-time.After(10 * time.Second):
	}
}

func (m *ServerManager) instanceFor(kind string) *instance {
	if kind == "chat" {
		return m.chat
	}
	return m.embed
}

func (m *ServerManager) setInstance(kind string, inst *instance) {
	if kind == "chat" {
		m.chat = inst
	} else {
		m.embed = inst
	}
}

func (m *ServerManager) clearInstance(kind string, inst *instance) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.instanceFor(kind) == inst {
		m.setInstance(kind, nil)
	}
}

func (m *ServerManager) Close() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	chat := m.chat
	embed := m.embed
	m.chat = nil
	m.embed = nil
	m.mu.Unlock()

	if chat != nil {
		chat.kill()
	}
	if embed != nil {
		embed.kill()
	}
}
