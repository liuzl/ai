package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPManager discovers and manages MCP tool executors from various sources.
// It acts as a central registry for all known tool servers.
type MCPManager struct {
	executors map[string]*MCPToolExecutor
}

// NewMCPManager creates a new, empty manager.
func NewMCPManager() *MCPManager {
	return &MCPManager{
		executors: make(map[string]*MCPToolExecutor),
	}
}

// LoadFromFile parses a standard mcp.json file and registers all defined
// servers with the manager.
func (m *MCPManager) LoadFromFile(configFile string) error {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read MCP config file '%s': %w", configFile, err)
	}
	var config MCPConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse MCP config JSON: %w", err)
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
		executor, err := newMCPToolExecutorWithTransport(transport)
		if err != nil {
			return fmt.Errorf("failed to create executor for '%s': %w", name, err)
		}
		m.executors[name] = executor
	}
	return nil
}

// AddRemoteServer programmatically registers a remote, HTTP-based MCP server.
func (m *MCPManager) AddRemoteServer(name, url string) error {
	if url == "" {
		return fmt.Errorf("url cannot be empty for remote server '%s'", name)
	}
	if _, exists := m.executors[name]; exists {
		return fmt.Errorf("server with name '%s' already exists", name)
	}
	transport := mcp.NewStreamableClientTransport(url, nil)
	executor, err := newMCPToolExecutorWithTransport(transport)
	if err != nil {
		return fmt.Errorf("failed to create executor for remote server '%s': %w", name, err)
	}
	m.executors[name] = executor
	return nil
}

// ListServerNames returns a slice of the names of all registered servers.
func (m *MCPManager) ListServerNames() []string {
	names := make([]string, 0, len(m.executors))
	for name := range m.executors {
		names = append(names, name)
	}
	return names
}

// GetExecutor retrieves a ready-to-use executor for the server with the given name.
func (m *MCPManager) GetExecutor(name string) (*MCPToolExecutor, bool) {
	executor, ok := m.executors[name]
	return executor, ok
}

// --- Lower-level Executor ---

// MCPToolExecutor handles the connection lifecycle for a single MCP server.
// It should be created and managed by the MCPManager.
type MCPToolExecutor struct {
	client    *mcp.Client
	transport mcp.Transport
	session   *mcp.ClientSession
}

// MCPServerConfig defines the configuration for a command-based MCP server
// as found in the mcp.json file.
type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

// MCPConfig defines the top-level structure of the mcp.json file.
type MCPConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// newMCPToolExecutorWithTransport is an internal helper to create the executor.
func newMCPToolExecutorWithTransport(transport mcp.Transport) (*MCPToolExecutor, error) {
	return &MCPToolExecutor{
		client:    mcp.NewClient(&mcp.Implementation{Name: "go-ai-lib", Version: "0.1.0"}, nil),
		transport: transport,
	}, nil
}

// Connect establishes a session with the MCP server.
func (e *MCPToolExecutor) Connect(ctx context.Context) error {
	if e.session != nil {
		return fmt.Errorf("session already established")
	}
	session, err := e.client.Connect(ctx, e.transport)
	if err != nil {
		return fmt.Errorf("mcp connect failed: %w", err)
	}
	e.session = session
	return nil
}

// Close terminates the session with the MCP server.
func (e *MCPToolExecutor) Close() error {
	if e.session != nil {
		err := e.session.Close()
		e.session = nil
		return err
	}
	return nil
}

// FetchTools lists available tools using the established session.
func (e *MCPToolExecutor) FetchTools(ctx context.Context) ([]Tool, error) {
	if e.session == nil {
		return nil, fmt.Errorf("not connected to MCP server, call Connect() first")
	}
	mcpTools, err := e.session.ListTools(ctx, &mcp.ListToolsParams{})
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

// ExecuteTool executes a tool call using the established session.
func (e *MCPToolExecutor) ExecuteTool(ctx context.Context, toolCall ToolCall) (string, error) {
	if e.session == nil {
		return "", fmt.Errorf("not connected to MCP server, call Connect() first")
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(toolCall.Arguments), &args); err != nil {
		return "", fmt.Errorf("failed to unmarshal tool arguments: %w", err)
	}
	params := &mcp.CallToolParams{Name: toolCall.Function, Arguments: args}
	res, err := e.session.CallTool(ctx, params)
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
