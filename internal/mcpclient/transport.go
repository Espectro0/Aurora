package mcpclient

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/Espectro0/AuroraProject/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func NewTransport(s *config.McpServer) (mcp.Transport, error) {
	switch s.Type {
	case config.TypeStdio:
		cmd := exec.Command(s.Command, s.Args...)
		if s.Cwd != "" {
			cmd.Dir = s.Cwd
		}

		cmd.Env = os.Environ()
		for key, value := range s.Env {
			cmd.Env = append(cmd.Env, key+"="+value)
		}

		cmd.Stderr = newLogWriter(s.Name)
		return &mcp.CommandTransport{Command: cmd}, nil

	case config.TypeHTTP:
		return &mcp.StreamableClientTransport{
			Endpoint:   s.URL,
			HTTPClient: newHTTPClient(s.Name, s.Headers),
			MaxRetries: s.MaxRetries,
		}, nil

	case config.TypeSSE:
		return &mcp.SSEClientTransport{
			Endpoint:   s.URL,
			HTTPClient: newHTTPClient(s.Name, s.Headers),
		}, nil

	default:
		return nil, fmt.Errorf("[mcp] %s: Unknown type %q", s.Name, s.Type)
	}
}

func newHTTPClient(name string, headers map[string]string) *http.Client {
	var rt http.RoundTripper = &errorBodyLogger{base: http.DefaultTransport, name: name}
	if len(headers) > 0 {
		rt = &headerTransport{base: rt, headers: headers}
	}
	return &http.Client{Transport: rt}
}

type errorBodyLogger struct {
	base http.RoundTripper
	name string
}

func (t *errorBodyLogger) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp.StatusCode < 400 {
		return resp, err
	}
	if req.Method == http.MethodGet && resp.StatusCode == http.StatusMethodNotAllowed {
		return resp, nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))

	if isJSONRPCResponse(body) {
		log.Printf("[mcp:%s] HTTP %d con respuesta JSON-RPC válida: se trata como 200", t.name, resp.StatusCode)
		resp.StatusCode, resp.Status = http.StatusOK, "200 OK"
		resp.Header.Set("Content-Type", "application/json")
		resp.ContentLength = int64(len(body))
		return resp, nil
	}

	msg := strings.TrimSpace(string(body))
	if len(msg) > 500 {
		msg = msg[:500] + "…"
	}
	log.Printf("[mcp:%s] HTTP %d: %s", t.name, resp.StatusCode, msg)
	return resp, nil
}

func isJSONRPCResponse(body []byte) bool {
	var m struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	if json.Unmarshal(body, &m) != nil {
		return false
	}
	return m.JSONRPC == "2.0" && len(m.ID) > 0 && (len(m.Result) > 0 || len(m.Error) > 0)
}

type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	for k, v := range t.headers {
		r.Header.Set(k, v)
	}
	return t.base.RoundTrip(r)
}

func newLogWriter(name string) io.Writer {
	pr, pw := io.Pipe()
	go func() {
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			log.Printf("[mcp:%s] %s", name, sc.Text())
		}
		if err := sc.Err(); err != nil {
			log.Printf("[mcp:%s] stderr: %v", name, err)
			_, _ = io.Copy(io.Discard, pr)
		}
		pr.Close()
	}()
	return pw
}
