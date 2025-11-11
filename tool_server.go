package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolServerManager discovers and manages tool server clients from various sources.
// It acts as a central registry for all known tool servers.
// All methods are safe for concurrent use.
type ToolServerManager struct {
	mu      sync.RWMutex
	clients map[string]*ToolServerClient
}

// NewToolServerManager creates a new, empty manager.
func NewToolServerManager() *ToolServerManager {
	return &ToolServerManager{
		clients: make(map[string]*ToolServerClient),
	}
}

// LoadFromFile parses a standard mcp.json file and registers all defined
// servers with the manager.
func (m *ToolServerManager) LoadFromFile(configFile string) error {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read tool server config file '%s': %w", configFile, err)
	}
	var config ToolServerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse tool server config JSON: %w", err)
	}

	for name, serverConfig := range config.MCPServers {
		if serverConfig.Command == "" {
			// Silently skip invalid entries, or return an error if strictness is preferred.
			continue
		}
		cmd := exec.Command(serverConfig.Command, serverConfig.Args...)
		if len(serverConfig.Env) > 0 {
			cmd.Env = os.Environ()
			for key, value := range serverConfig.Env {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
			}
		}
		transport := mcp.NewCommandTransport(cmd)
		client, err := newToolServerClientWithTransport(transport)
		if err != nil {
			return fmt.Errorf("failed to create client for '%s': %w", name, err)
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		m.clients[name] = client
	}
	return nil
}

// AddRemoteServer programmatically registers a remote, HTTP-based tool server.
func (m *ToolServerManager) AddRemoteServer(name, url string) error {
	if url == "" {
		return fmt.Errorf("url cannot be empty for remote server '%s'", name)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[name]; exists {
		return fmt.Errorf("server with name '%s' already exists", name)
	}
	transport := mcp.NewStreamableClientTransport(url, nil)
	client, err := newToolServerClientWithTransport(transport)
	if err != nil {
		return fmt.Errorf("failed to create client for remote server '%s': %w", name, err)
	}
	m.clients[name] = client
	return nil
}

// ListServerNames returns a slice of the names of all registered servers.
func (m *ToolServerManager) ListServerNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.clients))
	for name := range m.clients {
		names = append(names, name)
	}
	return names
}

// GetClient retrieves a ready-to-use client for the server with the given name.
func (m *ToolServerManager) GetClient(name string) (*ToolServerClient, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, ok := m.clients[name]
	return client, ok
}

// --- Lower-level Client ---

// ToolServerClient handles the connection lifecycle for a single tool server.
// It should be created and managed by the ToolServerManager.
type ToolServerClient struct {
	client    *mcp.Client
	transport mcp.Transport
	session   *mcp.ClientSession
}

// ToolServerDetails defines the configuration for a command-based tool server
// as found in the mcp.json file.
type ToolServerDetails struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

// ToolServerConfig defines the top-level structure of the mcp.json file.
type ToolServerConfig struct {
	MCPServers map[string]ToolServerDetails `json:"mcpServers"`
}

// newToolServerClientWithTransport is an internal helper to create the client.
func newToolServerClientWithTransport(transport mcp.Transport) (*ToolServerClient, error) {
	return &ToolServerClient{
		client:    mcp.NewClient(&mcp.Implementation{Name: "go-ai-lib", Version: "0.1.0"}, nil),
		transport: transport,
	}, nil
}

// Connect establishes a session with the tool server. It is optional to call
// this manually; methods like FetchTools and ExecuteTool will call it
// automatically if a session is not already active.
func (c *ToolServerClient) Connect(ctx context.Context) error {
	if c.session != nil {
		return fmt.Errorf("session already established")
	}
	session, err := c.client.Connect(ctx, c.transport)
	if err != nil {
		return fmt.Errorf("mcp connect failed: %w", err)
	}
	c.session = session
	return nil
}

// ensureConnected is an internal helper to establish a session if one doesn't exist.
func (c *ToolServerClient) ensureConnected(ctx context.Context) error {
	if c.session != nil {
		return nil // Already connected
	}
	return c.Connect(ctx)
}

// Close terminates the session with the tool server.
func (c *ToolServerClient) Close() error {
	if c.session != nil {
		err := c.session.Close()
		c.session = nil
		return err
	}
	return nil
}

// FetchTools lists available tools, automatically connecting if necessary.
func (c *ToolServerClient) FetchTools(ctx context.Context) ([]Tool, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, err
	}
	mcpTools, err := c.session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, fmt.Errorf("mcp list tools failed: %w", err)
	}
	var aiTools []Tool
	for _, mcpTool := range mcpTools.Tools {
		paramsJSON, err := json.Marshal(mcpTool.InputSchema)
		if err != nil {
			continue
		}
		aiTools = append(aiTools, Tool{
			Type: "function",
			Function: FunctionDefinition{
				Name:        mcpTool.Name,
				Description: mcpTool.Description,
				Parameters:  json.RawMessage(paramsJSON),
			},
		})
	}
	return aiTools, nil
}

// ExecuteTool executes a tool call, automatically connecting if necessary.
func (c *ToolServerClient) ExecuteTool(ctx context.Context, toolCall ToolCall) (string, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return "", err
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(toolCall.Arguments), &args); err != nil {
		return "", fmt.Errorf("failed to unmarshal tool arguments: %w", err)
	}
	params := &mcp.CallToolParams{Name: toolCall.Function, Arguments: args}
	res, err := c.session.CallTool(ctx, params)
	if err != nil {
		return "", fmt.Errorf("mcp call tool '%s' failed: %w", toolCall.Function, err)
	}
	var output string
	if res.IsError {
		output = "Error: "
	}
	for _, contentItem := range res.Content {
		if textContent, ok := contentItem.(*mcp.TextContent); ok {
			output += textContent.Text
		}
	}
	return output, nil
}
