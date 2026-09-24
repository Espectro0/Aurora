package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

const (
	TypeStdio = "stdio"
	TypeHTTP  = "http"
	TypeSSE   = "sse"

	defaultTimeout = 30 * time.Second
)

type McpConfig struct {
	Servers map[string]*McpServer `json:"mcpServers"`
}

type McpServer struct {
	Type    string            `json:"type,omitempty"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Cwd     string            `json:"cwd,omitempty"`
	Env     map[string]string `json:"env,omitempty"`

	URL        string            `json:"url,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	MaxRetries int               `json:"maxRetries,omitempty"`

	Allow          []string `json:"allow,omitempty"` // "" -> Any Tool | "*" -> All tools
	Confirm        []string `json:"confirm,omitempty"`
	TimeoutSeconds int      `json:"timeoutSeconds,omitempty"`
	Disabled       bool     `json:"disabled,omitempty"`

	Hints map[string]string `json:"hints,omitempty"`

	Name string `json:"-"` // LoadMCP (NOT LOADED IN JSON)
	Err  error  `json:"-"`
}

func LoadMCP(path string) (*McpConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("mcp config: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var cfg McpConfig
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("mcp config %s: %w", path, err)
	}

	if cfg.Servers == nil {
		cfg.Servers = map[string]*McpServer{}
	}

	for name, s := range cfg.Servers {
		if s == nil {
			s = &McpServer{}
			cfg.Servers[name] = s
		}
		s.Name = name
		if s.Disabled {
			continue
		}
		s.Err = s.prepare()
	}
	return &cfg, nil
}

func (c *McpConfig) Enabled() []*McpServer {
	out := make([]*McpServer, 0, len(c.Servers))
	for _, s := range c.Servers {
		if !s.Disabled && s.Err == nil {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *McpServer) Timeout() time.Duration {
	if s.TimeoutSeconds <= 0 {
		return defaultTimeout
	}
	return time.Duration(s.TimeoutSeconds) * time.Second
}

func (s *McpServer) prepare() error {
	var missing []string
	expand := func(v string) string {
		return os.Expand(v, func(key string) string {
			val, ok := os.LookupEnv(key)
			if !ok || val == "" {
				missing = append(missing, key)
			}
			return val
		})
	}

	s.Command = expand(s.Command)
	s.Cwd = expand(s.Cwd)
	s.URL = expand(s.URL)
	for i, a := range s.Args {
		s.Args[i] = expand(a)
	}
	for k, v := range s.Env {
		s.Env[k] = expand(v)
	}
	for k, v := range s.Headers {
		s.Headers[k] = expand(v)
	}
	if len(missing) > 0 {
		return fmt.Errorf("Vars undefined: %s", strings.Join(dedupe(missing), ", "))
	}

	s.Type = strings.ToLower(strings.TrimSpace(s.Type))
	if s.Type == "" {
		switch {
		case s.Command != "" && s.URL != "":
			return errors.New("You've command & url; Need indicate type")
		case s.Command != "":
			s.Type = TypeStdio
		case s.URL != "":
			s.Type = TypeHTTP
		default:
			return errors.New("Command or URL missing")
		}
	}

	switch s.Type {
	case TypeStdio:
		if s.Command == "" {
			return errors.New("type=stdio command required")
		}
	case TypeHTTP, TypeSSE:
		if s.URL == "" {
			return fmt.Errorf("type=%s url required", s.Type)
		}
		if !strings.HasPrefix(s.URL, "http://") && !strings.HasPrefix(s.URL, "https://") {
			return fmt.Errorf("Invalid url: %q", s.URL)
		}
	default:
		return fmt.Errorf("Unknown type: %q", s.Type)
	}

	for _, g := range append(append([]string{}, s.Allow...), s.Confirm...) {
		if _, err := path.Match(g, ""); err != nil {
			return fmt.Errorf("Invalid glob %q: %w", g, err)
		}
	}
	return nil
}

func (s *McpServer) Allowed(tool string) bool      { return matchAny(s.Allow, tool) }
func (s *McpServer) NeedsConfirm(tool string) bool { return matchAny(s.Confirm, tool) }

func matchAny(globs []string, name string) bool {
	for _, g := range globs {
		if ok, _ := path.Match(g, name); ok {
			return true
		}
	}
	return false
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := in[:0]
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
