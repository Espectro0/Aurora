package guard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
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

	args       any
	argsSecret bool
	argsReady  bool
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

	mu   sync.Mutex
	out  *os.File
	now  func() time.Time
	hist *history
}

func New(logPath string, policies ...Policy) (*Guard, error) {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, fmt.Errorf("[guard] Create File: %w", err)
	}
	hist := newHistory()
	if err := hist.load(logPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("[guard] load history: %v", err)
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("[guard] Open log: %w", err)
	}
	return &Guard{policies: policies, out: f, now: time.Now, hist: hist}, nil
}

func (g *Guard) Close() error {
	g.hist.close()
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.out.Close()
}

func (g *Guard) Run(ctx context.Context, s skills.Skill, argsJSON string) (skills.Result, error) {
	call := describe(s, argsJSON)
	call.UserMessage = UserMessage(ctx)
	call.redactedArgs()
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

// Entry is one guarded skill call, as written to the log and served by the API.
type Entry struct {
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

func entry(c Call, v Verdict, start, end time.Time, res skills.Result, err error) Entry {
	args, _ := c.redactedArgs()
	e := Entry{
		Time:        start,
		Skill:       c.Skill,
		Server:      c.Server,
		Tool:        c.Tool,
		Args:        args,
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

func (g *Guard) write(e Entry) {
	g.hist.add(e)
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	_, _ = g.out.Write(append(b, '\n'))
}

const maxArgChars = 500

// redactWindow bounds how much of a long string argument is redacted for
// display: only the first maxArgChars runes are kept, and the extra margin
// lets secrets that start inside the visible part and run past it still be
// matched whole. Detection (the reported bool) always scans the full string.
const redactWindow = maxArgChars + 8192

// redactedArgs returns the display form of the call's arguments and whether
// they carry secrets. The result is computed once and reused by every policy
// and by the log entry.
func (c *Call) redactedArgs() (any, bool) {
	if !c.argsReady {
		c.args, c.argsSecret = redactArgs(c.ArgsJSON)
		c.argsReady = true
	}
	return c.args, c.argsSecret
}

func redactArgs(argsJSON string) (any, bool) {
	argsJSON = strings.TrimSpace(argsJSON)
	if argsJSON == "" || argsJSON == "{}" || argsJSON == "null" {
		return nil, false
	}
	var v any
	if err := json.Unmarshal([]byte(argsJSON), &v); err != nil {
		return redactString(argsJSON)
	}
	return redact(v)
}

// redact returns a copy of v with secret-named fields replaced by "***" and
// secrets inside strings (keys included) redacted, and reports whether it
// found any.
func redact(v any) (any, bool) {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		found := false
		for k, val := range t {
			rk, keyFound := RedactSecrets(k)
			found = found || keyFound
			if isSecret(k) {
				out[rk] = "***"
				found = found || (val != nil && val != "")
				continue
			}
			rv, f := redact(val)
			out[rk] = rv
			found = found || f
		}
		return out, found
	case []any:
		out := make([]any, len(t))
		found := false
		for i, val := range t {
			rv, f := redact(val)
			out[i] = rv
			found = found || f
		}
		return out, found
	case string:
		return redactString(t)
	default:
		return v, false
	}
}

func redactString(s string) (string, bool) {
	if len(s) <= redactWindow {
		red, found := RedactSecrets(s)
		return clip(red), found
	}
	found := ContainsSecret(s)
	red, _ := RedactSecrets(s[:runeOffset(s, redactWindow)])
	if out := clip(red); len(out) != len(red) {
		return out, found
	}
	return red + "…", found
}

// secretKeyWords are matched against each word of a field name, and against
// each pair of adjacent words joined, as a suffix: accessToken, x-api-key,
// AWS_SECRET_ACCESS_KEY and set_cookie match, max_tokens, tokenizer and
// secretary don't.
var secretKeyWords = []string{
	"password", "passwords", "passwd", "passphrase", "secret", "secrets", "token",
	"apikey", "privatekey", "authorization", "cookie", "cookies", "credential", "credentials",
}

func isSecret(key string) bool {
	words := keyWords(key)
	for i, w := range words {
		if w == "pwd" || hasSecretSuffix(w) {
			return true
		}
		if i > 0 && hasSecretSuffix(words[i-1]+w) {
			return true
		}
	}
	return false
}

func hasSecretSuffix(w string) bool {
	for _, s := range secretKeyWords {
		if strings.HasSuffix(w, s) {
			return true
		}
	}
	return false
}

// keyWords splits an identifier into lowercase words on non-alphanumeric
// characters and camelCase boundaries: "userAPIKey_v2" -> user, api, key, v2.
func keyWords(key string) []string {
	var words []string
	rs := []rune(key)
	start := -1
	for i, r := range rs {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			if start >= 0 {
				words = append(words, strings.ToLower(string(rs[start:i])))
				start = -1
			}
			continue
		}
		if start >= 0 && unicode.IsUpper(r) {
			prev := rs[i-1]
			nextLower := i+1 < len(rs) && unicode.IsLower(rs[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower) {
				words = append(words, strings.ToLower(string(rs[start:i])))
				start = i
			}
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		words = append(words, strings.ToLower(string(rs[start:])))
	}
	return words
}

func clip(s string) string {
	if i := runeOffset(s, maxArgChars); i < len(s) {
		return s[:i] + "…"
	}
	return s
}

// runeOffset returns the byte offset just past the first n runes of s, or
// len(s) if s is shorter.
func runeOffset(s string, n int) int {
	if len(s) <= n {
		return len(s)
	}
	for i := range s {
		if n == 0 {
			return i
		}
		n--
	}
	return len(s)
}
