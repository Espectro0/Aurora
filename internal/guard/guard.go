package guard

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Espectro0/AuroraProject/internal/skills"
)

type Decision string

const (
	Allow   Decision = "allow"
	Deny    Decision = "deny"
	Confirm Decision = "confirm"
)

type Call struct {
	Skill       string
	Description string
	Server      string
	Tool        string
	ArgsJSON    string
	UserMessage string
}
type Verdict struct {
	Decision  Decision
	Reason    string
	Sensitive bool
	Details   map[string]any
}

type Policy interface {
	Name() string
	Check(ctx context.Context, c Call) Verdict
}

var _ skills.Guard = (*Guard)(nil)

type Guard struct {
	policies []Policy

	mu  sync.Mutex
	out *os.File
	now func() time.Time
}

func New(logPath string, policies ...Policy) (*Guard, error) {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, fmt.Errorf("[guard] Create File: %w", err)
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("[guard] Open log: %w", err)
	}
	return &Guard{policies: policies, out: f, now: time.Now}, nil
}

func (g *Guard) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.out.Close()
}

func (g *Guard) Run(ctx context.Context, s skills.Skill, argsJSON string) (skills.Result, error) {
	call := describe(s, argsJSON)
	call.UserMessage = UserMessage(ctx)
	start := g.now()

	verdict := g.evaluate(ctx, call)
	if verdict.Decision != Allow {
		err := denial(verdict)
		g.write(entry(call, verdict, start, g.now(), skills.Result{}, err))
		return skills.Result{}, err
	}

	res, err := s.Execute(ctx, argsJSON)
	g.write(entry(call, verdict, start, g.now(), res, err))
	return res, err
}

func (g *Guard) evaluate(ctx context.Context, c Call) Verdict {
	out := Verdict{Decision: Allow}
	for _, p := range g.policies {
		v := p.Check(ctx, c)
		out.Sensitive = out.Sensitive || v.Sensitive
		if len(v.Details) > 0 {
			if out.Details == nil {
				out.Details = map[string]any{}
			}
			out.Details[p.Name()] = v.Details
		}
		if v.Decision != Allow {
			out.Decision, out.Reason = v.Decision, v.Reason
			if out.Reason == "" {
				out.Reason = "bloqueado por " + p.Name()
			}
			return out
		}
	}
	return out
}

func denial(v Verdict) error {
	switch v.Decision {
	case Confirm:
		return fmt.Errorf("acción no ejecutada: requiere confirmación del usuario (%s). Díselo al usuario; no la reintentes", v.Reason)
	default:
		return fmt.Errorf("acción bloqueada: %s. No la reintentes", v.Reason)
	}
}

func describe(s skills.Skill, argsJSON string) Call {
	c := Call{Skill: s.Name(), Description: s.Description(), ArgsJSON: argsJSON}
	if mcp, ok := s.(interface {
		Server() string
		ToolName() string
	}); ok {
		c.Server, c.Tool = mcp.Server(), mcp.ToolName()
	}
	return c
}

type logEntry struct {
	Time        time.Time `json:"time"`
	Skill       string    `json:"skill"`
	Server      string    `json:"server,omitempty"`
	Tool        string    `json:"tool,omitempty"`
	Args        any       `json:"args,omitempty"`
	Decision    Decision  `json:"decision"`
	Reason      string    `json:"reason,omitempty"`
	Checks      any       `json:"checks,omitempty"`
	DurationMS  int64     `json:"duration_ms"`
	Error       string    `json:"error,omitempty"`
	ResultChars int       `json:"result_chars,omitempty"`
	Attachments int       `json:"attachments,omitempty"`
}

func entry(c Call, v Verdict, start, end time.Time, res skills.Result, err error) logEntry {
	e := logEntry{
		Time:        start,
		Skill:       c.Skill,
		Server:      c.Server,
		Tool:        c.Tool,
		Args:        redactArgs(c.ArgsJSON),
		Decision:    v.Decision,
		Reason:      v.Reason,
		Checks:      v.Details,
		DurationMS:  end.Sub(start).Milliseconds(),
		ResultChars: utf8.RuneCountInString(res.Text),
		Attachments: len(res.Attachments),
	}
	if v.Sensitive {
		e.Args = "[oculto: contiene información sensible]"
	}
	if err != nil {
		e.Error = err.Error()
	}
	return e
}

func (g *Guard) write(e logEntry) {
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	_, _ = g.out.Write(append(b, '\n'))
}

const maxArgChars = 500

var secretKeys = []string{"token", "password", "passwd", "secret", "apikey", "api_key", "authorization", "cookie", "credential"}

func redactArgs(argsJSON string) any {
	argsJSON = strings.TrimSpace(argsJSON)
	if argsJSON == "" || argsJSON == "{}" || argsJSON == "null" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(argsJSON), &v); err != nil {
		red, _ := RedactSecrets(argsJSON)
		return clip(red)
	}
	return redact(v)
}

func redact(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if isSecret(k) {
				t[k] = "***"
			} else {
				t[k] = redact(val)
			}
		}
		return t
	case []any:
		for i := range t {
			t[i] = redact(t[i])
		}
		return t
	case string:
		red, _ := RedactSecrets(t)
		return clip(red)
	default:
		return v
	}
}

func isSecret(key string) bool {
	k := strings.ToLower(key)
	for _, s := range secretKeys {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

func clip(s string) string {
	if utf8.RuneCountInString(s) <= maxArgChars {
		return s
	}
	return string([]rune(s)[:maxArgChars]) + "…"
}

func argsHaveSecretKeys(argsJSON string) bool {
	var v any
	if json.Unmarshal([]byte(argsJSON), &v) != nil {
		return false
	}
	var walk func(any) bool
	walk = func(v any) bool {
		switch t := v.(type) {
		case map[string]any:
			for k, val := range t {
				if isSecret(k) || walk(val) {
					return true
				}
			}
		case []any:
			for _, val := range t {
				if walk(val) {
					return true
				}
			}
		}
		return false
	}
	return walk(v)
}
