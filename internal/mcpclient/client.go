package mcpclient

import (
	"context"
	"fmt"
	"sync"

	"github.com/Espectro0/AuroraProject/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var impl = &mcp.Implementation{
	Name:    "aurora",
	Version: "0.1.0",
}

type Conn struct {
	Cfg     *config.McpServer
	Session *mcp.ClientSession

	mu    sync.RWMutex
	tools []*mcp.Tool
}

func (c *Conn) Tools() []*mcp.Tool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]*mcp.Tool(nil), c.tools...)
}

func Connect(ctx context.Context, s *config.McpServer, onToolsChanged func(server string)) (*Conn, error) {
	t, err := NewTransport(s)
	if err != nil {
		return nil, err
	}

	var opts *mcp.ClientOptions
	if onToolsChanged != nil {
		opts = &mcp.ClientOptions{
			ToolListChangedHandler: func(context.Context, *mcp.ToolListChangedRequest) {
				onToolsChanged(s.Name)
			},
		}
	}
	client := mcp.NewClient(impl, opts)

	connectCtx := ctx
	if s.Type != config.TypeSSE {
		var cancel context.CancelFunc
		connectCtx, cancel = context.WithTimeout(ctx, s.Timeout())
		defer cancel()
	}

	session, err := client.Connect(connectCtx, t, nil)
	if err != nil {
		return nil, fmt.Errorf("[mcp] %s connect: %w", s.Name, err)
	}

	c := &Conn{Cfg: s, Session: session}
	if err := c.RefreshTools(ctx); err != nil {
		session.Close()
		return nil, err
	}

	return c, nil
}

func (c *Conn) RefreshTools(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.Cfg.Timeout())
	defer cancel()

	var tools []*mcp.Tool
	for tool, err := range c.Session.Tools(ctx, nil) {
		if err != nil {
			return fmt.Errorf("[mcp] %s tools/list: %w", c.Cfg.Name, err)
		}
		if c.Cfg.Allowed(tool.Name) {
			tools = append(tools, tool)
		}
	}
	c.tools = tools
	return nil
}

func (c *Conn) CallTool(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.Cfg.Timeout())
	defer cancel()
	return c.Session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
}

func (c *Conn) Close() error { return c.Session.Close() }
