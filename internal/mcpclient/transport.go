package mcpclient

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"

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
			HTTPClient: newHTTPClient(s.Headers),
			MaxRetries: s.MaxRetries,
		}, nil

	case config.TypeSSE:
		return &mcp.SSEClientTransport{
			Endpoint:   s.URL,
			HTTPClient: newHTTPClient(s.Headers),
		}, nil

	default:
		return nil, fmt.Errorf("[mcp] %s: Unknown type %q", s.Name, s.Type)
	}
}

func newHTTPClient(headers map[string]string) *http.Client {
	if len(headers) == 0 {
		return http.DefaultClient
	}
	return &http.Client{Transport: &headerTransport{base: http.DefaultTransport, headers: headers}}
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
