package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPToolExecutor handles the lifecycle of interacting with an MCP server.
// The typical usage pattern is:
// 1. Create an executor with NewMCPToolExecutor or NewMCPToolExecutorFromConfig.
// 2. Call Connect() to establish a session.
// 3. Call FetchTools() or ExecuteTool() as needed.
// 4. Call Close() to terminate the session.
type MCPToolExecutor struct {
	client    *mcp.Client
	transport *mcp.StreamableClientTransport
	session   *mcp.ClientSession
}

// MCPServerConfig defines the configuration for a single MCP server.
type MCPServerConfig struct {
	Transport string `json:"transport"`
	URL       string `json:"url"`
}

// MCPConfig defines the structure of the MCP servers JSON configuration file.
type MCPConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// NewMCPToolExecutorFromConfig reads a JSON configuration file, finds the
// server by name, and returns a new MCPToolExecutor.
func NewMCPToolExecutorFromConfig(configFile, serverName string) (*MCPToolExecutor, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read MCP config file: %w", err)
	}
	var config MCPConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse MCP config JSON: %w", err)
	}
	serverConfig, ok := config.MCPServers[serverName]
	if !ok {
		return nil, fmt.Errorf("mcp server '%s' not found in config file", serverName)
	}
	return NewMCPToolExecutor(serverConfig.URL, serverConfig.Transport)
}

// NewMCPToolExecutor creates a new executor for a specific MCP server.
func NewMCPToolExecutor(url, transportType string) (*MCPToolExecutor, error) {
	var transport *mcp.StreamableClientTransport
	switch transportType {
	case "streamable-http":
		transport = mcp.NewStreamableClientTransport(url, nil)
	default:
		return nil, fmt.Errorf("unsupported MCP transport type: %s", transportType)
	}
	return &MCPToolExecutor{
		client:    mcp.NewClient(&mcp.Implementation{Name: "go-ai-lib", Version: "0.1.0"}, nil),
		transport: transport,
	}, nil
}

// Connect establishes a session with the MCP server.
func (e *MCPToolExecutor) Connect(ctx context.Context) error {
	if e.session != nil {
		return fmt.Errorf("already connected")
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
// Connect() must be called before using this method.
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
			continue // Skip tools with invalid parameter schemas
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
// Connect() must be called before using this method.
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
