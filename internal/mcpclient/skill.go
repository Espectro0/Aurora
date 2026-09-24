package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Espectro0/AuroraProject/internal/skills"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	maxNameLen     = 64
	maxResultChars = 10_000
)

var invalidNameChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

type ConnFunc func() (*Conn, error)

var _ skills.Skill = (*ToolSkill)(nil)

type ToolSkill struct {
	server   string
	toolName string
	name     string
	desc     string
	params   map[string]any
	conn     ConnFunc
}

func NewToolSkills(server string, tools []*sdk.Tool, conn ConnFunc, hints map[string]string) []skills.Skill {
	out := make([]skills.Skill, 0, len(tools))
	for _, t := range tools {
		s := NewToolSkill(server, t, conn)
		if h := strings.TrimSpace(hints[t.Name]); h != "" {
			s.desc += "\nIMPORTANTE: " + h
		}
		out = append(out, s)
	}
	return out
}

func NewToolSkill(server string, t *sdk.Tool, conn ConnFunc) *ToolSkill {
	desc := strings.TrimSpace(t.Description)
	if desc == "" {
		desc = t.Name
	}
	return &ToolSkill{
		server:   server,
		toolName: t.Name,
		name:     SkillName(server, t.Name),
		desc:     fmt.Sprintf("[%s] %s", server, desc),
		params:   schemaToMap(t.InputSchema),
		conn:     conn,
	}
}

func SkillName(server, tool string) string {
	name := invalidNameChars.ReplaceAllString(server+"__"+tool, "_")
	if len(name) > maxNameLen {
		name = name[:maxNameLen]
	}
	return name
}

func (s *ToolSkill) Name() string               { return s.name }
func (s *ToolSkill) Description() string        { return s.desc }
func (s *ToolSkill) Parameters() map[string]any { return s.params }
func (s *ToolSkill) Server() string             { return s.server }
func (s *ToolSkill) ToolName() string           { return s.toolName }

func (s *ToolSkill) Execute(ctx context.Context, argsJSON string) (skills.Result, error) {
	args := map[string]any{}
	if trimmed := strings.TrimSpace(argsJSON); trimmed != "" && trimmed != "null" {
		if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
			return skills.Result{}, fmt.Errorf("%s: argumentos inválidos: %w", s.name, err)
		}
	}
	args = pruneEmpty(args).(map[string]any)

	conn, err := s.conn()
	if err != nil {
		return skills.Result{}, fmt.Errorf("servidor MCP %s no disponible: %w", s.server, err)
	}

	res, err := conn.CallTool(ctx, s.toolName, args)
	if err != nil {
		return skills.Result{}, fmt.Errorf("%s: %w", s.name, err)
	}

	out, err := s.convert(res)
	if err != nil {
		return skills.Result{}, err
	}
	if res.IsError {
		msg := out.Text
		if msg == "" {
			msg = "la tool devolvió un error sin detalle"
		}
		return skills.Result{}, errors.New(msg)
	}
	return out, nil
}

func (s *ToolSkill) convert(res *sdk.CallToolResult) (skills.Result, error) {
	var parts []string
	var attachments []skills.Attachment

	for _, c := range res.Content {
		switch c := c.(type) {
		case *sdk.TextContent:
			parts = append(parts, c.Text)

		case *sdk.ImageContent:
			a, err := s.saveFile(c.Data, c.MIMEType, "imagen")
			if err != nil {
				return skills.Result{}, err
			}
			attachments = append(attachments, a)
			parts = append(parts, fmt.Sprintf("[imagen enviada al usuario: %s]", a.Filename))

		case *sdk.AudioContent:
			a, err := s.saveFile(c.Data, c.MIMEType, "audio")
			if err != nil {
				return skills.Result{}, err
			}
			attachments = append(attachments, a)
			parts = append(parts, fmt.Sprintf("[audio enviado al usuario: %s]", a.Filename))

		case *sdk.ResourceLink:
			parts = append(parts, fmt.Sprintf("[recurso: %s %s]", c.Name, c.URI))

		case *sdk.EmbeddedResource:
			if c.Resource == nil {
				continue
			}
			if c.Resource.Text != "" {
				parts = append(parts, c.Resource.Text)
			} else if len(c.Resource.Blob) > 0 {
				a, err := s.saveFile(c.Resource.Blob, c.Resource.MIMEType, "archivo")
				if err != nil {
					return skills.Result{}, err
				}
				attachments = append(attachments, a)
				parts = append(parts, fmt.Sprintf("[archivo enviado al usuario: %s]", a.Filename))
			}

		default:
			parts = append(parts, fmt.Sprintf("[contenido %T no soportado]", c))
		}
	}

	if len(parts) == 0 && res.StructuredContent != nil {
		b, err := json.Marshal(res.StructuredContent)
		if err == nil {
			parts = append(parts, string(b))
		}
	}

	return skills.Result{
		Text:        truncate(strings.Join(parts, "\n")),
		Attachments: attachments,
	}, nil
}

func (s *ToolSkill) saveFile(data []byte, mimeType, kind string) (skills.Attachment, error) {
	ext := extFor(mimeType)
	filename := fmt.Sprintf("%s_%s%s", invalidNameChars.ReplaceAllString(s.server, "_"), kind, ext)
	path := filepath.Join(os.TempDir(), fmt.Sprintf("mcp_%d_%s", time.Now().UnixNano(), filename))

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return skills.Attachment{}, fmt.Errorf("%s: guardar %s: %w", s.name, kind, err)
	}
	return skills.Attachment{Path: path, Filename: filename}, nil
}

var preferredExt = map[string]string{
	"image/jpeg": ".jpg",
	"audio/mpeg": ".mp3",
	"text/plain": ".txt",
}

func extFor(mimeType string) string {
	if ext, ok := preferredExt[mimeType]; ok {
		return ext
	}
	if exts, _ := mime.ExtensionsByType(mimeType); len(exts) > 0 {
		return exts[0]
	}
	return ".bin"
}

func schemaToMap(schema any) map[string]any {
	m := map[string]any{}
	if schema != nil {
		if b, err := json.Marshal(schema); err == nil {
			_ = json.Unmarshal(b, &m)
		}
	}
	if m["type"] == nil {
		m["type"] = "object"
	}
	if m["properties"] == nil {
		m["properties"] = map[string]any{}
	}
	return m
}

func truncate(s string) string {
	if utf8.RuneCountInString(s) <= maxResultChars {
		return s
	}
	r := []rune(s)
	return string(r[:maxResultChars]) + "\n[respuesta recortada]"
}

func pruneEmpty(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			val = pruneEmpty(val)
			if isEmpty(val) {
				delete(t, k)
			} else {
				t[k] = val
			}
		}
		return t
	case []any:
		out := t[:0]
		for _, val := range t {
			if val = pruneEmpty(val); !isEmpty(val) {
				out = append(out, val)
			}
		}
		return out
	default:
		return v
	}
}

func isEmpty(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case map[string]any:
		return len(t) == 0 || allZero(t)
	case []any:
		return len(t) == 0
	}
	return false
}

func allZero(m map[string]any) bool {
	for _, v := range m {
		if n, ok := v.(float64); !ok || n != 0 {
			return false
		}
	}
	return true
}
