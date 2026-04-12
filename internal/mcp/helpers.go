package mcp

import (
	"encoding/json"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// confirmGate checks whether a mutating tool was called with confirm=true.
// Returns (errorResult, false) if not confirmed, (nil, true) if confirmed.
func confirmGate(confirm bool, guidance string) (*sdkmcp.CallToolResult, bool) {
	if confirm {
		return nil, true
	}
	return errorResult(guidance), false
}

// textResult builds a successful tool result with a single text content block.
func textResult(text string) (*sdkmcp.CallToolResult, any, error) {
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: text},
		},
	}, nil, nil
}

// errorResult builds an error tool result with IsError=true.
func errorResult(msg string) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: msg},
		},
		IsError: true,
	}
}

// jsonResult marshals v to indented JSON and wraps it in a text result.
func jsonResult(v any) (*sdkmcp.CallToolResult, any, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errorResult(fmt.Sprintf("marshal error: %v", err)), nil, nil
	}
	return textResult(string(data))
}

// resolveProfile returns the explicit profile or falls back to the server default.
func resolveProfile(svcs Services, explicit string) string {
	if explicit != "" {
		return explicit
	}
	return svcs.Profile
}
